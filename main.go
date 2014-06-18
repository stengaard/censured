package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"sync"
)

func usage() {
	me := path.Base(os.Args[0])
	fmt.Fprintf(os.Stderr, "Usage of %s:\n\n", me)
	fmt.Fprintf(os.Stderr, " %s [opts] <url> <content> <proxyfile>\n\n", me)

	flag.PrintDefaults()
}

var logger = log.New(os.Stderr, "", log.LstdFlags)

func main() {

	var (
		concurrent = flag.Int("c", 10, "number of concurrent workers")
	)
	flag.Usage = usage
	flag.Parse()
	args := flag.Args()
	urlToCheck := args[0]
	expect := []byte(args[1])
	f, err := os.Open(args[2])
	if err != nil {
		log.Fatalf("%s", err)
	}

	sem, out := make(chan int, *concurrent), make(chan *result)
	wg := &sync.WaitGroup{}

	go func() {
		for proxy := range proxyGen(f) {
			wg.Add(1)
			sem <- 1 // sem is here for ratelimiting
			go func(proxy *url.URL) {
				c := &http.Client{
					Transport: &http.Transport{
						Proxy: http.ProxyURL(proxy),
					},
				}

				out <- doCheck(urlToCheck, expect, c)
				<-sem
				wg.Done()
			}(proxy)
		}

		// wait for last checker to be done, then close
		wg.Wait()
		close(out)

	}()

	r := make(map[string]stat)
	for result := range out {
		if result.country == "" {
			result.country = "N/A"
		}

		s := r[result.country]
		switch result.status {
		case statBlocked:
			s.blocks++
		case statErr:
			s.errs++
		case statOk:
			s.oks++
		}
		r[result.country] = s
		if result.err != nil {
			logger.Printf("Error: %s - %s", result.country, result.err)
		}
	}

	fmt.Printf("%-16s : %5s %5s %5s\n", "country", "ok", "block", "err")
	for country, stat := range r {
		fmt.Printf("%-16s : %5d %5d %5d\n", country, stat.oks, stat.blocks, stat.errs)
	}

}

type stat struct {
	errs, oks, blocks int
}

// a result is the
type result struct {
	// indicates a status of a check
	status status

	// which country did the check originate in?
	country string

	// an error, if status != ok
	err error
}

type status int

const (
	statErr status = iota
	statBlocked
	statOk
)

func doCheck(url string, expect []byte, client *http.Client) *result {
	res := &result{}

	loc, err := getExitLocation(client)
	if err != nil {
		res.err = err
		return res
	}
	res.country = loc.Country

	r, err := client.Get(url)
	if err != nil {
		res.err = err
		return res
	}
	defer r.Body.Close()

	buf, err := ioutil.ReadAll(r.Body)
	if err != nil {
		res.err = err
		return res
	}

	if !bytes.Equal(buf, expect) {
		res.status = statBlocked
		res.err = fmt.Errorf("blocked with: '%s' - '%s'", string(buf), string(expect))
		return res
	}

	res.status = statOk
	return res

}

func proxyGen(r io.Reader) chan *url.URL {
	out := make(chan *url.URL)
	s := bufio.NewScanner(r)
	go func() {
		defer close(out)

		for i := 0; s.Scan(); i++ {
			line := s.Text()
			u, err := url.Parse(line)
			if err != nil {
				logger.Println("Error on line %d: '%s' is not URL", i, line)
				continue
			}

			out <- u
		}

		if err := s.Err(); err != nil {
			logger.Println("Error while scanning urls: %s", err)
		}

	}()

	return out

}
