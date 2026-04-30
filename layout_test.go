package golitehtml

import "testing"

type recordingCanvas struct {
	rects  []RectOp
	texts  []TextOp
	images []ImageOp
}

func (c *recordingCanvas) DrawRect(op RectOp)   { c.rects = append(c.rects, op) }
func (c *recordingCanvas) DrawText(op TextOp)   { c.texts = append(c.texts, op) }
func (c *recordingCanvas) DrawImage(op ImageOp) { c.images = append(c.images, op) }

func TestRenderProducesDisplayList(t *testing.T) {
	doc, err := ParseString(`<p style="background:#fff3cc;padding:4px">Hello <b>Go</b></p>`)
	if err != nil {
		t.Fatal(err)
	}
	frame, err := doc.Render(120)
	if err != nil {
		t.Fatal(err)
	}
	if frame.Width != 120 {
		t.Fatalf("width = %v, want 120", frame.Width)
	}
	if frame.Height <= 0 {
		t.Fatalf("height = %v, want positive", frame.Height)
	}
	var textOps int
	for _, op := range frame.Ops {
		if _, ok := op.(TextOp); ok {
			textOps++
		}
	}
	if textOps == 0 {
		t.Fatal("no text operations produced")
	}
	canvas := &recordingCanvas{}
	frame.Draw(canvas, 10, 20, nil)
	if len(canvas.texts) == 0 {
		t.Fatal("draw did not replay text operations")
	}
	if canvas.texts[0].Rect.X < 10 || canvas.texts[0].Rect.Y < 20 {
		t.Fatalf("draw offset not applied: %+v", canvas.texts[0].Rect)
	}
}

func TestImageResolver(t *testing.T) {
	doc, err := ParseString(`<p>before <img src="logo.png" alt="logo"> after</p>`)
	if err != nil {
		t.Fatal(err)
	}
	frame, err := doc.Render(300, WithImageResolver(ImageResolverFunc(func(src string) (Size, bool) {
		if src != "logo.png" {
			t.Fatalf("unexpected src %q", src)
		}
		return Size{W: 32, H: 16}, true
	})))
	if err != nil {
		t.Fatal(err)
	}
	var image ImageOp
	for _, op := range frame.Ops {
		if img, ok := op.(ImageOp); ok {
			image = img
			break
		}
	}
	if image.Rect.W != 32 || image.Rect.H != 16 {
		t.Fatalf("image rect = %+v, want 32x16", image.Rect)
	}
}

func TestMixedBlockAndInlineChildrenKeepBlockFlow(t *testing.T) {
	doc, err := ParseString(`
		<div style="padding:10px">
			<h1 style="margin:0;font-size:20px;line-height:24px">Rendering title</h1>
			<p style="margin:8px 0 0 0">Body copy</p>
		</div>
	`)
	if err != nil {
		t.Fatal(err)
	}
	frame, err := doc.Render(300)
	if err != nil {
		t.Fatal(err)
	}
	var rendering, body TextOp
	for _, op := range frame.Ops {
		txt, ok := op.(TextOp)
		if !ok {
			continue
		}
		switch txt.Text {
		case "Rendering":
			rendering = txt
		case "Body":
			body = txt
		}
	}
	if rendering.Text == "" || body.Text == "" {
		t.Fatalf("missing text ops: rendering=%q body=%q", rendering.Text, body.Text)
	}
	if body.Rect.Y <= rendering.Rect.Y {
		t.Fatalf("block child was flattened: rendering y=%v body y=%v", rendering.Rect.Y, body.Rect.Y)
	}
}

func TestDefaultMeasurerKeepsUppercaseWordsApart(t *testing.T) {
	doc, err := ParseString(`<p style="font-size:13px;font-weight:700;margin:0">ENGINEERING NOTE</p>`)
	if err != nil {
		t.Fatal(err)
	}
	frame, err := doc.Render(300)
	if err != nil {
		t.Fatal(err)
	}
	var engineering, note TextOp
	for _, op := range frame.Ops {
		txt, ok := op.(TextOp)
		if !ok {
			continue
		}
		switch txt.Text {
		case "ENGINEERING":
			engineering = txt
		case "NOTE":
			note = txt
		}
	}
	if engineering.Text == "" || note.Text == "" {
		t.Fatalf("missing text ops: engineering=%q note=%q", engineering.Text, note.Text)
	}
	if note.Rect.X <= engineering.Rect.Right() {
		t.Fatalf("words overlap: engineering=%+v note=%+v", engineering.Rect, note.Rect)
	}
	if gap := note.Rect.X - engineering.Rect.Right(); gap < 2 || gap > 8 {
		t.Fatalf("unexpected word gap %v between %+v and %+v", gap, engineering.Rect, note.Rect)
	}
}

func TestListItemsEmitMarkers(t *testing.T) {
	doc, err := ParseString(`<ul><li>first</li><li>second</li></ul>`)
	if err != nil {
		t.Fatal(err)
	}
	frame, err := doc.Render(240)
	if err != nil {
		t.Fatal(err)
	}
	var bullets int
	for _, op := range frame.Ops {
		txt, ok := op.(TextOp)
		if ok && txt.Text == "\u2022" {
			bullets++
		}
	}
	if bullets != 2 {
		t.Fatalf("bullet count = %d, want 2", bullets)
	}
}
