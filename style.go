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
	Display              string
	Color                Color
	BackgroundColor      Color
	BackgroundImage      string
	BackgroundRepeat     string
	BackgroundAttachment string
	BackgroundPositionX  string
	BackgroundPositionY  string
	FontFamily           string
	FontSize             float64
	FontSizeAdjust       string
	LineHeight           float64
	FontWeight           int
	Italic               bool
	Underline            bool
	TextAlign            string
	Direction            string
	ListStyleType        string

	Width          length
	Height         length
	MinWidth       length
	MinHeight      length
	MaxWidth       length
	MaxHeight      length
	Margin         edgeLengths
	Padding        edgeLengths
	Border         edgeLengths
	BorderColor    Color
	BorderColors   edgeColors
	BorderStyle    edgeStrings
	BorderRadius   edgeLengths
	BorderCollapse string
	BorderSpacingX length
	BorderSpacingY length
	BoxSizing      string
	CaptionSide    string
	EmptyCells     string

	Position         string
	Top              length
	Right            length
	Bottom           length
	Left             length
	Float            string
	Clear            string
	Clip             string
	Content          string
	CounterIncrement string
	CounterReset     string
	Cursor           string
}

type edgeLengths struct {
	Top    length
	Right  length
	Bottom length
	Left   length
}

type edgeStrings struct {
	Top    string
	Right  string
	Bottom string
	Left   string
}

