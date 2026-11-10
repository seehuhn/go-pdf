// Copyright 2024 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

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
	"seehuhn.de/go/pdf/font/standard"
	"seehuhn.de/go/pdf/font/subset"
	"seehuhn.de/go/pdf/font/type3"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/content/builder"
	"seehuhn.de/go/pdf/internal/debug/makefont"
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

	F := font.Must(standard.Helvetica.New())

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
	b := builder.New(content.Glyph, nil, pdf.V2_0)
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
	return font.Must(markerFont.New())
}

func makeTestFont() (font.Instance, error) {
	// start with a Type 1 font which has the glyphs we need
	F := makefont.Type1()

	// copy the glyphs we want to keep
	oldGlyphs := F.Glyphs
	F.Glyphs = make(map[string]*type1.Glyph)
	F.Glyphs["space"] = oldGlyphs["space"]
	F.Glyphs["Yacute"] = oldGlyphs["Yacute"]
	F.Glyphs["AEacute"] = oldGlyphs["AEacute"]

	// Since the original font has a blank .notdef glyph, we draw one manually.
	// The coordinates are given in 1/1000 em and scaled to font units.
	q := 1 / (F.FontInfo.FontMatrix[0] * 1000)
	notdef := &type1.Glyph{
		WidthX: 500 * q,
	}
	notdef.MoveTo(10*q, 10*q)
	notdef.LineTo(490*q, 10*q)
	notdef.LineTo(490*q, 990*q)
	notdef.LineTo(10*q, 990*q)
	notdef.ClosePath()
	notdef.MoveTo(100*q, 100*q)
	notdef.LineTo(100*q, 900*q)
	notdef.LineTo(400*q, 900*q)
	notdef.LineTo(400*q, 100*q)
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

	psName := F.FontInfo.FontName
	metrics := makefont.AFM()
	fd := &font.Descriptor{
		FontName:     subset.Join("AAAAAA", psName),
		IsSerif:      makefont.TrueType().IsSerif,
		FontBBox:     F.FontBBoxPDF().Rounded(),
		Ascent:       pdf.Round(metrics.Ascent, 1),
		Descent:      pdf.Round(metrics.Descent, 1),
		CapHeight:    pdf.Round(metrics.CapHeight, 1),
		XHeight:      pdf.Round(metrics.XHeight, 1),
		MissingWidth: 500,
	}
	dict := dict.Type1{
		PostScriptName: psName,
		SubsetTag:      "AAAAAA",
		Descriptor:     fd,
		Encoding:       enc,
		FontFile:       type1glyphs.ToStream(F),
	}
	for code := range 256 {
		dict.Width[code] = 500
	}
	dict.Width[' '] = pdf.Round(F.Glyphs["space"].WidthX*F.FontInfo.FontMatrix[0]*1000, 1)
	dict.Width['A'] = pdf.Round(F.Glyphs["AEacute"].WidthX*F.FontInfo.FontMatrix[0]*1000, 1)
	dict.Width[0o335] = pdf.Round(F.Glyphs["Yacute"].WidthX*F.FontInfo.FontMatrix[0]*1000, 1)
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
