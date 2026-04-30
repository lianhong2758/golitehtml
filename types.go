package golitehtml

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Size 表示 CSS 像素单位下的二维尺寸。
type Size struct {
	W float64
	H float64
}

// Rect 表示 CSS 像素单位下的矩形。
type Rect struct {
	X float64
	Y float64
	W float64
	H float64
}

// Right 返回 X+W。
func (r Rect) Right() float64 { return r.X + r.W }

// Bottom 返回 Y+H。
func (r Rect) Bottom() float64 { return r.Y + r.H }

// Empty 判断矩形是否没有可绘制区域。
func (r Rect) Empty() bool { return r.W <= 0 || r.H <= 0 }

// Intersects 判断 r 是否与 other 相交。nil clip 会被视为无限大矩形。
func (r Rect) Intersects(other *Rect) bool {
	if other == nil {
		return true
	}
	return r.X <= other.Right() && r.Right() >= other.X &&
		r.Y <= other.Bottom() && r.Bottom() >= other.Y
}

// Insets 保存 CSS 像素单位下的盒模型边距宽度。
type Insets struct {
	Top    float64
	Right  float64
	Bottom float64
	Left   float64
}

// Horizontal 返回 Left+Right。
func (e Insets) Horizontal() float64 { return e.Left + e.Right }

// Vertical 返回 Top+Bottom。
func (e Insets) Vertical() float64 { return e.Top + e.Bottom }

// Color 表示 RGBA 颜色。零值为透明黑色。
type Color struct {
	R uint8
	G uint8
	B uint8
	A uint8
}

// Transparent 是完全透明的颜色。
var Transparent = Color{}

// Opaque 判断颜色是否完全不透明。
func (c Color) Opaque() bool { return c.A == 255 }

// String 返回类似 CSS 的十六进制颜色字符串。
func (c Color) String() string {
	if c.A == 255 {
		return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B)
	}
	return fmt.Sprintf("rgba(%d,%d,%d,%.3g)", c.R, c.G, c.B, float64(c.A)/255)
}

var namedColors = map[string]Color{
	"transparent": Transparent,
	"black":       {0, 0, 0, 255},
	"silver":      {192, 192, 192, 255},
	"gray":        {128, 128, 128, 255},
	"white":       {255, 255, 255, 255},
	"maroon":      {128, 0, 0, 255},
	"red":         {255, 0, 0, 255},
	"purple":      {128, 0, 128, 255},
	"fuchsia":     {255, 0, 255, 255},
	"green":       {0, 128, 0, 255},
	"lime":        {0, 255, 0, 255},
	"olive":       {128, 128, 0, 255},
	"yellow":      {255, 255, 0, 255},
	"navy":        {0, 0, 128, 255},
	"blue":        {0, 0, 255, 255},
	"teal":        {0, 128, 128, 255},
	"aqua":        {0, 255, 255, 255},
}

// ParseColor 解析一个小而实用的 CSS 颜色子集：命名颜色、#rgb、#rgba、
// #rrggbb、#rrggbbaa、rgb(...) 和 rgba(...)。
func ParseColor(s string) (Color, bool) {
	s = strings.TrimSpace(strings.ToLower(s))
	if c, ok := namedColors[s]; ok {
		return c, true
	}
	if strings.HasPrefix(s, "#") {
		return parseHexColor(s[1:])
	}
	if strings.HasPrefix(s, "rgb(") && strings.HasSuffix(s, ")") {
		return parseRGBFunc(s[4 : len(s)-1])
	}
	if strings.HasPrefix(s, "rgba(") && strings.HasSuffix(s, ")") {
		return parseRGBFunc(s[5 : len(s)-1])
	}
	return Color{}, false
}

func parseHexColor(s string) (Color, bool) {
	switch len(s) {
	case 3, 4:
		var parts [4]uint8
		parts[3] = 255
		for i := 0; i < len(s); i++ {
			v, ok := fromHex(s[i])
			if !ok {
				return Color{}, false
			}
			parts[i] = v<<4 | v
		}
		return Color{parts[0], parts[1], parts[2], parts[3]}, true
	case 6, 8:
		var parts [4]uint8
		parts[3] = 255
		for i := 0; i < len(s); i += 2 {
			hi, ok1 := fromHex(s[i])
			lo, ok2 := fromHex(s[i+1])
			if !ok1 || !ok2 {
				return Color{}, false
			}
			parts[i/2] = hi<<4 | lo
		}
		return Color{parts[0], parts[1], parts[2], parts[3]}, true
	default:
		return Color{}, false
	}
}

func fromHex(b byte) (uint8, bool) {
	switch {
	case b >= '0' && b <= '9':
		return b - '0', true
	case b >= 'a' && b <= 'f':
		return b - 'a' + 10, true
	case b >= 'A' && b <= 'F':
		return b - 'A' + 10, true
	default:
		return 0, false
	}
}

func parseRGBFunc(s string) (Color, bool) {
	fields := strings.Split(s, ",")
	if len(fields) != 3 && len(fields) != 4 {
		return Color{}, false
	}
	var rgba [4]uint8
	rgba[3] = 255
	for i := 0; i < 3; i++ {
		v, ok := parseColorByte(strings.TrimSpace(fields[i]))
		if !ok {
			return Color{}, false
		}
		rgba[i] = v
	}
	if len(fields) == 4 {
		alpha := strings.TrimSpace(fields[3])
		if strings.HasSuffix(alpha, "%") {
			v, err := strconv.ParseFloat(strings.TrimSuffix(alpha, "%"), 64)
			if err != nil {
				return Color{}, false
			}
			rgba[3] = uint8(math.Round(clamp(v, 0, 100) * 2.55))
		} else {
			v, err := strconv.ParseFloat(alpha, 64)
			if err != nil {
				return Color{}, false
			}
			rgba[3] = uint8(math.Round(clamp(v, 0, 1) * 255))
		}
	}
	return Color{rgba[0], rgba[1], rgba[2], rgba[3]}, true
}

func parseColorByte(s string) (uint8, bool) {
	if strings.HasSuffix(s, "%") {
		v, err := strconv.ParseFloat(strings.TrimSuffix(s, "%"), 64)
		if err != nil {
			return 0, false
		}
		return uint8(math.Round(clamp(v, 0, 100) * 2.55)), true
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, false
	}
	return uint8(math.Round(clamp(v, 0, 255))), true
}

func clamp(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
