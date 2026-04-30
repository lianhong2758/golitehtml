package main

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lianhong2758/golitehtml"
	xhtml "golang.org/x/net/html"
)

const liteHTMLWebsiteURL = "http://www.litehtml.com/"

func TestRenderLiteHTMLWebsiteToPNG(t *testing.T) {
	if testing.Short() {
		t.Skip("skip network rendering test in short mode")
	}

	client := &http.Client{Timeout: 15 * time.Second}
	page, err := fetchText(client, liteHTMLWebsiteURL)
	if err != nil {
		t.Fatalf("fetch %s: %v", liteHTMLWebsiteURL, err)
	}

	pageURL, err := url.Parse(page.finalURL)
	if err != nil {
		t.Fatal(err)
	}

	cssText := downloadStylesheets(t, client, page.body, pageURL)
	renderer, err := golitehtml.New(golitehtml.Options{
		Width:   1200,
		BaseDir: pageURL.String(),
		UserCSS: cssText,
	})
	if err != nil {
		t.Fatal(err)
	}

	outputPath := filepath.Join(".", "litehtml-com.png")
	if err := renderer.RenderToFile([]byte(page.body), outputPath); err != nil {
		t.Fatal(err)
	}
	t.Logf("rendered %s to %s", pageURL.String(), outputPath)
}

type fetchedText struct {
	body     string
	finalURL string
}

func fetchText(client *http.Client, rawURL string) (fetchedText, error) {
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return fetchedText{}, err
	}
	req.Header.Set("User-Agent", "golitehtml-url-test/0.1")

	resp, err := client.Do(req)
	if err != nil {
		return fetchedText{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fetchedText{}, fmt.Errorf("%s returned %s", rawURL, resp.Status)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, 5<<20))
	if err != nil {
		return fetchedText{}, err
	}
	return fetchedText{
		body:     string(data),
		finalURL: resp.Request.URL.String(),
	}, nil
}

func downloadStylesheets(t *testing.T, client *http.Client, htmlText string, pageURL *url.URL) string {
	t.Helper()

	var css strings.Builder
	for _, cssURL := range collectStylesheets(htmlText, pageURL) {
		sheet, err := fetchText(client, cssURL)
		if err != nil {
			t.Logf("skip stylesheet %s: %v", cssURL, err)
			continue
		}
		sheetURL, err := url.Parse(sheet.finalURL)
		if err != nil {
			t.Logf("skip stylesheet url rewrite %s: %v", cssURL, err)
			sheetURL = pageURL
		}
		css.WriteString("\n")
		css.WriteString(rewriteCSSURLs(sheet.body, sheetURL))
	}
	return css.String()
}

func rewriteCSSURLs(cssText string, base *url.URL) string {
	var out strings.Builder
	for {
		idx := strings.Index(strings.ToLower(cssText), "url(")
		if idx < 0 {
			out.WriteString(cssText)
			break
		}
		out.WriteString(cssText[:idx])
		rest := cssText[idx+4:]
		end := strings.IndexByte(rest, ')')
		if end < 0 {
			out.WriteString(cssText[idx:])
			break
		}
		raw := strings.Trim(strings.TrimSpace(rest[:end]), `"'`)
		if resolved, ok := resolveURL(base, raw); ok {
			out.WriteString("url(")
			out.WriteString(resolved)
			out.WriteByte(')')
		} else {
			out.WriteString(cssText[idx : idx+4+end+1])
		}
		cssText = rest[end+1:]
	}
	return out.String()
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
