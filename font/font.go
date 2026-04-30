package font

import _ "embed"

// TTF 是默认嵌入字体，包含常用中文字符。
//
//go:embed MaokenZhuyuanTi.ttf
var TTF []byte
