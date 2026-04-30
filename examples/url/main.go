package main

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/lianhong2758/golitehtml"
	xhtml "golang.org/x/net/html"
)

func main() {
	targetURL := "https://www.runoob.com/"
	outputFile := "url-example.png"
	renderWidth := 700
	timeout := 12 * time.Second

	client := &http.Client{Timeout: timeout}
	pageURL, err := url.Parse(targetURL)
	must(err)

	htmlText, cssText, err := downloadPage(client, pageURL)
	must(err)

	renderer, err := golitehtml.New(golitehtml.Options{
		Width:   renderWidth,
		BaseDir: pageURL.String(),
		UserCSS: cssText,
	})
	must(err)

	must(renderer.RenderToFile([]byte(htmlText), outputFile))
	fmt.Printf("下载 %s\n", pageURL.String())
	fmt.Printf("输出 %s\n", outputFile)
}

func downloadPage(client *http.Client, pageURL *url.URL) (string, string, error) {
	body, err := fetchText(client, pageURL.String())
	if err != nil {
		return "", "", err
	}

	cssLinks := collectStylesheets(body, pageURL)
	var css strings.Builder
	for _, cssURL := range cssLinks {
		text, err := fetchText(client, cssURL)
		if err != nil {
			fmt.Fprintf(os.Stderr, "跳过样式表 %s: %v\n", cssURL, err)
			continue
		}
		css.WriteString("\n")
		css.WriteString(text)
	}
	return body, css.String(), nil
}

func fetchText(client *http.Client, rawURL string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "golitehtml-url-example/0.1")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("%s returned %s", rawURL, resp.Status)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, 5<<20))
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func collectStylesheets(src string, base *url.URL) []string {
	root, err := xhtml.Parse(strings.NewReader(src))
	if err != nil {
		return nil
	}

	var links []string
	var walk func(*xhtml.Node)
	walk = func(n *xhtml.Node) {
		if n.Type == xhtml.ElementNode && strings.EqualFold(n.Data, "link") {
			rel := attr(n, "rel")
			href := attr(n, "href")
			if href != "" && strings.Contains(strings.ToLower(rel), "stylesheet") {
				if resolved, ok := resolveURL(base, href); ok {
					links = append(links, resolved)
				}
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(root)
	return links
}

func attr(n *xhtml.Node, name string) string {
	for _, a := range n.Attr {
		if strings.EqualFold(a.Key, name) {
			return a.Val
		}
	}
	return ""
}

func resolveURL(base *url.URL, raw string) (string, bool) {
	if strings.TrimSpace(raw) == "" || strings.HasPrefix(raw, "data:") {
		return "", false
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", false
	}
	return base.ResolveReference(u).String(), true
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
