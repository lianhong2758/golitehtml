package golitehtml

import (
	"image"
	"image/color"
	stddraw "image/draw"
	"math"
	"strconv"
	"strings"
	"unicode"

	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/math/fixed"
)

type DrawingLibrary string

const (
	DrawingLibraryGG       DrawingLibrary = "gg"
	DrawingLibraryTinySkia DrawingLibrary = "tinyskia"
)

var DrawingLibraries = []DrawingLibrary{
	DrawingLibraryGG,
	DrawingLibraryTinySkia,
}

type rasterCanvas interface {
	Canvas
	clear(Color)
	image() image.Image
}

func newRasterCanvas(width, height int, scale float64, drawing DrawingLibrary, fonts *fontManager, images *imageLoader) rasterCanvas {
	switch drawing {
	case DrawingLibraryTinySkia:
		return newTinySkiaCanvas(width, height, scale, fonts, images)
	default:
		return newGGCanvas(width, height, scale, fonts, images)
	}
}

func validDrawingLibrary(drawing DrawingLibrary) bool {
	for _, value := range DrawingLibraries {
		if drawing == value {
			return true
		}
	}
	return false
}

// TextStyle 表示宿主侧文本测量和绘制所需的字体与颜色状态。
type TextStyle struct {
	Family     string
	Size       float64
	LineHeight float64
	Weight     int
	Italic     bool
	Color      Color
	Underline  bool
}

// TextMeasurer 由嵌入方实现，用来让布局引擎不绑定任何字体或图形库。
type TextMeasurer interface {
	MeasureText(text string, style TextStyle) Size
}

// TextMeasurerFunc 将普通函数适配为 TextMeasurer。
type TextMeasurerFunc func(text string, style TextStyle) Size

// MeasureText 实现 TextMeasurer。
func (f TextMeasurerFunc) MeasureText(text string, style TextStyle) Size {
	return f(text, style)
}

// DefaultMeasurer 是确定性的无依赖测量器，适合测试、服务端预处理和近似布局。
// 图形界面集成时应优先提供真实字体指标。
type DefaultMeasurer struct{}

// MeasureText 使用简洁的字符宽度估算实现 TextMeasurer。
func (DefaultMeasurer) MeasureText(text string, style TextStyle) Size {
	size := style.Size
	if size <= 0 {
		size = 16
	}
	w := 0.0
	for _, r := range text {
		w += size * glyphWidthFactor(r)
	}
	if style.Weight >= 600 {
		w *= 1.04
	}
	if style.Italic {
		w *= 1.02
	}
	line := style.LineHeight
	if line <= 0 {
		line = size * 1.25
	}
	return Size{W: w, H: line}
}

func glyphWidthFactor(r rune) float64 {
	switch {
	case r == ' ':
		return 0.28
	case r == '\t':
		return 1.12
	case r == '.' || r == ',' || r == ':' || r == ';' || r == '\'' || r == '"' || r == '`':
		return 0.24
	case r == '!' || r == '|' || r == 'i' || r == 'j' || r == 'l' || r == 'I':
		return 0.28
	case r == 't' || r == 'f' || r == 'r':
		return 0.34
	case r == 'm' || r == 'w' || r == 'M' || r == 'W':
		return 0.82
	case unicode.IsUpper(r):
		return 0.66
	case unicode.IsLower(r):
		return 0.5
	case unicode.IsDigit(r):
		return 0.55
	case r < 128:
		return 0.38
	default:
		return 1
	}
}

// ImageResolver 返回图片固有尺寸。返回 false 表示图片没有可用固有尺寸；
// 除非 CSS width/height 提供尺寸，否则它会以零尺寸替换元素参与布局。
type ImageResolver interface {
	ImageSize(src string) (Size, bool)
}

// ImageResolverFunc 将普通函数适配为 ImageResolver。
type ImageResolverFunc func(src string) (Size, bool)

// ImageSize 实现 ImageResolver。
func (f ImageResolverFunc) ImageSize(src string) (Size, bool) { return f(src) }

// Canvas 是绘制回调接口。实现方可以把这些操作转发到 image/draw、Gio、Ebiten、
// OpenGL、平台控件或测试记录器。
type Canvas interface {
	DrawRect(RectOp)
	DrawText(TextOp)
	DrawImage(ImageOp)
}

// Op 表示一条显示列表操作。
type Op interface {
	Bounds() Rect
	draw(Canvas, float64, float64)
}