type edgeColors struct {
	Top    Color
	Right  Color
	Bottom Color
	Left   Color
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
		Display:              displayInline,
		Color:                Color{0x20, 0x24, 0x28, 255},
		BackgroundColor:      Transparent,
		BackgroundRepeat:     "repeat",
		BackgroundAttachment: "scroll",
		BackgroundPositionX:  "0%",
		BackgroundPositionY:  "0%",
		FontFamily:           "",
		FontSize:             16,
		LineHeight:           20,
		FontWeight:           400,
		TextAlign:            "left",
		Direction:            "ltr",
		ListStyleType:        "disc",
		BorderColor:          Color{0, 0, 0, 255},
		BorderColors: edgeColors{
			Top:    Color{0, 0, 0, 255},
			Right:  Color{0, 0, 0, 255},
			Bottom: Color{0, 0, 0, 255},
			Left:   Color{0, 0, 0, 255},
		},
		BorderStyle: edgeStrings{Top: "none", Right: "none", Bottom: "none", Left: "none"},
		BoxSizing:   "content-box",
		CaptionSide: "top",
		EmptyCells:  "show",
		Position:    "static",
		Clear:       "none",
		Float:       "none",
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
	s.Direction = parent.Direction
	s.ListStyleType = parent.ListStyleType
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
		case "block", "inline", "inline-block", "inline-table", "list-item", "none",
			"table", "table-row", "table-cell", "table-caption", "table-column",
			"table-column-group", "table-footer-group", "table-header-group", "table-row-group":
			s.Display = lower
		}
	case "color":
		if c, ok := ParseColor(value); ok {
			s.Color = c
		}
	case "background":
		s.applyBackground(value)
	case "background-color":
		if c, ok := ParseColor(firstBackgroundToken(value)); ok {
			s.BackgroundColor = c
		}
	case "background-image":
		s.BackgroundImage = parseImageValue(value)
	case "background-repeat":
		switch lower {
		case "repeat", "repeat-x", "repeat-y", "no-repeat", "space", "round":
			s.BackgroundRepeat = lower
		}
	case "background-attachment":
		switch lower {
		case "scroll", "fixed", "local":
			s.BackgroundAttachment = lower
		}
	case "background-position":
		s.applyBackgroundPosition(value)
	case "background-position-x":
		s.BackgroundPositionX = value
	case "background-position-y":
		s.BackgroundPositionY = value
	case "font-family":
		if family := cleanFontFamilyList(value); family != "" {
			s.FontFamily = family
		}
	case "font-size":
		if v, ok := parseFontSize(value, s.FontSize); ok && v > 0 {
			s.FontSize = v
			s.LineHeight = v * 1.25
		}
	case "font":
		s.applyFont(value)
	case "font-size-adjust":
		if lower == "none" || lower == "inherit" {
			s.FontSizeAdjust = lower
		} else if _, err := strconv.ParseFloat(lower, 64); err == nil {
			s.FontSizeAdjust = lower
		}
	case "line-height":
		if lower == "normal" {
			s.LineHeight = s.FontSize * 1.25
			return
		}
		if isUnitlessNumber(value) {
			if f, err := strconv.ParseFloat(value, 64); err == nil && f > 0 {
				s.LineHeight = s.FontSize * f
			}
			return
		}
		if l := parseLength(value); l.set {
			if l.unit == unitAuto {
				return
			}
			if v, ok := l.resolve(s.FontSize, s.FontSize); ok && v > 0 {
				s.LineHeight = v
			}
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
		if lower == "left" || lower == "right" || lower == "center" || lower == "justify" {
			s.TextAlign = lower
		}
	case "direction":
		if lower == "ltr" || lower == "rtl" {
			s.Direction = lower
		}
	case "list-style", "list-style-type":
		for _, part := range strings.Fields(lower) {
			switch part {
			case "none", "disc", "circle", "square", "decimal":
				s.ListStyleType = part
			}
		}
	case "width":
		s.Width = parseLength(value)
	case "height":
		s.Height = parseLength(value)
	case "min-width":
		s.MinWidth = parseLength(value)
	case "min-height":
		s.MinHeight = parseLength(value)
	case "max-width":
		s.MaxWidth = parseLength(value)
	case "max-height":
		s.MaxHeight = parseLength(value)
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
		s.applyBorderColor(value)
	case "border-style":
		s.BorderStyle = parseBoxStrings(value, isBorderStyle)
	case "border-top":
		s.applyBorderSide("top", value)
	case "border-right":
		s.applyBorderSide("right", value)
	case "border-bottom":
		s.applyBorderSide("bottom", value)
	case "border-left":
		s.applyBorderSide("left", value)
	case "border-top-width":
		s.Border.Top = parseBorderWidth(value)
	case "border-right-width":
		s.Border.Right = parseBorderWidth(value)
	case "border-bottom-width":
		s.Border.Bottom = parseBorderWidth(value)
	case "border-left-width":
		s.Border.Left = parseBorderWidth(value)
	case "border-top-color":
		if c, ok := ParseColor(value); ok {
			s.BorderColor = c
			s.BorderColors.Top = c
		}
	case "border-right-color":
		if c, ok := ParseColor(value); ok {
			s.BorderColor = c
			s.BorderColors.Right = c
		}
	case "border-bottom-color":
		if c, ok := ParseColor(value); ok {
			s.BorderColor = c
			s.BorderColors.Bottom = c
		}
	case "border-left-color":
		if c, ok := ParseColor(value); ok {
			s.BorderColor = c
			s.BorderColors.Left = c
		}
	case "border-top-style":
		if isBorderStyle(lower) {
			s.BorderStyle.Top = lower
		}
	case "border-right-style":
		if isBorderStyle(lower) {
			s.BorderStyle.Right = lower
		}
	case "border-bottom-style":
		if isBorderStyle(lower) {
			s.BorderStyle.Bottom = lower
		}
	case "border-left-style":
		if isBorderStyle(lower) {
			s.BorderStyle.Left = lower
		}
	case "border-radius":
		s.BorderRadius = parseBoxLengths(strings.Split(value, "/")[0])
	case "border-collapse":
		if lower == "collapse" || lower == "separate" {
			s.BorderCollapse = lower
		}
	case "border-spacing":
		parts := strings.Fields(value)
		if len(parts) > 0 {
			s.BorderSpacingX = parseLength(parts[0])
			s.BorderSpacingY = s.BorderSpacingX
		}
		if len(parts) > 1 {
			s.BorderSpacingY = parseLength(parts[1])
		}
	case "box-sizing":
		if lower == "content-box" || lower == "border-box" {
			s.BoxSizing = lower
		}
	case "caption-side":
		if lower == "top" || lower == "bottom" {
			s.CaptionSide = lower
		}
	case "empty-cells":
		if lower == "show" || lower == "hide" {
			s.EmptyCells = lower
		}
	case "position":
		switch lower {
		case "static", "relative", "absolute", "fixed", "sticky":
			s.Position = lower
		}
	case "top":
		s.Top = parseLength(value)
	case "right":
		s.Right = parseLength(value)
	case "bottom":
		s.Bottom = parseLength(value)
	case "left":
		s.Left = parseLength(value)
	case "float":
		if lower == "left" || lower == "right" || lower == "none" {
			s.Float = lower
		}
	case "clear":
		if lower == "left" || lower == "right" || lower == "both" || lower == "none" {
			s.Clear = lower
		}
	case "clip":
		s.Clip = value
	case "content":
		s.Content = strings.Trim(value, `"'`)
	case "counter-increment":
		s.CounterIncrement = value
	case "counter-reset":
		s.CounterReset = value
	case "cursor":
		s.Cursor = lower
	}
}

func (s *Style) applyBackground(value string) {
	for _, part := range strings.Fields(value) {
		lower := strings.ToLower(part)
		if c, ok := ParseColor(part); ok {
			s.BackgroundColor = c
			continue
		}
		if img := parseImageValue(part); img != "" {
			s.BackgroundImage = img
			continue
		}
		switch lower {
		case "repeat", "repeat-x", "repeat-y", "no-repeat", "space", "round":
			s.BackgroundRepeat = lower
		case "scroll", "fixed", "local":
			s.BackgroundAttachment = lower
		case "left", "right", "center":
			s.BackgroundPositionX = lower
		case "top", "bottom":
			s.BackgroundPositionY = lower
		default:
			if parseLength(part).set {
				if s.BackgroundPositionX == "0%" {
					s.BackgroundPositionX = part
				} else {
					s.BackgroundPositionY = part
				}
			}
		}
	}
}

