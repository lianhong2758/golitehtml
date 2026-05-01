package golitehtml

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"unicode"

	"github.com/FloatTech/gg"
	builtfont "github.com/lianhong2758/golitehtml/font"
	xfont "golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"
)

type fontFace = xfont.Face

type fontSynthesis struct {
	Bold   bool
	Italic bool
}

type resolvedFont struct {
	Entry     *fontEntry
	Face      fontFace
	Size      float64
	Synthesis fontSynthesis
}

type resolvedTextRun struct {
	Text string
	Font resolvedFont
}

type fontEntry struct {
	font      *opentype.Font
	family    string
	subfamily string
	aliases   []string
	weight    int
	italic    bool
	order     int
	index     int
	provided  bool
	path      string
}

type fontManager struct {
	entries      []*fontEntry
	defaultEntry *fontEntry
	faces        map[string]resolvedFont
	glyphs       map[string]bool
}

var systemFontCache struct {
	once    sync.Once
	entries []*fontEntry
}

// newFontManager 解析调用方字体和系统字体，并按 font-family/字号缓存 font.Face。
func newFontManager(fonts [][]byte) (*fontManager, error) {
	if len(fonts) == 0 {
		fonts = [][]byte{builtfont.TTF}
	}

	m := &fontManager{
		faces:  make(map[string]resolvedFont, 64),
		glyphs: make(map[string]bool, 256),
	}
	for _, data := range fonts {
		if len(data) == 0 {
			continue
		}
		entries, err := newFontEntriesFromData(data, len(m.entries), true)
		if err != nil {
			return nil, fmt.Errorf("parse font: %w", err)
		}
		for _, entry := range entries {
			m.addEntry(entry)
			if m.defaultEntry == nil {
				m.defaultEntry = entry
			}
		}
	}
	if m.defaultEntry == nil {
		entries, err := newFontEntriesFromData(builtfont.TTF, 0, true)
		if err != nil {
			return nil, fmt.Errorf("parse default font: %w", err)
		}
		for _, entry := range entries {
			m.addEntry(entry)
			if m.defaultEntry == nil {
				m.defaultEntry = entry
			}
		}
	}

	m.scanSystemFonts()
	return m, nil
}

func (m *fontManager) addEntry(entry *fontEntry) {
	entry.order = len(m.entries)
	m.entries = append(m.entries, entry)
}

// MeasureText 使用匹配到的真实字体度量文本宽度，保证布局和最终绘制尽量一致。
func (m *fontManager) MeasureText(text string, style TextStyle) Size {
	if text == "" {
		return Size{}
	}
	runs, err := m.textRuns(text, style)
	if err != nil {
		return DefaultMeasurer{}.MeasureText(text, style)
	}
	dc := gg.NewContext(8, 8)
	width := 0.0
	for _, run := range runs {
		dc.SetFontFace(run.Font.Face)
		runWidth, _ := dc.MeasureString(run.Text)
		if run.Font.Synthesis.Bold {
			runWidth += 0.8
		}
		if run.Font.Synthesis.Italic {
			runWidth *= 1.02
		}
		width += runWidth
	}
	lineHeight := style.LineHeight
	if lineHeight <= 0 {
		size := style.Size
		if size <= 0 {
			size = 16
		}
		lineHeight = size * 1.25
	}
	return Size{W: width, H: lineHeight}
}

func (m *fontManager) textRuns(text string, style TextStyle) ([]resolvedTextRun, error) {
	entry, synth := m.match(style)
	if entry == nil {
		entry = m.defaultEntry
	}
	if entry == nil {
		return nil, fmt.Errorf("no font available")
	}
	var runs []resolvedTextRun
	var b strings.Builder
	var current *fontEntry
	var currentSynth fontSynthesis
	flush := func() error {
		if b.Len() == 0 || current == nil {
			return nil
		}
		face, err := m.faceForEntry(current, style, currentSynth)
		if err != nil {
			return err
		}
		runs = append(runs, resolvedTextRun{Text: b.String(), Font: face})
		b.Reset()
		return nil
	}
	for _, r := range text {
		runEntry, runSynth := entry, synth
		if !isFontControlRune(r) && !m.entryHasGlyph(entry, r) {
			if fallback := m.fallbackEntry(r); fallback != nil {
				runEntry = fallback
				runSynth = fontSynthesis{
					Bold:   normalizedWeight(style.Weight) >= 600 && fallback.weight < 600,
					Italic: style.Italic && !fallback.italic,
				}
			}
		}
		if current != nil && (current != runEntry || currentSynth != runSynth) {
			if err := flush(); err != nil {
				return nil, err
			}
		}
		current = runEntry
		currentSynth = runSynth
		b.WriteRune(r)
	}
	if err := flush(); err != nil {
		return nil, err
	}
	return runs, nil
}

