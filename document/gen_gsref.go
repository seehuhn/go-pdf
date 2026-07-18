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

//go:build ignore

// Command gen_gsref generates the reference glyph positions used by
// TestTextShowRaw, TestTextShowRaw2, and TestVarFontShowRaw.  For each test
// case it lays out the text, records the glyph x positions, and verifies
// with ghostscript that the different ways of placing the glyphs produce
// identical images.  The validated positions are written to gsref_test.go,
// so the tests run without ghostscript.
//
// This program is the single source of truth for the test inputs: it emits the
// test string and case parameters it used alongside the validated positions, so
// the tests always run with exactly the inputs that produced the references.
package main

import (
	"bytes"
	"fmt"
	"go/format"
	"image"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"

	"seehuhn.de/go/geom/matrix"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font/truetype"
	"seehuhn.de/go/pdf/internal/debug/varfont"
	"seehuhn.de/go/pdf/internal/fonttypes"
	"seehuhn.de/go/pdf/internal/ghostscript"
)

// inputs for TestTextShowRaw
const showRawString = "CADABX"

type rawCase struct {
	fontSize float64
	m        matrix.Matrix
	stretch  float64
}

var showRawCases = []rawCase{
	{fontSize: 100, m: matrix.Identity, stretch: 1},
}

// inputs for TestTextShowRaw2
const (
	showRaw2String   = ".MiAbc"
	showRaw2FontSize = 100
)

// inputs for TestVarFontShowRaw: a non-default instance of the synthetic
// glyf variable font, whose advance width for "A" differs from the default
// instance (see internal/debug/varfont.Glyf).
const (
	varFontString   = "AAA"
	varFontFontSize = 100
)

var varFontVariations = map[string]float64{"wght": 700}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	showRaw2 := make(map[string][]float64)
	for _, v := range []pdf.Version{pdf.V1_7, pdf.V2_0} {
		for _, sample := range fonttypes.All {
			key := fmt.Sprintf("%s/%s", v, sample.Label)
			xx, err := measureShowRaw2(v, sample, showRaw2String, showRaw2FontSize)
			if err != nil {
				return fmt.Errorf("TestTextShowRaw2 %s: %w", key, err)
			}
			showRaw2[key] = xx
		}
	}

	want := make([][]float64, len(showRawCases))
	for i, c := range showRawCases {
		xx, err := measureShowRaw(pdf.String(showRawString), c.fontSize, c.m, c.stretch)
		if err != nil {
			return fmt.Errorf("TestTextShowRaw case %d: %w", i, err)
		}
		want[i] = xx
	}

	varFontWidths, err := measureVarFontShowRaw(varFontString, varFontFontSize, varFontVariations)
	if err != nil {
		return fmt.Errorf("TestVarFontShowRaw: %w", err)
	}

	return write("gsref_test.go", showRaw2, want, varFontWidths)
}

