// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2023  Jochen Voss <voss@seehuhn.de>
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package main

import (
	"flag"
	"fmt"
	"log"
	"math"
	"os"

	"seehuhn.de/go/geom/matrix"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/pagetree"
	"seehuhn.de/go/pdf/reader"
)

var pages = flag.String("p", "", "Only include text on pages `A-B`")
var xRange = flag.String("x", "", "Only include text at x coordinates `A-B`")

var pageMin, pageMax int
var xRangeMin, xRangeMax float64

func main() {
	flag.Parse()

	xRangeMin = math.Inf(-1)
	xRangeMax = math.Inf(1)
	if *xRange != "" {
		_, err := fmt.Sscanf(*xRange, "%f-%f", &xRangeMin, &xRangeMax)
		if err != nil || xRangeMin >= xRangeMax {
			log.Fatalf("invalid x-range %q", *xRange)
		}
	}

	if *pages != "" {
		_, err := fmt.Sscanf(*pages, "%d-%d", &pageMin, &pageMax)
		if err != nil || pageMin < 1 || pageMax < pageMin {
			log.Fatalf("invalid page range %q", *pages)
		}
	} else {
		pageMin, pageMax = 1, math.MaxInt
	}

	for _, fname := range flag.Args() {
		err := extractText(fname)
		if err != nil {
			log.Fatal(err)
		}
	}
}

func extractText(fname string) error {
	fd, err := os.Open(fname)
	if err != nil {
		return err
	}
	defer fd.Close()

	r, err := pdf.NewReader(fd, nil)
	if err != nil {
		return err
	}

	contents := reader.New(r, nil)
	contents.Text = func(text string) error {
		x, _ := contents.GetTextPositionDevice()
		if x >= xRangeMin && x < xRangeMax {
			fmt.Print(text)
		}
		return nil
	}

	pageNo := 1
	for _, pageDict := range pagetree.NewIterator(r).All() {
		if pageNo >= pageMin {
			fmt.Println("Page", pageNo)
			fmt.Println()

			err := contents.ParsePage(pageDict, matrix.Identity)
			if err != nil {
				log.Fatal(err)
			}

			fmt.Println()
		}

		pageNo++
		if pageNo > pageMax {
			break
		}
	}
	return nil
}
