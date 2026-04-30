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

func TestAutoHorizontalMarginsCenterOnlyWhenBoxFits(t *testing.T) {
	doc, err := ParseString(`
		<div id="fits" style="width:100px;margin-left:auto;margin-right:auto;height:10px"></div>
		<div id="wide" style="width:300px;margin-left:auto;margin-right:auto;height:10px"></div>
	`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := doc.Render(200); err != nil {
		t.Fatal(err)
	}
	fits := doc.QueryOne("#fits")
	wide := doc.QueryOne("#wide")
	if fits == nil || wide == nil {
		t.Fatal("missing boxes")
	}
	if fits.Box.X != 50 {
		t.Fatalf("centered box x = %v, want 50", fits.Box.X)
	}
	if wide.Box.X != 0 {
		t.Fatalf("overflowing box x = %v, want left aligned at 0", wide.Box.X)
	}
}

func TestFloatedMenuItemsStayOnOneRow(t *testing.T) {
	doc, err := ParseString(`
		<style>
			.menu { width: 500px; margin: auto; }
			.menu ul { margin: 0; padding: 0; float: left; }
			.menu li { float: left; list-style: none; }
			.menu a:link, .menu a:visited { display: block; padding: 10px 12px; font-size: 13px; }
		</style>
		<div class="menu">
			<ul>
				<li><a href="/">Home</a></li>
				<li><a href="/download">Download</a></li>
			</ul>
		</div>
	`)
	if err != nil {
		t.Fatal(err)
	}
	frame, err := doc.Render(700)
	if err != nil {
		t.Fatal(err)
	}
	var home, download TextOp
	var bullets int
	for _, op := range frame.Ops {
		txt, ok := op.(TextOp)
		if !ok {
			continue
		}
		switch txt.Text {
		case "Home":
			home = txt
		case "Download":
			download = txt
		case "\u2022":
			bullets++
		}
	}
	if home.Text == "" || download.Text == "" {
		t.Fatalf("missing menu text: home=%q download=%q", home.Text, download.Text)
	}
	if home.Rect.Y != download.Rect.Y {
		t.Fatalf("menu items are not on one row: home=%+v download=%+v", home.Rect, download.Rect)
	}
	if download.Rect.X <= home.Rect.Right() {
		t.Fatalf("download did not render to the right of home: home=%+v download=%+v", home.Rect, download.Rect)
	}
	if bullets != 0 {
		t.Fatalf("list-style:none still emitted %d bullets", bullets)
	}
}

func TestSiblingVerticalMarginsCollapse(t *testing.T) {
	doc, err := ParseString(`
		<div>
			<h1 id="title" style="margin:20px 0;line-height:20px;font-size:20px">Title</h1>
			<p id="body" style="margin:10px 0;line-height:10px;font-size:10px">Body</p>
		</div>
	`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := doc.Render(300); err != nil {
		t.Fatal(err)
	}
	title := doc.QueryOne("#title")
	body := doc.QueryOne("#body")
	if title == nil || body == nil {
		t.Fatal("missing nodes")
	}
	if gap := body.Box.Y - title.Box.Bottom(); gap != 20 {
		t.Fatalf("collapsed margin gap = %v, want 20", gap)
	}
}

func TestNormalBlockDoesNotExpandAroundFloats(t *testing.T) {
	doc, err := ParseString(`
		<div id="menu">
			<div style="float:left;width:100px;height:30px"></div>
		</div>
		<div id="content" style="margin-top:20px;height:10px"></div>
	`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := doc.Render(300); err != nil {
		t.Fatal(err)
	}
	menu := doc.QueryOne("#menu")
	content := doc.QueryOne("#content")
	if menu == nil || content == nil {
		t.Fatal("missing nodes")
	}
	if menu.Box.H != 0 {
		t.Fatalf("normal block height around floats = %v, want 0", menu.Box.H)
	}
	if content.Box.Y != 20 {
		t.Fatalf("content y = %v, want 20", content.Box.Y)
	}
}

func TestBlockElementBoxNotOverwrittenByInlineText(t *testing.T) {
	doc, err := ParseString(`
		<div>
			<span id="block" style="display:block;padding:6px;background:#fff8c5">Only span of type.</span>
		</div>
	`)
	if err != nil {
		t.Fatal(err)
	}
	frame, err := doc.Render(300)
	if err != nil {
		t.Fatal(err)
	}
	span := doc.QueryOne("#block")
	if span == nil {
		t.Fatal("missing span")
	}
	var background RectOp
	var text TextOp
	for _, op := range frame.Ops {
		switch v := op.(type) {
		case RectOp:
			if v.Node == span {
				background = v
			}
		case TextOp:
			if v.Text == "Only" {
				text = v
			}
		}
	}
	if background.Rect.H <= 0 || text.Text == "" {
		t.Fatalf("missing background/text: bg=%+v text=%+v", background, text)
	}
	if span.Box.Y != background.Rect.Y || span.Box.H != background.Rect.H {
		t.Fatalf("span box overwritten by text: box=%+v background=%+v", span.Box, background.Rect)
	}
	if text.Rect.Y <= background.Rect.Y {
		t.Fatalf("text not inside padded background: text=%+v background=%+v", text.Rect, background.Rect)
	}
}

func TestUnitlessLineHeightUsesFontSizeMultiplier(t *testing.T) {
	doc, err := ParseString(`<span id="s" style="display:block;font-size:20px;line-height:1.5;background:#fff8c5">Line</span>`)
	if err != nil {
		t.Fatal(err)
	}
	frame, err := doc.Render(240)
	if err != nil {
		t.Fatal(err)
	}
	span := doc.QueryOne("#s")
	if span == nil {
		t.Fatal("missing span")
	}
	var text TextOp
	for _, op := range frame.Ops {
		if v, ok := op.(TextOp); ok && v.Text == "Line" {
			text = v
		}
	}
	if text.Text == "" {
		t.Fatal("missing text op")
	}
	if text.Rect.H != 30 {
		t.Fatalf("text line-height = %v, want 30", text.Rect.H)
	}
	if span.Box.H < 30 {
		t.Fatalf("span box height = %v, want at least 30", span.Box.H)
	}
}

func TestBorderBoxHeightIncludesPaddingAndBorder(t *testing.T) {
	doc, err := ParseString(`<div id="box" style="box-sizing:border-box;height:40px;padding:8px;border:2px solid #000;background:#0969da">float left</div>`)
	if err != nil {
		t.Fatal(err)
	}
	frame, err := doc.Render(240)
	if err != nil {
		t.Fatal(err)
	}
	box := doc.QueryOne("#box")
	if box == nil {
		t.Fatal("missing box")
	}
	if box.Box.H != 40 {
		t.Fatalf("border-box height = %v, want 40", box.Box.H)
	}
	var background RectOp
	for _, op := range frame.Ops {
		if v, ok := op.(RectOp); ok && v.Node == box && v.Color == (Color{9, 105, 218, 255}) {
			background = v
			break
		}
	}
	if background.Rect.H != 40 {
		t.Fatalf("background height = %v, want 40", background.Rect.H)
	}
}
