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
	"seehuhn.de/go/geom/rect"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/cidfont"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/standard"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/postscript/cid"
	"seehuhn.de/go/postscript/funit"
	"seehuhn.de/go/postscript/type1"

	"seehuhn.de/go/sfnt/cff"
	"seehuhn.de/go/sfnt/glyph"
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

	opt := &pdf.WriterOptions{
		HumanReadable: true,
	}
	w, err := document.CreateSinglePage(filename, document.A5, pdf.V2_0, opt)
	if err != nil {
		return err
	}

	w.TextBegin()
	w.TextFirstLine(36, 532)
	w.TextSetFont(textFont, 10)
	w.SetFillColor(black)
	w.TextShow("The glyphs in the test font (red) are mapped using two different code ranges:")
	w.TextSecondLine(0, -14)
	w.TextShow("[")
	w.TextSetFont(testFont, 10)
	w.SetFillColor(red)
	w.TextShowRaw(pdf.String("ABC"))
	w.TextSetFont(textFont, 10)
	w.SetFillColor(black)
	w.TextShow("]")
	w.TextSecondLine(0, -12)
	w.TextShow("[")
	w.TextSetFont(testFont, 10)
	w.SetFillColor(red)
	w.TextShowRaw(pdf.String("abc"))
	w.TextSetFont(textFont, 10)
	w.SetFillColor(black)
	w.TextShow("]")
	w.TextSecondLine(0, -14)
	w.TextShow("These three glyphs are assigned CIDs 0, 1, and 2.")
	w.TextSecondLine(0, -13)
	w.TextShow("The CMap embedded in the PDF file also maps CIDs 3 and 4,")
	w.TextNextLine()
	w.TextShow("and the CIDFont dictionary assigns widths to all five CIDs.")
	w.TextNextLine()
	w.TextShow("The assigned widths are 1000, 3000, 1000, 2000 and 4000.")
	w.TextNextLine()
	w.TextShow("Notdef ranges in the CMap are used to assign custom notdef characters")
	w.TextNextLine()
	w.TextShow("for some codes.")
	w.TextEnd()

	w.TextBegin()
	w.TextFirstLine(36, 370)
	w.TextSetFont(textFont, 10)
	w.SetFillColor(black)
	w.TextShow("A character code:")
	w.TextSecondLine(0, -14)
	w.TextShow("[")
	w.TextSetFont(testFont, 10)
	w.SetFillColor(red)
	w.TextShowRaw(pdf.String("!"))
	w.TextSetFont(textFont, 10)
	w.SetFillColor(black)
	w.TextShow("]")
	w.TextEnd()

	w.TextBegin()
	w.TextFirstLine(36, 320)
	w.TextSetFont(textFont, 10)
	w.SetFillColor(black)
	w.TextShow("A valid, unmapped code:")
	w.TextSecondLine(0, -14)
	w.TextShow("[")
	w.TextSetFont(testFont, 10)
	w.SetFillColor(red)
	w.TextShowRaw(pdf.String("X"))
	w.TextSetFont(textFont, 10)
	w.SetFillColor(black)
	w.TextShow("]")
	w.TextEnd()

	w.TextBegin()
	w.TextFirstLine(36, 270)
	w.TextSetFont(textFont, 10)
	w.SetFillColor(black)
	w.TextShow("A valid code, mapped to CID 3 (missing):")
	w.TextSecondLine(0, -14)
	w.TextShow("[")
	w.TextSetFont(testFont, 10)
	w.SetFillColor(red)
	w.TextShowRaw(pdf.String("D"))
	w.TextSetFont(textFont, 10)
	w.SetFillColor(black)
	w.TextShow("]")
	w.TextEnd()

	w.TextBegin()
	w.TextFirstLine(36, 220)
	w.TextSetFont(textFont, 10)
	w.SetFillColor(black)
	w.TextShow("A valid, unmapped code, notdef = CID 1:")
	w.TextSecondLine(0, -14)
	w.TextShow("[")
	w.TextSetFont(testFont, 10)
	w.SetFillColor(red)
	w.TextShowRaw(pdf.String("x"))
	w.TextSetFont(textFont, 10)
	w.SetFillColor(black)
	w.TextShow("]")
	w.TextEnd()

	w.TextBegin()
	w.TextFirstLine(36, 170)
	w.TextSetFont(textFont, 10)
	w.SetFillColor(black)
	w.TextShow("A valid code, mapped to CID 3 (missing), notdef = CID 1:")
	w.TextSecondLine(0, -14)
	w.TextShow("[")
	w.TextSetFont(testFont, 10)
	w.SetFillColor(red)
	w.TextShowRaw(pdf.String("d"))
	w.TextSetFont(textFont, 10)
	w.SetFillColor(black)
	w.TextShow("]")
	w.TextEnd()

	w.TextBegin()
	w.TextFirstLine(36, 120)
	w.TextSetFont(textFont, 10)
	w.SetFillColor(black)
	w.TextShow("A valid, unmapped code, notdef = CID 4 (missing):")
	w.TextSecondLine(0, -14)
	w.TextShow("[")
	w.TextSetFont(testFont, 10)
	w.SetFillColor(red)
	w.TextShowRaw(pdf.String("z"))
	w.TextSetFont(textFont, 10)
	w.SetFillColor(black)
	w.TextShow("]")
	w.TextEnd()

	w.TextBegin()
	w.TextFirstLine(36, 70)
	w.TextSetFont(textFont, 10)
	w.SetFillColor(black)
	w.TextShow("A valid code, mapped to CID 3 (missing), notdef = CID 4 (missing):")
	w.TextSecondLine(0, -14)
	w.TextShow("[")
	w.TextSetFont(testFont, 10)
	w.SetFillColor(red)
	w.TextShowRaw(pdf.String("y"))
	w.TextSetFont(textFont, 10)
	w.SetFillColor(black)
	w.TextShow("]")
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
				BlueValues: []funit.Int16{-100, 0, 900, 1000},
				BlueScale:  0,
				BlueShift:  0,
				BlueFuzz:   0,
				StdHW:      100,
				StdVW:      100,
				ForceBold:  false,
			},
		},
		FDSelect: func(glyph.ID) int {
			return 0
		},
		ROS:      testCharacterCollection,
		GIDToCID: []cid.CID{0, 1, 2}, // identity GID <-> CID mapping
	}
	fontCFF := &cff.Font{
		FontInfo: &type1.FontInfo{
			FontName:   "Test",
			FontMatrix: [6]float64{0.001, 0, 0, 0.001, 0, 0},
		},
		Outlines: o,
	}

	res := &testFont{
		data: fontCFF,
	}
	return res
}

