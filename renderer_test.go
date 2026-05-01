package golitehtml

import "testing"

func TestRendererRenderProducesImage(t *testing.T) {
	renderer, err := New(Options{Width: 320})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	img, err := renderer.Render([]byte(`
		<style>
			body { background: #fff; }
			.card { margin: 10px; padding: 12px; border: 1px solid #333; }
		</style>
		<div class="card"><h1>Hello</h1><p>HTML <strong>to PNG</strong></p></div>
	`))
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	bounds := img.Bounds()
	if bounds.Dx() != 320 {
		t.Fatalf("image width = %d, want 320", bounds.Dx())
	}
	if bounds.Dy() <= 1 {
		t.Fatalf("image height = %d, want > 1", bounds.Dy())
	}
}

func TestRendererRenderScaleIncreasesOutputSize(t *testing.T) {
	renderer, err := New(Options{Width: 320, RenderScale: 2})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	img, err := renderer.Render([]byte(`<p style="font-size:18px">超采样 text</p>`))
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if img.Bounds().Dx() != 640 {
		t.Fatalf("image width = %d, want 640", img.Bounds().Dx())
	}
	if img.Bounds().Dy() <= 2 {
		t.Fatalf("image height = %d, want > 2", img.Bounds().Dy())
	}
}
