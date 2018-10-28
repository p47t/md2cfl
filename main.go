package main

import (
	"fmt"
	bfconfluence "github.com/p47t/blackfriday-confluence"
	bf "github.com/russross/blackfriday/v2"
)

func main() {
	renderer := &bfconfluence.Renderer{}
	extensions := bf.CommonExtensions
	md := bf.New(bf.WithRenderer(renderer), bf.WithExtensions(extensions))
	input := "# sample text" // # sample text
	ast := md.Parse([]byte(input))
	output := renderer.Render(ast) // h1. sample text
	fmt.Printf("%s\n", output)
}