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

// todo it works on my machine with staging docs even without custom cert file?
// https://forfuncsake.github.io/post/2017/08/trust-extra-ca-cert-in-go-app/
func createTlsConfig(localCertFile string, insecure bool) *tls.Config {
	// Get the SystemCertPool, continue with an empty pool on error
	rootCAs, _ := x509.SystemCertPool()
	if rootCAs == nil {
		rootCAs = x509.NewCertPool()
	}

	if localCertFile != "" {
		// Read in the cert file
		certs, err := ioutil.ReadFile(localCertFile)
		if err != nil {
			log.Fatalf("Failed to append %q to RootCAs: %v", localCertFile, err)
		}

		// Append our cert to the system pool
		if ok := rootCAs.AppendCertsFromPEM(certs); !ok {
			log.Println("No certs appended, using system certs only")
		}
	}

	// Trust the augmented cert pool in our client
	config := &tls.Config{
		InsecureSkipVerify: insecure,
		RootCAs:            rootCAs,
	}

	return config
}

func serveDirectory(path string) []string {
	// Setup FS handler
	fs := &fasthttp.FS{
		Root: path,
		//IndexNames:         []string{"index.html"},
		GenerateIndexPages: true,
		//Compress:           *compress,
		//AcceptByteRange:    *byteRange,
	}
	fsHandler := fs.NewRequestHandler()

	// Start HTTP server.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	mustNot(err)
	randomPort := listener.Addr().(*net.TCPAddr).Port

	log.Printf("Starting HTTP server on %q", listener.Addr())
	go func() {
		if err := fasthttp.Serve(listener, fsHandler); err != nil {
			log.Fatalf("error in ListenAndServe: %s", err)
		}
	}()

	urls := make([]string, 0)

	d, err := os.Open(path)
	mustNot(err)
	ns, err := d.Readdirnames(0)
	mustNot(err)
	for _, dir := range ns {
		if dir == "images" {
			continue
		}
		if dir == "ccutil" {
			continue
		}
		if dir == "index.html" {
			continue
		}
		if dir == "welcome" {
			continue
		}
		url := fmt.Sprintf("http://127.0.0.1:%d/%s/index.html", randomPort, dir)
		//fmt.Println(url)
		urls = append(urls, url)
	}

	return urls
}

func CheckListOfLinks(links []string) {
	// doc-stage_usersys_redhat_com.crt
	serve := flag.String("serve", "", "Directory to serve over http")
	insecure := flag.Bool("insecure-ssl", false, "Accept/Ignore all server SSL certificates")
	certFile := flag.String("cert-file", "", "Path to certificate file")

	flag.Parse()

	tlsConfig := createTlsConfig(*certFile, *insecure)
	f := newFetcher(&fasthttp.Client{TLSConfig: tlsConfig}, fetcherOptions{})

	failures := make(Failures)

	if *serve != "" {
		links = serveDirectory(*serve)
	}

	if links == nil {
		links = flag.Args()
	}

	// has position args
	for _, arg := range links {
		checkDocPage(arg, f, failures)
	}
}

func CheckReleased() {
	// doc-stage_usersys_redhat_com.crt
	insecure := flag.Bool("insecure-ssl", false, "Accept/Ignore all server SSL certificates")
	certFile := flag.String("cert-file", "", "Path to certificate file")

	flag.Parse()

	tlsConfig := createTlsConfig(*certFile, *insecure)
	f := newFetcher(&fasthttp.Client{TLSConfig: tlsConfig}, fetcherOptions{})

	failures := make(Failures)

	// has position args
	for _, arg := range flag.Args() {
		checkDocPage(arg, f, failures)
	}
	if len(flag.Args()) > 0 {
		return
	}

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

			checkDocPage(link, f, failures)
		}

		printFailures(failures)
	}
}

func checkDocPage(docPage string, f fetcher, failures Failures) {
	fmt.Println("* " + docPage)
	r, err := f.Fetch(docPage)
	a, ok := r.Page()
	if r.statusCode != 200 || err != nil || !ok {
		fmt.Printf("ERROR: %d, %s %s\n", r.statusCode, docPage, err)
		return
	}
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
