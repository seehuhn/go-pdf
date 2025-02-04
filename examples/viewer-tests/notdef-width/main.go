// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2025  Jochen Voss <voss@seehuhn.de>
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
	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/geom/rect"

	"seehuhn.de/go/postscript/cid"
	"seehuhn.de/go/postscript/funit"
	"seehuhn.de/go/postscript/type1"

	"seehuhn.de/go/sfnt/cff"
	"seehuhn.de/go/sfnt/glyph"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/dict"
	"seehuhn.de/go/pdf/font/glyphdata"
	"seehuhn.de/go/pdf/font/glyphdata/cffglyphs"
	"seehuhn.de/go/pdf/font/standard"
	"seehuhn.de/go/pdf/graphics/color"
)

func main() {
	err := writeTestFile("test.pdf")
	if err != nil {
		panic(err)
	}
}

func writeTestFile(filename string) error {
	textFont := standard.Helvetica.Must(nil)
	testFont := makeTestFont()

	black := color.DeviceGray(0)
	red := color.DeviceRGB(0.9, 0, 0)
	gray1 := color.DeviceGray(0.9)
	gray2 := color.DeviceGray(0.75)

	opt := &pdf.WriterOptions{
		HumanReadable: true,
	}
	w, err := document.CreateSinglePage(filename, document.A5, pdf.V2_0, opt)
	if err != nil {
		return err
	}

	w.TextBegin()
	w.TextFirstLine(36, 550)
	w.TextSetFont(textFont, 10)
	w.SetFillColor(black)
	w.TextShow("The glyphs in the test font (red) are mapped using two different code ranges:")
	w.TextSecondLine(0, -14)
	w.TextShow("[")
	xBase, _ := w.GetTextPositionUser() // record the current horizontal position
	w.TextSetFont(testFont, 10)
	w.SetFillColor(red)
	w.TextShowRaw(pdf.String("ABC"))
	w.TextSetFont(textFont, 10)
	w.SetFillColor(black)
	w.TextShow("] codes: A B C, code space range A–Z")
	w.TextSecondLine(0, -12)
	w.TextShow("[")
	w.TextSetFont(testFont, 10)
	w.SetFillColor(red)
	w.TextShowRaw(pdf.String("abc"))
	w.TextSetFont(textFont, 10)
	w.SetFillColor(black)
	w.TextShow("] codes: a b c, code space range a–z")
	w.TextSecondLine(0, -14)
	w.TextShow("These three glyphs are assigned CIDs 0, 1, and 2.")
	w.TextSecondLine(0, -13)
	w.TextShow("The CMap embedded in the PDF file also maps CIDs 3 and 4,")
	w.TextNextLine()
	w.TextShow("which are missing from the embedded font.")
	w.TextNextLine()
	w.TextNextLine()
	w.TextShow("The CIDFont dictionary assigns widths to CIDs 0, 1, …, 4.")
	w.TextNextLine()
	w.TextShow("The assigned widths are 1000, 3000, 1000, 2000 and 4000.")
	w.TextNextLine()
	w.TextNextLine()
	w.TextShow("Notdef ranges in the CMap are used to assign custom notdef characters")
	w.TextNextLine()
	w.TextShow("for some code ranges: CID 1 for a–x, CID 4 for y–z.")
	w.TextEnd()

	w.SetStrokeColor(gray1)
	w.SetLineWidth(0.5)
	for _, step := range []float64{0, 10, 20, 30, 40} {
		w.MoveTo(xBase+step, 385)
		w.LineTo(xBase+step, 45)
	}
	w.Stroke()

	w.TextBegin()
	w.TextFirstLine(36, 370)
	w.TextSetFont(textFont, 10)
	w.SetFillColor(black)
	w.TextShow("Test 1: invalid character code")
	w.TextSecondLine(0, -14)
	w.TextShow("[")
	w.TextSetFont(testFont, 10)
	w.SetFillColor(red)
	w.TextShowRaw(pdf.String("!"))
	w.TextSetFont(textFont, 10)
	w.SetFillColor(black)
	w.TextShow("] code: !")
	w.TextEnd()

	w.TextBegin()
	w.TextFirstLine(36, 320)
	w.TextSetFont(textFont, 10)
	w.SetFillColor(black)
	w.TextShow("Test 2: valid, unmapped code")
	w.TextSecondLine(0, -14)
	w.TextShow("[")
	w.TextSetFont(testFont, 10)
	w.SetFillColor(red)
	w.TextShowRaw(pdf.String("X"))
	w.TextSetFont(textFont, 10)
	w.SetFillColor(black)
	w.TextShow("] code: X")
	w.TextEnd()

	w.TextBegin()
	w.TextFirstLine(36, 270)
	w.TextSetFont(textFont, 10)
	w.SetFillColor(black)
	w.TextShow("Test 3: valid code, mapped to CID 3 (missing)")
	w.TextSecondLine(0, -14)
	w.TextShow("[")
	w.TextSetFont(testFont, 10)
	w.SetFillColor(red)
	w.TextShowRaw(pdf.String("D"))
	w.TextSetFont(textFont, 10)
	w.SetFillColor(black)
	w.TextShow("] code: D")
	w.TextEnd()

	w.TextBegin()
	w.TextFirstLine(36, 220)
	w.TextSetFont(textFont, 10)
	w.SetFillColor(black)
	w.TextShow("Test 4: valid, unmapped code; notdef = CID 1")
	w.TextSecondLine(0, -14)
	w.TextShow("[")
	w.TextSetFont(testFont, 10)
	w.SetFillColor(red)
	w.TextShowRaw(pdf.String("x"))
	w.TextSetFont(textFont, 10)
	w.SetFillColor(black)
	w.TextShow("] code: x")
	w.TextEnd()

	w.TextBegin()
	w.TextFirstLine(36, 170)
	w.TextSetFont(textFont, 10)
	w.SetFillColor(black)
	w.TextShow("Test 5: valid code, mapped to CID 3 (missing); notdef = CID 1")
	w.TextSecondLine(0, -14)
	w.TextShow("[")
	w.TextSetFont(testFont, 10)
	w.SetFillColor(red)
	w.TextShowRaw(pdf.String("d"))
	w.TextSetFont(textFont, 10)
	w.SetFillColor(black)
	w.TextShow("] code: d")
	w.TextEnd()

	w.TextBegin()
	w.TextFirstLine(36, 120)
	w.TextSetFont(textFont, 10)
	w.SetFillColor(black)
	w.TextShow("Test 6: valid, unmapped code; notdef = CID 4 (missing)")
	w.TextSecondLine(0, -14)
	w.TextShow("[")
	w.TextSetFont(testFont, 10)
	w.SetFillColor(red)
	w.TextShowRaw(pdf.String("z"))
	w.TextSetFont(textFont, 10)
	w.SetFillColor(black)
	w.TextShow("] code: z")
	w.TextEnd()

	w.TextBegin()
	w.TextFirstLine(36, 70)
	w.TextSetFont(textFont, 10)
	w.SetFillColor(black)
	w.TextShow("test 7: valid code, mapped to CID 3 (missing); notdef = CID 4 (missing)")
	w.TextSecondLine(0, -14)
	w.TextShow("[")
	w.TextSetFont(testFont, 10)
	w.SetFillColor(red)
	w.TextShowRaw(pdf.String("y"))
	w.TextSetFont(textFont, 10)
	w.SetFillColor(black)
	w.TextShow("] code: y")
	w.TextEnd()

	w.Transform(matrix.Translate(xBase-2.5, 43))
	w.Transform(matrix.RotateDeg(-90))
	w.TextBegin()
	w.TextSetFont(textFont, 7)
	w.SetFillColor(gray2)
	w.TextShow("0")
	w.TextSecondLine(0, 10)
	w.TextShow("1000")
	w.TextNextLine()
	w.TextShow("2000")
	w.TextNextLine()
	w.TextShow("3000")
	w.TextNextLine()
	w.TextShow("4000")
	w.TextEnd()

	return w.Close()
}

