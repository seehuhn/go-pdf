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

// Command varfonts is a throw-away acceptance program for the font-variations
// project: it exercises TrueType, CFF2 and Multiple Master Type 1 variable
// fonts end to end, instancing each at several design-space coordinates and
// embedding all of them into a single PDF page.
package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"seehuhn.de/go/sfnt"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cff"
	"seehuhn.de/go/pdf/font/embed"
	"seehuhn.de/go/pdf/font/truetype"
	"seehuhn.de/go/pdf/internal/debug/makefont"
	"seehuhn.de/go/pdf/internal/debug/varfont"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	ttFont, ttSample, err := loadJunicode()
	if err != nil {
		return err
	}

	axes := ttFont.VariationAxes()
	fmt.Println("Junicode variation axes:")
	if len(axes) == 0 {
		fmt.Println("  (none -- static fallback font)")
	}
	for _, a := range axes {
		fmt.Printf("  %s (%s): min=%g default=%g max=%g\n", a.Tag, a.Name, a.Min, a.Default, a.Max)
	}

	doc, err := document.CreateSinglePage("test.pdf", document.A4r, pdf.V2_0, nil)
	if err != nil {
		return err
	}

	const size = 18.0
	const step = 60.0
	y := 540.0

	addLine := func(label string, inst font.Layouter, text string) {
		doc.TextSetFont(inst, size)
		doc.TextBegin()
		doc.TextFirstLine(50, y)
		doc.TextShow(label + ": " + text)
		doc.TextEnd()
		y -= step
	}

	// Junicode simple, at defaults, a mixed instance, and an extreme instance.
	for _, inst := range axisInstances(axes) {
		simple, err := truetype.NewSimple(ttFont, &truetype.OptionsSimple{Variations: inst.coords})
		if err != nil {
			return err
		}
		addLine("TT simple "+inst.label, simple, ttSample)
	}

	// Junicode composite, at wght=300 (or the lowest axis value as fallback).
	compCoords := singleAxisCoords(axes, "wght", 300)
	composite, err := truetype.NewComposite(ttFont, &truetype.OptionsComposite{Variations: compCoords})
	if err != nil {
		return err
	}
	addLine("TT composite "+formatCoords(axes, compCoords), composite, ttSample)

	// synthetic variable CFF2 font, at two instances.
	cff2Font := varfont.CFF2()
	for _, inst := range []instanceSpec{
		{"default", nil},
		{"wght=900", map[string]float64{"wght": 900}},
	} {
		simple, err := cff.NewSimple(cff2Font, &cff.OptionsSimple{Variations: inst.coords})
		if err != nil {
			return err
		}
		addLine("CFF2 simple "+inst.label, simple, "A")
	}

	// synthetic Multiple Master Type 1 font, at defaults and one corner.
	mmFont := makefont.MMType1()
	mmAxes := mmFont.VariationAxes()
	corner := make(map[string]float64, len(mmAxes))
	for _, a := range mmAxes {
		corner[a.Name] = a.Max
	}
	for _, inst := range []instanceSpec{
		{"defaults", nil},
		{"corner", corner},
	} {
		var opt *embed.Type1Options
		if inst.coords != nil {
			opt = &embed.Type1Options{Variations: inst.coords}
		}
		t1, err := embed.Type1Font(mmFont, nil, opt)
		if err != nil {
			return err
		}
		addLine("MM Type1 "+inst.label, t1, "Hamburgefonstiv")
	}

	return doc.Close()
}

// loadJunicode returns the variable TrueType font to use, together with a
// sample string that exercises its cmap.  It prefers Junicode-VF.ttf under
// $QUIRE_TESTFONTS (default $HOME/testfonts), falling back to the synthetic
// glyf fixture from varfont.Glyf when the file is not present.
func loadJunicode() (*sfnt.Font, string, error) {
	dir := os.Getenv("QUIRE_TESTFONTS")
	if dir == "" {
		if home, err := os.UserHomeDir(); err == nil {
			dir = filepath.Join(home, "testfonts")
		}
	}
	if dir != "" {
		path := filepath.Join(dir, "Junicode-VF.ttf")
		if _, err := os.Stat(path); err == nil {
			f, err := sfnt.ReadFile(path)
			if err != nil {
				return nil, "", err
			}
			fmt.Printf("loaded variable TrueType font from %s\n", path)
			return f, "Hamburgefonstiv", nil
		}
	}

	fmt.Println("Junicode-VF.ttf not found under $QUIRE_TESTFONTS; using synthetic glyf fixture (varfont.Glyf)")
	return varfont.Glyf(), "A", nil
}

// instanceSpec names one design-space coordinate to instance a font at.
// A nil coords map selects the font's default instance.
type instanceSpec struct {
	label  string
	coords map[string]float64
}

// axisInstances builds the "defaults / mixed / extreme" instance list for
// the Junicode simple font.  It adapts to whichever axes are actually
// present, so the synthetic wght-only fallback font also works: wght=700 and
// wdth=87.5 are used when those axes exist, the ENLA axis is driven to its
// maximum for the extreme instance, and otherwise the first/last axis
// substitutes.
func axisInstances(axes []sfnt.VariationAxis) []instanceSpec {
	find := func(tag string) (sfnt.VariationAxis, bool) {
		for _, a := range axes {
			if a.Tag == tag {
				return a, true
			}
		}
		return sfnt.VariationAxis{}, false
	}

	out := []instanceSpec{{"defaults", nil}}

	mixed := map[string]float64{}
	if _, ok := find("wght"); ok {
		mixed["wght"] = 700
	}
	if _, ok := find("wdth"); ok {
		mixed["wdth"] = 87.5
	}
	if len(mixed) == 0 && len(axes) > 0 {
		mixed[axes[0].Tag] = axes[0].Max
	}
	out = append(out, instanceSpec{formatCoords(axes, mixed), mixed})

	extreme := map[string]float64{}
	if a, ok := find("ENLA"); ok {
		extreme[a.Tag] = a.Max
	} else if len(axes) > 0 {
		last := axes[len(axes)-1]
		extreme[last.Tag] = last.Max
	}
	out = append(out, instanceSpec{formatCoords(axes, extreme), extreme})

	return out
}

// singleAxisCoords pins tag to val if the font has that axis, or falls back
// to the minimum of the first axis otherwise.
func singleAxisCoords(axes []sfnt.VariationAxis, tag string, val float64) map[string]float64 {
	for _, a := range axes {
		if a.Tag == tag {
			return map[string]float64{tag: val}
		}
	}
	if len(axes) > 0 {
		return map[string]float64{axes[0].Tag: axes[0].Min}
	}
	return nil
}

// formatCoords renders a coordinate map as "tag1=v1 tag2=v2", in axis order.
func formatCoords(axes []sfnt.VariationAxis, coords map[string]float64) string {
	if len(coords) == 0 {
		return "defaults"
	}
	parts := make([]string, 0, len(coords))
	for _, a := range axes {
		if v, ok := coords[a.Tag]; ok {
			parts = append(parts, fmt.Sprintf("%s=%g", a.Tag, v))
		}
	}
	return strings.Join(parts, " ")
}
