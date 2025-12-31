package main

import (
	"flag"
	"fmt"
	"image/png"
	"os"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/converter"
)

func main() {
	dpi := flag.Float64("dpi", 72.0, "DPI for rendering")
	pageNum := flag.Int("page", 1, "Page number to render (1-based)")
	flag.Parse()

	if flag.NArg() < 2 {
		fmt.Printf("Usage: %s [options] input.pdf output.png\n", os.Args[0])
		flag.PrintDefaults()
		os.Exit(1)
	}

	inputFile := flag.Arg(0)
	outputFile := flag.Arg(1)

	f, err := os.Open(inputFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening input file: %v\n", err)
		os.Exit(1)
	}
	defer f.Close()

	r, err := pdf.NewReader(f, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating PDF reader: %v\n", err)
		os.Exit(1)
	}

	conv := converter.NewConverter(r)
	img, err := conv.RenderPageToImage(*pageNum, *dpi)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error rendering page: %v\n", err)
		os.Exit(1)
	}

	out, err := os.Create(outputFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating output file: %v\n", err)
		os.Exit(1)
	}
	defer out.Close()

	err = png.Encode(out, img)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding PNG: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Successfully rendered page %d of %s to %s\n", *pageNum, inputFile, outputFile)
}
