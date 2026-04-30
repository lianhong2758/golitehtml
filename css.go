package golitehtml

import (
	"sort"
	"strings"
	"unicode"
)

type declaration struct {
	name  string
	value string
}

type cssRule struct {
	selector    selector
	decls       []declaration
	specificity int
	order       int
}

const uaCSS = `
html, body { display: block; }
head, meta, link, style, script, title { display: none; }
div, section, article, aside, header, footer, main, nav, p, h1, h2, h3, h4, h5, h6, ul, ol, li, blockquote, pre { display: block; }
li { display: list-item; }
span, a, strong, b, em, i, code, small { display: inline; }
img { display: inline-block; }
h1 { font-size: 2em; font-weight: 700; margin: 0.67em 0; }
h2 { font-size: 1.5em; font-weight: 700; margin: 0.83em 0; }
h3 { font-size: 1.17em; font-weight: 700; margin: 1em 0; }
h4, p, blockquote, ul, ol { margin: 1em 0; }
h5 { font-size: 0.83em; font-weight: 700; margin: 1.67em 0; }
h6 { font-size: 0.67em; font-weight: 700; margin: 2.33em 0; }
strong, b { font-weight: 700; }
em, i { font-style: italic; }
a { color: #0645ad; text-decoration: underline; }
ul, ol { padding-left: 2em; }
`

// parseCSS 解析当前库支持的 CSS 子集，并保留规则顺序供层叠排序使用。
func parseCSS(css string, startOrder int) []cssRule {
	css = stripCSSComments(css)
	var rules []cssRule
	order := startOrder
	for {
		open := strings.IndexByte(css, '{')
		if open < 0 {
			break
		}
		close := findRuleClose(css, open+1)
		if close < 0 {
			break
		}
		rawSelectors := strings.TrimSpace(css[:open])
		body := css[open+1 : close]
		css = css[close+1:]
		if strings.HasPrefix(strings.TrimSpace(rawSelectors), "@") {
			continue
		}
		decls := parseDeclarations(body)
		if len(decls) == 0 {
			continue
		}
		for _, raw := range splitSelectors(rawSelectors) {
			sel, ok := parseSelector(raw)
			if !ok {
				continue
			}
			rules = append(rules, cssRule{
				selector:    sel,
				decls:       decls,
				specificity: sel.specificity(),
				order:       order,
			})
			order++
		}
	}
	return rules
}

// parseDeclarations 解析一组 CSS 声明；冒号和分号之外的复杂语法会尽量跳过。
func parseDeclarations(body string) []declaration {
	parts := splitDeclarations(body)
	decls := make([]declaration, 0, len(parts))
	for _, part := range parts {
		name, value, ok := strings.Cut(part, ":")
		if !ok {
			continue
		}
		name = strings.TrimSpace(strings.ToLower(name))
		value = strings.TrimSpace(value)
		if name != "" && value != "" {
			decls = append(decls, declaration{name: name, value: value})
		}
	}
	return decls
}

// splitDeclarations 按顶层分号切分声明，避免把函数参数里的分号误拆。
func splitDeclarations(body string) []string {
	var out []string
	start := 0
	depth := 0
	quote := rune(0)
	for i, r := range body {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			}
		case r == '\'' || r == '"':
			quote = r
		case r == '(':
			depth++
		case r == ')' && depth > 0:
			depth--
		case r == ';' && depth == 0:
			out = append(out, body[start:i])
			start = i + 1
		}
	}
	if strings.TrimSpace(body[start:]) != "" {
		out = append(out, body[start:])
	}
	return out
}

// splitSelectors 按顶层逗号切分选择器列表。
func splitSelectors(s string) []string {
	var out []string
	start := 0
	depth := 0
	for i, r := range s {
		switch r {
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		case ',':
			if depth == 0 {
				out = append(out, s[start:i])
				start = i + 1
			}
		}
	}
	out = append(out, s[start:])
	return out
}

// stripCSSComments 去掉 CSS 块注释，简化后续规则扫描。
func stripCSSComments(s string) string {
	var b strings.Builder
	for {
		start := strings.Index(s, "/*")
		if start < 0 {
			b.WriteString(s)
			return b.String()
		}
		b.WriteString(s[:start])
		end := strings.Index(s[start+2:], "*/")
		if end < 0 {
			return b.String()
		}
		s = s[start+2+end+2:]
	}
}

// findRuleClose 找到与规则左花括号配对的右花括号。
func findRuleClose(s string, from int) int {
	depth := 0
	quote := rune(0)
	for i, r := range s[from:] {
		pos := from + i
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			}
		case r == '\'' || r == '"':
			quote = r
		case r == '(':
			depth++
		case r == ')' && depth > 0:
			depth--
		case r == '}' && depth == 0:
			return pos
		}
	}
	return -1
}

// applyStyles 按“继承默认值 -> CSS 规则 -> HTML 旧属性 -> 内联 style”的顺序计算样式。
func applyStyles(root *Node, rules []cssRule) {
	var walk func(*Node)
	walk = func(n *Node) {
		if n.Type == TextNode {
			if n.Parent != nil {
				n.Style = n.Parent.Style
			}
			return
		}
		parent := defaultStyle()
		if n.Parent != nil {
			parent = n.Parent.Style
		}
		if n.Type == DocumentNode {
			n.Style = defaultStyle()
			n.Style.Display = displayBlock
		} else {
			n.Style = inheritStyle(parent)
		}
		var matches []cssRule
		for _, rule := range rules {
			if rule.selector.matches(n) {
				matches = append(matches, rule)
			}
		}
		// 先按权重再按出现顺序排序，后应用的声明自然覆盖前面的声明。
		sort.SliceStable(matches, func(i, j int) bool {
			if matches[i].specificity == matches[j].specificity {
				return matches[i].order < matches[j].order
			}
			return matches[i].specificity < matches[j].specificity
		})
		for _, rule := range matches {
			for _, decl := range rule.decls {
				n.Style.applyProperty(decl.name, decl.value)
			}
		}
		if n.Type == ElementNode {
			applyHTMLHints(n)
			if raw, ok := n.AttrValue("style"); ok {
				for _, decl := range parseDeclarations(raw) {
					n.Style.applyProperty(decl.name, decl.value)
				}
			}
		}
		for _, child := range n.Children {
			walk(child)
		}
	}
	walk(root)
}

// applyHTMLHints 把少量历史 HTML 属性和语义标签转换成样式。
func applyHTMLHints(n *Node) {
	switch n.Tag {
	case "body":
		if v, ok := n.AttrValue("bgcolor"); ok {
			n.Style.applyProperty("background-color", v)
		}
	case "font":
		if v, ok := n.AttrValue("color"); ok {
			n.Style.applyProperty("color", v)
		}
	case "b", "strong":
		n.Style.FontWeight = 700
	case "i", "em":
		n.Style.Italic = true
	case "u":
		n.Style.Underline = true
	}
}

// normalizeSpace 实现普通 HTML 文本节点的空白折叠。
func normalizeSpace(s string) string {
	var b strings.Builder
	space := false
	for _, r := range s {
		if unicode.IsSpace(r) {
			space = true
			continue
		}
		if space && b.Len() > 0 {
			b.WriteByte(' ')
		}
		b.WriteRune(r)
		space = false
	}
	return b.String()
}
