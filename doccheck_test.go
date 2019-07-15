package muffet

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/valyala/fasthttp"
	"golang.org/x/net/html"
	"log"
	"strings"
	"testing"
)

func TestVersions(t *testing.T) {
	f := newFetcher(&fasthttp.Client{}, fetcherOptions{})
	docBaseUrl := "https://access.redhat.com/documentation/en-us/red_hat_amq/"
	r, err := fetchVersions(f, docBaseUrl)
	assert.Nil(t, err)
	log.Println(r)
}

const HTML = ` 
<!DOCTYPE html>
<html lang="en">
     <head>
        <meta charset="utf-8"/>
        <title>selected attribute</title>
    </head>
    <body>
        <form method="GET">
            <input type="submit" value="submit">
        </form>
    </body>
</html>
`

func TestMaine(t *testing.T) {
	z := html.NewTokenizer(strings.NewReader(HTML))
	tt := html.TokenType(7)
	for tt != html.ErrorToken {
		tt = z.Next()
		t := z.Token()
		fmt.Println(t.Data)
		if tt == html.StartTagToken || tt == html.SelfClosingTagToken {
			name, _ := z.TagName()
			fmt.Println(string(name))
		}
	}
}
