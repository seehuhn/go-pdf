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

// Command mmtype1 is a throw-away program that exercises the Multiple
// Master Type 1 font pipeline end to end: it loads an MM font, prints its
// variation axes, instantiates it at several design-space coordinates, and
// embeds each instance into a single-page PDF.
package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	pst1 "seehuhn.de/go/postscript/type1"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font/embed"
	"seehuhn.de/go/pdf/internal/debug/makefont"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	psFont, err := loadMMFont()
	if err != nil {
		return err
	}

	axes := psFont.VariationAxes()
	if len(axes) == 0 {
		return fmt.Errorf("font is not a multiple master font")
	}

	fmt.Println("variation axes:")
	for _, a := range axes {
		fmt.Printf("  %s: min=%g default=%g max=%g\n", a.Name, a.Min, a.Default, a.Max)
	}

	defaults := make(map[string]float64, len(axes))
	corner := make(map[string]float64, len(axes))
	midpoint := make(map[string]float64, len(axes))
	for _, a := range axes {
		defaults[a.Name] = a.Default
		corner[a.Name] = a.Max
		midpoint[a.Name] = (a.Min + a.Max) / 2
	}

	instances := []struct {
		label  string
		coords map[string]float64 // nil selects the font's default instance
	}{
		{axisLabel(axes, defaults), nil},
		{axisLabel(axes, corner), corner},
		{axisLabel(axes, midpoint), midpoint},
	}

	doc, err := document.CreateSinglePage("test.pdf", document.A4r, pdf.V2_0, nil)
	if err != nil {
		return err
	}

	y := 500.0
	for _, inst := range instances {
		var opt *embed.Type1Options
		if inst.coords != nil {
			opt = &embed.Type1Options{Variations: inst.coords}
		}
		fontInstance, err := embed.Type1Font(psFont, nil, opt)
		if err != nil {
			return err
		}

		doc.TextSetFont(fontInstance, 24)
		doc.TextBegin()
		doc.TextFirstLine(50, y)
		doc.TextShow(inst.label)
		doc.TextEnd()

		y -= 80
	}

	return doc.Close()
}

// loadMMFont returns the MM font to use.  It prefers the first .pfb/.pfa
// file found under $QUIRE_TESTFONTS/mm/, falling back to the synthetic
// fixture from makefont.MMType1 when no such directory or file is found.
func loadMMFont() (*pst1.Font, error) {
	if dir := os.Getenv("QUIRE_TESTFONTS"); dir != "" {
		mmDir := filepath.Join(dir, "mm")

		var candidates []string
		for _, pattern := range []string{"*.pfb", "*.pfa"} {
			matches, err := filepath.Glob(filepath.Join(mmDir, pattern))
			if err != nil {
				return nil, err
			}
			candidates = append(candidates, matches...)
		}

		if len(candidates) > 0 {
			sort.Strings(candidates)
			path := candidates[0]
			fmt.Printf("loading MM font from %s\n", path)

			fd, err := os.Open(path)
			if err != nil {
				return nil, err
			}
			defer fd.Close()
			return pst1.Read(fd)
		}
	}

	fmt.Println("using synthetic MM font fixture (makefont.MMType1)")
	return makefont.MMType1(), nil
}

// axisLabel formats a design-space coordinate as "Axis1=v1 Axis2=v2: ..."
// text, e.g. "Weight=900 Width=50: Hamburgefonstiv".
func axisLabel(axes []pst1.VariationAxis, coords map[string]float64) string {
	parts := make([]string, len(axes))
	for i, a := range axes {
		parts[i] = fmt.Sprintf("%s=%g", a.Name, coords[a.Name])
	}
	return strings.Join(parts, " ") + ": Hamburgefonstiv"
}