func (s *Style) applyBorder(value string) {
	for _, part := range strings.Fields(value) {
		if l := parseBorderWidth(part); l.set && l.unit != unitAuto {
			s.Border = edgeLengths{Top: l, Right: l, Bottom: l, Left: l}
			continue
		}
		if c, ok := ParseColor(part); ok {
			s.BorderColor = c
			s.BorderColors = edgeColors{Top: c, Right: c, Bottom: c, Left: c}
			continue
		}
		if lower := strings.ToLower(part); isBorderStyle(lower) {
			s.BorderStyle = edgeStrings{Top: lower, Right: lower, Bottom: lower, Left: lower}
		}
	}
}

func (s *Style) applyBorderSide(side, value string) {
	for _, part := range strings.Fields(value) {
		lower := strings.ToLower(part)
		if l := parseBorderWidth(part); l.set && l.unit != unitAuto {
			s.setBorderWidth(side, l)
			continue
		}
		if c, ok := ParseColor(part); ok {
			s.BorderColor = c
			s.setBorderColor(side, c)
			continue
		}
		if isBorderStyle(lower) {
			s.setBorderStyle(side, lower)
		}
	}
}

func (s *Style) setBorderWidth(side string, l length) {
	switch side {
	case "top":
		s.Border.Top = l
	case "right":
		s.Border.Right = l
	case "bottom":
		s.Border.Bottom = l
	case "left":
		s.Border.Left = l
	}
}

func (s *Style) setBorderStyle(side, value string) {
	switch side {
	case "top":
		s.BorderStyle.Top = value
	case "right":
		s.BorderStyle.Right = value
	case "bottom":
		s.BorderStyle.Bottom = value
	case "left":
		s.BorderStyle.Left = value
	}
}

func (s *Style) setBorderColor(side string, c Color) {
	switch side {
	case "top":
		s.BorderColors.Top = c
	case "right":
		s.BorderColors.Right = c
	case "bottom":
		s.BorderColors.Bottom = c
	case "left":
		s.BorderColors.Left = c
	}
}

func (s *Style) applyBorderColor(value string) {
	colors := parseBoxColors(value)
	if colors == nil {
		return
	}
	s.BorderColor = colors.Top
	s.BorderColors = *colors
}

func (s *Style) applyBackgroundPosition(value string) {
	parts := strings.Fields(value)
	if len(parts) == 0 {
		return
	}
	s.BackgroundPositionX = parts[0]
	if len(parts) > 1 {
		s.BackgroundPositionY = parts[1]
	}
}

