package golitehtml

import (
	"errors"
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
	// Font 是用于所有文字样式的 TTF/OTF 字体字节。留空时使用内嵌中文字体。
	Font []byte
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
// 3. 使用 gg 将显示列表绘制到位图
type Renderer struct {
	width       int
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

	fonts, err := newFontManager(opts.Font)
	if err != nil {
		return nil, err
	}

	background := opts.Background
	if background.A == 0 && !opts.Transparent {
		background = Color{R: 255, G: 255, B: 255, A: 255}
	}

	return &Renderer{
		width:       width,
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

	canvas := newGGCanvas(r.width, height, r.fonts, r.images)
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