func makeTestFont() *testFont {
	var glyphs []*cff.Glyph

	// CID 0: a 800x800 square, glyph width 1000 (regular notdef glyph)
	g := &cff.Glyph{
		Width: 1000,
	}
	g.MoveTo(0, 0)
	g.LineTo(800, 0)
	g.LineTo(800, 800)
	g.LineTo(0, 800)
	glyphs = append(glyphs, g)

	// CID 1: a 2800x500 rectangle, glyph width 3000 (alternate notdef glyph)
	g = &cff.Glyph{
		Width: 3000,
	}
	g.MoveTo(0, 0)
	g.LineTo(2800, 0)
	g.LineTo(2800, 500)
	g.LineTo(0, 500)
	glyphs = append(glyphs, g)

	// CID 2: a 800x100 rectangle, glyph width 1000 (regular glyph)
	g = &cff.Glyph{
		Width: 1000,
	}
	g.MoveTo(0, 0)
	g.LineTo(800, 0)
	g.LineTo(800, 100)
	g.LineTo(0, 100)
	glyphs = append(glyphs, g)

	// CID 3 is referenced in the CMap but missing in the font, glyph width 2000

	// CID 4 is referenced in the CMap but missing in the font, glyph width 4000

	o := &cff.Outlines{
		Glyphs: glyphs,
		Private: []*type1.PrivateDict{
			{
				BlueValues: []funit.Int16{-10, 0, 990, 1000},
				BlueScale:  0.039625,
				BlueShift:  7,
				BlueFuzz:   1,
				StdHW:      100,
				StdVW:      100,
				ForceBold:  false,
			},
		},
		FDSelect: func(glyph.ID) int {
			return 0
		},
		ROS: &cid.SystemInfo{
			Registry:   "seehuhn.de",
			Ordering:   "test",
			Supplement: 0,
		},
		GIDToCID:     []cid.CID{0, 1, 2}, // identity GID <-> CID mapping
		FontMatrices: []matrix.Matrix{matrix.Identity},
	}
	fontCFF := &cff.Font{
		FontInfo: &type1.FontInfo{
			FontName:   "Test",
			FontMatrix: [6]float64{0.001, 0, 0, 0.001, 0, 0},
		},
		Outlines: o,
	}

	cmap := &cmap.File{
		Name: "TestCMap",
		ROS: &cmap.CIDSystemInfo{
			Registry:   o.ROS.Registry,
			Ordering:   o.ROS.Ordering,
			Supplement: pdf.Integer(o.ROS.Supplement),
		},
		WMode: cmap.Horizontal,
		CodeSpaceRange: []charcode.Range{
			{Low: []byte{'A'}, High: []byte{'Z'}},
			{Low: []byte{'a'}, High: []byte{'z'}},
		},
		CIDRanges: []cmap.Range{
			// Two mappings for each glyph (including the missing CIDs 3 and 4).
			{First: []byte{'A'}, Last: []byte{'E'}, Value: 0},
			{First: []byte{'a'}, Last: []byte{'e'}, Value: 0},
		},
		CIDSingles: []cmap.Single{
			{Code: []byte{'y'}, Value: 3},
		},
		NotdefRanges: []cmap.Range{
			// Codes 'y' and 'z' use a non-existent glyph as the notdef character.
			{First: []byte{'y'}, Last: []byte{'z'}, Value: 4},
			// For the rest of the lowercase range we use the alternative
			// notdef glyph (CID 1).
			{First: []byte{'a'}, Last: []byte{'x'}, Value: 1},
		},
	}

	res := &testFont{
		data: fontCFF,
		cmap: cmap,
	}
	return res
}

