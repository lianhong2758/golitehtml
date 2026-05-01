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

func TestFontFamilyFromCSSAndHTMLFontFace(t *testing.T) {
	doc, err := ParseString(`
		<style>
			p { font-family: "Times New Roman", serif; }
		</style>
		<p id="css">CSS family</p>
		<font id="html" face="Georgia, serif">HTML family</font>
	`)
	if err != nil {
		t.Fatal(err)
	}
	cssNode := doc.QueryOne("#css")
	if cssNode == nil {
		t.Fatal("missing CSS node")
	}
	if cssNode.Style.FontFamily != "Times New Roman, serif" {
		t.Fatalf("CSS font family = %q", cssNode.Style.FontFamily)
	}
	htmlNode := doc.QueryOne("#html")
	if htmlNode == nil {
		t.Fatal("missing HTML font node")
	}
	if htmlNode.Style.FontFamily != "Georgia, serif" {
		t.Fatalf("HTML font face = %q", htmlNode.Style.FontFamily)
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
