package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/converter"
)

func main() {
	outputFile := flag.String("o", "output.html", "output HTML file")
	flow := flag.Bool("flow", false, "use flow-based HTML instead of absolute positioning")
	flag.Parse()

	if len(flag.Args()) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: pdftohtml-go [options] <input.pdf>")
		os.Exit(1)
	}

	inputFile := flag.Arg(0)
	fd, err := os.Open(inputFile)
	if err != nil {
		log.Fatalf("failed to open input file: %v", err)
	}
	defer fd.Close()

	r, err := pdf.NewReader(fd, nil)
	if err != nil {
		log.Fatalf("failed to create PDF reader: %v", err)
	}

	conv := converter.NewConverter(r)
	pages, err := conv.ConvertDocument()
	if err != nil {
		log.Fatalf("conversion failed: %v", err)
	}

	out, err := os.Create(*outputFile)
	if err != nil {
		log.Fatalf("failed to create output file: %v", err)
	}
	defer out.Close()

	writer := converter.NewHTMLWriter(out, conv.Tracker)
	writer.FlowMode = *flow
	writer.WriteHeader()
	for _, p := range pages {
		writer.WritePage(p)
	}
	writer.WriteFooter()

	fmt.Printf("Successfully converted %d pages to %s\n", len(pages), *outputFile)
}
