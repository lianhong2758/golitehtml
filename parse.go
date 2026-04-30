package golitehtml

import (
	"errors"
	"io"
	"strings"

	"golang.org/x/net/html"
)

// Option 用于自定义文档解析。
type Option func(*parseConfig)

type parseConfig struct {
	userCSS string
}

// WithUserCSS 将调用方提供的 CSS 追加到内置用户代理 CSS 和文档内嵌样式之后。
func WithUserCSS(css string) Option {
	return func(c *parseConfig) {
		c.userCSS += "\n" + css
	}
}

// Document 表示已经解析并计算样式的 HTML 文档。
type Document struct {
	Root  *Node
	rules []cssRule
}

// ParseString 解析 UTF-8 HTML 字符串并计算样式。
func ParseString(src string, opts ...Option) (*Document, error) {
	return Parse(strings.NewReader(src), opts...)
}

// Parse 从 r 读取 UTF-8 HTML 并计算样式。
func Parse(r io.Reader, opts ...Option) (*Document, error) {
	if r == nil {
		return nil, errors.New("golitehtml: nil reader")
	}
	cfg := parseConfig{}
	for _, opt := range opts {
		opt(&cfg)
	}
	parsed, err := html.Parse(r)
	if err != nil {
		return nil, err
	}
	root := convertHTMLNode(parsed)
	rules := parseCSS(uaCSS, 0)
	order := len(rules)
	for _, css := range collectEmbeddedCSS(root) {
		more := parseCSS(css, order)
		order += len(more)
		rules = append(rules, more...)
	}
	if strings.TrimSpace(cfg.userCSS) != "" {
		more := parseCSS(cfg.userCSS, order)
		rules = append(rules, more...)
	}
	applyStyles(root, rules)
	return &Document{Root: root, rules: rules}, nil
}

func convertHTMLNode(h *html.Node) *Node {
	switch h.Type {
	case html.DocumentNode:
		n := &Node{Type: DocumentNode}
		for child := h.FirstChild; child != nil; child = child.NextSibling {
			n.append(convertHTMLNode(child))
		}
		return n
	case html.ElementNode:
		n := &Node{Type: ElementNode, Tag: strings.ToLower(h.Data), Attr: make(map[string]string, len(h.Attr))}
		for _, a := range h.Attr {
			n.Attr[strings.ToLower(a.Key)] = a.Val
		}
		for child := h.FirstChild; child != nil; child = child.NextSibling {
			n.append(convertHTMLNode(child))
		}
		return n
	case html.TextNode:
		return &Node{Type: TextNode, Data: h.Data}
	default:
		n := &Node{Type: DocumentNode}
		for child := h.FirstChild; child != nil; child = child.NextSibling {
			n.append(convertHTMLNode(child))
		}
		return n
	}
}

func collectEmbeddedCSS(root *Node) []string {
	var out []string
	var walk func(*Node)
	walk = func(n *Node) {
		if n.Type == ElementNode && n.Tag == "style" {
			out = append(out, n.Text())
			return
		}
		for _, child := range n.Children {
			walk(child)
		}
	}
	walk(root)
	return out
}

// QueryOne 返回第一个匹配受支持选择器子集的元素。
func (d *Document) QueryOne(cssSelector string) *Node {
	matches := d.Query(cssSelector)
	if len(matches) == 0 {
		return nil
	}
	return matches[0]
}

// Query 返回所有匹配受支持选择器子集的元素。
func (d *Document) Query(cssSelector string) []*Node {
	if d == nil || d.Root == nil {
		return nil
	}
	sel, ok := parseSelector(cssSelector)
	if !ok {
		return nil
	}
	var out []*Node
	var walk func(*Node)
	walk = func(n *Node) {
		if sel.matches(n) {
			out = append(out, n)
		}
		for _, child := range n.Children {
			walk(child)
		}
	}
	walk(d.Root)
	return out
}
