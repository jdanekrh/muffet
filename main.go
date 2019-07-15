package muffet

import (
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"github.com/valyala/fasthttp"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
)

func main() {
	failures := make(Failures)

	f := newFetcher(&fasthttp.Client{}, fetcherOptions{})

	domain := "https://access.redhat.com"
	_ = domain
	docBaseUrl := "https://access.redhat.com/documentation/en-us/red_hat_amq/"

	//varsions, err := fetchVersions(f, docBaseUrl)
	//mustNot(err)
	//for _, suffix := range varsions {
	//	s := domain + suffix
	for _, s := range []string{docBaseUrl} {
		r, err := f.Fetch(s)
		a, ok := r.Page()
		//fmt.Printf("%#v", a.links)

		if r.statusCode != 200 || err != nil || !ok {
			fmt.Printf("ERROR: %d, %s\n", r.statusCode, err)
			continue
		}

		for link := range a.links {
			if !isSinglePageHtmlDocLink(link) {
				continue
			}

			fmt.Println("* " + link)

			r, err = f.Fetch(link)
			a, ok = r.Page()

			if r.statusCode != 200 || err != nil || !ok {
				fmt.Printf("ERROR: %d, %s %s\n", r.statusCode, link, err)
				continue
			}

			docPage := link

			for link, _ := range a.links {
				//log.Println("fetching " + link)

		for _, whitelisted := range Whitelist {
			if link == whitelisted[0] {
				goto skip
			}
		}

		r, err = f.Fetch(link)

		if r.statusCode != 200 || err != nil {
			fmt.Printf("**\t%s\n", link)
			fmt.Printf("***\t%s\n", err)

			addFailure(failures, docPage, link, err.Error())
		}

	skip:
	}
}

func command(ss []string, w io.Writer) (int, error) {
	args, err := getArguments(ss)

	if err != nil {
		return 0, err
	}

	c, err := newChecker(args.URL, checkerOptions{
		fetcherOptions{
			args.Concurrency,
			args.ExcludedPatterns,
			args.Headers,
			args.IgnoreFragments,
			args.MaxRedirections,
			args.Timeout,
			args.OnePageOnly,
		},
		args.FollowRobotsTxt,
		args.FollowSitemapXML,
		args.SkipTLSVerification,
	})

	if err != nil {
		return 0, err
	}

	go c.Check()

	s := 0

	for r := range c.Results() {
		if !r.OK() || args.Verbose {
			fprintln(w, r.String(args.Verbose))
		}

		if !r.OK() {
			s = 1
		}
	}

	return s, nil
}

func fprintln(w io.Writer, xs ...interface{}) {
	if _, err := fmt.Fprintln(w, xs...); err != nil {
		panic(err)
	}
}
