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
	"strings"
	"time"

	"seehuhn.de/go/geom/rect"

	"seehuhn.de/go/postscript/afm"
	pstype1 "seehuhn.de/go/postscript/type1"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/dict"
	"seehuhn.de/go/pdf/font/encoding"
	"seehuhn.de/go/pdf/font/extended"
	"seehuhn.de/go/pdf/font/standard"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/text"
)

const TestFontName = "Test17755"

func main() {
	err := createFont("test.pfb")
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	err = createDocument("test.pdf")
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

var (
	fontBBox      rect.Rect
	fontAscent    float64
	fontDescent   float64
	fontCapHeight float64
	fontXHeight   float64
	glyphWidths   float64
)

func createFont(filename string) error {
	F := extended.NimbusMonoPSRegular.New()

	psFont := F.Font
	psFont.FontInfo.FontName = TestFontName
	psFont.CreationDate = time.Now()
	psFont.Encoding = make([]string, 256)

	// include enough glyphs to spell either "STD" or "BIE"
	newGlyphs := make(map[string]*pstype1.Glyph)
	newGlyphs[".notdef"] = psFont.Glyphs[".notdef"]

	// If the viewer uses the built-in encoding to display the text string
	// "XNO", this should visually spell "BIE".
	newGlyphs["B"] = psFont.Glyphs["B"]
	newGlyphs["I"] = psFont.Glyphs["I"]
	newGlyphs["E"] = psFont.Glyphs["E"]
	psFont.Encoding['X'] = "B"
	psFont.Encoding['N'] = "I"
	psFont.Encoding['O'] = "E"

	// If the viewer uses the standard encoding to display the text string
	// "XNO", this should visually spell "STD".
	newGlyphs["X"] = psFont.Glyphs["S"]
	newGlyphs["N"] = psFont.Glyphs["T"]
	newGlyphs["O"] = psFont.Glyphs["D"]

	psFont.Glyphs = newGlyphs

	fontBBox = psFont.FontBBoxPDF()
	glyphWidths = psFont.GlyphWidthPDF("X")
	fontAscent = F.Metrics.Ascent
	fontDescent = F.Metrics.Descent
	fontCapHeight = F.Metrics.CapHeight
	fontXHeight = F.Metrics.XHeight

	opt := &pstype1.WriterOptions{
		Format: pstype1.FormatPFB,
	}
	fd, err := os.Create(filename)
	if err != nil {
		return err
	}
	err = psFont.Write(fd, opt)
	if err != nil {
		return err
	}
	err = fd.Close()
	if err != nil {
		return err
	}

	metrics := &afm.Metrics{
		Glyphs:       make(map[string]*afm.GlyphInfo),
		Encoding:     psFont.Encoding,
		FontName:     psFont.FontName,
		FullName:     psFont.FullName,
		Version:      psFont.Version,
		Notice:       psFont.Notice,
		IsFixedPitch: true,
	}
	for name, glyph := range psFont.Glyphs {
		metrics.Glyphs[name] = &afm.GlyphInfo{
			WidthX: glyph.WidthX,
			BBox:   psFont.GlyphBBoxPDF(name),
		}
	}
	afmName := strings.TrimSuffix(filename, ".pfb") + ".afm"
	fd, err = os.Create(afmName)
	if err != nil {
		return err
	}
	err = metrics.Write(fd)
	if err != nil {
		return err
	}
	err = fd.Close()
	if err != nil {
		return err
	}

	return nil
}

func createDocument(filename string) error {
	opt := &pdf.WriterOptions{
		HumanReadable: true,
	}
	doc, err := document.CreateSinglePage(filename, document.A5r, pdf.V2_0, opt)
	if err != nil {
		return err
	}

	body := text.F{
		Font:  standard.TimesRoman.New(),
		Size:  12,
		Color: color.Black,
	}
	test1 := text.F{
		Font:  &testFont{false},
		Size:  12,
		Color: color.Blue,
	}
	test2 := text.F{
		Font:  &testFont{true},
		Size:  12,
		Color: color.Blue,
	}
	label := text.F{
		Font:  extended.NimbusMonoPSRegular.New(),
		Size:  12,
		Color: color.Blue,
	}

	text.Show(doc.Writer,
		text.M{X: 50, Y: 370},
		body,
		text.Wrap(400, `
		This test file checks whether PDF viewers distinguish between
		Type 1 font dictionaries that have no encoding dictionary and those
		that have an empty encoding dictionary when using external
		(non-embedded) fonts.`),
		text.NL,
		text.Wrap(400, `
		For the test to work, the font file "test.pfb" must be
		installed on the system, to make the font `+TestFontName+` available to the
		PDF viewer.`),
		text.NL,
		"Test cases:",
		text.NL,
		"1. No encoding dictionary: ", test1, pdf.String("XNO"), body, text.NL,
		"2. Empty encoding dictionary: ", test2, pdf.String("XNO"), body, text.NL,
		text.NL,
		text.Wrap(400, `
		For each test case, a three-letter code should be displayed in blue.
		The codes have the following meanings:`),
		label, "BIE", body, " = the built-in encoding was used", text.NL,
		label, "STD", body, " = the standard encoding was used", text.NL,
		label, "XNO", body, " = the font was not loaded", text.NL,
		text.NL,
		text.Wrap(400, `
		My reading of the PDF specification is that test case 1 should
		show "BIE" and test case 2 should show "STD".`),
	)

	err = doc.Close()
	if err != nil {
		return err
	}

	return nil
}

type testFont struct {
	useEncodingDict bool
}

func (f *testFont) PostScriptName() string {
	return TestFontName
}

func (f *testFont) Embed(rm *pdf.ResourceManager) (pdf.Native, font.Embedded, error) {
	fontDictRef := rm.Out.Alloc()

	fd := &font.Descriptor{
		FontName:     TestFontName,
		IsFixedPitch: true,
		IsSerif:      true,
		IsSymbolic:   false,
		IsAllCap:     true,
		FontBBox:     fontBBox,
		Ascent:       fontAscent,
		Descent:      fontDescent,
		CapHeight:    fontCapHeight,
		XHeight:      fontXHeight,
		MissingWidth: glyphWidths,
	}
	dict := &dict.Type1{
		PostScriptName: TestFontName,
		Descriptor:     fd,
		FontFile:       nil, // external font
	}
	if f.useEncodingDict {
		// The standard encoding is represented by an encoding dictionary
		// without a /BaseEncoding field.
		dict.Encoding = encoding.Standard
	} else {
		// The built-in encoding is represented by an absent /Encoding field.
		dict.Encoding = encoding.Builtin
	}
	for i := range 256 {
		dict.Width[i] = glyphWidths
	}

	err := dict.WriteToPDF(rm, fontDictRef)
	if err != nil {
		return nil, nil, err
	}

	E := dict.MakeFont()

	return fontDictRef, E, nil
}
