package golitehtml

import "strings"

// NodeType 标识解析后节点的类型。
type NodeType uint8

const (
	// ElementNode 表示 HTML 元素节点。
	ElementNode NodeType = iota + 1
	// TextNode 表示文本节点。
	TextNode
	// DocumentNode 表示文档根节点或非元素容器节点。
	DocumentNode
)

// Node 是紧凑的 DOM 节点，用于解析、选择器匹配和布局。
type Node struct {
	Type     NodeType
	Tag      string
	Data     string
	Attr     map[string]string
	Parent   *Node
	Children []*Node

	Style Style
	Box   Rect
}

// ID 返回元素的 id 属性。
func (n *Node) ID() string {
	if n == nil || n.Attr == nil {
		return ""
	}
	return n.Attr["id"]
}

// Classes 从 class 属性返回类名列表。
func (n *Node) Classes() []string {
	if n == nil || n.Attr == nil {
		return nil
	}
	return strings.Fields(n.Attr["class"])
}

// AttrValue 返回属性值以及该属性是否存在。
func (n *Node) AttrValue(name string) (string, bool) {
	if n == nil || n.Attr == nil {
		return "", false
	}
	v, ok := n.Attr[strings.ToLower(name)]
	return v, ok
}

// Text 返回 n 下方所有文本内容的拼接结果。
func (n *Node) Text() string {
	if n == nil {
		return ""
	}
	if n.Type == TextNode {
		return n.Data
	}
	var b strings.Builder
	var walk func(*Node)
	walk = func(cur *Node) {
		if cur.Type == TextNode {
			b.WriteString(cur.Data)
			return
		}
		for _, child := range cur.Children {
			walk(child)
		}
	}
	walk(n)
	return b.String()
}

func (n *Node) append(child *Node) {
	if child == nil {
		return
	}
	child.Parent = n
	n.Children = append(n.Children, child)
}

func elementChildren(n *Node) []*Node {
	if n == nil || len(n.Children) == 0 {
		return nil
	}
	out := make([]*Node, 0, len(n.Children))
	for _, child := range n.Children {
		if child.Type == ElementNode {
			out = append(out, child)
		}
	}
	return out
}