func (s *Style) applyFont(value string) {
	parts := strings.Fields(value)
	if len(parts) == 0 {
		return
	}
	familyStart := -1
	for i, part := range parts {
		lower := strings.ToLower(strings.Trim(part, ","))
		if lower == "italic" || lower == "oblique" {
			s.Italic = true
			continue
		}
		switch lower {
		case "normal":
			continue
		case "bold", "bolder":
			s.FontWeight = 700
			continue
		case "lighter":
			s.FontWeight = 400
			continue
		}
		if v, err := strconv.Atoi(lower); err == nil {
			s.FontWeight = v
			continue
		}
		sizePart := part
		lineHeightPart := ""
		if before, after, ok := strings.Cut(part, "/"); ok {
			sizePart = before
			lineHeightPart = after
		}
		if v, ok := parseFontSize(sizePart, s.FontSize); ok {
			s.FontSize = v
			if lineHeightPart != "" {
				s.applyProperty("line-height", lineHeightPart)
			} else {
				s.LineHeight = v * 1.25
			}
			familyStart = i + 1
			break
		}
	}
	if familyStart >= 0 && familyStart < len(parts) {
		if family := cleanFontFamilyList(strings.Join(parts[familyStart:], " ")); family != "" {
			s.FontFamily = family
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
	switch s {
	case "thin":
		return length{value: 1, unit: unitPx, set: true}
	case "medium":
		return length{value: 3, unit: unitPx, set: true}
	case "thick":
		return length{value: 5, unit: unitPx, set: true}
	}
	unit := unitPx
	num := s
	switch {
	case strings.HasSuffix(s, "px"):
		num = strings.TrimSuffix(s, "px")
	case strings.HasSuffix(s, "pt"):
		num = strings.TrimSuffix(s, "pt")
		unit = unitPx
	case strings.HasSuffix(s, "pc"):
		num = strings.TrimSuffix(s, "pc")
		unit = unitPx
	case strings.HasSuffix(s, "in"):
		num = strings.TrimSuffix(s, "in")
		unit = unitPx
	case strings.HasSuffix(s, "cm"):
		num = strings.TrimSuffix(s, "cm")
		unit = unitPx
	case strings.HasSuffix(s, "mm"):
		num = strings.TrimSuffix(s, "mm")
		unit = unitPx
	case strings.HasSuffix(s, "%"):
		num = strings.TrimSuffix(s, "%")
		unit = unitPercent
	case strings.HasSuffix(s, "rem"):
		num = strings.TrimSuffix(s, "rem")
		unit = unitEm
	case strings.HasSuffix(s, "em"):
		num = strings.TrimSuffix(s, "em")
		unit = unitEm
	case strings.HasSuffix(s, "ex"):
		num = strings.TrimSuffix(s, "ex")
		unit = unitEm
	}
	v, err := strconv.ParseFloat(strings.TrimSpace(num), 64)
	if err != nil {
		return length{}
	}
	if strings.HasSuffix(s, "pt") {
		v *= 96.0 / 72.0
	} else if strings.HasSuffix(s, "pc") {
		v *= 16
	} else if strings.HasSuffix(s, "in") {
		v *= 96
	} else if strings.HasSuffix(s, "cm") {
		v *= 96.0 / 2.54
	} else if strings.HasSuffix(s, "mm") {
		v *= 96.0 / 25.4
	} else if strings.HasSuffix(s, "ex") {
		v *= 0.5
	}
	return length{value: v, unit: unit, set: true}
}

func parseBorderWidth(s string) length {
	return parseLength(s)
}

func isUnitlessNumber(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	_, err := strconv.ParseFloat(value, 64)
	return err == nil
}

func parseFontSize(value string, current float64) (float64, bool) {
	lower := strings.ToLower(strings.TrimSpace(value))
	switch lower {
	case "xx-small":
		return 9, true
	case "x-small":
		return 10, true
	case "small":
		return 13, true
	case "medium":
		return 16, true
	case "large":
		return 18, true
	case "x-large":
		return 24, true
	case "xx-large":
		return 32, true
	case "smaller":
		return current / 1.2, true
	case "larger":
		return current * 1.2, true
	}
	if v, ok := parseLength(value).resolve(current, current); ok {
		return v, true
	}
	return 0, false
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

func parseImageValue(value string) string {
	value = strings.TrimSpace(value)
	lower := strings.ToLower(value)
	if lower == "none" {
		return ""
	}
	if strings.HasPrefix(lower, "url(") && strings.HasSuffix(value, ")") {
		return strings.Trim(strings.TrimSpace(value[4:len(value)-1]), `"'`)
	}
	return ""
}

func isBorderStyle(value string) bool {
	switch value {
	case "none", "hidden", "dotted", "dashed", "solid", "double", "groove", "ridge", "inset", "outset":
		return true
	default:
		return false
	}
}

func parseBoxStrings(s string, valid func(string) bool) edgeStrings {
	parts := strings.Fields(s)
	vals := make([]string, 0, 4)
	for _, part := range parts {
		part = strings.ToLower(part)
		if valid(part) {
			vals = append(vals, part)
		}
		if len(vals) == 4 {
			break
		}
	}
	switch len(vals) {
	case 1:
		return edgeStrings{Top: vals[0], Right: vals[0], Bottom: vals[0], Left: vals[0]}
	case 2:
		return edgeStrings{Top: vals[0], Right: vals[1], Bottom: vals[0], Left: vals[1]}
	case 3:
		return edgeStrings{Top: vals[0], Right: vals[1], Bottom: vals[2], Left: vals[1]}
	case 4:
		return edgeStrings{Top: vals[0], Right: vals[1], Bottom: vals[2], Left: vals[3]}
	default:
		return edgeStrings{}
	}
}

func parseBoxColors(s string) *edgeColors {
	parts := strings.Fields(s)
	vals := make([]Color, 0, 4)
	for _, part := range parts {
		if c, ok := ParseColor(part); ok {
			vals = append(vals, c)
		}
		if len(vals) == 4 {
			break
		}
	}
	switch len(vals) {
	case 1:
		return &edgeColors{Top: vals[0], Right: vals[0], Bottom: vals[0], Left: vals[0]}
	case 2:
		return &edgeColors{Top: vals[0], Right: vals[1], Bottom: vals[0], Left: vals[1]}
	case 3:
		return &edgeColors{Top: vals[0], Right: vals[1], Bottom: vals[2], Left: vals[1]}
	case 4:
		return &edgeColors{Top: vals[0], Right: vals[1], Bottom: vals[2], Left: vals[3]}
	default:
		return nil
	}
}
