package golitehtml

import (
	"errors"
	"math"
	"strconv"
	"strings"
)

// RenderOption 用于自定义布局过程。
type RenderOption func(*renderConfig)

type renderConfig struct {
	measurer TextMeasurer
	images   ImageResolver
}

// WithMeasurer 提供文本测量器。若省略，则使用 DefaultMeasurer。
func WithMeasurer(m TextMeasurer) RenderOption {
	return func(c *renderConfig) {
		if m != nil {
			c.measurer = m
		}
	}
}

// WithImageResolver 提供图片固有尺寸查询器。
func WithImageResolver(r ImageResolver) RenderOption {
	return func(c *renderConfig) {
		c.images = r
	}
}

// Render 按给定宽度布局文档，并生成显示列表。
func (d *Document) Render(width float64, opts ...RenderOption) (*Frame, error) {
	if d == nil || d.Root == nil {
		return nil, errors.New("golitehtml: nil document")
	}
	if width <= 0 {
		return nil, errors.New("golitehtml: width must be positive")
	}
	cfg := renderConfig{measurer: DefaultMeasurer{}}
	for _, opt := range opts {
		opt(&cfg)
	}
	l := &layoutContext{cfg: cfg, ops: make([]Op, 0, 128)}
	height := l.layoutBlock(d.Root, 0, 0, width)
	return &Frame{Width: width, Height: height, Ops: l.ops, Root: d.Root}, nil
}

type layoutContext struct {
	cfg renderConfig
	ops []Op
}

// layoutBlock 完成一个块级盒子的盒模型计算，并把可绘制内容追加到显示列表。
func (l *layoutContext) layoutBlock(n *Node, x, y, width float64) float64 {
	if n == nil || n.Style.Display == displayNone {
		return 0
	}
	if n.Type == TextNode {
		return 0
	}
	style := n.Style
	if style.Display == displayInline || style.Display == displayInlineBlock {
		return l.layoutInlineContainer(n, x, y, width)
	}

	font := style.FontSize
	margin := style.Margin.resolve(width, font)
	padding := style.Padding.resolve(width, font)
	border := style.Border.resolve(width, font)
	outerX := x + margin.Left
	outerY := y + margin.Top
	boxW := width - margin.Horizontal()
	if v, ok := style.Width.resolve(width, font); ok {
		boxW = v + padding.Horizontal() + border.Horizontal()
	}
	if boxW < 0 {
		boxW = 0
	}
	contentX := outerX + border.Left + padding.Left
	contentY := outerY + border.Top + padding.Top
	contentW := boxW - border.Horizontal() - padding.Horizontal()
	if contentW < 0 {
		contentW = 0
	}

	if style.BackgroundColor.A != 0 {
		// 背景高度要等子内容布局完成后才知道，这里先用 0 占位。
		l.ops = append(l.ops, RectOp{
			Rect:  Rect{X: outerX, Y: outerY, W: boxW, H: 0},
			Color: style.BackgroundColor,
			Node:  n,
		})
	}
	if style.Display == displayListItem {
		l.emitListMarker(n, outerX, contentY)
	}
	curY := contentY
	if n.Tag == "img" {
		contentH := l.layoutBlockImage(n, contentX, contentY, contentW, width)
		curY += contentH
	} else {
		var line *lineBuilder
		for _, child := range n.Children {
			if child.Style.Display == displayNone {
				continue
			}
			if child.Type == TextNode || child.Style.Display == displayInline || child.Style.Display == displayInlineBlock {
				if line == nil {
					line = newLineBuilder(l, contentX, curY, contentW, n.Style.TextAlign)
				}
				l.addInlineNode(line, child)
				continue
			}
			if line != nil {
				curY += line.finish()
				line = nil
			}
			curY += l.layoutBlock(child, contentX, curY, contentW)
		}
		if line != nil {
			curY += line.finish()
		}
	}
	contentH := curY - contentY
	if v, ok := style.Height.resolve(width, font); ok && v > contentH {
		contentH = v
	}
	totalH := margin.Top + border.Top + padding.Top + contentH + padding.Bottom + border.Bottom + margin.Bottom
	n.Box = Rect{X: outerX, Y: outerY, W: boxW, H: totalH - margin.Vertical()}
	l.patchBoxHeight(n, n.Box)
	if border.Horizontal()+border.Vertical() > 0 {
		l.emitBorder(n, n.Box, border, style.BorderColor)
	}
	return totalH
}