// RectOp 表示填充一个矩形。
type RectOp struct {
	Rect  Rect
	Color Color
	Node  *Node
}

// Bounds 实现 Op。
func (op RectOp) Bounds() Rect { return op.Rect }

func (op RectOp) draw(c Canvas, dx, dy float64) {
	op.Rect.X += dx
	op.Rect.Y += dy
	c.DrawRect(op)
}

// TextOp 表示绘制一段文本。
type TextOp struct {
	Rect     Rect
	Text     string
	Style    TextStyle
	Baseline float64
	Node     *Node
}

// Bounds 实现 Op。
func (op TextOp) Bounds() Rect { return op.Rect }

func (op TextOp) draw(c Canvas, dx, dy float64) {
	op.Rect.X += dx
	op.Rect.Y += dy
	if op.Baseline != 0 {
		op.Baseline += dy
	}
	c.DrawText(op)
}

// ImageOp 表示绘制一张图片。
type ImageOp struct {
	Rect Rect
	Src  string
	Alt  string
	Node *Node
}

// Bounds 实现 Op。
func (op ImageOp) Bounds() Rect { return op.Rect }

func (op ImageOp) draw(c Canvas, dx, dy float64) {
	op.Rect.X += dx
	op.Rect.Y += dy
	c.DrawImage(op)
}

type backgroundImageCanvas interface {
	DrawBackgroundImage(BackgroundImageOp)
}

// BackgroundImageOp 表示元素背景图，绘制时会被裁剪在元素背景盒内。
type BackgroundImageOp struct {
	Rect      Rect
	Src       string
	Repeat    string
	PositionX string
	PositionY string
	Node      *Node
}

// Bounds 实现 Op。
func (op BackgroundImageOp) Bounds() Rect { return op.Rect }

func (op BackgroundImageOp) draw(c Canvas, dx, dy float64) {
	bg, ok := c.(backgroundImageCanvas)
	if !ok {
		return
	}
	op.Rect.X += dx
	op.Rect.Y += dy
	bg.DrawBackgroundImage(op)
}

// Frame 是文档在指定视口宽度下渲染得到的不可变结果。
type Frame struct {
	Width  float64
	Height float64
	Ops    []Op
	Root   *Node
}

// Draw 将显示列表回放到 c。可选 clip 使用文档坐标，且在 dx/dy 偏移应用之前生效。
func (f *Frame) Draw(c Canvas, dx, dy float64, clip *Rect) {
	if f == nil || c == nil {
		return
	}
	for _, op := range f.Ops {
		if op.Bounds().Intersects(clip) {
			op.draw(c, dx, dy)
		}
	}
}

func scaleRect(r Rect, scale float64) Rect {
	if scale == 1 {
		return r
	}
	return Rect{
		X: r.X * scale,
		Y: r.Y * scale,
		W: r.W * scale,
		H: r.H * scale,
	}
}

func scaleTextStyle(style TextStyle, scale float64) TextStyle {
	if scale == 1 {
		return style
	}
	style.Size *= scale
	style.LineHeight *= scale
	return style
}

func backgroundImageLayer(op BackgroundImageOp, scale float64, images *imageLoader) (image.Image, Rect, bool) {
	if op.Rect.Empty() || op.Src == "" {
		return nil, Rect{}, false
	}
	img, ok := images.Image(op.Src)
	if !ok {
		return nil, Rect{}, false
	}
	bounds := img.Bounds()
	tileW := float64(bounds.Dx())
	tileH := float64(bounds.Dy())
	if tileW <= 0 || tileH <= 0 {
		return nil, Rect{}, false
	}
	scaledTile := img
	if scale != 1 {
		scaledTile = scaleImage(img, int(math.Round(tileW*scale)), int(math.Round(tileH*scale)))
	}

	rect := scaleRect(op.Rect, scale)
	dstW := int(math.Ceil(rect.W))
	dstH := int(math.Ceil(rect.H))
	if dstW <= 0 || dstH <= 0 {
		return nil, Rect{}, false
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
				int(math.Round((x-op.Rect.X)*scale)),
				int(math.Round((y-op.Rect.Y)*scale)),
				int(math.Round((x-op.Rect.X+tileW)*scale)),
				int(math.Round((y-op.Rect.Y+tileH)*scale)),
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
	return dst, rect, true
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
