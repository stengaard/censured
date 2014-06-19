// +build ignore

// fetch
package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"

	"code.google.com/p/go.net/html"
)

func main() {

	for _, elem := range os.Args[1:] {
		url := os.Args[1]
		if url[:4] != "http" {
			getLinksFromFile(elem)
		} else {
			getLinksFromUrl(elem)
		}
	}
}

func getLinksFromUrl(url string) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Fatal(err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Mac OS X) Gecko/20100101 Firefox")
	r, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatal(err)
	}

	defer r.Body.Close()

	printProxyLinks(r.Body)

}

func getLinksFromFile(filename string) {
	f, err := os.Open(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	printProxyLinks(f)
}

func printProxyLinks(r io.Reader) {
	doc, err := html.Parse(r)
	if err != nil {
		log.Fatal(err)
	}

	for _, link := range links(doc, []string{}) {
		proxy, err := parseProxyLink(link)
		if err != nil {
			continue
		}
		fmt.Println(proxy)
	}

}

// parse a proxy URL
// Format is:
//    proxy:<querystring>
// querystring is then a normal HTTP query string
// Common fields include:
//  - host: IP/Host address of the proxy
//  - port: TCP port
//  - isSocks: SOCKS proxy indication
//  - name: some logical name
//  - notes: a description
//  - foxyproxymode: unknown
//  - confirmation: some url
// returns the URL of the proxy if it can be parsed, else
// it returns an error.
func parseProxyLink(link string) (*url.URL, error) {
	u, err := url.Parse(link)
	if err != nil {
		return nil, err
	}
	if u.Scheme != "proxy" {
		return nil, fmt.Errorf("wrong scheme %s", u.Scheme)
	}

	q, err := url.ParseQuery(u.Opaque)
	if err != nil {
		return nil, fmt.Errorf("bad format in query string %s", err)
	}

	host := q.Get("host")
	if host == "" {
		return nil, fmt.Errorf("no host given")
	}

	port := q.Get("port")
	if port != "" {
		host = host + ":" + port
	}

	scheme := "http"
	if q.Get("isSocks") != "" {
		scheme = "socks"
	}

	return &url.URL{
		Scheme: scheme,
		Host:   host,
	}, nil

}

func links(n *html.Node, linkVals []string) []string {
	if n.Type == html.ElementNode && n.Data == "a" {
		for _, a := range n.Attr {
			if a.Key == "href" {
				linkVals = append(linkVals, a.Val)
			}
		}
	}

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		linkVals = links(c, linkVals)
	}
	return linkVals
}
