package golitehtml

import (
	"strings"
	"unicode"
)

type selector struct {
	parts []simpleSelector
}

type simpleSelector struct {
	tag     string
	id      string
	classes []string
}

func parseSelector(raw string) (selector, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return selector{}, false
	}
	tokens := strings.Fields(raw)
	parts := make([]simpleSelector, 0, len(tokens))
	for _, token := range tokens {
		if strings.ContainsAny(token, ">+~[:") {
			return selector{}, false
		}
		part, ok := parseSimpleSelector(token)
		if !ok {
			return selector{}, false
		}
		parts = append(parts, part)
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
		if !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_') {
			return s[:i], s[i:]
		}
	}
	return s, ""
}

func (s selector) specificity() int {
	score := 0
	for _, part := range s.parts {
		if part.id != "" {
			score += 100
		}
		score += len(part.classes) * 10
		if part.tag != "" {
			score++
		}
	}
	return score
}

func (s selector) matches(n *Node) bool {
	if n == nil || n.Type != ElementNode || len(s.parts) == 0 {
		return false
	}
	idx := len(s.parts) - 1
	if !s.parts[idx].matches(n) {
		return false
	}
	idx--
	for p := n.Parent; idx >= 0 && p != nil; p = p.Parent {
		if s.parts[idx].matches(p) {
			idx--
		}
	}
	return idx < 0
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
	if len(s.classes) > 0 {
		have := map[string]struct{}{}
		for _, c := range n.Classes() {
			have[c] = struct{}{}
		}
		for _, c := range s.classes {
			if _, ok := have[c]; !ok {
				return false
			}
		}
	}
	return true
}
