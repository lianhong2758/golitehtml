# golitehtml

`golitehtml` 是一个轻量的 Go HTML 转 PNG 库，解析和布局思路参考
`litehtml`：库负责解析 HTML/CSS、计算元素位置，再把显示列表交给绘图后端。
本项目内置 `gg` 后端，可直接输出 PNG。

它适合渲染富文本片段、帮助文档、提示卡片、简单文章和服务端图片生成；不是完整浏览器，
不执行 JavaScript，也不实现 float、flex、grid、定位和复杂表格布局。

## 快速运行

```cmd
go run ./cmd
```

渲染指定 HTML：

```cmd
go run ./cmd -in ./example.html -out ./output/example.png -width 900
```

默认内嵌一份中文字体。也可以指定字体和额外 CSS：

```cmd
go run ./cmd -in ./example.html -out ./output/example.png -font ./font.ttf -css ./extra.css
```

`-font` 可以重复传入，或用逗号分隔多个 TTF/OTF 文件。第一个字体作为默认字体，
所有传入字体都会和系统已安装字体一起参与 CSS `font-family` / HTML `<font face="">` 匹配。
如需更平滑的文字和边缘，可以用 `-scale 2` 或更高倍率启用超采样绘制；输出图片宽高也会按倍率放大。
绘图库后端默认使用 `gg`，也可以用 `-draw tinyskia` 切换到 tinyskia 后端。

## 在代码中使用

```go
package main

import (
	"os"

	"github.com/lianhong2758/golitehtml"
)

func main() {
	fontData, err := os.ReadFile("font.ttf")
	if err != nil {
		panic(err)
	}

	r, err := golitehtml.New(golitehtml.Options{
		Width:       900,
		RenderScale: 2,
		DrawingLibrary: golitehtml.DrawingLibraryTinySkia,
		Fonts:       [][]byte{fontData},
		BaseDir:     ".",
	})
	if err != nil {
		panic(err)
	}

	err = r.RenderToFile([]byte(`<h1>Hello</h1><p>HTML to PNG</p>`), "html.png")
	if err != nil {
		panic(err)
	}
}
```

## 当前支持的子集

- HTML 解析基于 `golang.org/x/net/html`
- 支持 `<style>`、调用方传入的 CSS、内联 `style=""`
- 选择器：标签、`.class`、`#id`、组合简单选择器和后代选择器
- 常用 CSS：`display`、`color`、`background-color`、`font-*`、`line-height`、
  `text-decoration`、`text-align`、`width`、`height`、`margin`、`padding`、`border`
- 字体：支持 `font-family` / `font` 简写和 `<font face="">`；会先匹配调用方传入字体，
  再匹配系统已安装字体，缺少 bold/italic 字体表时会基于常规字体本地绘制
- 布局：块盒、行内文本、自动换行、`<br>`、图片、链接、标题、段落、列表
- 图片：本地路径、相对路径、`file://`、`http(s)` 和常见 `data:` 图片

## 结构

- `ParseString` / `Parse`：解析 HTML 并计算样式
- `Document.Render`：生成显示列表 `Frame`
- `Renderer.Render` / `RenderToFile`：使用 `gg` 绘制 PNG
