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
	l := &layoutContext{cfg: cfg, ops: make([]Op, 0, 128), preferred: make(map[preferredKey]float64)}
	height := l.layoutBlock(d.Root, 0, 0, width)
	return &Frame{Width: width, Height: height, Ops: l.ops, Root: d.Root}, nil
}

type layoutContext struct {
	cfg       renderConfig
	ops       []Op
	preferred map[preferredKey]float64
}

type preferredKey struct {
	node *Node
	base float64
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
	padding := style.Padding.resolve(width, font)
	border := style.Border.resolve(width, font)
	margin := resolveMargins(style.Margin, width, font)
	boxW := resolveBoxWidth(style, width, padding, border, margin)
	margin.Left, margin.Right = resolveHorizontalMargins(style, width, boxW, margin)
	outerX := x + margin.Left
	outerY := y + margin.Top
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
	if style.BackgroundImage != "" {
		l.ops = append(l.ops, BackgroundImageOp{
			Rect:      Rect{X: outerX, Y: outerY, W: boxW, H: 0},
			Src:       style.BackgroundImage,
			Repeat:    style.BackgroundRepeat,
			PositionX: style.BackgroundPositionX,
			PositionY: style.BackgroundPositionY,
			Node:      n,
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
		floatBottom := contentY
		floatLeftX := contentX
		floatRightX := contentX + contentW
		floatRowY := contentY
		floatRowH := 0.0
		havePrevBlock := false
		prevBlockMarginBottom := 0.0
		for _, child := range n.Children {
			if child.Style.Display == displayNone {
				continue
			}
			if child.Type == TextNode && normalizeSpace(child.Data) == "" {
				continue
			}
			if child.Type == ElementNode && child.Style.Float != "none" {
				if line != nil {
					curY += line.finish()
					line = nil
				}
				floatW := l.floatWidth(child, contentW, contentW)
				layoutW := floatW
				if child.Style.Width.set {
					layoutW = contentW
				}
				if floatW > contentW {
					floatW = contentW
				}
				if child.Style.Float == "right" {
					if floatRightX-floatW < floatLeftX && floatRowH > 0 {
						floatRowY += floatRowH
						floatLeftX = contentX
						floatRightX = contentX + contentW
						floatRowH = 0
					}
					floatRightX -= floatW
					h := l.layoutBlock(child, floatRightX, floatRowY, layoutW)
					floatRowH = math.Max(floatRowH, h)
				} else {
					if floatLeftX+floatW > floatRightX && floatRowH > 0 {
						floatRowY += floatRowH
						floatLeftX = contentX
						floatRightX = contentX + contentW
						floatRowH = 0
					}
					h := l.layoutBlock(child, floatLeftX, floatRowY, layoutW)
					floatLeftX += floatW
					floatRowH = math.Max(floatRowH, h)
				}
				floatBottom = math.Max(floatBottom, floatRowY+floatRowH)
				continue
			}
			if child.Type == ElementNode && child.Style.Clear != "none" {
				curY = math.Max(curY, floatBottom)
				floatLeftX = contentX
				floatRightX = contentX + contentW
				floatRowY = curY
				floatRowH = 0
				havePrevBlock = false
				prevBlockMarginBottom = 0
			}
			if child.Type == TextNode || child.Style.Display == displayInline || child.Style.Display == displayInlineBlock {
				if line == nil {
					line = newLineBuilder(l, contentX, curY, contentW, n.Style.TextAlign)
				}
				l.addInlineNode(line, child)
				havePrevBlock = false
				prevBlockMarginBottom = 0
				continue
			}
			if line != nil {
				curY += line.finish()
				line = nil
				havePrevBlock = false
				prevBlockMarginBottom = 0
			}
			childMargins := resolveMargins(child.Style.Margin, contentW, child.Style.FontSize)
			if havePrevBlock {
				curY -= marginCollapseAdjustment(prevBlockMarginBottom, childMargins.Top)
			}
			curY += l.layoutBlock(child, contentX, curY, contentW)
			havePrevBlock = true
			prevBlockMarginBottom = childMargins.Bottom
		}
		if line != nil {
			curY += line.finish()
		}
		if style.Float != "none" {
			curY = math.Max(curY, floatBottom)
		}
	}
	contentH := curY - contentY
	if v, ok := style.Height.resolve(width, font); ok {
		if style.BoxSizing == "border-box" {
			innerH := v - padding.Vertical() - border.Vertical()
			if innerH < 0 {
				innerH = 0
			}
			contentH = innerH
		} else if v > contentH {
			contentH = v
		}
	}
	totalH := margin.Top + border.Top + padding.Top + contentH + padding.Bottom + border.Bottom + margin.Bottom
	n.Box = Rect{X: outerX, Y: outerY, W: boxW, H: totalH - margin.Vertical()}
	l.patchBoxHeight(n, n.Box)
	if border.Horizontal()+border.Vertical() > 0 {
		l.emitBorder(n, n.Box, border, style.BorderColors, style.BorderStyle)
	}
	return totalH
}

func resolveMargins(edges edgeLengths, base, font float64) Insets {
	top, _ := edges.Top.resolve(base, font)
	right, _ := edges.Right.resolve(base, font)
	bottom, _ := edges.Bottom.resolve(base, font)
	left, _ := edges.Left.resolve(base, font)
	return Insets{Top: top, Right: right, Bottom: bottom, Left: left}
}

func resolveBoxWidth(style Style, containingWidth float64, padding, border, margin Insets) float64 {
	if v, ok := style.Width.resolve(containingWidth, style.FontSize); ok {
		boxW := v
		if style.BoxSizing != "border-box" {
			boxW += padding.Horizontal() + border.Horizontal()
		}
		return clampLength(boxW, style, containingWidth, padding, border)
	}
	boxW := containingWidth - margin.Horizontal()
	return clampLength(boxW, style, containingWidth, padding, border)
}

func resolveHorizontalMargins(style Style, containingWidth, boxW float64, margin Insets) (float64, float64) {
	leftAuto := style.Margin.Left.set && style.Margin.Left.unit == unitAuto
	rightAuto := style.Margin.Right.set && style.Margin.Right.unit == unitAuto
	if !leftAuto && !rightAuto {
		return margin.Left, margin.Right
	}
	free := containingWidth - boxW - nonAutoMargin(style.Margin.Left, margin.Left) - nonAutoMargin(style.Margin.Right, margin.Right)
	if free < 0 {
		free = 0
	}
	switch {
	case leftAuto && rightAuto:
		return free / 2, free / 2
	case leftAuto:
		return free, margin.Right
	case rightAuto:
		return margin.Left, free
	default:
		return margin.Left, margin.Right
	}
}

func nonAutoMargin(l length, resolved float64) float64 {
	if l.set && l.unit == unitAuto {
		return 0
	}
	return resolved
}

func marginCollapseAdjustment(previousBottom, currentTop float64) float64 {
	return previousBottom + currentTop - collapseMargins(previousBottom, currentTop)
}

func collapseMargins(a, b float64) float64 {
	switch {
	case a >= 0 && b >= 0:
		return math.Max(a, b)
	case a <= 0 && b <= 0:
		return math.Min(a, b)
	default:
		return a + b
	}
}

func clampLength(boxW float64, style Style, containingWidth float64, padding, border Insets) float64 {
	extras := 0.0
	if style.BoxSizing != "border-box" {
		extras = padding.Horizontal() + border.Horizontal()
	}
	if min, ok := style.MinWidth.resolve(containingWidth, style.FontSize); ok {
		boxW = math.Max(boxW, min+extras)
	}
	if max, ok := style.MaxWidth.resolve(containingWidth, style.FontSize); ok && max >= 0 {
		boxW = math.Min(boxW, max+extras)
	}
	return boxW
}

func (l *layoutContext) floatWidth(n *Node, width, parentWidth float64) float64 {
	if n == nil {
		return width
	}
	font := n.Style.FontSize
	padding := n.Style.Padding.resolve(parentWidth, font)
	border := n.Style.Border.resolve(parentWidth, font)
	if v, ok := n.Style.Width.resolve(parentWidth, font); ok {
		if n.Style.BoxSizing == "border-box" {
			return math.Min(math.Max(v, 0), width)
		}
		return math.Min(math.Max(v+padding.Horizontal()+border.Horizontal(), 0), width)
	}
	preferred := l.preferredWidth(n, parentWidth)
	if preferred <= 0 {
		return width
	}
	return math.Min(preferred+n.Style.FontSize, width)
}

func (l *layoutContext) preferredWidth(n *Node, base float64) float64 {
	if n == nil || n.Style.Display == displayNone {
		return 0
	}
	if l.preferred != nil {
		key := preferredKey{node: n, base: base}
		if v, ok := l.preferred[key]; ok {
			return v
		}
		v := l.computePreferredWidth(n, base)
		l.preferred[key] = v
		return v
	}
	return l.computePreferredWidth(n, base)
}

func (l *layoutContext) computePreferredWidth(n *Node, base float64) float64 {
	if n.Type == TextNode {
		text := normalizeSpace(n.Data)
		if text == "" {
			return 0
		}
		return l.cfg.measurer.MeasureText(text, n.Style.textStyle()).W
	}
	if n.Type != ElementNode {
		return l.preferredChildrenWidth(n, base)
	}
	font := n.Style.FontSize
	padding := n.Style.Padding.resolve(base, font)
	border := n.Style.Border.resolve(base, font)
	margin := resolveMargins(n.Style.Margin, base, font)
	if v, ok := n.Style.Width.resolve(base, font); ok {
		if n.Style.BoxSizing == "border-box" {
			return v + margin.Horizontal()
		}
		return v + padding.Horizontal() + border.Horizontal() + margin.Horizontal()
	}
	if n.Tag == "img" {
		if w, ok := n.Style.Width.resolve(base, font); ok {
			return w + padding.Horizontal() + border.Horizontal() + margin.Horizontal()
		}
		if l.cfg.images != nil {
			src, _ := n.AttrValue("src")
			if sz, ok := l.cfg.images.ImageSize(src); ok {
				return sz.W + padding.Horizontal() + border.Horizontal() + margin.Horizontal()
			}
		}
	}
	return l.preferredChildrenWidth(n, base) + padding.Horizontal() + border.Horizontal() + margin.Horizontal()
}

func (l *layoutContext) preferredChildrenWidth(n *Node, base float64) float64 {
	totalFloats := 0.0
	hasFloat := false
	inlineTotal := 0.0
	blockMax := 0.0
	for _, child := range n.Children {
		if child.Style.Display == displayNone {
			continue
		}
		w := l.preferredWidth(child, base)
		if child.Type == TextNode || child.Style.Display == displayInline || child.Style.Display == displayInlineBlock {
			inlineTotal += w
			continue
		}
		if child.Type == ElementNode && child.Style.Float != "none" {
			hasFloat = true
			totalFloats += w + child.Style.FontSize
			continue
		}
		blockMax = math.Max(blockMax, w)
	}
	if hasFloat && blockMax == 0 && inlineTotal == 0 {
		return totalFloats
	}
	return math.Max(blockMax, inlineTotal)
}

// emitListMarker 为 li 生成无序圆点或有序数字 marker。
func (l *layoutContext) emitListMarker(n *Node, x, y float64) {
	if n.Style.ListStyleType == "none" {
		return
	}
	text := "\u2022"
	if parent := n.Parent; parent != nil && parent.Tag == "ol" {
		text = listItemOrdinal(n) + "."
	} else {
		switch n.Style.ListStyleType {
		case "circle":
			text = "\u25e6"
		case "square":
			text = "\u25aa"
		}
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
		line.addText(normalizeInlineSpace(n.Data), n.Style, n)
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
func (l *layoutContext) emitBorder(n *Node, r Rect, b Insets, c edgeColors, styles edgeStrings) {
	if b.Top > 0 && drawableBorderStyle(styles.Top) && c.Top.A != 0 {
		l.ops = append(l.ops, RectOp{Rect: Rect{X: r.X, Y: r.Y, W: r.W, H: b.Top}, Color: c.Top, Node: n})
	}
	if b.Right > 0 && drawableBorderStyle(styles.Right) && c.Right.A != 0 {
		l.ops = append(l.ops, RectOp{Rect: Rect{X: r.Right() - b.Right, Y: r.Y, W: b.Right, H: r.H}, Color: c.Right, Node: n})
	}
	if b.Bottom > 0 && drawableBorderStyle(styles.Bottom) && c.Bottom.A != 0 {
		l.ops = append(l.ops, RectOp{Rect: Rect{X: r.X, Y: r.Bottom() - b.Bottom, W: r.W, H: b.Bottom}, Color: c.Bottom, Node: n})
	}
	if b.Left > 0 && drawableBorderStyle(styles.Left) && c.Left.A != 0 {
		l.ops = append(l.ops, RectOp{Rect: Rect{X: r.X, Y: r.Y, W: b.Left, H: r.H}, Color: c.Left, Node: n})
	}
}

func drawableBorderStyle(style string) bool {
	return style != "" && style != "none" && style != "hidden"
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
		case BackgroundImageOp:
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
	start := -1
	flushWord := func(end int) {
		if start >= 0 && start < end {
			l.addMeasuredText(text[start:end], style, node)
			l.lastSpace = false
		}
		start = -1
	}
	for i, r := range text {
		if r == ' ' {
			flushWord(i)
			if l.cursor != 0 && !l.lastSpace {
				l.addMeasuredText(" ", style, node)
				l.lastSpace = true
			}
			continue
		}
		if start < 0 {
			start = i
		}
	}
	flushWord(len(text))
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
	baseline := l.y + l.lineH*0.8
	x := l.x + offset
	for _, item := range l.items {
		item.rect.X = x
		item.rect.Y = l.y
		if item.img {
			item.node.Box = item.rect
			l.ctx.ops = append(l.ctx.ops, ImageOp{Rect: item.rect, Src: item.src, Alt: item.alt, Node: item.node})
		} else if item.text != "" {
			item.rect.H = l.lineH
			if item.node.Type == TextNode {
				item.node.Box = item.rect
			}
			l.ctx.ops = append(l.ctx.ops, TextOp{Rect: item.rect, Text: item.text, Style: item.style, Baseline: baseline, Node: item.node})
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