// emitListMarker 为 li 生成无序圆点或有序数字 marker。
func (l *layoutContext) emitListMarker(n *Node, x, y float64) {
	text := "\u2022"
	if parent := n.Parent; parent != nil && parent.Tag == "ol" {
		text = listItemOrdinal(n) + "."
	}
	style := n.Style.textStyle()
	style.Color = n.Style.Color
	markerSize := l.cfg.measurer.MeasureText(text, style)
	l.ops = append(l.ops, TextOp{
		Rect:  Rect{X: x - markerSize.W - n.Style.FontSize*0.45, Y: y, W: markerSize.W, H: markerSize.H},
		Text:  text,
		Style: style,
		Node:  n,
	})
}

// listItemOrdinal 计算当前 li 在同级列表项中的序号。
func listItemOrdinal(n *Node) string {
	count := 1
	for prev := previousSibling(n); prev != nil; prev = previousSibling(prev) {
		if prev.Type == ElementNode && prev.Tag == "li" {
			count++
		}
	}
	return strconv.Itoa(count)
}

// previousSibling 返回 n 的前一个兄弟节点。
func previousSibling(n *Node) *Node {
	if n == nil || n.Parent == nil {
		return nil
	}
	children := n.Parent.Children
	for i := 1; i < len(children); i++ {
		if children[i] == n {
			return children[i-1]
		}
	}
	return nil
}

// layoutBlockImage 处理作为块级盒子的图片，优先使用 CSS 尺寸，再回退到图片固有尺寸。
func (l *layoutContext) layoutBlockImage(n *Node, x, y, width, parentWidth float64) float64 {
	src, _ := n.AttrValue("src")
	alt, _ := n.AttrValue("alt")
	font := n.Style.FontSize
	w, wok := n.Style.Width.resolve(parentWidth, font)
	h, hok := n.Style.Height.resolve(parentWidth, font)
	if (!wok || !hok) && l.cfg.images != nil {
		if sz, ok := l.cfg.images.ImageSize(src); ok {
			if !wok {
				w = sz.W
			}
			if !hok {
				h = sz.H
			}
		}
	}
	if w <= 0 {
		w = width
	}
	if h <= 0 {
		h = w
	}
	n.Box = Rect{X: x, Y: y, W: w, H: h}
	l.ops = append(l.ops, ImageOp{Rect: n.Box, Src: src, Alt: alt, Node: n})
	return h
}

// layoutInlineContainer 将行内容器的所有子节点排成若干文本行。
func (l *layoutContext) layoutInlineContainer(n *Node, x, y, width float64) float64 {
	return l.layoutInlineChildren(n, x, y, width)
}

// layoutInlineRun 把单个行内节点放入独立行盒中。
func (l *layoutContext) layoutInlineRun(parent, child *Node, x, y, width float64) float64 {
	line := newLineBuilder(l, x, y, width, parent.Style.TextAlign)
	l.addInlineNode(line, child)
	return line.finish()
}

// layoutInlineChildren 按文档顺序收集行内子节点。
func (l *layoutContext) layoutInlineChildren(n *Node, x, y, width float64) float64 {
	line := newLineBuilder(l, x, y, width, n.Style.TextAlign)
	for _, child := range n.Children {
		l.addInlineNode(line, child)
	}
	return line.finish()
}

