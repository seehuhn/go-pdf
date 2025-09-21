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
	"bytes"
	"fmt"
	"math"
	"os"

	"golang.org/x/image/font/gofont/goregular"

	"seehuhn.de/go/postscript/cid"

	"seehuhn.de/go/sfnt"
	"seehuhn.de/go/sfnt/glyph"

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
)

const description = `
This file tests CMap file behavior when CID ranges span holes in the code space.
The test font defines valid codes as 0x00-0x41 and 0x43-0xFF, excluding 0x42.
The CMap maps range 0x41-0x43 to consecutive CIDs starting at 43 (representing the character “A”).
The text contains codes 0x41 and 0x43.  There is some ambiguity:
0x43 could be interpreted as “B” (the second valid code in the range)
or “C” (the third element in the range).
`

func main() {
	err := createDocument("test.pdf")
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func createDocument(fname string) error {
	opts := &pdf.WriterOptions{
		HumanReadable: true,
	}
	page, err := document.CreateSinglePage(fname, document.A5r, pdf.V2_0, opts)
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
		text.M{X: 50, Y: 370},
		note,
		text.Wrap(360, description),
		test,
		text.NL,
		pdf.String{0x41, 0x43},
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

func (testFont) Embed(rm *pdf.EmbedHelper) (pdf.Native, font.Embedded, error) {
	fontDictRef := rm.Alloc()
	fontType := glyphdata.TrueType

	numCID := 34 + 3
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
	for r := 'A'; r <= 'C'; r++ {
		// CID 34 = A, ...
		cid := cmap.CID(r - 'A' + 34)
		origGID := cmapTable.Lookup(r)
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
		FontBBox:     subsetFont.FontBBoxPDF().Rounded(),
		ItalicAngle:  subsetFont.ItalicAngle,
		Ascent:       ascent,
		Descent:      descent,
		CapHeight:    capHeight,
		StemV:        0,
	}
	cmapFile := &cmap.File{
		Name: "Test",
		ROS:  ros,
		CodeSpaceRange: charcode.CodeSpaceRange{
			{Low: []byte{0x00}, High: []byte{0x41}},
			{Low: []byte{0x42, 0x00}, High: []byte{0x42, 0xFF}},
			{Low: []byte{0x43}, High: []byte{0xFF}},
		},
		CIDRanges: []cmap.Range{
			{
				First: []byte{0x41},
				Last:  []byte{0x43},
				Value: 34, // 'A'
			},
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
		CIDToGID:        cidToGID,
		FontFile:        sfntglyphs.ToStream(subsetFont, fontType),
	}

	err = dict.WriteToPDF(rm.GetRM(), fontDictRef)
	if err != nil {
		return nil, nil, err
	}

	E := dict.MakeFont()
	return fontDictRef, E, nil
}
