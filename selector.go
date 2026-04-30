package golitehtml

import (
	"strconv"
	"strings"
	"unicode"
)

type selector struct {
	parts []selectorPart
}

type selectorPart struct {
	combinator byte
	simple     simpleSelector
}

type simpleSelector struct {
	tag     string
	id      string
	classes []string
	attrs   []attrSelector
	pseudos []pseudoSelector
	not     *simpleSelector
}

type attrSelector struct {
	name  string
	op    string
	value string
}

type pseudoSelector struct {
	name string
	arg  string
	nth  nthExpr
}

type nthExpr struct {
	a   int
	b   int
	set bool
}

func parseSelector(raw string) (selector, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return selector{}, false
	}
	var parts []selectorPart
	pending := byte(0)
	for i := 0; i < len(raw); {
		skippedSpace := false
		for i < len(raw) && unicode.IsSpace(rune(raw[i])) {
			skippedSpace = true
			i++
		}
		if skippedSpace && len(parts) > 0 && pending == 0 {
			pending = ' '
		}
		if i >= len(raw) {
			break
		}
		if strings.ContainsRune(">+~", rune(raw[i])) {
			if len(parts) == 0 {
				return selector{}, false
			}
			pending = raw[i]
			i++
			continue
		}
		start := i
		depthParen := 0
		depthBracket := 0
		quote := byte(0)
		for i < len(raw) {
			ch := raw[i]
			switch {
			case quote != 0:
				if ch == quote {
					quote = 0
				}
			case ch == '\'' || ch == '"':
				quote = ch
			case ch == '[':
				depthBracket++
			case ch == ']' && depthBracket > 0:
				depthBracket--
			case ch == '(':
				depthParen++
			case ch == ')' && depthParen > 0:
				depthParen--
			case depthBracket == 0 && depthParen == 0 && (unicode.IsSpace(rune(ch)) || strings.ContainsRune(">+~", rune(ch))):
				goto done
			}
			i++
		}
	done:
		if start == i {
			return selector{}, false
		}
		part, ok := parseSimpleSelector(raw[start:i])
		if !ok {
			return selector{}, false
		}
		parts = append(parts, selectorPart{combinator: pending, simple: part})
		pending = 0
	}
	if pending != 0 {
		return selector{}, false
	}
	return selector{parts: parts}, len(parts) > 0
}

func parseSimpleSelector(token string) (simpleSelector, bool) {
	var out simpleSelector
	for len(token) > 0 {
		switch token[0] {
		case '#':
			token = token[1:]
			id, rest := readIdent(token)
			if id == "" {
				return simpleSelector{}, false
			}
			out.id = id
			token = rest
		case '.':
			token = token[1:]
			class, rest := readIdent(token)
			if class == "" {
				return simpleSelector{}, false
			}
			out.classes = append(out.classes, class)
			token = rest
		case '*':
			token = token[1:]
		case '[':
			attr, rest, ok := readAttrSelector(token)
			if !ok {
				return simpleSelector{}, false
			}
			out.attrs = append(out.attrs, attr)
			token = rest
		case ':':
			pseudo, rest, ok := readPseudoSelector(token)
			if !ok {
				return simpleSelector{}, false
			}
			if pseudo.name == "not" {
				notSel, ok := parseNotArgument(pseudo.arg)
				if !ok {
					return simpleSelector{}, false
				}
				out.not = &notSel
			} else {
				out.pseudos = append(out.pseudos, pseudo)
			}
			token = rest
		default:
			tag, rest := readIdent(token)
			if tag == "" {
				return simpleSelector{}, false
			}
			out.tag = strings.ToLower(tag)
			token = rest
		}
	}
	return out, true
}

func readIdent(s string) (string, string) {
	for i, r := range s {
		if !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' || r >= 0x80) {
			return s[:i], s[i:]
		}
	}
	return s, ""
}

func readAttrSelector(token string) (attrSelector, string, bool) {
	end := findClosing(token, 0, '[', ']')
	if end < 0 {
		return attrSelector{}, "", false
	}
	body := strings.TrimSpace(token[1:end])
	rest := token[end+1:]
	ops := []string{"~=", "|=", "^=", "$=", "*=", "="}
	for _, op := range ops {
		if idx := strings.Index(body, op); idx >= 0 {
			name := strings.TrimSpace(strings.ToLower(body[:idx]))
			value := strings.Trim(strings.TrimSpace(body[idx+len(op):]), `"'`)
			if name == "" {
				return attrSelector{}, "", false
			}
			return attrSelector{name: name, op: op, value: value}, rest, true
		}
	}
	name := strings.TrimSpace(strings.ToLower(body))
	if name == "" {
		return attrSelector{}, "", false
	}
	return attrSelector{name: name}, rest, true
}

