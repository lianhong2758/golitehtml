package golitehtml

import (
	"strconv"
	"strings"
)

const (
	displayBlock       = "block"
	displayInline      = "inline"
	displayInlineBlock = "inline-block"
	displayNone        = "none"
	displayListItem    = "list-item"
)

type lengthUnit uint8

const (
	unitPx lengthUnit = iota
	unitPercent
	unitEm
	unitAuto
)

type length struct {
	value float64
	unit  lengthUnit
	set   bool
}

func (l length) resolve(base, font float64) (float64, bool) {
	if !l.set || l.unit == unitAuto {
		return 0, false
	}
	switch l.unit {
	case unitPercent:
		return base * l.value / 100, true
	case unitEm:
		return font * l.value, true
	default:
		return l.value, true
	}
}

// Style 是当前渲染器理解的计算后样式子集。
type Style struct {
	Display         string
	Color           Color
	BackgroundColor Color
	FontFamily      string
	FontSize        float64
	LineHeight      float64
	FontWeight      int
	Italic          bool
	Underline       bool
	TextAlign       string

	Width       length
	Height      length
	Margin      edgeLengths
	Padding     edgeLengths
	Border      edgeLengths
	BorderColor Color
}

type edgeLengths struct {
	Top    length
	Right  length
	Bottom length
	Left   length
}

func (e edgeLengths) resolve(base, font float64) Insets {
	top, _ := e.Top.resolve(base, font)
	right, _ := e.Right.resolve(base, font)
	bottom, _ := e.Bottom.resolve(base, font)
	left, _ := e.Left.resolve(base, font)
	return Insets{Top: top, Right: right, Bottom: bottom, Left: left}
}

func defaultStyle() Style {
	return Style{
		Display:         displayInline,
		Color:           Color{0x20, 0x24, 0x28, 255},
		BackgroundColor: Transparent,
		FontFamily:      "sans-serif",
		FontSize:        16,
		LineHeight:      20,
		FontWeight:      400,
		TextAlign:       "left",
		BorderColor:     Color{0, 0, 0, 255},
	}
}

func inheritStyle(parent Style) Style {
	s := defaultStyle()
	s.Color = parent.Color
	s.FontFamily = parent.FontFamily
	s.FontSize = parent.FontSize
	s.LineHeight = parent.LineHeight
	s.FontWeight = parent.FontWeight
	s.Italic = parent.Italic
	s.Underline = parent.Underline
	s.TextAlign = parent.TextAlign
	return s
}

func (s Style) textStyle() TextStyle {
	line := s.LineHeight
	if line <= 0 {
		line = s.FontSize * 1.25
	}
	return TextStyle{
		Family:     s.FontFamily,
		Size:       s.FontSize,
		LineHeight: line,
		Weight:     s.FontWeight,
		Italic:     s.Italic,
		Color:      s.Color,
		Underline:  s.Underline,
	}
}

