package main

import (
	_ "embed"
	"os"
	"testing"
	"time"

	"github.com/lianhong2758/golitehtml"
)

//go:embed syntax.html
var page []byte

func TestRenderSyntaxExampleToPNG(t *testing.T) {
	totalStart := time.Now()

	initStart := time.Now()
	renderer, err := golitehtml.New(golitehtml.Options{
		Width:   980,
		BaseDir: ".",
	})
	if err != nil {
		t.Fatal(err)
	}
	initElapsed := time.Since(initStart)

	renderStart := time.Now()
	const outputPath = "syntax.png"
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
