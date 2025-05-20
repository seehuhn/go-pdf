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
	"slices"

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

	// -----------------------------------------------------------------------

	extraTextCache := make(map[font.Embedded]map[cid.CID]string)
	spaceWidth := make(map[font.Embedded]float64)

	contents := reader.New(r, nil)
	contents.TextEvent = func(op reader.TextEvent, arg float64) {
		switch op {
		case reader.TextEventSpace:
			w0, ok := spaceWidth[contents.TextFont]
			if !ok {
				w0 = getSpaceWidth(contents.TextFont)
				spaceWidth[contents.TextFont] = w0
			}

			if arg > 0.3*w0 {
				fmt.Print(" ")
			}
		case reader.TextEventNL:
			fmt.Println()
		case reader.TextEventMove:
			fmt.Println()
		}
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

	// -----------------------------------------------------------------------

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

func getSpaceWidth(F font.Embedded) float64 {
	Fe, ok := F.(font.FromFile)
	if !ok {
		return 280
	}

	d := Fe.GetDict()
	if d == nil {
		return 0
	}

	return spaceWidthHeuristic(d)
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

type affine struct {
	intercept, slope float64
}

var commonCharacters = map[string]affine{
	" ": {0, 1},
	" ": {0, 1},
	")": {-43.01937, 1.0268},
	"/": {-10.99708, 0.9623335},
	"•": {-24.2725, 0.9956384},
	"−": {-439.6255, 1.238626},
	"∗": {91.30598, 0.7265824},
	"1": {-130.7855, 0.9746186},
	"a": {-131.2164, 0.9740258},
	"A": {72.40703, 0.4928694},
	"e": {-136.5258, 0.9895894},
	"E": {-28.76257, 0.6957778},
	"i": {51.62929, 0.8973944},
	"ε": {-56.25771, 0.9947787},
	"Ω": {-132.9966, 1.002173},
	"中": {-356.8609, 1.215483},
}

func spaceWidthHeuristic(dict font.Dict) float64 {
	guesses := []float64{280}
	for _, info := range dict.Characters() {
		if coef, ok := commonCharacters[info.Text]; ok && info.Width > 0 {
			guesses = append(guesses, coef.intercept+coef.slope*info.Width)
		}
	}
	slices.Sort(guesses)

	// calculate the median
	var guess float64
	n := len(guesses)
	if n%2 == 0 {
		guess = (guesses[n/2-1] + guesses[n/2]) / 2
	} else {
		guess = guesses[n/2]
	}

	// adjustment to remove empirical bias
	guess = 1.366239*guess - 139.183703

	// clamp to approximate [0.01, 0.99] quantile range
	if guess < 200 {
		guess = 200
	} else if guess > 1000 {
		guess = 1000
	}

	return guess
}
