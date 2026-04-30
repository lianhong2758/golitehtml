package golitehtml

import "testing"

func TestCSSCascadeAndInlineStyle(t *testing.T) {
	doc, err := ParseString(`
		<style>
			p { color: red; }
			#lead { color: blue; }
			.note strong { font-weight: 700; }
		</style>
		<p id="lead" style="color: #00ff00">Hello <strong>world</strong></p>
	`)
	if err != nil {
		t.Fatal(err)
	}
	p := doc.QueryOne("#lead")
	if p == nil {
		t.Fatal("missing paragraph")
	}
	if p.Style.Color != (Color{0, 255, 0, 255}) {
		t.Fatalf("inline color did not win cascade: %v", p.Style.Color)
	}
	strong := doc.QueryOne("strong")
	if strong == nil {
		t.Fatal("missing strong")
	}
	if strong.Style.FontWeight != 700 {
		t.Fatalf("strong weight = %d, want 700", strong.Style.FontWeight)
	}
}

func TestParseColor(t *testing.T) {
	tests := map[string]Color{
		"red":              {255, 0, 0, 255},
		"#0f0":             {0, 255, 0, 255},
		"#336699cc":        {0x33, 0x66, 0x99, 0xcc},
		"rgb(255, 0, 128)": {255, 0, 128, 255},
		"rgba(0,0,0,.5)":   {0, 0, 0, 128},
	}
	for input, want := range tests {
		got, ok := ParseColor(input)
		if !ok {
			t.Fatalf("ParseColor(%q) failed", input)
		}
		if got != want {
			t.Fatalf("ParseColor(%q) = %v, want %v", input, got, want)
		}
	}
}
