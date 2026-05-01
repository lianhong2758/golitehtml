package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lianhong2758/golitehtml"
)

const sampleHTML = `<!doctype html>
<html>
<head>
  <style>
    body { font-family: sans-serif; color: #202428; background: #f6f8fa; }
    .panel { margin: 24px; padding: 20px; background: white; border: 1px solid #d0d7de; }
    h1 { margin-top: 0; color: #0969da; }
    code { background: #eef2f7; padding: 2px 4px; }
    .muted { color: #57606a; }
  </style>
</head>
<body>
  <div class="panel">
    <h1>HTML to PNG</h1>
    <p>这是一个使用 <strong>Go</strong> 和 <code>gg</code> 绘制的轻量 HTML 渲染示例。</p>
    <p class="muted">支持基础 CSS、块级布局、行内换行、列表、链接、图片、背景和边框。</p>
    <ul>
      <li>解析 HTML 与 CSS</li>
      <li>计算盒模型和文本行</li>
      <li>输出 PNG 图片</li>
    </ul>
  </div>
</body>
</html>`

type fontPathsFlag []string

func (f *fontPathsFlag) String() string {
	return strings.Join(*f, ",")
}

func (f *fontPathsFlag) Set(value string) error {
	for _, part := range strings.Split(value, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			*f = append(*f, part)
		}
	}
	return nil
}

func main() {
	var fonts fontPathsFlag
	var (
		in    = flag.String("in", "", "HTML 文件路径，留空则使用内置示例")
		out   = flag.String("out", "output/html.png", "输出 PNG 路径")
		width = flag.Int("width", 900, "图片宽度")
		scale = flag.Float64("scale", 1, "超采样绘制倍率，大于 1 可提升边缘平滑度")
		css   = flag.String("css", "", "额外 CSS 文件路径")
	)
	flag.Var(&fonts, "font", "外部 TTF/OTF 字体文件路径，可重复或用逗号分隔；第一个作为默认字体")
	flag.Parse()

	html := []byte(sampleHTML)
	baseDir := ""
	if *in != "" {
		data, err := os.ReadFile(*in)
		if err != nil {
			fmt.Fprintf(os.Stderr, "读取 HTML 文件失败: %v\n", err)
			os.Exit(1)
		}
		html = data
		baseDir = filepath.Dir(*in)
	}

	fontData := make([][]byte, 0, len(fonts))
	for _, path := range fonts {
		data, err := os.ReadFile(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "读取字体文件失败: %v\n", err)
			os.Exit(1)
		}
		fontData = append(fontData, data)
	}

	var userCSS string
	if *css != "" {
		data, err := os.ReadFile(*css)
		if err != nil {
			fmt.Fprintf(os.Stderr, "读取 CSS 文件失败: %v\n", err)
			os.Exit(1)
		}
		userCSS = string(data)
	}

	renderer, err := golitehtml.New(golitehtml.Options{
		Width:       *width,
		RenderScale: *scale,
		Fonts:       fontData,
		BaseDir:     baseDir,
		UserCSS:     strings.TrimSpace(userCSS),
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "初始化渲染器失败: %v\n", err)
		os.Exit(1)
	}

	output := filepath.Clean(*out)
	if err := renderer.RenderToFile(html, output); err != nil {
		fmt.Fprintf(os.Stderr, "生成 PNG 失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("已生成图片: %s\n", output)
}