// measureShowRaw2 lays out testString with the sample font, records the glyph x
// positions, and checks with ghostscript that the sequential and
// individually-positioned renderings are pixel-identical.
func measureShowRaw2(v pdf.Version, sample *fonttypes.Sample, testString string, fontSize float64) ([]float64, error) {
	F := sample.MakeFont()
	codec := F.Codec()
	var s pdf.String

	// sequential layout, recording the x positions
	var xx []float64
	img1, err := ghostscript.RenderImage(400, 120, v, func(r *document.Page) error {
		r.TextSetFont(F, fontSize)
		r.TextBegin()
		r.TextFirstLine(10, 10)
		for _, g := range r.TextLayout(nil, testString).Seq {
			xx = append(xx, r.State.GState.TextMatrix[4])
			code, ok := F.Encode(g.GID, g.Text)
			if !ok {
				return fmt.Errorf("cannot encode glyph ID %d (%q)", g.GID, g.Text)
			}
			s = codec.AppendCode(s[:0], code)
			r.TextShowRaw(s)
		}
		r.TextEnd()
		return nil
	})
	if err != nil {
		return nil, err
	}

	// each glyph individually, placed at the recorded x positions
	img2, err := ghostscript.RenderImage(400, 120, v, func(r *document.Page) error {
		r.TextSetFont(F, fontSize)
		for i, g := range r.TextLayout(nil, testString).Seq {
			r.TextBegin()
			r.TextFirstLine(xx[i], 10)
			code, ok := F.Encode(g.GID, g.Text)
			if !ok {
				return fmt.Errorf("cannot encode glyph ID %d (%q)", g.GID, g.Text)
			}
			s = codec.AppendCode(s[:0], code)
			r.TextShowRaw(s)
			r.TextEnd()
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	if err := compareImages(img1, img2); err != nil {
		return nil, err
	}
	return xx, nil
}

// measureShowRaw renders testString in one call, glyph-by-glyph (recording the
// x positions), and once more with each glyph placed individually at the
// recorded positions.  It checks with ghostscript that all three renderings are
// pixel-identical and returns the recorded positions.
func measureShowRaw(testString pdf.String, fontSize float64, M matrix.Matrix, stretch float64) ([]float64, error) {
	F := fonttypes.CFFSimple()

	// all glyphs in one string
	img1, err := ghostscript.RenderImage(200, 120, pdf.V1_7, func(r *document.Page) error {
		r.TextBegin()
		r.TextSetFont(F, fontSize)
		r.TextSetMatrix(M)
		r.TextSetHorizontalScaling(stretch)
		r.TextFirstLine(10, 10)
		r.TextShowRaw(testString)
		r.TextEnd()
		return nil
	})
	if err != nil {
		return nil, err
	}

	// glyphs one-by-one, recording the x positions
	var xx []float64
	img2, err := ghostscript.RenderImage(200, 120, pdf.V1_7, func(r *document.Page) error {
		r.TextBegin()
		r.TextSetFont(F, fontSize)
		r.TextSetMatrix(M)
		r.TextSetHorizontalScaling(stretch)
		r.TextFirstLine(10, 10)
		for i := range testString {
			xx = append(xx, r.State.GState.TextMatrix[4])
			r.TextShowRaw(testString[i : i+1])
		}
		r.TextEnd()
		return nil
	})
	if err != nil {
		return nil, err
	}

	// each glyph at the recorded x positions
	img3, err := ghostscript.RenderImage(200, 120, pdf.V1_7, func(r *document.Page) error {
		for i := range testString {
			r.TextBegin()
			r.TextSetFont(F, fontSize)
			r.TextSetMatrix(M)
			r.TextSetHorizontalScaling(stretch)
			r.TextFirstLine(xx[i], 10)
			r.TextShowRaw(testString[i : i+1])
			r.TextEnd()
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	if err := compareImages(img1, img2); err != nil {
		return nil, fmt.Errorf("sequential vs glyph-by-glyph: %w", err)
	}
	if err := compareImages(img1, img3); err != nil {
		return nil, fmt.Errorf("sequential vs positioned: %w", err)
	}
	return xx, nil
}

// measureVarFontShowRaw lays out testString with a non-default instance of
// the synthetic glyf variable font (internal/debug/varfont.Glyf), records
// the glyph x positions, and checks with ghostscript that the sequential and
// individually-positioned renderings are pixel-identical.  This validates
// that the advance width ghostscript reads from the embedded, instanced
// font's PDF Widths array agrees with the width our own layout used to
// track position -- for the instanced glyph, not just the font's default.
func measureVarFontShowRaw(testString string, fontSize float64, variations map[string]float64) ([]float64, error) {
	F, err := truetype.NewSimple(varfont.Glyf(), &truetype.OptionsSimple{Variations: variations})
	if err != nil {
		return nil, err
	}
	codec := F.Codec()
	var s pdf.String

	// sequential layout, recording the x positions
	var xx []float64
	img1, err := ghostscript.RenderImage(400, 120, pdf.V1_7, func(r *document.Page) error {
		r.TextSetFont(F, fontSize)
		r.TextBegin()
		r.TextFirstLine(10, 10)
		for _, g := range r.TextLayout(nil, testString).Seq {
			xx = append(xx, r.State.GState.TextMatrix[4])
			code, ok := F.Encode(g.GID, g.Text)
			if !ok {
				return fmt.Errorf("cannot encode glyph ID %d (%q)", g.GID, g.Text)
			}
			s = codec.AppendCode(s[:0], code)
			r.TextShowRaw(s)
		}
		r.TextEnd()
		return nil
	})
	if err != nil {
		return nil, err
	}

	// each glyph individually, placed at the recorded x positions
	img2, err := ghostscript.RenderImage(400, 120, pdf.V1_7, func(r *document.Page) error {
		r.TextSetFont(F, fontSize)
		for i, g := range r.TextLayout(nil, testString).Seq {
			r.TextBegin()
			r.TextFirstLine(xx[i], 10)
			code, ok := F.Encode(g.GID, g.Text)
			if !ok {
				return fmt.Errorf("cannot encode glyph ID %d (%q)", g.GID, g.Text)
			}
			s = codec.AppendCode(s[:0], code)
			r.TextShowRaw(s)
			r.TextEnd()
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	if err := compareImages(img1, img2); err != nil {
		return nil, err
	}
	return xx, nil
}

func compareImages(img1, img2 image.Image) error {
	rect := img1.Bounds()
	if rect != img2.Bounds() {
		return fmt.Errorf("image bounds differ: %v, %v", rect, img2.Bounds())
	}
	for i := rect.Min.X; i < rect.Max.X; i++ {
		for j := rect.Min.Y; j < rect.Max.Y; j++ {
			r1, g1, b1, a1 := img1.At(i, j).RGBA()
			r2, g2, b2, a2 := img2.At(i, j).RGBA()
			if r1 != r2 || g1 != g2 || b1 != b2 || a1 != a2 {
				return fmt.Errorf("pixel (%d,%d) differs", i, j)
			}
		}
	}
	return nil
}

func write(fname string, showRaw2 map[string][]float64, showRaw [][]float64, varFontWidths []float64) error {
	var b bytes.Buffer

	fmt.Fprintln(&b, "// Code generated by gen_gsref.go . DO NOT EDIT.")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "package document_test")
	fmt.Fprintln(&b)
	fmt.Fprintf(&b, "var gsTextShowRawString = %q\n", showRawString)
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "// gsTextShowRaw holds, for each TestTextShowRaw case, the input parameters and")
	fmt.Fprintln(&b, "// the glyph x positions recorded during text layout.  The positions were")
	fmt.Fprintln(&b, "// validated with ghostscript at generation time; see gen_gsref.go.")
	fmt.Fprintln(&b, "var gsTextShowRaw = []textShowRawCase{")
	for i, c := range showRawCases {
		fmt.Fprintf(&b, "\t{fontSize: %s, m: [6]float64{%s}, stretch: %s, want: []float64{%s}},\n",
			formatFloat(c.fontSize), formatFloats(c.m[:]), formatFloat(c.stretch), formatFloats(showRaw[i]))
	}
	fmt.Fprintln(&b, "}")
	fmt.Fprintln(&b)
	fmt.Fprintf(&b, "var gsTextShowRaw2String = %q\n", showRaw2String)
	fmt.Fprintf(&b, "var gsTextShowRaw2FontSize float64 = %s\n", formatFloat(showRaw2FontSize))
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "// gsTextShowRaw2 holds, for each PDF version and font sample, the glyph x")
	fmt.Fprintln(&b, "// positions recorded during text layout.  The positions were validated with")
	fmt.Fprintln(&b, "// ghostscript at generation time; see gen_gsref.go.")
	fmt.Fprintln(&b, "var gsTextShowRaw2 = map[string][]float64{")
	keys := make([]string, 0, len(showRaw2))
	for k := range showRaw2 {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, key := range keys {
		fmt.Fprintf(&b, "\t%q: {%s},\n", key, formatFloats(showRaw2[key]))
	}
	fmt.Fprintln(&b, "}")
	fmt.Fprintln(&b)
	fmt.Fprintf(&b, "var gsVarFontString = %q\n", varFontString)
	fmt.Fprintf(&b, "var gsVarFontFontSize float64 = %s\n", formatFloat(varFontFontSize))
	fmt.Fprintln(&b, "var gsVarFontVariations = map[string]float64{")
	varKeys := make([]string, 0, len(varFontVariations))
	for k := range varFontVariations {
		varKeys = append(varKeys, k)
	}
	sort.Strings(varKeys)
	for _, k := range varKeys {
		fmt.Fprintf(&b, "\t%q: %s,\n", k, formatFloat(varFontVariations[k]))
	}
	fmt.Fprintln(&b, "}")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "// gsVarFontWidths holds the glyph x positions recorded during text layout")
	fmt.Fprintln(&b, "// of gsVarFontString with the synthetic glyf variable font instanced at")
	fmt.Fprintln(&b, "// gsVarFontVariations.  The positions were validated with ghostscript at")
	fmt.Fprintln(&b, "// generation time; see gen_gsref.go.")
	fmt.Fprintf(&b, "var gsVarFontWidths = []float64{%s}\n", formatFloats(varFontWidths))

	src, err := format.Source(b.Bytes())
	if err != nil {
		return fmt.Errorf("format generated source: %w", err)
	}
	return os.WriteFile(fname, src, 0o644)
}

func formatFloat(x float64) string {
	return strconv.FormatFloat(x, 'g', -1, 64)
}

func formatFloats(xx []float64) string {
	var b strings.Builder
	for i, x := range xx {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(formatFloat(x))
	}
	return b.String()
}
