package golitehtml

import "testing"

func TestParseAndQuery(t *testing.T) {
	doc, err := ParseString(`<html><body><div class="card"><p id="msg">Hi</p></div></body></html>`)
	if err != nil {
		t.Fatal(err)
	}
	if doc.Root == nil {
		t.Fatal("nil root")
	}
	p := doc.QueryOne(".card #msg")
	if p == nil {
		t.Fatal("selector did not match paragraph")
	}
	if got := p.Text(); got != "Hi" {
		t.Fatalf("text = %q, want Hi", got)
	}
}