func (m *fontManager) faceForEntry(entry *fontEntry, style TextStyle, synth fontSynthesis) (resolvedFont, error) {
	size := style.Size
	if size <= 0 {
		size = 16
	}
	key := strings.Join([]string{
		strconv.Itoa(entry.order),
		strconv.FormatFloat(size, 'f', 2, 64),
		strconv.Itoa(style.Weight),
		strconv.FormatBool(style.Italic),
	}, "|")
	if cached, ok := m.faces[key]; ok {
		return cached, nil
	}
	font, err := entry.loadFont()
	if err != nil {
		return resolvedFont{}, err
	}
	face, err := opentype.NewFace(font, &opentype.FaceOptions{
		Size:    size,
		DPI:     72,
		Hinting: xfont.HintingFull,
	})
	if err != nil {
		return resolvedFont{}, err
	}
	resolved := resolvedFont{Entry: entry, Face: face, Size: size, Synthesis: synth}
	m.faces[key] = resolved
	return resolved, nil
}

func (m *fontManager) match(style TextStyle) (*fontEntry, fontSynthesis) {
	reqWeight := normalizedWeight(style.Weight)
	families := parseFontFamilies(style.Family)
	for _, family := range families {
		for _, candidate := range expandGenericFamily(family) {
			if entry := m.bestMatch(candidate, reqWeight, style.Italic); entry != nil {
				return entry, fontSynthesis{
					Bold:   reqWeight >= 600 && entry.weight < 600,
					Italic: style.Italic && !entry.italic,
				}
			}
		}
	}
	entry := m.defaultEntry
	return entry, fontSynthesis{
		Bold:   reqWeight >= 600 && entry != nil && entry.weight < 600,
		Italic: style.Italic && entry != nil && !entry.italic,
	}
}

func (m *fontManager) bestMatch(family string, reqWeight int, reqItalic bool) *fontEntry {
	name := normalizeFontName(family)
	if name == "" {
		return nil
	}
	var best *fontEntry
	bestScore := int(^uint(0) >> 1)
	for _, entry := range m.entries {
		if !entry.matches(name) {
			continue
		}
		score := absInt(entry.weight - reqWeight)
		if reqItalic && !entry.italic {
			score += 150
		} else if !reqItalic && entry.italic {
			score += 1000
		}
		if entry.provided {
			score -= 100
		}
		if score < bestScore || (score == bestScore && best != nil && entry.order < best.order) {
			best = entry
			bestScore = score
		}
	}
	return best
}

func (e *fontEntry) matches(name string) bool {
	for _, alias := range e.aliases {
		if alias == name {
			return true
		}
	}
	return false
}

func (m *fontManager) fallbackEntry(r rune) *fontEntry {
	if m.defaultEntry != nil && m.entryHasGlyph(m.defaultEntry, r) {
		return m.defaultEntry
	}
	for _, entry := range m.entries {
		if entry != m.defaultEntry && m.entryHasGlyph(entry, r) {
			return entry
		}
	}
	return nil
}

func (m *fontManager) entryHasGlyph(entry *fontEntry, r rune) bool {
	if entry == nil {
		return false
	}
	if isFontControlRune(r) {
		return true
	}
	key := strconv.Itoa(entry.order) + "|" + string(r)
	if ok, cached := m.glyphs[key]; cached {
		return ok
	}
	font, err := entry.loadFont()
	if err != nil {
		m.glyphs[key] = false
		return false
	}
	idx, err := font.GlyphIndex(nil, r)
	ok := err == nil && idx != 0
	m.glyphs[key] = ok
	return ok
}

func isFontControlRune(r rune) bool {
	return r == '\n' || r == '\r' || r == '\t'
}

func newFontEntryFromData(data []byte, order int, provided bool) (*fontEntry, error) {
	entries, err := newFontEntriesFromData(data, order, provided)
	if err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("empty font collection")
	}
	return entries[0], nil
}