func (s *Style) applyProperty(name, value string) {
	name = strings.ToLower(strings.TrimSpace(name))
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	lower := strings.ToLower(value)
	switch name {
	case "display":
		switch lower {
		case "block", "inline", "inline-block", "none", "list-item":
			s.Display = lower
		}
	case "color":
		if c, ok := ParseColor(value); ok {
			s.Color = c
		}
	case "background", "background-color":
		if c, ok := ParseColor(firstBackgroundToken(value)); ok {
			s.BackgroundColor = c
		}
	case "font-family":
		s.FontFamily = strings.Trim(value, `"'`)
	case "font-size":
		if v, ok := parseLength(value).resolve(s.FontSize, s.FontSize); ok && v > 0 {
			s.FontSize = v
			if s.LineHeight <= 0 {
				s.LineHeight = v * 1.25
			}
		}
	case "line-height":
		if lower == "normal" {
			s.LineHeight = s.FontSize * 1.25
			return
		}
		if l := parseLength(value); l.set {
			if l.unit == unitAuto {
				return
			}
			if v, ok := l.resolve(s.FontSize, s.FontSize); ok && v > 0 {
				s.LineHeight = v
			}
		} else if f, err := strconv.ParseFloat(value, 64); err == nil && f > 0 {
			s.LineHeight = s.FontSize * f
		}
	case "font-weight":
		switch lower {
		case "bold", "bolder":
			s.FontWeight = 700
		case "normal", "lighter":
			s.FontWeight = 400
		default:
			if v, err := strconv.Atoi(lower); err == nil {
				s.FontWeight = v
			}
		}
	case "font-style":
		s.Italic = lower == "italic" || lower == "oblique"
	case "text-decoration", "text-decoration-line":
		s.Underline = strings.Contains(lower, "underline")
	case "text-align":
		if lower == "left" || lower == "right" || lower == "center" {
			s.TextAlign = lower
		}
	case "width":
		s.Width = parseLength(value)
	case "height":
		s.Height = parseLength(value)
	case "margin":
		s.Margin = parseBoxLengths(value)
	case "margin-top":
		s.Margin.Top = parseLength(value)
	case "margin-right":
		s.Margin.Right = parseLength(value)
	case "margin-bottom":
		s.Margin.Bottom = parseLength(value)
	case "margin-left":
		s.Margin.Left = parseLength(value)
	case "padding":
		s.Padding = parseBoxLengths(value)
	case "padding-top":
		s.Padding.Top = parseLength(value)
	case "padding-right":
		s.Padding.Right = parseLength(value)
	case "padding-bottom":
		s.Padding.Bottom = parseLength(value)
	case "padding-left":
		s.Padding.Left = parseLength(value)
	case "border":
		s.applyBorder(value)
	case "border-width":
		s.Border = parseBoxLengths(value)
	case "border-color":
		if c, ok := ParseColor(value); ok {
			s.BorderColor = c
		}
	}
}

func (s *Style) applyBorder(value string) {
	for _, part := range strings.Fields(value) {
		if l := parseLength(part); l.set && l.unit != unitAuto {
			s.Border = edgeLengths{Top: l, Right: l, Bottom: l, Left: l}
			continue
		}
		if c, ok := ParseColor(part); ok {
			s.BorderColor = c
		}
	}
}

func parseLength(s string) length {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return length{}
	}
	if s == "auto" {
		return length{unit: unitAuto, set: true}
	}
	unit := unitPx
	num := s
	switch {
	case strings.HasSuffix(s, "px"):
		num = strings.TrimSuffix(s, "px")
	case strings.HasSuffix(s, "pt"):
		num = strings.TrimSuffix(s, "pt")
		unit = unitPx
	case strings.HasSuffix(s, "%"):
		num = strings.TrimSuffix(s, "%")
		unit = unitPercent
	case strings.HasSuffix(s, "em"):
		num = strings.TrimSuffix(s, "em")
		unit = unitEm
	}
	v, err := strconv.ParseFloat(strings.TrimSpace(num), 64)
	if err != nil {
		return length{}
	}
	if strings.HasSuffix(s, "pt") {
		v *= 96.0 / 72.0
	}
	return length{value: v, unit: unit, set: true}
}

func parseBoxLengths(s string) edgeLengths {
	parts := strings.Fields(s)
	if len(parts) == 0 {
		return edgeLengths{}
	}
	vals := make([]length, 0, 4)
	for _, part := range parts {
		vals = append(vals, parseLength(part))
		if len(vals) == 4 {
			break
		}
	}
	switch len(vals) {
	case 1:
		return edgeLengths{Top: vals[0], Right: vals[0], Bottom: vals[0], Left: vals[0]}
	case 2:
		return edgeLengths{Top: vals[0], Right: vals[1], Bottom: vals[0], Left: vals[1]}
	case 3:
		return edgeLengths{Top: vals[0], Right: vals[1], Bottom: vals[2], Left: vals[1]}
	default:
		return edgeLengths{Top: vals[0], Right: vals[1], Bottom: vals[2], Left: vals[3]}
	}
}

func firstBackgroundToken(value string) string {
	for _, part := range strings.Fields(value) {
		if _, ok := ParseColor(part); ok {
			return part
		}
	}
	return value
}