// addInlineNode 把文本、换行、图片和嵌套行内元素转换成 lineBuilder 项。
func (l *layoutContext) addInlineNode(line *lineBuilder, n *Node) {
	if n == nil || n.Style.Display == displayNone {
		return
	}
	if n.Type == TextNode {
		line.addText(normalizeSpace(n.Data), n.Style, n)
		return
	}
	if n.Type != ElementNode {
		for _, child := range n.Children {
			l.addInlineNode(line, child)
		}
		return
	}
	switch n.Tag {
	case "br":
		line.breakLine()
		return
	case "img":
		line.addImage(n)
		return
	}
	for _, child := range n.Children {
		l.addInlineNode(line, child)
	}
}

// emitBorder 把四条边框拆成矩形绘制操作。
func (l *layoutContext) emitBorder(n *Node, r Rect, b Insets, c Color) {
	if c.A == 0 {
		return
	}
	if b.Top > 0 {
		l.ops = append(l.ops, RectOp{Rect: Rect{X: r.X, Y: r.Y, W: r.W, H: b.Top}, Color: c, Node: n})
	}
	if b.Right > 0 {
		l.ops = append(l.ops, RectOp{Rect: Rect{X: r.Right() - b.Right, Y: r.Y, W: b.Right, H: r.H}, Color: c, Node: n})
	}
	if b.Bottom > 0 {
		l.ops = append(l.ops, RectOp{Rect: Rect{X: r.X, Y: r.Bottom() - b.Bottom, W: r.W, H: b.Bottom}, Color: c, Node: n})
	}
	if b.Left > 0 {
		l.ops = append(l.ops, RectOp{Rect: Rect{X: r.X, Y: r.Y, W: b.Left, H: r.H}, Color: c, Node: n})
	}
}

// patchBoxHeight 回填之前占位的背景矩形高度。
func (l *layoutContext) patchBoxHeight(n *Node, box Rect) {
	for i, op := range l.ops {
		switch v := op.(type) {
		case RectOp:
			if v.Node == n && v.Rect.H == 0 {
				v.Rect.H = box.H
				l.ops[i] = v
			}
		}
	}
}

type lineItem struct {
	text  string
	style TextStyle
	node  *Node
	rect  Rect
	img   bool
	src   string
	alt   string
}

type lineBuilder struct {
	ctx       *layoutContext
	x         float64
	y         float64
	width     float64
	cursor    float64
	lineH     float64
	items     []lineItem
	align     string
	totalH    float64
	lastSpace bool
}

func newLineBuilder(ctx *layoutContext, x, y, width float64, align string) *lineBuilder {
	return &lineBuilder{ctx: ctx, x: x, y: y, width: width, align: align}
}

// addText 把折叠后的文本继续拆成单词和空格，便于行尾换行。
func (l *lineBuilder) addText(text string, style Style, node *Node) {
	if text == "" {
		return
	}
	parts := splitWords(text)
	for _, part := range parts {
		if part == "" {
			continue
		}
		if part == " " {
			if l.cursor == 0 || l.lastSpace {
				continue
			}
			l.addMeasuredText(part, style, node)
			l.lastSpace = true
			continue
		}
		l.addMeasuredText(part, style, node)
		l.lastSpace = false
	}
}

// addMeasuredText 将一个已切分文本片段加入当前行，超宽时先换行。
func (l *lineBuilder) addMeasuredText(text string, style Style, node *Node) {
	ts := style.textStyle()
	sz := l.ctx.cfg.measurer.MeasureText(text, ts)
	if l.cursor > 0 && l.cursor+sz.W > l.width {
		l.breakLine()
	}
	if sz.W > l.width && l.width > 0 {
		l.addLongText(text, ts, node)
		return
	}
	l.items = append(l.items, lineItem{text: text, style: ts, node: node, rect: Rect{W: sz.W, H: sz.H}})
	l.cursor += sz.W
	l.lineH = math.Max(l.lineH, sz.H)
}

