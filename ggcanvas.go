package golitehtml

import (
	"fmt"
	"image"
	"image/color"
	stddraw "image/draw"
	"math"
	"strconv"
	"strings"

	"github.com/FloatTech/gg"
	builtfont "github.com/lianhong2758/golitehtml/font"
	xdraw "golang.org/x/image/draw"
	xfont "golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

type fontManager struct {
	font  *opentype.Font
	faces map[string]xfont.Face
}

// newFontManager 解析调用方字体或内嵌字体，并按字号缓存 font.Face。
func newFontManager(data []byte) (*fontManager, error) {
	if len(data) == 0 {
		data = builtfont.TTF
	}
	ttf, err := opentype.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("parse font: %w", err)
	}
	return &fontManager{
		font:  ttf,
		faces: make(map[string]xfont.Face, 32),
	}, nil
}

// MeasureText 使用真实字体度量文本宽度，保证布局和最终绘制尽量一致。
func (m *fontManager) MeasureText(text string, style TextStyle) Size {
	if text == "" {
		return Size{}
	}
	face, err := m.face(style.Size)
	if err != nil {
		return DefaultMeasurer{}.MeasureText(text, style)
	}
	dc := gg.NewContext(8, 8)
	dc.SetFontFace(face)
	width, _ := dc.MeasureString(text)
	if style.Weight >= 600 {
		width += 0.8
	}
	lineHeight := style.LineHeight
	if lineHeight <= 0 {
		lineHeight = style.Size * 1.25
	}
	return Size{W: width, H: lineHeight}
}

// face 返回指定字号的字体 face；同一字号会复用缓存。
func (m *fontManager) face(size float64) (xfont.Face, error) {
	if size <= 0 {
		size = 16
	}
	key := strconv.FormatFloat(size, 'f', 2, 64)
	if face, ok := m.faces[key]; ok {
		return face, nil
	}
	face, err := opentype.NewFace(m.font, &opentype.FaceOptions{
		Size:    size,
		DPI:     72,
		Hinting: xfont.HintingNone,
	})
	if err != nil {
		return nil, err
	}
	m.faces[key] = face
	return face, nil
}

type ggCanvas struct {
	dc     *gg.Context
	fonts  *fontManager
	images *imageLoader
}

func newGGCanvas(width, height int, fonts *fontManager, images *imageLoader) *ggCanvas {
	return &ggCanvas{
		dc:     gg.NewContext(width, height),
		fonts:  fonts,
		images: images,
	}
}

func (c *ggCanvas) clear(clr Color) {
	c.dc.SetColor(toRGBA(clr))
	c.dc.Clear()
}

func (c *ggCanvas) image() image.Image {
	return c.dc.Image()
}

// DrawRect 绘制背景、边框等矩形操作。
func (c *ggCanvas) DrawRect(op RectOp) {
	if op.Rect.Empty() || op.Color.A == 0 {
		return
	}
	c.dc.SetColor(toRGBA(op.Color))
	c.dc.DrawRectangle(op.Rect.X, op.Rect.Y, op.Rect.W, op.Rect.H)
	c.dc.Fill()
}

// DrawText 根据 TextOp 绘制文字，并用轻量偏移/剪切模拟粗体和斜体。
func (c *ggCanvas) DrawText(op TextOp) {
	if op.Text == "" || op.Style.Color.A == 0 {
		return
	}

	face, err := c.fonts.face(op.Style.Size)
	if err != nil {
		return
	}
	metrics := face.Metrics()
	ascent := fixedToFloat(metrics.Ascent)
	descent := fixedToFloat(metrics.Descent)
	glyphHeight := ascent + descent
	// TextOp.Rect.H 是行盒高度，文字需要按实际字形高度居中，否则有背景时会偏上。
	baseline := op.Rect.Y + maxFloat(0, (op.Rect.H-glyphHeight)/2) + ascent

	c.dc.SetFontFace(face)
	c.dc.SetColor(toRGBA(op.Style.Color))
	if op.Style.Italic {
		c.dc.Push()
		c.dc.ShearAbout(-0.18, 0, op.Rect.X, baseline)
	}
	c.dc.DrawString(op.Text, op.Rect.X, baseline)
	if op.Style.Weight >= 600 {
		c.dc.DrawString(op.Text, op.Rect.X+0.8, baseline)
	}
	if op.Style.Italic {
		c.dc.Pop()
	}
	if op.Style.Underline {
		c.dc.SetLineWidth(1.2)
		y := baseline + 2
		c.dc.DrawLine(op.Rect.X, y, op.Rect.X+op.Rect.W, y)
		c.dc.Stroke()
	}
}

