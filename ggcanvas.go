package golitehtml

import (
	"image"
	"image/color"
	stddraw "image/draw"
	"math"
	"strconv"
	"strings"

	"github.com/FloatTech/gg"
	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/math/fixed"
)

type ggCanvas struct {
	dc     *gg.Context
	scale  float64
	fonts  *fontManager
	images *imageLoader
}

func newGGCanvas(width, height int, scale float64, fonts *fontManager, images *imageLoader) *ggCanvas {
	if scale <= 0 {
		scale = 1
	}
	return &ggCanvas{
		dc:     gg.NewContext(width, height),
		scale:  scale,
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
	rect := c.scaleRect(op.Rect)
	c.dc.DrawRectangle(rect.X, rect.Y, rect.W, rect.H)
	c.dc.Fill()
}

// DrawText 根据 TextOp 绘制文字，并在缺少对应字体表时本地模拟粗体和斜体。
func (c *ggCanvas) DrawText(op TextOp) {
	if op.Text == "" || op.Style.Color.A == 0 {
		return
	}

	style := c.scaleTextStyle(op.Style)
	runs, err := c.fonts.textRuns(op.Text, style)
	if err != nil {
		return
	}
	if len(runs) == 0 {
		return
	}
	face := runs[0].Font.Face
	metrics := face.Metrics()
	ascent := fixedToFloat(metrics.Ascent)
	descent := fixedToFloat(metrics.Descent)
	glyphHeight := ascent + descent
	rect := c.scaleRect(op.Rect)
	baseline := rect.Y + maxFloat(0, (rect.H-glyphHeight)/2) + ascent
	if op.Baseline != 0 {
		baseline = op.Baseline * c.scale
	}

	c.dc.SetColor(toRGBA(op.Style.Color))
	x := rect.X
	for _, run := range runs {
		drawLocalText(c.dc, run.Font.Face, run.Text, x, baseline, run.Font.Synthesis, c.scale)
		runWidth, _ := c.dc.MeasureString(run.Text)
		if run.Font.Synthesis.Bold {
			runWidth += 0.8 * c.scale
		}
		if run.Font.Synthesis.Italic {
			runWidth *= 1.02
		}
		x += runWidth
	}
	if op.Style.Underline {
		c.dc.SetLineWidth(1.2 * c.scale)
		y := baseline + 2*c.scale
		c.dc.DrawLine(rect.X, y, rect.X+rect.W, y)
		c.dc.Stroke()
	}
}

// drawLocalText 绘制文字，并按需用局部变换/偏移模拟缺失的 italic/bold 字形。
func drawLocalText(dc *gg.Context, face fontFace, text string, x, baseline float64, synth fontSynthesis, scale float64) {
	dc.SetFontFace(face)
	if synth.Italic {
		dc.Push()
		dc.ShearAbout(-0.18, 0, x, baseline)
	}
	dc.DrawString(text, x, baseline)
	if synth.Bold {
		dc.DrawString(text, x+0.8*scale, baseline)
	}
	if synth.Italic {
		dc.Pop()
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
	rect := c.scaleRect(op.Rect)
	drawW := int(rect.W + 0.5)
	drawH := int(rect.H + 0.5)
	if drawW <= 0 || drawH <= 0 {
		return
	}
	bounds := img.Bounds()
	if bounds.Dx() == drawW && bounds.Dy() == drawH {
		c.dc.DrawImage(img, int(rect.X+0.5), int(rect.Y+0.5))
		return
	}
	c.dc.DrawImage(scaleImage(img, drawW, drawH), int(rect.X+0.5), int(rect.Y+0.5))
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
	scaledTile := img
	if c.scale != 1 {
		scaledTile = scaleImage(img, int(math.Round(tileW*c.scale)), int(math.Round(tileH*c.scale)))
	}

	rect := c.scaleRect(op.Rect)
	dstW := int(math.Ceil(rect.W))
	dstH := int(math.Ceil(rect.H))
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
				int(math.Round((x-op.Rect.X)*c.scale)),
				int(math.Round((y-op.Rect.Y)*c.scale)),
				int(math.Round((x-op.Rect.X+tileW)*c.scale)),
				int(math.Round((y-op.Rect.Y+tileH)*c.scale)),
			)
			stddraw.Draw(dst, dstRect, scaledTile, scaledTile.Bounds().Min, stddraw.Over)
			if !repeatX {
				break
			}
		}
		if !repeatY {
			break
		}
	}
	c.dc.DrawImage(dst, int(math.Round(rect.X)), int(math.Round(rect.Y)))
}

func (c *ggCanvas) scaleRect(r Rect) Rect {
	if c.scale == 1 {
		return r
	}
	return Rect{
		X: r.X * c.scale,
		Y: r.Y * c.scale,
		W: r.W * c.scale,
		H: r.H * c.scale,
	}
}

func (c *ggCanvas) scaleTextStyle(style TextStyle) TextStyle {
	if c.scale == 1 {
		return style
	}
	style.Size *= c.scale
	style.LineHeight *= c.scale
	return style
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