func newFontEntriesFromData(data []byte, order int, provided bool) ([]*fontEntry, error) {
	collection, err := sfnt.ParseCollection(data)
	if err != nil {
		return nil, err
	}
	entries := make([]*fontEntry, 0, collection.NumFonts())
	for i := 0; i < collection.NumFonts(); i++ {
		ttf, err := collection.Font(i)
		if err != nil {
			continue
		}
		entries = append(entries, describeFont(ttf, "", i, order+len(entries), provided))
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("empty font collection")
	}
	return entries, nil
}

func newFontEntriesFromPath(path string, order int) ([]*fontEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	collection, err := sfnt.ParseCollection(data)
	if err != nil {
		return nil, err
	}
	entries := make([]*fontEntry, 0, collection.NumFonts())
	for i := 0; i < collection.NumFonts(); i++ {
		ttf, err := collection.Font(i)
		if err != nil {
			continue
		}
		entry := describeFont(ttf, path, i, order+len(entries), false)
		entry.font = nil
		entries = append(entries, entry)
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("empty font collection")
	}
	return entries, nil
}

func (e *fontEntry) loadFont() (*opentype.Font, error) {
	if e.font != nil {
		return e.font, nil
	}
	if e.path == "" {
		return nil, fmt.Errorf("font %q has no source", e.family)
	}
	data, err := os.ReadFile(e.path)
	if err != nil {
		return nil, err
	}
	collection, err := sfnt.ParseCollection(data)
	if err != nil {
		return nil, err
	}
	font, err := collection.Font(e.index)
	if err != nil {
		return nil, err
	}
	e.font = font
	return e.font, nil
}

func describeFont(ttf *opentype.Font, path string, index, order int, provided bool) *fontEntry {
	family := firstFontName(ttf, sfnt.NameIDTypographicFamily, sfnt.NameIDFamily, sfnt.NameIDWWSFamily)
	subfamily := firstFontName(ttf, sfnt.NameIDTypographicSubfamily, sfnt.NameIDSubfamily, sfnt.NameIDWWSSubfamily)
	full := firstFontName(ttf, sfnt.NameIDFull, sfnt.NameIDCompatibleFull, sfnt.NameIDPostScript)
	if family == "" {
		family = full
	}
	if family == "" && path != "" {
		family = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}
	weight := fontWeightFromName(subfamily + " " + full)
	italic := fontNameHasItalic(subfamily + " " + full)
	aliases := fontAliases(path, family, full, firstFontName(ttf, sfnt.NameIDPostScript))
	return &fontEntry{
		font:      ttf,
		family:    family,
		subfamily: subfamily,
		aliases:   aliases,
		weight:    weight,
		italic:    italic,
		order:     order,
		index:     index,
		provided:  provided,
		path:      path,
	}
}

func firstFontName(ttf *opentype.Font, ids ...sfnt.NameID) string {
	for _, id := range ids {
		name, err := ttf.Name(nil, id)
		if err == nil && strings.TrimSpace(name) != "" {
			return strings.TrimSpace(name)
		}
	}
	return ""
}

func fontAliases(path string, names ...string) []string {
	seen := make(map[string]bool, len(names)+1)
	var out []string
	for _, name := range names {
		key := normalizeFontName(name)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, key)
	}
	if path != "" {
		base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		key := normalizeFontName(base)
		if key != "" && !seen[key] {
			out = append(out, key)
		}
	}
	return out
}

func (m *fontManager) scanSystemFonts() {
	for _, cached := range systemFontEntries() {
		entry := *cached
		m.addEntry(&entry)
	}
}

func systemFontEntries() []*fontEntry {
	systemFontCache.once.Do(func() {
		systemFontCache.entries = scanSystemFontEntries()
	})
	return systemFontCache.entries
}

func scanSystemFontEntries() []*fontEntry {
	seen := make(map[string]bool)
	var entries []*fontEntry
	for _, dir := range systemFontDirs() {
		if dir == "" || seen[dir] {
			continue
		}
		seen[dir] = true
		_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() || !isFontFile(path) {
				return nil
			}
			fonts, err := newFontEntriesFromPath(path, len(entries))
			if err == nil {
				entries = append(entries, fonts...)
			}
			return nil
		})
	}
	return entries
}