// DrawImage 加载图片并按布局尺寸绘制，尺寸不一致时做高质量缩放。
func (c *ggCanvas) DrawImage(op ImageOp) {
	if op.Rect.Empty() || op.Src == "" {
		return
	}
	img, ok := c.images.Image(op.Src)
	if !ok {
		return
	}
	drawW := int(op.Rect.W + 0.5)
	drawH := int(op.Rect.H + 0.5)
	if drawW <= 0 || drawH <= 0 {
		return
	}
	bounds := img.Bounds()
	if bounds.Dx() == drawW && bounds.Dy() == drawH {
		c.dc.DrawImage(img, int(op.Rect.X+0.5), int(op.Rect.Y+0.5))
		return
	}
	c.dc.DrawImage(scaleImage(img, drawW, drawH), int(op.Rect.X+0.5), int(op.Rect.Y+0.5))
}

// DrawBackgroundImage 绘制 CSS 背景图，支持常见的 repeat/no-repeat 和基础位置关键字。
func (c *ggCanvas) DrawBackgroundImage(op BackgroundImageOp) {
	if op.Rect.Empty() || op.Src == "" {
		return
	}
	img, ok := c.images.Image(op.Src)
	if !ok {
		return
	}
	bounds := img.Bounds()
	tileW := float64(bounds.Dx())
	tileH := float64(bounds.Dy())
	if tileW <= 0 || tileH <= 0 {
		return
	}

	dstW := int(math.Ceil(op.Rect.W))
	dstH := int(math.Ceil(op.Rect.H))
	if dstW <= 0 || dstH <= 0 {
		return
	}
	dst := image.NewRGBA(image.Rect(0, 0, dstW, dstH))
	startX := backgroundStart(op.Rect.X, op.Rect.W, tileW, op.PositionX)
	startY := backgroundStart(op.Rect.Y, op.Rect.H, tileH, op.PositionY)
	repeatX, repeatY := backgroundRepeat(op.Repeat)
	if repeatX {
		for startX > op.Rect.X {
			startX -= tileW
		}
	}
	if repeatY {
		for startY > op.Rect.Y {
			startY -= tileH
		}
	}

	for y := startY; y < op.Rect.Bottom(); y += tileStep(tileH, repeatY) {
		for x := startX; x < op.Rect.Right(); x += tileStep(tileW, repeatX) {
			dstRect := image.Rect(
				int(math.Round(x-op.Rect.X)),
				int(math.Round(y-op.Rect.Y)),
				int(math.Round(x-op.Rect.X+tileW)),
				int(math.Round(y-op.Rect.Y+tileH)),
			)
			stddraw.Draw(dst, dstRect, img, img.Bounds().Min, stddraw.Over)
			if !repeatX {
				break
			}
		}
		if !repeatY {
			break
		}
	}
	c.dc.DrawImage(dst, int(math.Round(op.Rect.X)), int(math.Round(op.Rect.Y)))
}

func backgroundStart(origin, size, tile float64, pos string) float64 {
	pos = strings.ToLower(strings.TrimSpace(pos))
	switch pos {
	case "right", "bottom", "100%":
		return origin + size - tile
	case "center", "50%":
		return origin + (size-tile)/2
	default:
		if strings.HasSuffix(pos, "%") {
			v, err := strconv.ParseFloat(strings.TrimSuffix(pos, "%"), 64)
			if err == nil {
				return origin + (size-tile)*v/100
			}
		}
		if l := parseLength(pos); l.set && l.unit != unitAuto && l.unit != unitPercent {
			if v, ok := l.resolve(size, 16); ok {
				return origin + v
			}
		}
		return origin
	}
}

func backgroundRepeat(repeat string) (bool, bool) {
	switch strings.ToLower(strings.TrimSpace(repeat)) {
	case "no-repeat":
		return false, false
	case "repeat-x":
		return true, false
	case "repeat-y":
		return false, true
	default:
		return true, true
	}
}

func tileStep(tile float64, repeat bool) float64 {
	if !repeat {
		return math.MaxFloat64
	}
	if tile <= 0 {
		return 1
	}
	return tile
}

func toRGBA(c Color) color.NRGBA {
	return color.NRGBA{R: c.R, G: c.G, B: c.B, A: c.A}
}

func fixedToFloat(v fixed.Int26_6) float64 {
	return float64(v) / 64
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func scaleImage(src image.Image, width, height int) image.Image {
	dst := image.NewRGBA(image.Rect(0, 0, width, height))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), xdraw.Over, nil)
	return dst
}