var testCharacterCollection = &cff.CIDSystemInfo{
	Registry:   "seehuhn.de",
	Ordering:   "Test",
	Supplement: 0,
}

var _ font.Font = (*testFont)(nil)

type testFont struct {
	data *cff.Font
}

func (f *testFont) PostScriptName() string {
	return "Test"
}

func (f *testFont) Embed(rm *pdf.ResourceManager) (pdf.Native, font.Embedded, error) {
	fontDictRef := rm.Out.Alloc()

	fd := &font.Descriptor{
		FontName:   "Test",
		FontFamily: "Test",
		FontBBox:   rect.Rect{LLx: 0, LLy: 0, URx: 3000, URy: 1000},
		Ascent:     1000,
		Descent:    0,
		Leading:    1000,
		CapHeight:  1000,
		XHeight:    500,
	}
	dicts := &cidfont.Type0Dict{
		Ref:            fontDictRef,
		PostScriptName: "Test",
		Descriptor:     fd,
		Encoding: &cmap.InfoNew{
			Name: "TestCMap",
			ROS: &cmap.CIDSystemInfo{
				Registry:   testCharacterCollection.Registry,
				Ordering:   testCharacterCollection.Ordering,
				Supplement: pdf.Integer(testCharacterCollection.Supplement),
			},
			WMode: cmap.Horizontal,
			CodeSpaceRange: []charcode.Range{
				{Low: []byte{'A'}, High: []byte{'Z'}},
				{Low: []byte{'a'}, High: []byte{'z'}},
			},
			CIDRanges: []cmap.RangeNew{
				// Map all glyphs twice (including the missing CIDs 3 and 4).
				{First: []byte{'A'}, Last: []byte{'E'}, Value: 0},
				{First: []byte{'a'}, Last: []byte{'e'}, Value: 0},
			},
			CIDSingles: []cmap.SingleNew{
				{Code: []byte{'y'}, Value: 3},
			},
			NotdefRanges: []cmap.RangeNew{
				// Codes 'y' and 'z' use a non-existent glyph as the notdef character.
				{First: []byte{'y'}, Last: []byte{'z'}, Value: 4},
				// For the rest of the lowercase range we use the alternative
				// notdef glyph (CID 1).
				{First: []byte{'a'}, Last: []byte{'x'}, Value: 1},
			},
		},
		Width: map[cmap.CID]float64{
			0: 1000,
			1: 3000,
			2: 1000,
			3: 2000,
			4: 4000,
		},
		DefaultWidth: 1000,
		Text: &cmap.ToUnicodeInfo{
			CodeSpaceRange: []charcode.Range{
				{Low: []byte{'A'}, High: []byte{'Z'}},
				{Low: []byte{'a'}, High: []byte{'z'}},
			},
			Ranges: []cmap.ToUnicodeRange{
				{First: []byte{'A'}, Last: []byte{'Z'}, Values: [][]rune{{'A'}}},
				{First: []byte{'a'}, Last: []byte{'z'}, Values: [][]rune{{'a'}}},
			},
		},
		GetFont: func() (cidfont.Type0FontData, error) {
			return f.data, nil
		},
	}
	err := dicts.Finish(rm)
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
	default:
		return 1000, 1
	}
}
