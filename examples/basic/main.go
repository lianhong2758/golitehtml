package main

import (
	_ "embed"
	"fmt"

	"github.com/lianhong2758/golitehtml"
)

//go:embed basic.html
var page []byte

func main() {
	renderer, err := golitehtml.New(golitehtml.Options{
		Width:   760,
		BaseDir: ".",
	})
	must(err)

	must(renderer.RenderToFile(page, "basic.png"))
	fmt.Println("wrote basic.png")
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