func systemFontDirs() []string {
	var dirs []string
	home, _ := os.UserHomeDir()
	switch runtime.GOOS {
	case "windows":
		winDir := os.Getenv("WINDIR")
		if winDir == "" {
			winDir = `C:\Windows`
		}
		dirs = append(dirs, filepath.Join(winDir, "Fonts"))
		if local := os.Getenv("LOCALAPPDATA"); local != "" {
			dirs = append(dirs, filepath.Join(local, "Microsoft", "Windows", "Fonts"))
		}
	case "darwin":
		dirs = append(dirs, "/System/Library/Fonts", "/Library/Fonts")
		if home != "" {
			dirs = append(dirs, filepath.Join(home, "Library", "Fonts"))
		}
	default:
		dirs = append(dirs, "/usr/share/fonts", "/usr/local/share/fonts")
		if home != "" {
			dirs = append(dirs, filepath.Join(home, ".fonts"), filepath.Join(home, ".local", "share", "fonts"))
		}
	}
	return dirs
}

func isFontFile(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".ttf", ".otf", ".ttc", ".otc":
		return true
	default:
		return false
	}
}

func parseFontFamilies(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	parts := splitCommaList(value)
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		name := strings.TrimSpace(part)
		name = strings.Trim(name, `"'`)
		if name != "" {
			out = append(out, name)
		}
	}
	return out
}

func cleanFontFamilyList(value string) string {
	return strings.Join(parseFontFamilies(value), ", ")
}

func splitCommaList(value string) []string {
	var out []string
	start := 0
	quote := rune(0)
	for i, r := range value {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			}
		case r == '\'' || r == '"':
			quote = r
		case r == ',':
			out = append(out, value[start:i])
			start = i + 1
		}
	}
	out = append(out, value[start:])
	return out
}

func expandGenericFamily(family string) []string {
	switch normalizeFontName(family) {
	case "serif":
		return []string{"Times New Roman", "Georgia", "Noto Serif", "Liberation Serif", "DejaVu Serif", "serif"}
	case "monospace":
		return []string{"Consolas", "Courier New", "Menlo", "Monaco", "Noto Sans Mono", "Liberation Mono", "DejaVu Sans Mono", "monospace"}
	case "cursive":
		return []string{"Comic Sans MS", "Segoe Script", "Apple Chancery", "cursive"}
	case "fantasy":
		return []string{"Impact", "Papyrus", "fantasy"}
	case "sansserif":
		return []string{"Arial", "Segoe UI", "Helvetica", "Noto Sans", "Liberation Sans", "DejaVu Sans", "sans-serif"}
	default:
		return []string{family}
	}
}

func normalizeFontName(name string) string {
	name = strings.TrimSpace(strings.ToLower(name))
	name = strings.Trim(name, `"'`)
	var b strings.Builder
	for _, r := range name {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func normalizedWeight(weight int) int {
	if weight <= 0 {
		return 400
	}
	if weight < 100 {
		return 100
	}
	if weight > 900 {
		return 900
	}
	return ((weight + 50) / 100) * 100
}

func fontWeightFromName(name string) int {
	lower := strings.ToLower(name)
	switch {
	case strings.Contains(lower, "thin"):
		return 100
	case strings.Contains(lower, "extra light"), strings.Contains(lower, "extralight"),
		strings.Contains(lower, "ultra light"), strings.Contains(lower, "ultralight"):
		return 200
	case strings.Contains(lower, "light"):
		return 300
	case strings.Contains(lower, "medium"):
		return 500
	case strings.Contains(lower, "semi bold"), strings.Contains(lower, "semibold"),
		strings.Contains(lower, "demi bold"), strings.Contains(lower, "demibold"):
		return 600
	case strings.Contains(lower, "extra bold"), strings.Contains(lower, "extrabold"),
		strings.Contains(lower, "ultra bold"), strings.Contains(lower, "ultrabold"):
		return 800
	case strings.Contains(lower, "black"), strings.Contains(lower, "heavy"):
		return 900
	case strings.Contains(lower, "bold"):
		return 700
	default:
		return 400
	}
}

func fontNameHasItalic(name string) bool {
	lower := strings.ToLower(name)
	return strings.Contains(lower, "italic") || strings.Contains(lower, "oblique")
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
