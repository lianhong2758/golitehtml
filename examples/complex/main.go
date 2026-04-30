package main

import (
	_ "embed"
	"fmt"

	"github.com/lianhong2758/golitehtml"
)

//go:embed complex.html
var page []byte

func main() {
	renderer, err := golitehtml.New(golitehtml.Options{
		Width:   700,
		BaseDir: ".",
	})
	must(err)

	must(renderer.RenderToFile(page, "complex.png"))
	fmt.Println("wrote complex.png")
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