func readPseudoSelector(token string) (pseudoSelector, string, bool) {
	name, rest := readIdent(token[1:])
	if name == "" {
		return pseudoSelector{}, "", false
	}
	name = strings.ToLower(name)
	pseudo := pseudoSelector{name: name}
	if strings.HasPrefix(rest, "(") {
		end := findClosing(rest, 0, '(', ')')
		if end < 0 {
			return pseudoSelector{}, "", false
		}
		pseudo.arg = strings.TrimSpace(rest[1:end])
		rest = rest[end+1:]
	}
	switch name {
	case "first-child", "last-child", "only-child", "first-of-type", "last-of-type", "only-of-type", "empty",
		"link", "visited", "any-link", "hover", "active", "focus":
		if pseudo.arg != "" {
			return pseudoSelector{}, "", false
		}
	case "nth-child", "nth-last-child", "nth-of-type", "nth-last-of-type":
		nth, ok := parseNthExpr(pseudo.arg)
		if !ok {
			return pseudoSelector{}, "", false
		}
		pseudo.nth = nth
	case "not":
		if pseudo.arg == "" {
			return pseudoSelector{}, "", false
		}
	default:
		return pseudoSelector{}, "", false
	}
	return pseudo, rest, true
}

func findClosing(s string, start int, open, close byte) int {
	depth := 0
	quote := byte(0)
	for i := start; i < len(s); i++ {
		ch := s[i]
		switch {
		case quote != 0:
			if ch == quote {
				quote = 0
			}
		case ch == '\'' || ch == '"':
			quote = ch
		case ch == open:
			depth++
		case ch == close:
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

func parseNotArgument(arg string) (simpleSelector, bool) {
	sel, ok := parseSelector(arg)
	if !ok || len(sel.parts) != 1 || sel.parts[0].combinator != 0 {
		return simpleSelector{}, false
	}
	return sel.parts[0].simple, true
}

func parseNthExpr(raw string) (nthExpr, bool) {
	s := strings.ToLower(strings.ReplaceAll(raw, " ", ""))
	switch s {
	case "odd":
		return nthExpr{a: 2, b: 1, set: true}, true
	case "even":
		return nthExpr{a: 2, b: 0, set: true}, true
	case "":
		return nthExpr{}, false
	}
	if !strings.Contains(s, "n") {
		v, err := strconv.Atoi(s)
		return nthExpr{b: v, set: true}, err == nil
	}
	parts := strings.SplitN(s, "n", 2)
	a := 1
	switch parts[0] {
	case "":
		a = 1
	case "+":
		a = 1
	case "-":
		a = -1
	default:
		v, err := strconv.Atoi(parts[0])
		if err != nil {
			return nthExpr{}, false
		}
		a = v
	}
	b := 0
	if parts[1] != "" {
		v, err := strconv.Atoi(parts[1])
		if err != nil {
			return nthExpr{}, false
		}
		b = v
	}
	return nthExpr{a: a, b: b, set: true}, true
}

func (n nthExpr) matches(pos int) bool {
	if !n.set || pos < 1 {
		return false
	}
	if n.a == 0 {
		return pos == n.b
	}
	diff := pos - n.b
	if n.a > 0 && diff < 0 {
		return false
	}
	if n.a < 0 && diff > 0 {
		return false
	}
	return diff%n.a == 0
}

func (s selector) specificity() int {
	score := 0
	for _, part := range s.parts {
		score += part.simple.specificity()
	}
	return score
}

func (s selector) rightmost() (simpleSelector, bool) {
	if len(s.parts) == 0 {
		return simpleSelector{}, false
	}
	return s.parts[len(s.parts)-1].simple, true
}

func (s simpleSelector) specificity() int {
	score := 0
	if s.id != "" {
		score += 100
	}
	score += (len(s.classes) + len(s.attrs) + len(s.pseudos)) * 10
	if s.not != nil {
		score += s.not.specificity()
	}
	if s.tag != "" {
		score++
	}
	return score
}

func (s selector) matches(n *Node) bool {
	if n == nil || n.Type != ElementNode || len(s.parts) == 0 {
		return false
	}
	return s.matchesPart(len(s.parts)-1, n)
}

func (s selector) matchesPart(idx int, n *Node) bool {
	if idx < 0 {
		return true
	}
	if n == nil || !s.parts[idx].simple.matches(n) {
		return false
	}
	if idx == 0 {
		return true
	}
	switch s.parts[idx].combinator {
	case '>':
		return s.matchesPart(idx-1, parentElement(n))
	case '+':
		return s.matchesPart(idx-1, previousElementSibling(n))
	case '~':
		for p := previousElementSibling(n); p != nil; p = previousElementSibling(p) {
			if s.matchesPart(idx-1, p) {
				return true
			}
		}
		return false
	default:
		for p := parentElement(n); p != nil; p = parentElement(p) {
			if s.matchesPart(idx-1, p) {
				return true
			}
		}
		return false
	}
}

func (s simpleSelector) matches(n *Node) bool {
	if n == nil || n.Type != ElementNode {
		return false
	}
	if s.tag != "" && n.Tag != s.tag {
		return false
	}
	if s.id != "" && n.ID() != s.id {
		return false
	}
	for _, c := range s.classes {
		if !nodeHasClass(n, c) {
			return false
		}
	}
	for _, attr := range s.attrs {
		if !attr.matches(n) {
			return false
		}
	}
	for _, pseudo := range s.pseudos {
		if !pseudo.matches(n) {
			return false
		}
	}
	if s.not != nil && s.not.matches(n) {
		return false
	}
	return true
}

func nodeHasClass(n *Node, class string) bool {
	if n == nil || n.Attr == nil {
		return false
	}
	for _, have := range strings.Fields(n.Attr["class"]) {
		if have == class {
			return true
		}
	}
	return false
}

func (a attrSelector) matches(n *Node) bool {
	value, ok := n.AttrValue(a.name)
	if !ok {
		return false
	}
	switch a.op {
	case "":
		return true
	case "=":
		return value == a.value
	case "~=":
		for _, field := range strings.Fields(value) {
			if field == a.value {
				return true
			}
		}
		return false
	case "|=":
		return value == a.value || strings.HasPrefix(value, a.value+"-")
	case "^=":
		return strings.HasPrefix(value, a.value)
	case "$=":
		return strings.HasSuffix(value, a.value)
	case "*=":
		return strings.Contains(value, a.value)
	default:
		return false
	}
}

func (p pseudoSelector) matches(n *Node) bool {
	switch p.name {
	case "first-child":
		return elementIndex(n, false) == 1
	case "last-child":
		return elementIndex(n, true) == 1
	case "only-child":
		return elementIndex(n, false) == 1 && elementIndex(n, true) == 1
	case "first-of-type":
		return typeIndex(n, false) == 1
	case "last-of-type":
		return typeIndex(n, true) == 1
	case "only-of-type":
		return typeIndex(n, false) == 1 && typeIndex(n, true) == 1
	case "empty":
		for _, child := range n.Children {
			if child.Type == ElementNode || (child.Type == TextNode && child.Data != "") {
				return false
			}
		}
		return true
	case "nth-child":
		return p.nth.matches(elementIndex(n, false))
	case "nth-last-child":
		return p.nth.matches(elementIndex(n, true))
	case "nth-of-type":
		return p.nth.matches(typeIndex(n, false))
	case "nth-last-of-type":
		return p.nth.matches(typeIndex(n, true))
	case "link", "visited", "any-link":
		_, ok := n.AttrValue("href")
		return n.Tag == "a" && ok
	case "hover", "active", "focus":
		return false
	default:
		return false
	}
}

func parentElement(n *Node) *Node {
	if n == nil {
		return nil
	}
	for p := n.Parent; p != nil; p = p.Parent {
		if p.Type == ElementNode {
			return p
		}
	}
	return nil
}

func previousElementSibling(n *Node) *Node {
	if n == nil || n.Parent == nil {
		return nil
	}
	children := n.Parent.Children
	for i := 1; i < len(children); i++ {
		if children[i] != n {
			continue
		}
		for j := i - 1; j >= 0; j-- {
			if children[j].Type == ElementNode {
				return children[j]
			}
		}
		break
	}
	return nil
}

func elementIndex(n *Node, fromEnd bool) int {
	if n == nil || n.Parent == nil {
		return 0
	}
	children := n.Parent.Children
	pos := 0
	if fromEnd {
		for i := len(children) - 1; i >= 0; i-- {
			if children[i].Type != ElementNode {
				continue
			}
			pos++
			if children[i] == n {
				return pos
			}
		}
		return 0
	}
	for _, child := range children {
		if child.Type != ElementNode {
			continue
		}
		pos++
		if child == n {
			return pos
		}
	}
	return 0
}

func typeIndex(n *Node, fromEnd bool) int {
	if n == nil || n.Parent == nil {
		return 0
	}
	children := n.Parent.Children
	pos := 0
	if fromEnd {
		for i := len(children) - 1; i >= 0; i-- {
			if children[i].Type != ElementNode || children[i].Tag != n.Tag {
				continue
			}
			pos++
			if children[i] == n {
				return pos
			}
		}
		return 0
	}
	for _, child := range children {
		if child.Type != ElementNode || child.Tag != n.Tag {
			continue
		}
		pos++
		if child == n {
			return pos
		}
	}
	return 0
}