// addLongText 按 rune 拆分无法整体放入一行的长文本。
func (l *lineBuilder) addLongText(text string, ts TextStyle, node *Node) {
	var b strings.Builder
	for _, r := range text {
		candidate := b.String() + string(r)
		sz := l.ctx.cfg.measurer.MeasureText(candidate, ts)
		if b.Len() > 0 && l.cursor+sz.W > l.width {
			chunk := b.String()
			chunkSize := l.ctx.cfg.measurer.MeasureText(chunk, ts)
			l.items = append(l.items, lineItem{text: chunk, style: ts, node: node, rect: Rect{W: chunkSize.W, H: chunkSize.H}})
			l.cursor += chunkSize.W
			l.lineH = math.Max(l.lineH, chunkSize.H)
			l.breakLine()
			b.Reset()
		}
		b.WriteRune(r)
	}
	if b.Len() > 0 {
		sz := l.ctx.cfg.measurer.MeasureText(b.String(), ts)
		l.items = append(l.items, lineItem{text: b.String(), style: ts, node: node, rect: Rect{W: sz.W, H: sz.H}})
		l.cursor += sz.W
		l.lineH = math.Max(l.lineH, sz.H)
	}
}

// addImage 将行内图片作为一个不可拆分的行内项参与换行。
func (l *lineBuilder) addImage(n *Node) {
	src, _ := n.AttrValue("src")
	alt, _ := n.AttrValue("alt")
	font := n.Style.FontSize
	w, wok := n.Style.Width.resolve(l.width, font)
	h, hok := n.Style.Height.resolve(l.width, font)
	if (!wok || !hok) && l.ctx.cfg.images != nil {
		if sz, ok := l.ctx.cfg.images.ImageSize(src); ok {
			if !wok {
				w = sz.W
			}
			if !hok {
				h = sz.H
			}
		}
	}
	if w <= 0 && h <= 0 {
		return
	}
	if w <= 0 {
		w = h
	}
	if h <= 0 {
		h = w
	}
	if l.cursor > 0 && l.cursor+w > l.width {
		l.breakLine()
	}
	l.items = append(l.items, lineItem{img: true, src: src, alt: alt, node: n, rect: Rect{W: w, H: h}})
	l.cursor += w
	l.lineH = math.Max(l.lineH, h)
	n.Box = Rect{W: w, H: h}
}

// breakLine 结束当前行，应用 text-align 后生成文字和图片绘制操作。
func (l *lineBuilder) breakLine() {
	if len(l.items) == 0 {
		lineH := l.lineH
		if lineH <= 0 {
			lineH = 0
		}
		l.y += lineH
		l.totalH += lineH
		l.cursor = 0
		l.lineH = 0
		l.lastSpace = false
		return
	}
	offset := 0.0
	switch l.align {
	case "center":
		offset = math.Max(0, (l.width-l.cursor)/2)
	case "right":
		offset = math.Max(0, l.width-l.cursor)
	}
	x := l.x + offset
	for _, item := range l.items {
		item.rect.X = x
		item.rect.Y = l.y
		if item.img {
			item.node.Box = item.rect
			l.ctx.ops = append(l.ctx.ops, ImageOp{Rect: item.rect, Src: item.src, Alt: item.alt, Node: item.node})
		} else if item.text != "" {
			item.rect.H = l.lineH
			item.node.Box = item.rect
			l.ctx.ops = append(l.ctx.ops, TextOp{Rect: item.rect, Text: item.text, Style: item.style, Node: item.node})
		}
		x += item.rect.W
	}
	l.y += l.lineH
	l.totalH += l.lineH
	l.items = l.items[:0]
	l.cursor = 0
	l.lineH = 0
	l.lastSpace = false
}

// finish 结束最后一行并返回该行构建器累计的高度。
func (l *lineBuilder) finish() float64 {
	l.breakLine()
	return l.totalH
}

// splitWords 把普通文本拆成单词和单个空格，保留可换行边界。
func splitWords(s string) []string {
	if s == "" {
		return nil
	}
	var out []string
	var b strings.Builder
	inSpace := false
	for _, r := range s {
		if r == ' ' {
			if b.Len() > 0 {
				out = append(out, b.String())
				b.Reset()
			}
			if !inSpace {
				out = append(out, " ")
			}
			inSpace = true
			continue
		}
		inSpace = false
		b.WriteRune(r)
	}
	if b.Len() > 0 {
		out = append(out, b.String())
	}
	return out
}
