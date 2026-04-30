// Package golitehtml 提供一个小型、可嵌入的 HTML/CSS 布局和 PNG 渲染引擎。
//
// 这个包沿用 litehtml 的集成思路：负责解析 HTML/CSS、计算元素位置并返回绘制命令；
// 同时内置基于 gg 的 PNG 绘制入口。
//
// 当前实现有意聚焦常见的嵌入式 HTML 子集：块级与行内流、文本换行、基础字体、
// 盒模型、颜色、背景、边框、链接、列表和图片。它不是浏览器引擎，也不会执行脚本。
package golitehtml
