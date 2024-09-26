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
	"log"
	"os"
	"strings"
	"time"

	"golang.org/x/exp/maps"

	"seehuhn.de/go/postscript/afm"
	pstype1 "seehuhn.de/go/postscript/type1"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/loader"
	"seehuhn.de/go/pdf/font/standard"
)

func main() {
	err := write_font("test.pfb")
	if err != nil {
		log.Fatal(err)
	}

	err = write_pdf("test.pdf")
	if err != nil {
		log.Fatal(err)
	}
}

func write_font(filename string) error {
	name := "NimbusMonoPS-Regular"

	builtin := loader.NewFontLoader()
	fontData, err := builtin.Open(name, loader.FontTypeType1)
	if err != nil {
		return err
	}
	psFont, err := pstype1.Read(fontData)
	if err != nil {
		return err
	}
	fontData.Close()

	psFont.FontInfo.FontName = "test"
	psFont.CreationDate = time.Now()

	// codes: 121, 101, 115
	// standard encoding: "yes"
	// built-in encoding: "no "
	include := map[string]bool{
		"y":     true,
		"e":     true,
		"s":     true,
		"n":     true,
		"o":     true,
		"space": true,
	}
	newGlyphs := make(map[string]*pstype1.Glyph)
	for key := range psFont.Glyphs {
		if include[key] {
			newGlyphs[key] = psFont.Glyphs[key]
		}
	}
	psFont.Glyphs = newGlyphs
	fmt.Println(maps.Keys(newGlyphs))

	// make the built-in encoding spell "no "
	psFont.Encoding = make([]string, 256)
	psFont.Encoding[121] = "n"
	psFont.Encoding[101] = "o"
	psFont.Encoding[115] = "space"

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
			BBox:   glyph.BBox(),
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

func write_pdf(filename string) error {
	opt := &pdf.WriterOptions{
		HumanReadable: true,
	}
	doc, err := document.CreateSinglePage(filename, document.A5r, pdf.V1_7, opt)
	if err != nil {
		return err
	}

	labelFont, err := standard.TimesRoman.New(nil)
	if err != nil {
		return err
	}
	testFont1 := &funnyFont{true}
	testFont2 := &funnyFont{false}

	doc.TextBegin()
	doc.TextFirstLine(36, 350)
	doc.TextSetFont(labelFont, 24)
	doc.TextShow("StandardEncoding used with Encoding dict: ")
	doc.TextSetFont(testFont1, 24)
	doc.TextShowRaw([]byte{121, 101, 115})
	doc.TextSecondLine(0, -30)
	doc.TextSetFont(labelFont, 24)
	doc.TextShow("StandardEncoding used without Encoding dict: ")
	doc.TextSetFont(testFont2, 24)
	doc.TextShowRaw([]byte{121, 101, 115})
	doc.TextEnd()

	err = doc.Close()
	if err != nil {
		return err
	}

	return nil
}

type funnyFont struct {
	useEncodingDict bool
}

func (f *funnyFont) PostScriptName() string {
	return "test"
}

func (f *funnyFont) Embed(rm *pdf.ResourceManager) (pdf.Native, font.Embedded, error) {
	fontDictRef := rm.Out.Alloc()

	fontDict := pdf.Dict{
		"Type":      pdf.Name("Font"),
		"Subtype":   pdf.Name("Type1"),
		"BaseFont":  pdf.Name(f.PostScriptName()),
		"FirstChar": pdf.Integer(101),
		"LastChar":  pdf.Integer(101),
		"Widths":    pdf.Array{pdf.Integer(600)},
		"FontDescriptor": pdf.Dict{
			"Type":         pdf.Name("FontDescriptor"),
			"FontName":     pdf.Name(f.PostScriptName()),
			"Flags":        pdf.Integer(32), // 4=symbolic, 32=non-symbolic
			"FontBBox":     pdf.Array{pdf.Integer(-100), pdf.Integer(-100), pdf.Integer(700), pdf.Integer(1000)},
			"ItalicAngle":  pdf.Integer(0),
			"Ascent":       pdf.Integer(1000),
			"Descent":      pdf.Integer(-200),
			"CapHeight":    pdf.Integer(800),
			"StemV":        pdf.Integer(0),
			"MissingWidth": pdf.Integer(600),
		},
	}
	if f.useEncodingDict {
		fontDict["Encoding"] = pdf.Dict{
			"Type": pdf.Name("Encoding"),
		}
	}

	rm.Out.Put(fontDictRef, fontDict)

	return fontDictRef, f, nil
}

func (f *funnyFont) WritingMode() cmap.WritingMode {
	return cmap.Horizontal
}

// DecodeWidth reads one character code from the given string and returns
// the width of the corresponding glyph in PDF text space units (still to
// be multiplied by the font size) and the number of bytes read from the
// string.
func (f *funnyFont) DecodeWidth(pdf.String) (float64, int) {
	return 1000, 1
}
