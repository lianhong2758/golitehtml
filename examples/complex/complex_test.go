package main

import (
	_ "embed"
	"os"
	"testing"
	"time"

	"github.com/lianhong2758/golitehtml"
)

//go:embed complex.html
var page []byte

func TestRenderComplexExampleToPNG(t *testing.T) {
	totalStart := time.Now()

	initStart := time.Now()
	renderer, err := golitehtml.New(golitehtml.Options{
		RenderScale:    2,
		Width:          700,
		BaseDir:        ".",
		DrawingLibrary: golitehtml.DrawingLibraryTinySkia,
	})
	if err != nil {
		t.Fatal(err)
	}
	initElapsed := time.Since(initStart)

	renderStart := time.Now()
	const outputPath = "complex.png"
	if err := renderer.RenderToFile(page, outputPath); err != nil {
		t.Fatal(err)
	}
	renderElapsed := time.Since(renderStart)

	info, err := os.Stat(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() == 0 {
		t.Fatalf("%s is empty", outputPath)
	}

	t.Logf("rendered %s (%d bytes): init=%s render+write=%s total=%s", outputPath, info.Size(), initElapsed, renderElapsed, time.Since(totalStart))
}

func BenchmarkRenderComplexDrawingLibraries(b *testing.B) {
	for _, drawing := range golitehtml.DrawingLibraries {
		b.Run(string(drawing), func(b *testing.B) {
			renderer, err := golitehtml.New(golitehtml.Options{
				RenderScale:    2,
				Width:          700,
				BaseDir:        ".",
				DrawingLibrary: drawing,
			})
			if err != nil {
				b.Fatal(err)
			}

			if img, err := renderer.Render(page); err != nil {
				b.Fatal(err)
			} else if img.Bounds().Empty() {
				b.Fatal("warmup render produced an empty image")
			}

			b.ReportAllocs()
			b.ResetTimer()
			renderStart := time.Now()
			for i := 0; i < b.N; i++ {
				img, err := renderer.Render(page)
				if err != nil {
					b.Fatal(err)
				}
				if img.Bounds().Empty() {
					b.Fatal("render produced an empty image")
				}
			}
			renderElapsed := time.Since(renderStart)
			b.StopTimer()
			b.ReportMetric(float64(renderElapsed.Nanoseconds())/float64(b.N)/float64(time.Millisecond), "ms/op")
		})
	}
}
