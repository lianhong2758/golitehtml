package golitehtml

import (
	"testing"

	builtfont "github.com/lianhong2758/golitehtml/font"
)

func TestFontManagerUsesProvidedFontsAsDefaultAndMatchTargets(t *testing.T) {
	manager, err := newFontManager([][]byte{builtfont.TTF})
	if err != nil {
		t.Fatal(err)
	}
	if manager.defaultEntry == nil {
		t.Fatal("missing default font")
	}
	if !manager.defaultEntry.provided {
		t.Fatal("first provided font was not marked as the default provided font")
	}
	if manager.defaultEntry.family == "" {
		t.Fatal("provided font did not expose a family name")
	}
	entry, _ := manager.match(TextStyle{Size: 16})
	if entry != manager.defaultEntry {
		t.Fatalf("empty family matched %v, want default provided entry", entry)
	}

	entry, synth := manager.match(TextStyle{
		Family: manager.defaultEntry.family,
		Size:   16,
		Weight: 700,
		Italic: true,
	})
	if entry != manager.defaultEntry {
		t.Fatalf("matched entry = %v, want default provided entry", entry)
	}
	if manager.defaultEntry.weight < 600 && !synth.Bold {
		t.Fatal("bold synthesis was not enabled for a regular default font")
	}
	if !manager.defaultEntry.italic && !synth.Italic {
		t.Fatal("italic synthesis was not enabled for a regular default font")
	}
}

func TestRendererAcceptsFontArray(t *testing.T) {
	renderer, err := New(Options{
		Width: 320,
		Fonts: [][]byte{builtfont.TTF},
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	img, err := renderer.Render([]byte(`<p style="font-family: sans-serif">Fonts</p>`))
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if img.Bounds().Dx() != 320 {
		t.Fatalf("image width = %d, want 320", img.Bounds().Dx())
	}
}

func TestFontManagerPrefersStandardBaseWhenItalicVariantIsMissing(t *testing.T) {
	manager := &fontManager{
		entries: []*fontEntry{
			{aliases: []string{normalizeFontName("Example")}, weight: 400, order: 0},
			{aliases: []string{normalizeFontName("Example")}, weight: 700, italic: true, order: 1},
		},
	}
	entry := manager.bestMatch("Example", 400, true)
	if entry != manager.entries[0] {
		t.Fatal("regular italic request should use the regular face before a bold italic face")
	}
	entry = manager.bestMatch("Example", 700, false)
	if entry != manager.entries[0] {
		t.Fatal("bold regular request should not use an italic face when a regular base exists")
	}
}

func TestFontManagerFallsBackPerGlyph(t *testing.T) {
	primary, err := newFontEntryFromData(builtfont.TTF, 0, true)
	if err != nil {
		t.Fatal(err)
	}
	primary.aliases = []string{normalizeFontName("Primary")}
	fallback, err := newFontEntryFromData(builtfont.TTF, 1, true)
	if err != nil {
		t.Fatal(err)
	}
	manager := &fontManager{
		entries:      []*fontEntry{primary, fallback},
		defaultEntry: fallback,
		faces:        make(map[string]resolvedFont),
		glyphs: map[string]bool{
			"0|字": false,
			"1|字": true,
		},
	}
	runs, err := manager.textRuns("A字", TextStyle{Family: "Primary", Size: 16})
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 2 {
		t.Fatalf("runs = %d, want 2", len(runs))
	}
	if runs[0].Font.Entry != primary || runs[1].Font.Entry != fallback {
		t.Fatal("text did not fall back from primary font to default font for missing glyph")
	}
}
