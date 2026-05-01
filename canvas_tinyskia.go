package golitehtml

import (
	"image"
	"strconv"

	tinyskia "github.com/lumifloat/tinyskia"
)

type tinySkiaCanvas struct {
	dc        *tinyskia.Context
	scale     float64
	fonts     *fontManager
	images    *imageLoader
	fontCache map[string]*tinyskia.Font
}

func newTinySkiaCanvas(width, height int, scale float64, fonts *fontManager, images *imageLoader) *tinySkiaCanvas {
	if scale <= 0 {
		scale = 1
	}
	return &tinySkiaCanvas{
		dc:        tinyskia.NewContext(width, height),
		scale:     scale,
		fonts:     fonts,
		images:    images,
		fontCache: make(map[string]*tinyskia.Font, 32),
	}
}

func (c *tinySkiaCanvas) clear(clr Color) {
	c.dc.SetFillStyleSolidColor(toRGBA(clr))
	c.dc.FillRect(0, 0, float64(c.dc.Width()), float64(c.dc.Height()))
}

func (c *tinySkiaCanvas) image() image.Image {
	return c.dc.Image()
}

func (c *tinySkiaCanvas) DrawRect(op RectOp) {
	if op.Rect.Empty() || op.Color.A == 0 {
		return
	}
	rect := scaleRect(op.Rect, c.scale)
	c.dc.SetFillStyleSolidColor(toRGBA(op.Color))
	c.dc.FillRect(rect.X, rect.Y, rect.W, rect.H)
}

func (c *tinySkiaCanvas) DrawText(op TextOp) {
	if op.Text == "" || op.Style.Color.A == 0 {
		return
	}
	style := scaleTextStyle(op.Style, c.scale)
	runs, err := c.fonts.textRuns(op.Text, style)
	if err != nil || len(runs) == 0 {
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

	c.dc.SetFillStyleSolidColor(toRGBA(op.Style.Color))
	x := rect.X
	for _, run := range runs {
		c.drawTextRun(run, x, baseline)
		runWidth := c.measureRunWidth(run)
		x += runWidth
	}
	if op.Style.Underline {
		c.dc.SetStrokeStyleSolidColor(toRGBA(op.Style.Color))
		c.dc.SetLineWidth(1.2 * c.scale)
		y := baseline + 2*c.scale
		c.dc.BeginPath()
		c.dc.MoveTo(rect.X, y)
		c.dc.LineTo(rect.X+rect.W, y)
		c.dc.Stroke()
	}
}

func (c *tinySkiaCanvas) drawTextRun(run resolvedTextRun, x, baseline float64) {
	font, err := c.tinySkiaFont(run.Font)
	if err != nil {
		return
	}
	c.dc.SetFontFace(font)
	if run.Font.Synthesis.Italic {
		c.dc.Save()
		c.dc.Transform(1, 0, -0.18, 1, 0.18*baseline, 0)
	}
	c.dc.FillText(run.Text, x, baseline)
	if run.Font.Synthesis.Bold {
		c.dc.FillText(run.Text, x+0.8*c.scale, baseline)
	}
	if run.Font.Synthesis.Italic {
		c.dc.Restore()
	}
}

func (c *tinySkiaCanvas) measureRunWidth(run resolvedTextRun) float64 {
	font, err := c.tinySkiaFont(run.Font)
	if err != nil {
		return 0
	}
	c.dc.SetFontFace(font)
	width := c.dc.MeasureText(run.Text).Width
	if run.Font.Synthesis.Bold {
		width += 0.8 * c.scale
	}
	if run.Font.Synthesis.Italic {
		width *= 1.02
	}
	return width
}

func (c *tinySkiaCanvas) tinySkiaFont(resolved resolvedFont) (*tinyskia.Font, error) {
	if resolved.Entry == nil {
		return nil, nil
	}
	size := resolved.Size
	if size <= 0 {
		size = 16
	}
	key := strconv.Itoa(resolved.Entry.order) + "|" + strconv.FormatFloat(size, 'f', 2, 64)
	if font, ok := c.fontCache[key]; ok {
		return font, nil
	}
	sfntFont, err := resolved.Entry.loadFont()
	if err != nil {
		return nil, err
	}
	font := tinyskia.NewFont(sfntFont, size, 72)
	c.fontCache[key] = font
	return font, nil
}

func (c *tinySkiaCanvas) DrawImage(op ImageOp) {
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
	if bounds.Dx() != drawW || bounds.Dy() != drawH {
		img = scaleImage(img, drawW, drawH)
	}
	c.dc.DrawImage(img, rect.X, rect.Y)
}

func (c *tinySkiaCanvas) DrawBackgroundImage(op BackgroundImageOp) {
	img, rect, ok := backgroundImageLayer(op, c.scale, c.images)
	if !ok {
		return
	}
	c.dc.DrawImage(img, rect.X, rect.Y)
}
