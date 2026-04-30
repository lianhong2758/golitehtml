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

func main() {
	var (
		in    = flag.String("in", "", "HTML 文件路径，留空则使用内置示例")
		out   = flag.String("out", "output/html.png", "输出 PNG 路径")
		width = flag.Int("width", 900, "图片宽度")
		font  = flag.String("font", "", "外部 TTF/OTF 字体文件路径")
		css   = flag.String("css", "", "额外 CSS 文件路径")
	)
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

	var fontData []byte
	if *font != "" {
		data, err := os.ReadFile(*font)
		if err != nil {
			fmt.Fprintf(os.Stderr, "读取字体文件失败: %v\n", err)
			os.Exit(1)
		}
		fontData = data
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
		Width:   *width,
		Font:    fontData,
		BaseDir: baseDir,
		UserCSS: strings.TrimSpace(userCSS),
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
