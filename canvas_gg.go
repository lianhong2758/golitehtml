package golitehtml

import (
	"image"
	"math"

	"github.com/FloatTech/gg"
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
	rect := scaleRect(op.Rect, c.scale)
	c.dc.DrawRectangle(rect.X, rect.Y, rect.W, rect.H)
	c.dc.Fill()
}

// DrawText 根据 TextOp 绘制文字，并在缺少对应字体表时本地模拟粗体和斜体。
func (c *ggCanvas) DrawText(op TextOp) {
	if op.Text == "" || op.Style.Color.A == 0 {
		return
	}

	style := scaleTextStyle(op.Style, c.scale)
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
	rect := scaleRect(op.Rect, c.scale)
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
	rect := scaleRect(op.Rect, c.scale)
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
	img, rect, ok := backgroundImageLayer(op, c.scale, c.images)
	if !ok {
		return
	}
	c.dc.DrawImage(img, int(math.Round(rect.X)), int(math.Round(rect.Y)))
}
