package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"sync"
	"time"

	"code.google.com/p/go.net/proxy"
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
		timeout    = flag.Duration("t", 5*time.Second, "timeout")
		verbose    = flag.Bool("v", false, "verbose output on stderr - log errors from proxies")
	)

	flag.Usage = usage
	flag.Parse()
	args := flag.Args()

	urlToCheck := args[0]
	expect := []byte(args[1])

	var rawlist io.Reader

	// TODO: could deliver rawlist from an HTTP source?
	f, err := os.Open(args[2])
	if err != nil {
		log.Fatalf("%s", err)
	}
	defer f.Close()

	rawlist = f

	// rate limiting semaphor
	sem := make(chan int, *concurrent)

	// result channel
	out := make(chan *result)

	go func() {
		wg := &sync.WaitGroup{}
		for proxyUri := range proxyGen(rawlist) {
			wg.Add(1)
			sem <- 1
			go func(proxyUri *url.URL) {
				proxyFn := http.ProxyURL(proxyUri)
				var dialer interface {
					Dial(string, string) (net.Conn, error)
				}
				dialer = dialerFunc(getTimeOutDialer(*timeout))

				if proxyUri.Scheme == "socks" {
					proxyFn = nil
					dialer, err = proxy.SOCKS5("tcp", proxyUri.Host, nil, dialer)
					if err != nil {
						log.Fatal(err)
					}

				}

				c := &http.Client{
					Transport: &http.Transport{
						Proxy: proxyFn,
						//
						Dial: dialer.Dial,
					},
					Timeout: *timeout,
				}
				out <- doCheck(urlToCheck, expect, c)

				<-sem
				wg.Done()
			}(proxyUri)
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
		if result.status != statOk && *verbose {

			logger.Printf("Error: %s - %s", result.country, result.err)
		}

	}

	fmt.Printf("%5s %5s %5s  :  %16s\n", "ok", "block", "err", "country")
	for country, stat := range r {
		fmt.Printf("%5d %5d %5d  :  %16s\n", stat.oks, stat.blocks, stat.errs, country)
	}

}

type dialerFunc func(string, string) (net.Conn, error)

func (f dialerFunc) Dial(n, a string) (net.Conn, error) {
	return f(n, a)
}

func getTimeOutDialer(timeout time.Duration) func(string, string) (net.Conn, error) {
	return func(netw, addr string) (net.Conn, error) {
		deadline := time.Now().Add(timeout)
		c, err := net.DialTimeout(netw, addr, timeout)
		if err != nil {
			return nil, err
		}
		c.SetDeadline(deadline)
		return c, nil
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
	statTimeout
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
				logger.Printf("Error on line %d: '%s' is not URL", i, line)
				continue
			}

			out <- u
		}

		if err := s.Err(); err != nil {
			logger.Printf("Error while scanning urls: %s", err)
		}

	}()

	return out

}
