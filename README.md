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

## 在代码中使用

```go
package main

import (
	"github.com/lianhong2758/golitehtml"
)

func main() {
	r, err := golitehtml.New(golitehtml.Options{
		Width:   900,
		BaseDir: ".",
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
- 布局：块盒、行内文本、自动换行、`<br>`、图片、链接、标题、段落、列表
- 图片：本地路径、相对路径、`file://`、`http(s)` 和常见 `data:` 图片

## 结构

- `ParseString` / `Parse`：解析 HTML 并计算样式
- `Document.Render`：生成显示列表 `Frame`
- `Renderer.Render` / `RenderToFile`：使用 `gg` 绘制 PNG
