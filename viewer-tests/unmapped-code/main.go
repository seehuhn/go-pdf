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
	"os"

	"seehuhn.de/go/geom/matrix"

	"seehuhn.de/go/postscript/psenc"
	"seehuhn.de/go/postscript/type1"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/dict"
	"seehuhn.de/go/pdf/font/glyphdata/type1glyphs"
	"seehuhn.de/go/pdf/font/loader"
	"seehuhn.de/go/pdf/font/standard"
	"seehuhn.de/go/pdf/font/subset"
	"seehuhn.de/go/pdf/font/type3"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/content/builder"
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

	X, err := makeTestFont()
	if err != nil {
		return err
	}

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
func makeMarkerFont() font.Instance {
	// Build content stream for marker glyph
	b := builder.New(content.Glyph, nil)
	b.Type3ColoredGlyph(50, 0) // d0: colored glyph
	b.SetFillColor(color.DeviceGray(0.5))
	b.Rectangle(0, -500, 50, 2000)
	b.Fill()
	stream, err := b.Harvest()
	if err != nil {
		panic(err)
	}

	markerFont := &type3.Font{
		FontMatrix: matrix.Matrix{0.001, 0, 0, 0.001, 0, 0},
		Glyphs: []*type3.Glyph{
			{},
			{
				Name:    "I",
				Content: stream,
			},
		},
	}
	F, err := markerFont.New()
	if err != nil {
		panic(err)
	}
	return F
}

func makeTestFont() (font.Instance, error) {
	// load the original font
	ll := loader.NewFontLoader()
	r, err := ll.Open("Times-Roman", loader.FontTypeType1)
	if err != nil {
		return nil, err
	}
	F, err := type1.Read(r)
	if err != nil {
		return nil, err
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
		PostScriptName: "NimbusRoman-Regular",
		SubsetTag:      "AAAAAA",
		Descriptor:     fd,
		Encoding:       enc,
		FontFile:       type1glyphs.ToStream(F),
	}
	for code := range 256 {
		dict.Width[code] = 500
	}
	dict.Width[' '] = F.Glyphs["space"].WidthX * F.FontInfo.FontMatrix[0] * 1000
	dict.Width['A'] = F.Glyphs["AEacute"].WidthX * F.FontInfo.FontMatrix[0] * 1000
	dict.Width[0o335] = F.Glyphs["Yacute"].WidthX * F.FontInfo.FontMatrix[0] * 1000
	m := map[charcode.Code]string{
		'A':   "Ǽ",
		0o335: "Ý",
	}
	tu, err := cmap.NewToUnicodeFile(charcode.Simple, m)
	if err != nil {
		return nil, err
	}
	dict.ToUnicode = tu

	return dict.MakeFont(), nil
}
