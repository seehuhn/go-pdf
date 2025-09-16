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
	"bytes"
	"fmt"
	"math"
	"os"

	"golang.org/x/image/font/gofont/goregular"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/dict"
	"seehuhn.de/go/pdf/font/glyphdata"
	"seehuhn.de/go/pdf/font/glyphdata/sfntglyphs"
	"seehuhn.de/go/pdf/font/standard"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/text"
	"seehuhn.de/go/postscript/cid"
	"seehuhn.de/go/sfnt"
	"seehuhn.de/go/sfnt/glyph"
)

const description = `
This test file uses a composite font with one-byte character codes,
representing a subset of ASCII. The CID values chosen are from the
“Adobe-Japan1” character collection. Both uppercase and lowercase codes
are mapped to the same (uppercase) CID value. A ToUnicode CMap is used to
describe the text content for the lowercase codes, while the uppercase letters
rely on the standard interpretation of the CID values. In this scheme,
the text “Hello World” should be shown as “HELLO WORLD”, but when
copying and pasting the text from a PDF viewer, the lowercase letters
should be preserved.
`

func main() {
	err := createDocument("test.pdf")
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func createDocument(filename string) error {
	page, err := document.CreateSinglePage(filename, document.A4, pdf.V2_0, nil)
	if err != nil {
		return err
	}

	noteFont := standard.TimesRoman.New()
	note := text.F{
		Font:  noteFont,
		Size:  10,
		Color: color.DeviceGray(0.1),
	}

	test := text.F{
		Font:  testFont{},
		Size:  24,
		Color: color.DeviceRGB(0, 0, 0.7),
	}

	text.Show(page.Writer,
		text.M{X: 72, Y: 750},
		note,
		text.Wrap(340, description),
		test,
		text.NL,
		pdf.String("Hello World"),
	)

	err = page.Close()
	if err != nil {
		return err
	}

	return nil
}

type testFont struct{}

func (testFont) PostScriptName() string {
	return "Test"
}

func (testFont) Embed(rm *pdf.ResourceManager) (pdf.Native, font.Embedded, error) {
	fontDictRef := rm.Out.Alloc()
	fontType := glyphdata.TrueType

	numCID := 34 + 26
	cidToGID := make([]glyph.ID, numCID)
	width := map[cmap.CID]float64{}

	// Create a TrueType font with the required subset of glyphs.
	origFont, err := sfnt.Read(bytes.NewReader(goregular.TTF))
	if err != nil {
		return nil, nil, err
	}
	cmapTable, err := origFont.CMapTable.GetBest()
	if err != nil {
		return nil, nil, err
	}
	var subsetGlyphs []glyph.ID
	// CID 0 = .notdef
	cidToGID[0] = glyph.ID(len(subsetGlyphs))
	subsetGlyphs = append(subsetGlyphs, 0)
	width[0] = math.Round(origFont.GlyphWidthPDF(0))
	// CID 1 = space
	origGID := cmapTable.Lookup(' ')
	cidToGID[1] = glyph.ID(len(subsetGlyphs))
	width[1] = math.Round(origFont.GlyphWidthPDF(origGID))
	subsetGlyphs = append(subsetGlyphs, origGID)
	for r := 'A'; r <= 'Z'; r++ {
		// CID 34 = A, ...
		cid := cmap.CID(r - 'A' + 34)
		origGID = cmapTable.Lookup(r)
		cidToGID[cid] = glyph.ID(len(subsetGlyphs))
		width[cid] = math.Round(origFont.GlyphWidthPDF(origGID))
		subsetGlyphs = append(subsetGlyphs, origGID)
	}
	origFont.CMapTable = nil
	origFont.Gdef = nil
	origFont.Gsub = nil
	origFont.Gpos = nil
	subsetFont := origFont.Subset(subsetGlyphs)

	// Create a PDF font dictionary for the font.
	qv := subsetFont.FontMatrix[3] * 1000
	ascent := math.Round(float64(subsetFont.Ascent) * qv)
	descent := math.Round(float64(subsetFont.Descent) * qv)
	capHeight := math.Round(float64(subsetFont.CapHeight) * qv)

	ros := &cid.SystemInfo{
		Registry:   "Adobe",
		Ordering:   "Japan1",
		Supplement: 7,
	}
	fd := &font.Descriptor{
		FontName:     "ABCDEF+" + subsetFont.PostScriptName(),
		IsFixedPitch: subsetFont.IsFixedPitch(),
		IsSerif:      subsetFont.IsSerif,
		IsSymbolic:   false,
		IsScript:     subsetFont.IsScript,
		IsItalic:     subsetFont.IsItalic,
		IsAllCap:     true,
		IsSmallCap:   false,
		FontBBox:     subsetFont.FontBBoxPDF(),
		ItalicAngle:  subsetFont.ItalicAngle,
		Ascent:       ascent,
		Descent:      descent,
		CapHeight:    capHeight,
		StemV:        0,
	}
	cmapFile := &cmap.File{
		Name:           "Seehuhn-Test",
		ROS:            ros,
		WMode:          font.Horizontal,
		CodeSpaceRange: charcode.Simple,
		CIDSingles: []cmap.Single{
			{Code: []byte{' '}, Value: 1},
		},
		CIDRanges: []cmap.Range{
			{First: []byte{'A'}, Last: []byte{'Z'}, Value: 34},
			{First: []byte{'a'}, Last: []byte{'z'}, Value: 34},
		},
	}
	toUnicode := &cmap.ToUnicodeFile{
		CodeSpaceRange: charcode.Simple,
		Ranges: []cmap.ToUnicodeRange{
			{First: []byte{'a'}, Last: []byte{'z'}, Values: []string{"a"}},
		},
	}
	dict := &dict.CIDFontType2{
		PostScriptName:  subsetFont.PostScriptName(),
		SubsetTag:       "ABCDEF",
		Descriptor:      fd,
		ROS:             ros,
		CMap:            cmapFile,
		Width:           width,
		DefaultWidth:    width[0],
		DefaultVMetrics: dict.DefaultVMetricsDefault,
		ToUnicode:       toUnicode,
		CIDToGID:        cidToGID,
		FontFile:        sfntglyphs.ToStream(subsetFont, fontType),
	}

	err = dict.WriteToPDF(rm, fontDictRef)
	if err != nil {
		return nil, nil, err
	}

	E := dict.MakeFont()

	return fontDictRef, E, nil
}
