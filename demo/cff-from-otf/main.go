// Extract CFF data from OpenType font files.

package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"seehuhn.de/go/pdf/font/sfnt"
)

func main() {
	args := os.Args[1:]
	for _, fname := range args {
		outName := filepath.Base(strings.TrimSuffix(fname, ".otf") + ".cff")
		fmt.Println(fname, "->", outName)

		otf, err := sfnt.Open(fname, nil)
		if err != nil {
			log.Fatalf("%s: %v", fname, err)
		}

		cffData, err := otf.Header.ReadTableBytes(otf.Fd, "CFF ")
		if err != nil {
			log.Fatalf("%s: %v", fname, err)
		}

		err = otf.Close()
		if err != nil {
			log.Fatalf("%s: %v", fname, err)
		}

		err = os.WriteFile(outName, cffData, 0644)
		if err != nil {
			log.Fatalf("%s: %v", outName, err)
		}
	}
}
