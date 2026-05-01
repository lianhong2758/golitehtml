package golitehtml

import (
	"errors"
	"fmt"
	"image"
	"image/png"
	"math"
	"os"
	"path/filepath"
)

// Options 是 HTML 到 PNG 渲染器的初始化参数。
type Options struct {
	// Width 是输出图片宽度，单位为 CSS 像素。小于等于 0 时使用 800。
	Width int
	// RenderScale 是超采样绘制倍率。大于 1 时会按更高分辨率绘制并直接输出
	// 放大后的图片；例如 2 会输出 2 倍宽高。小于等于 1 时使用 1。
	RenderScale float64
	// DrawingLibrary 选择位图绘制后端。空值使用 gg；可选值为 "gg" 和 "tinyskia"。
	DrawingLibrary DrawingLibrary
	// Fonts 是调用方提供的 TTF/OTF 字体字节列表。列表中的字体会参与
	// font-family 匹配；第一个字体作为最终兜底默认字体。
	Fonts [][]byte
	// BaseDir 用于解析相对路径图片，也可以是页面 URL。
	BaseDir string
	// UserCSS 会追加到内置 UA CSS 和文档 <style> 之后。
	UserCSS string
	// Background 是画布底色。零值时默认白色。
	Background Color
	// Transparent 为 true 时不绘制默认画布底色。
	Transparent bool
}

// Renderer 负责 HTML 的解析、布局与 PNG 绘制。
//
// 结构上沿用 litehtml 的集成思路：
// 1. HTML/CSS 解析成 DOM 和计算样式
// 2. 在指定宽度下生成显示列表
// 3. 使用所选绘图库将显示列表绘制到位图
type Renderer struct {
	width       int
	renderScale float64
	drawing     DrawingLibrary
	userCSS     string
	background  Color
	transparent bool
	fonts       *fontManager
	images      *imageLoader
}

// New 创建 HTML 到图片渲染器。
func New(opts Options) (*Renderer, error) {
	width := opts.Width
	if width <= 0 {
		width = 800
	}
	renderScale := opts.RenderScale
	if renderScale <= 1 {
		renderScale = 1
	}
	drawing := opts.DrawingLibrary
	if drawing == "" {
		drawing = DrawingLibraryGG
	}
	if !validDrawingLibrary(drawing) {
		return nil, fmt.Errorf("golitehtml: unsupported drawing library %q", drawing)
	}

	fonts, err := newFontManager(opts.Fonts)
	if err != nil {
		return nil, err
	}

	background := opts.Background
	if background.A == 0 && !opts.Transparent {
		background = Color{R: 255, G: 255, B: 255, A: 255}
	}

	return &Renderer{
		width:       width,
		renderScale: renderScale,
		drawing:     drawing,
		userCSS:     opts.UserCSS,
		background:  background,
		transparent: opts.Transparent,
		fonts:       fonts,
		images:      newImageLoader(opts.BaseDir),
	}, nil
}

// Render 把 UTF-8 HTML 渲染为内存中的图片对象。
func (r *Renderer) Render(html []byte) (image.Image, error) {
	if r == nil {
		return nil, errors.New("golitehtml: nil renderer")
	}

	doc, err := ParseString(string(html), WithUserCSS(r.userCSS))
	if err != nil {
		return nil, err
	}

	frame, err := doc.Render(float64(r.width), WithMeasurer(r.fonts), WithImageResolver(r.images))
	if err != nil {
		return nil, err
	}

	height := int(math.Ceil(frame.Height))
	if height < 1 {
		height = 1
	}

	canvasWidth := int(math.Ceil(float64(r.width) * r.renderScale))
	canvasHeight := int(math.Ceil(float64(height) * r.renderScale))
	canvas := newRasterCanvas(canvasWidth, canvasHeight, r.renderScale, r.drawing, r.fonts, r.images)
	if !r.transparent && r.background.A != 0 {
		canvas.clear(r.background)
	}
	frame.Draw(canvas, 0, 0, nil)
	return canvas.image(), nil
}

// RenderToFile 直接把 HTML 输出为 PNG 文件。
func (r *Renderer) RenderToFile(html []byte, outputPath string) error {
	img, err := r.Render(html)
	if err != nil {
		return err
	}

	if dir := filepath.Dir(outputPath); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}

	file, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer file.Close()

	return png.Encode(file, img)
}
