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

	"seehuhn.de/go/postscript/cid"
	"seehuhn.de/go/postscript/type1/names"

	"seehuhn.de/go/sfnt"
	"seehuhn.de/go/sfnt/glyf"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/dict"
	"seehuhn.de/go/pdf/font/glyphdata"
	"seehuhn.de/go/pdf/internal/pagerange"
	"seehuhn.de/go/pdf/pagetree"
	"seehuhn.de/go/pdf/reader"
)

func main() {
	pages := &pagerange.PageRange{}
	flag.Var(pages, "p", "range of pages to extract")
	xRange := flag.String("x", "", "Only include text at x coordinates `A-B`")
	showPageNumbers := flag.Bool("P", false, "show page numbers")
	flag.Parse()

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

	e := &extractor{
		pageMin:         pages.Start,
		pageMax:         pages.End,
		xRangeMin:       xRangeMin,
		xRangeMax:       xRangeMax,
		showPageNumbers: *showPageNumbers,
	}

	for _, fname := range flag.Args() {
		err := e.extractText(fname)
		if err != nil {
			log.Fatal(err)
		}
	}
}

type extractor struct {
	pageMin, pageMax     int
	xRangeMin, xRangeMax float64
	showPageNumbers      bool
}

func (e *extractor) extractText(fname string) error {
	extraTextCache := make(map[font.Embedded]map[cid.CID]string)

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

	startPage := e.pageMin
	endPage := e.pageMax
	if endPage > numPages {
		endPage = numPages
	}

	contents := reader.New(r, nil)
	contents.TextSpace = func() error {
		fmt.Print(" ")
		return nil
	}
	contents.TextNL = func() error {
		fmt.Println()
		return nil
	}
	contents.Character = func(cid cid.CID, text string) error {
		if text == "" {
			F := contents.TextFont
			m, ok := extraTextCache[F]
			if !ok {
				m = getExtraMapping(r, contents.TextFont)
				extraTextCache[F] = m
			}
			text = m[cid]
		}

		// xUser, yUser := contents.GetTextPositionUser()

		xDev, _ := contents.GetTextPositionDevice()
		if xDev >= e.xRangeMin && xDev < e.xRangeMax {
			fmt.Print(text)
		}
		return nil
	}

	for pageNo := startPage; pageNo <= endPage; pageNo++ {
		_, pageDict, err := pagetree.GetPage(r, pageNo-1)
		if err != nil {
			return err
		}

		if e.showPageNumbers {
			fmt.Println("Page", pageNo)
			fmt.Println()
		}

		err = contents.ParsePage(pageDict, matrix.Identity)
		if err != nil {
			log.Fatal(err)
		}

		fmt.Println()
	}
	return nil
}

func getExtraMapping(r *pdf.Reader, F font.Embedded) map[cid.CID]string {
	Fe, ok := F.(font.FromFile)
	if !ok {
		return nil
	}

	d := Fe.GetDict()
	tp, ref := d.GlyphData()
	if ref == 0 {
		return nil
	}

	switch d := d.(type) {
	case *dict.CIDFontType2:
		if tp != glyphdata.TrueType {
			return nil
		}

		body, err := pdf.GetStreamReader(r, ref)
		if err != nil {
			return nil
		}
		info, err := sfnt.Read(body)
		if err != nil {
			return nil
		}
		outlines, ok := info.Outlines.(*glyf.Outlines)
		if !ok {
			return nil
		}

		m := make(map[cid.CID]string)

		// method 1: use glyph names, if present
		if outlines.Names != nil {
			if d.CIDToGID != nil {
				for cidVal, gid := range d.CIDToGID {
					if int(gid) > len(outlines.Names) {
						continue
					}
					name := outlines.Names[gid]
					if name == "" {
						continue
					}

					text := names.ToUnicode(name, d.PostScriptName)
					m[cid.CID(cidVal)] = string(text)
				}
			}
		}
		return m
	default:
		fmt.Printf("%v %T\n", tp, F)
		return nil
	}
}
