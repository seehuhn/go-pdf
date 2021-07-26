package main

import (
	"fmt"
	"log"
	"os"

	"seehuhn.de/go/pdf/font/sfnt"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: tt-illustrate-gpos font.ttf")
		os.Exit(1)
	}
	fontFileName := os.Args[1]

	tt, err := sfnt.Open(fontFileName)
	if err != nil {
		log.Fatal(err)
	}
	defer tt.Close()

	if !tt.HasTables("GPOS") {
		log.Fatal("font has no GPOS table")
	}
}
