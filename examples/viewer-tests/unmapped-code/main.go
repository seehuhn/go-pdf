// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2024  Jochen Voss <voss@seehuhn.de>
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
	"fmt"
	"iter"
	"os"

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/geom/rect"

	"seehuhn.de/go/postscript/psenc"
	"seehuhn.de/go/postscript/type1"

	"seehuhn.de/go/sfnt/glyph"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/dict"
	"seehuhn.de/go/pdf/font/glyphdata"
	"seehuhn.de/go/pdf/font/glyphdata/type1glyphs"
	"seehuhn.de/go/pdf/font/loader"
	"seehuhn.de/go/pdf/font/standard"
	"seehuhn.de/go/pdf/font/subset"
	"seehuhn.de/go/pdf/font/type3"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
)

func main() {
	err := createDocument("test.pdf")
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func createDocument(fname string) error {
	paper := document.A5r
	opt := &pdf.WriterOptions{
		HumanReadable: true,
	}
	page, err := document.CreateSinglePage(fname, paper, pdf.V2_0, opt)
	if err != nil {
		return err
	}

	F := standard.Helvetica.New()

	X := &testFont{}

	M := makeMarkerFont()

	yPos := paper.URy - 72 - 15

	type showBlock struct {
		label string
		code  pdf.String
	}
	blocks := []showBlock{
		{"code used in WinAnsi encoding:", pdf.String{0o335}},
		{"remapped code:", pdf.String{'A'}},
		{"space character:", pdf.String{' '}},
		{"code mapped to non-existent character:", pdf.String{'B'}},
		{"unmapped code:", pdf.String{0o010}},
	}

	page.TextBegin()
	for i, b := range blocks {
		switch i {
		case 0:
			page.TextFirstLine(72, yPos)
		case 1:
			page.TextSecondLine(0, -20)
		default:
			page.TextNextLine()
		}
		page.TextSetFont(F, 10)
		gg := page.TextLayout(nil, b.label)
		gg.Align(190, 0)
		page.TextShowGlyphs(gg)
		page.TextSetFont(M, 10)
		page.TextShow("I")
		page.TextSetFont(X, 10)
		page.TextShowRaw(b.code)
		page.TextSetFont(M, 10)
		page.TextShow("I")
	}
	page.TextEnd()

	return page.Close()
}

// makeMarkerFont creates a simple type3 font where "I" shows a gray, vertical
// line.
func makeMarkerFont() font.Font {
	markerFont := &type3.Font{
		FontMatrix: matrix.Matrix{0.001, 0, 0, 0.001, 0, 0},
		Glyphs: []*type3.Glyph{
			{},
			{
				Name:  "I",
				Width: 50,
				BBox:  rect.Rect{LLy: -500, URx: 50, URy: 1500},
				Color: true,
				Draw: func(w *graphics.Writer) {
					w.SetFillColor(color.DeviceGray(0.5))
					w.Rectangle(0, -500, 50, 2000)
					w.Fill()
				},
			},
		},
	}
	F := &type3.Instance{
		Font: markerFont,
		CMap: map[rune]glyph.ID{
			'I': 1,
		},
	}
	return F
}

// This is a modified version of the Times-Roman font,
// which only contains the following glyphs: .notdef, space, Yacute, AEacute.
type testFont struct{}

func (f *testFont) PostScriptName() string {
	return "Test"
}

// GetGeometry returns font metrics required for typesetting.
func (f *testFont) GetGeometry() *font.Geometry {
	panic("not implemented") // TODO: Implement
}

func (f *testFont) Embed(rm *pdf.ResourceManager) (pdf.Native, font.Embedded, error) {
	w := rm.Out
	fontDictRef := w.Alloc()

	// load the original font
	ll := loader.NewFontLoader()
	r, err := ll.Open("Times-Roman", loader.FontTypeType1)
	if err != nil {
		return nil, nil, err
	}
	F, err := type1.Read(r)
	if err != nil {
		return nil, nil, err
	}

	// copy the glyphs we want to keep
	oldGlyphs := F.Glyphs
	F.Glyphs = make(map[string]*type1.Glyph)
	F.Glyphs["space"] = oldGlyphs["space"]
	F.Glyphs["Yacute"] = oldGlyphs["Yacute"]
	F.Glyphs["AEacute"] = oldGlyphs["AEacute"]

	// Since the original font has a blank .notdef glyph, we draw one manually.
	notdef := &type1.Glyph{
		WidthX: 500,
	}
	notdef.MoveTo(10, 10)
	notdef.LineTo(490, 10)
	notdef.LineTo(490, 990)
	notdef.LineTo(10, 990)
	notdef.ClosePath()
	notdef.MoveTo(100, 100)
	notdef.LineTo(100, 900)
	notdef.LineTo(400, 900)
	notdef.LineTo(400, 100)
	notdef.ClosePath()
	F.Glyphs[".notdef"] = notdef

	F.Encoding = psenc.StandardEncoding[:]

	enc := func(code byte) string {
		switch code {
		case ' ':
			return "space"
		case 'A':
			return "AEacute"
		case 'B':
			return "does.not.exist"
		case 0o335:
			return "Yacute"
		default:
			return ""
		}
	}

	fd := &font.Descriptor{
		FontName:     subset.Join("AAAAAA", "NimbusRoman-Regular"),
		IsSerif:      true,
		FontBBox:     F.FontBBoxPDF(),
		Ascent:       1000,
		Descent:      100,
		Leading:      2000,
		CapHeight:    1000,
		XHeight:      500,
		MissingWidth: 500,
	}
	dict := dict.Type1{
		Ref:            fontDictRef,
		PostScriptName: "NimbusRoman-Regular",
		SubsetTag:      "AAAAAA",
		Descriptor:     fd,
		Encoding:       enc,
		FontType:       glyphdata.Type1,
		FontRef:        w.Alloc(),
	}
	for code := range 256 {
		dict.Width[code] = 500
	}
	dict.Width[' '] = F.Glyphs["space"].WidthX * F.FontInfo.FontMatrix[0] * 1000
	dict.Width['A'] = F.Glyphs["AEacute"].WidthX * F.FontInfo.FontMatrix[0] * 1000
	dict.Width[0o335] = F.Glyphs["Yacute"].WidthX * F.FontInfo.FontMatrix[0] * 1000
	dict.Text[' '] = " "
	dict.Text['A'] = "Ǽ"
	dict.Text[0o335] = "Ý"

	err = dict.WriteToPDF(rm)
	if err != nil {
		return nil, nil, err
	}

	err = type1glyphs.Embed(w, dict.FontType, dict.FontRef, F)
	if err != nil {
		return nil, nil, err
	}

	res := &testFontEmbedded{
		W: dict.Width[:],
	}
	return fontDictRef, res, nil
}

type testFontEmbedded struct {
	W []float64
}

func (f *testFontEmbedded) WritingMode() font.WritingMode {
	return 0
}

func (f *testFontEmbedded) Codes(s pdf.String) iter.Seq[*font.Code] {
	panic("not implemented") // TODO: Implement
}

// DecodeWidth reads one character code from the given string and returns
// the width of the corresponding glyph.
// This implements the [font.Embedded] interface.
func (f *testFontEmbedded) DecodeWidth(s pdf.String) (float64, int) {
	return f.W[s[0]] / 1000, 1
}