var _ font.Font = (*testFont)(nil)

type testFont struct {
	data *cff.Font
	cmap *cmap.File
}

func (f *testFont) PostScriptName() string {
	return f.data.FontName
}

func (f *testFont) Embed(rm *pdf.ResourceManager) (pdf.Native, font.Embedded, error) {
	fontDictRef := rm.Out.Alloc()

	fontType := glyphdata.CFF
	fontRef := rm.Out.Alloc()
	err := cffglyphs.Embed(rm.Out, fontType, fontRef, f.data)
	if err != nil {
		return nil, nil, err
	}

	fd := &font.Descriptor{
		FontName:   f.data.FontName,
		IsSymbolic: true,
		FontBBox:   rect.Rect{LLx: 0, LLy: 0, URx: 3000, URy: 1000},
		Ascent:     800,
		CapHeight:  800,
	}
	dicts := &dict.CIDFontType0{
		Ref:            fontDictRef,
		PostScriptName: f.data.FontName,
		Descriptor:     fd,
		ROS:            f.cmap.ROS,
		Encoding:       f.cmap,
		Width: map[cmap.CID]float64{
			0: 1000,
			1: 3000,
			2: 1000,
			3: 2000,
			4: 4000,
		},
		DefaultWidth: 1000,
		FontType:     fontType,
		FontRef:      fontRef,
	}
	err = dicts.WriteToPDF(rm)
	if err != nil {
		return nil, nil, err
	}

	e := &testFontEmbedded{
		ref: fontDictRef,
	}
	return fontDictRef, e, nil
}

type testFontEmbedded struct {
	ref pdf.Reference
}

func (e *testFontEmbedded) WritingMode() cmap.WritingMode {
	return cmap.Horizontal
}

func (e *testFontEmbedded) DecodeWidth(s pdf.String) (float64, int) {
	if len(s) == 0 {
		return 0, 0
	}

	switch s[0] {
	case 'B', 'b':
		return 3000, 1
	case 'D', 'd':
		return 2000, 1
	case 'E', 'e':
		return 4000, 1
	default:
		return 1000, 1
	}
}
