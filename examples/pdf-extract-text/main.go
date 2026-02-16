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
	"runtime"
	"runtime/pprof"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/pagerange"
	"seehuhn.de/go/pdf/pagetree"
	"seehuhn.de/go/pdf/tools/pdf-extract/text"
)

func main() {
	pages := &pagerange.PageRange{}
	flag.Var(pages, "p", "range of pages to extract")
	xRange := flag.String("x", "", "Only include text at x coordinates `A-B`")
	showPageNumbers := flag.Bool("P", false, "show page numbers")
	useActualText := flag.Bool("use-actualtext", false, "use ActualText from marked content")
	cpuprofile := flag.String("cpuprofile", "", "write cpu profile to `file`")
	memprofile := flag.String("memprofile", "", "write memory profile to `file`")
	flag.Parse()

	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			log.Fatal("could not create CPU profile: ", err)
		}
		defer f.Close()
		if err := pprof.StartCPUProfile(f); err != nil {
			log.Fatal("could not start CPU profile: ", err)
		}
		defer pprof.StopCPUProfile()
	}

	if pages.Start < 1 {
		pages.Start = 1
		pages.End = math.MaxInt
	}

	xRangeMin := math.Inf(-1)
	xRangeMax := math.Inf(1)
	if *xRange != "" {
		_, err := fmt.Sscanf(*xRange, "%f-%f", &xRangeMin, &xRangeMax)
		if err != nil || xRangeMin >= xRangeMax {
			log.Fatalf("invalid x-range %q", *xRange)
		}
	}

	for _, fname := range flag.Args() {
		err := extractText(fname, pages.Start, pages.End, xRangeMin, xRangeMax, *showPageNumbers, *useActualText)
		if err != nil {
			log.Fatal(err)
		}
	}

	if *memprofile != "" {
		f, err := os.Create(*memprofile)
		if err != nil {
			log.Fatal(err)
		}
		runtime.GC()
		if allocs := pprof.Lookup("allocs"); allocs == nil {
			log.Fatal("could not lookup memory profile")
		} else if err := allocs.WriteTo(f, 0); err != nil {
			log.Fatal("could not write memory profile: ", err)
		}
		err = f.Close()
		if err != nil {
			log.Fatal(err)
		}
	}
}

func extractText(fname string, pageMin, pageMax int, xRangeMin, xRangeMax float64, showPageNumbers, useActualText bool) error {
	fd, err := os.Open(fname)
	if err != nil {
		return err
	}
	defer fd.Close()

	r, err := pdf.NewReader(fd, nil)
	if err != nil {
		return err
	}

	numPages, err := pagetree.NumPages(r)
	if err != nil {
		return err
	}

	startPage := pageMin
	endPage := min(pageMax, numPages)

	extractor := text.New(r, os.Stdout)
	extractor.UseActualText = useActualText
	extractor.XRangeMin = xRangeMin
	extractor.XRangeMax = xRangeMax

	for pageNo := startPage; pageNo <= endPage; pageNo++ {
		_, pageDict, err := pagetree.GetPage(r, pageNo-1)
		if err != nil {
			return err
		}

		if showPageNumbers {
			fmt.Println("Page", pageNo)
			fmt.Println()
		}

		err = extractor.ExtractPage(pageDict)
		if err != nil {
			return err
		}

		fmt.Println()
	}
	return nil
}
