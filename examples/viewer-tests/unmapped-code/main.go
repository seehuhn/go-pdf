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
	"log"

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/geom/rect"

	"seehuhn.de/go/postscript/psenc"
	"seehuhn.de/go/postscript/type1"
	"seehuhn.de/go/sfnt/glyph"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/loader"
	"seehuhn.de/go/pdf/font/standard"
	"seehuhn.de/go/pdf/font/type3"
	"seehuhn.de/go/pdf/font/widths"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
)

func main() {
	err := run()
	if err != nil {
		log.Fatal(err)
	}
}

func run() error {
	paper := document.A5r
	opt := &pdf.WriterOptions{
		HumanReadable: true,
	}
	page, err := document.CreateSinglePage("test.pdf", paper, pdf.V1_7, opt)
	if err != nil {
		return err
	}

	F, err := standard.Helvetica.New(nil)
	if err != nil {
		return err
	}

	X := &testFont{}

	M := makeMarkerFont()

	yPos := paper.URy - 72 - 15

	page.TextBegin()
	page.TextSetFont(F, 10)
	page.TextFirstLine(72, yPos)
	gg := page.TextLayout(nil, "code used in WinAnsi encoding:")
	gg.Align(190, 0)
	page.TextShowGlyphs(gg)
	page.TextSetFont(M, 10)
	page.TextShow("I")
	page.TextSetFont(X, 10)
	page.TextShowRaw(pdf.String{0o335})
	page.TextSetFont(M, 10)
	page.TextShow("I")

	page.TextSecondLine(0, -20)
	page.TextSetFont(F, 10)
	gg = page.TextLayout(nil, "remapped code:")
	gg.Align(190, 0)
	page.TextShowGlyphs(gg)
	page.TextSetFont(M, 10)
	page.TextShow("I")
	page.TextSetFont(X, 10)
	page.TextShowRaw(pdf.String{'A'})
	page.TextSetFont(M, 10)
	page.TextShow("I")

	page.TextNextLine()
	page.TextSetFont(F, 10)
	gg = page.TextLayout(nil, "space character:")
	gg.Align(190, 0)
	page.TextShowGlyphs(gg)
	page.TextSetFont(M, 10)
	page.TextShow("I")
	page.TextSetFont(X, 10)
	page.TextShowRaw(pdf.String{' '})
	page.TextSetFont(M, 10)
	page.TextShow("I")

	page.TextNextLine()
	page.TextSetFont(F, 10)
	gg = page.TextLayout(nil, "code mapped to non-existent character:")
	gg.Align(190, 0)
	page.TextShowGlyphs(gg)
	page.TextSetFont(M, 10)
	page.TextShow("I")
	page.TextSetFont(X, 10)
	page.TextShowRaw(pdf.String{'B'})
	page.TextSetFont(M, 10)
	page.TextShow("I")

	page.TextNextLine()
	page.TextSetFont(F, 10)
	gg = page.TextLayout(nil, "unmapped code:")
	gg.Align(190, 0)
	page.TextShowGlyphs(gg)
	page.TextSetFont(M, 10)
	page.TextShow("I")
	page.TextSetFont(X, 10)
	page.TextShowRaw(pdf.String{0o010})
	page.TextSetFont(M, 10)
	page.TextShow("I")

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
	fd, err := ll.Open("Times-Roman", loader.FontTypeType1)
	if err != nil {
		return nil, nil, err
	}
	F, err := type1.Read(fd)
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

	// we manually write the font to the PDF file
	F.Encoding = psenc.StandardEncoding[:]

	fontFileRef := w.Alloc()
	length1 := pdf.NewPlaceholder(w, 10)
	length2 := pdf.NewPlaceholder(w, 10)
	fontFileDict := pdf.Dict{
		"Length1": length1,
		"Length2": length2,
		"Length3": pdf.Integer(0),
	}
	fontFileStream, err := w.OpenStream(fontFileRef, fontFileDict, pdf.FilterCompress{})
	if err != nil {
		return nil, nil, err
	}
	l1, l2, err := F.WritePDF(fontFileStream)
	if err != nil {
		return nil, nil, err
	}
	err = length1.Set(pdf.Integer(l1))
	if err != nil {
		return nil, nil, err
	}
	err = length2.Set(pdf.Integer(l2))
	if err != nil {
		return nil, nil, err
	}
	err = fontFileStream.Close()
	if err != nil {
		return nil, nil, err
	}

	var fontBBox rect.Rect
	bbox := F.FontBBox()
	q := 1000 * F.FontInfo.FontMatrix[0]
	fontBBox = rect.Rect{
		LLx: bbox.LLx * q,
		LLy: bbox.LLy * q,
		URx: bbox.URx * q,
		URy: bbox.URy * q,
	}
	fontDescRef := w.Alloc()
	fontDesc := &font.Descriptor{
		FontName:     "AAAAAA+NimbusRoman-Regular",
		IsSerif:      true,
		FontBBox:     fontBBox,
		Ascent:       1000,
		Descent:      100,
		Leading:      2000,
		CapHeight:    1000,
		XHeight:      500,
		MissingWidth: 500,
	}
	fontDescDict := fontDesc.AsDict()
	fontDescDict["FontFile"] = fontFileRef
	w.Put(fontDescRef, fontDescDict)

	encoding := pdf.Dict{
		"Type":         pdf.Name("Encoding"),
		"BaseEncoding": pdf.Name("WinAnsiEncoding"),
		"Differences": pdf.Array{
			pdf.Integer(65), pdf.Name("AEacute"), pdf.Name("does.not.exist"),
		},
	}

	ww := make([]float64, 256)
	for i := 0; i < 256; i++ {
		ww[i] = 500
	}
	ww[' '] = F.Glyphs["space"].WidthX * F.FontInfo.FontMatrix[0] * 1000
	ww[65] = F.Glyphs["AEacute"].WidthX * F.FontInfo.FontMatrix[0] * 1000
	ww[0o335] = F.Glyphs["Yacute"].WidthX * F.FontInfo.FontMatrix[0] * 1000
	widthsInfo := widths.EncodeSimple(ww)

	fontDict := pdf.Dict{
		"Type":           pdf.Name("Font"),
		"Subtype":        pdf.Name("Type1"),
		"BaseFont":       pdf.Name("AAAAAA+NimbusRoman-Regular"),
		"FirstChar":      widthsInfo.FirstChar,
		"LastChar":       widthsInfo.LastChar,
		"Widths":         widthsInfo.Widths,
		"Encoding":       encoding,
		"FontDescriptor": fontDescRef,
	}
	w.Put(fontDictRef, fontDict)

	res := &testFontEmbedded{
		Ref: fontDictRef,
		W:   ww,
	}
	return fontDictRef, res, nil
}

type testFontEmbedded struct {
	Ref pdf.Reference
	W   []float64
}

func (f *testFontEmbedded) WritingMode() font.WritingMode {
	return 0
}

// DecodeWidth reads one character code from the given string and returns
// the width of the corresponding glyph.
// This implements the [font.Embedded] interface.
func (f *testFontEmbedded) DecodeWidth(s pdf.String) (float64, int) {
	return f.W[s[0]], 1
}
