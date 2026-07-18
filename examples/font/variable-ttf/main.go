// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2026  Jochen Voss <voss@seehuhn.de>
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

// This program demonstrates the two variation axes of the Source Serif 4
// variable font.  It writes test.pdf, showing the letter "a" for each
// combination of weight (columns) and optical size (rows).
package main

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font/embed"
	"seehuhn.de/go/sfnt"
)

const (
	fontURL    = "https://cdn.jsdelivr.net/gh/google/fonts@7b203a635ebe80801c80f29633d4fc467cd1214e/ofl/sourceserif4/SourceSerif4%5Bopsz,wght%5D.ttf"
	fontFile   = "font.ttf"
	outputFile = "test.pdf"
)

const (
	sampleLetter = "a"
	glyphSize    = 42.0
	cellWidth    = 33.0
	cellHeight   = 36.0
	cellAscent   = 26.0
)

// The values of the two axes used for the columns and rows of the grid.
var (
	weights      = []float64{200, 300, 400, 500, 600, 700, 800, 900}
	opticalSizes = []float64{8, 12, 20, 36, 60}
)

func main() {
	err := run()
	if err != nil {
		log.Fatal(err)
	}
}

func run() error {
	err := downloadFontIfNeeded()
	if err != nil {
		return err
	}

	font, err := sfnt.ReadFile(fontFile)
	if err != nil {
		return err
	}
	printAxes(font)

	paper := &pdf.Rectangle{URx: 400, URy: 400}

	page, err := document.CreateSinglePage(outputFile, paper, pdf.V2_0, nil)
	if err != nil {
		return err
	}

	// the top left corner of the grid
	left := paper.LLx + (paper.Dx()-cellWidth*float64(len(weights)))/2
	top := paper.URy - (paper.Dy()-cellHeight*float64(len(opticalSizes)))/2

	for row, opsz := range opticalSizes {
		for col, wght := range weights {
			opt := &embed.Options{
				Variations: map[string]float64{
					"opsz": opsz,
					"wght": wght,
				},
			}
			F, err := embed.OpenTypeFont(font, opt)
			if err != nil {
				return err
			}

			// centre on the letter's ink, not on its advance width
			gg := F.Layout(nil, glyphSize, sampleLetter)
			bbox := F.GetGeometry().BoundingBox(glyphSize, gg)
			x := left + (float64(col)+0.5)*cellWidth - 0.5*(bbox.LLx+bbox.URx)

			y := top - float64(row)*cellHeight - cellAscent

			page.TextSetFont(F, glyphSize)
			page.TextBegin()
			page.TextFirstLine(x, y)
			page.TextShowGlyphs(gg)
			page.TextEnd()
		}
	}

	return page.Close()
}

func downloadFontIfNeeded() error {
	if _, err := os.Stat(fontFile); err == nil {
		return nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}

	resp, err := http.Get(fontURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s: %s", fontURL, resp.Status)
	}

	// Write to a temporary file first, so that an interrupted download
	// cannot leave a truncated font behind.
	tmp, err := os.CreateTemp(".", fontFile+".*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())

	_, err = io.Copy(tmp, resp.Body)
	if err != nil {
		tmp.Close()
		return err
	}
	err = tmp.Close()
	if err != nil {
		return err
	}

	return os.Rename(tmp.Name(), fontFile)
}

func printAxes(font *sfnt.Font) {
	axes := font.VariationAxes()
	if len(axes) == 0 {
		fmt.Printf("%s is not a variable font\n", font.FamilyName)
		return
	}

	fmt.Printf("%s has %d variation axes:\n", font.FamilyName, len(axes))
	for _, axis := range axes {
		hidden := ""
		if axis.Hidden {
			hidden = ", hidden"
		}
		fmt.Printf("  %s (%s): %g to %g, default %g%s\n",
			axis.Tag, axis.Name, axis.Min, axis.Max, axis.Default, hidden)
	}
}
