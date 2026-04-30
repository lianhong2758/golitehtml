package golitehtml

import "testing"

func TestImageLoaderDecodesUTF8SVGDataURL(t *testing.T) {
	src := `data:image/svg+xml;utf8,<svg xmlns="http://www.w3.org/2000/svg" width="120" height="40"><rect width="120" height="40" fill="%23dbeafe"/><circle cx="24" cy="20" r="12" fill="%230f7b6c"/><text x="44" y="25" fill="%2323527c" font-size="14">svg</text></svg>`
	loader := newImageLoader("")

	size, ok := loader.ImageSize(src)
	if !ok {
		t.Fatal("ImageSize() failed for SVG data URL")
	}
	if size.W != 120 || size.H != 40 {
		t.Fatalf("ImageSize() = %.0fx%.0f, want 120x40", size.W, size.H)
	}

	img, ok := loader.Image(src)
	if !ok {
		t.Fatal("Image() failed for SVG data URL")
	}
	if got := img.Bounds().Dx(); got != 120 {
		t.Fatalf("image width = %d, want 120", got)
	}
	if got := img.Bounds().Dy(); got != 40 {
		t.Fatalf("image height = %d, want 40", got)
	}
}
