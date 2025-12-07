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
	"fmt"
	"math"
	"os"

	"seehuhn.de/go/geom/matrix"

	"seehuhn.de/go/postscript/cid"
	"seehuhn.de/go/postscript/funit"
	"seehuhn.de/go/postscript/type1"

	sfntcff "seehuhn.de/go/sfnt/cff"
	"seehuhn.de/go/sfnt/glyph"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cff"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/dict"
	"seehuhn.de/go/pdf/font/glyphdata"
	"seehuhn.de/go/pdf/font/glyphdata/cffglyphs"
	"seehuhn.de/go/pdf/font/standard"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/text"
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
	data, err := makeTestFonts()
	if err != nil {
		return err
	}

	black := color.Black
	blue := color.DeviceRGB{0, 0, 0.9}
	red := color.DeviceRGB{0.9, 0, 0}

	const fontSize = 12

	noteFont := standard.TimesRoman.New()
	note := text.F{Font: noteFont, Size: fontSize, Color: black}

	orig := text.F{Font: data.origFont, Size: fontSize, Color: blue}

	test := text.F{Font: data.testFont, Size: fontSize, Color: red}
	testL := text.F{Font: data.testFont, Size: 100, Color: red}

	w, err := document.CreateSinglePage(fname, document.A5, pdf.V2_0, nil)
	if err != nil {
		return err
	}
	w.SetFontNameInternal(data.origFont, "Orig")
	w.SetFontNameInternal(data.testFont, "Test")

	// draw the text, including the large test glyphs
	var x, y float64
	text.Show(w.Writer,
		text.M{X: 36, Y: 530},
		note, "This file shows two versions of the same font.", text.NL,
		"One version is a regular CFF font, while the other version has", text.NL,
		"the coordinates of the glyph outlines rescaled, and the", text.NL,
		"font matrix is modified to compensate for this.  Different", text.NL,
		"scalings are used for uppercase and lowercase letters.", text.NL,
		text.NL,
		note, "Test 1: check that the font still renders correctly.", text.NL,
		"Blue text is rendered using the original font,", text.NL,
		"red text is rendered using the modified font:", text.NL,
		orig, "These two lines should look the same", text.NL,
		test, pdf.String("These two lines should look the same"), text.NL,
		text.NL,
		note, "Test 2: show some glyphs together with their bounding boxes.", text.NL,
		"The boxes should tightly enclose the glyphs:",
		text.M{X: 36, Y: -110},
		text.RecordPos{UserX: &x, UserY: &y},
		testL, pdf.String("ABC"),
		text.M{X: 0, Y: -100},
		testL, pdf.String("abc"),
	)

	// draw the bounding boxes
	testCFF := data.testCFF

	w.PushGraphicsState()
	w.SetLineWidth(0.5)

	x0 := x

	bbox := testCFF.GlyphBBoxPDF(testCFF.FontMatrix, 2+0)
	bbox.Scale(100.0 / 1000.0) // convert to font size 100, and from glyph space
	w.Rectangle(x+bbox.LLx, y+bbox.LLy, bbox.Dx(), bbox.Dy())
	x += 100 * testCFF.GlyphWidthPDF(2+0) / 1000

	bbox = testCFF.GlyphBBoxPDF(testCFF.FontMatrix, 2+1)
	bbox.Scale(100.0 / 1000.0)
	w.Rectangle(x+bbox.LLx, y+bbox.LLy, bbox.Dx(), bbox.Dy())
	x += 100 * testCFF.GlyphWidthPDF(2+1) / 1000

	bbox = testCFF.GlyphBBoxPDF(testCFF.FontMatrix, 2+2)
	bbox.Scale(100.0 / 1000.0)
	w.Rectangle(x+bbox.LLx, y+bbox.LLy, bbox.Dx(), bbox.Dy())

	x = x0 // second line
	y -= 100

	bbox = testCFF.GlyphBBoxPDF(testCFF.FontMatrix, 2+26+0)
	bbox.Scale(100.0 / 1000.0) // convert to font size 100, and from glyph space
	w.Rectangle(x+bbox.LLx, y+bbox.LLy, bbox.Dx(), bbox.Dy())
	x += 100 * testCFF.GlyphWidthPDF(2+26+0) / 1000

	bbox = testCFF.GlyphBBoxPDF(testCFF.FontMatrix, 2+26+1)
	bbox.Scale(100.0 / 1000.0)
	w.Rectangle(x+bbox.LLx, y+bbox.LLy, bbox.Dx(), bbox.Dy())
	x += 100 * testCFF.GlyphWidthPDF(2+26+1) / 1000

	bbox = testCFF.GlyphBBoxPDF(testCFF.FontMatrix, 2+26+2)
	bbox.Scale(100.0 / 1000.0)
	w.Rectangle(x+bbox.LLx, y+bbox.LLy, bbox.Dx(), bbox.Dy())

	w.Stroke()
	w.PopGraphicsState()

	return w.Close()
}

type testFonts struct {
	testFont font.Instance
	testCFF  *sfntcff.Font
	origFont font.Instance
}

func makeTestFonts() (*testFonts, error) {
	orig := makefont.OpenType()

	// disable kerning and ligatures for the test
	orig.Gdef = nil
	orig.Gsub = nil
	orig.Gpos = nil

	origOutlines := orig.Outlines.(*sfntcff.Outlines)
	if origOutlines.IsCIDKeyed() {
		panic("expected simple font")
	}

	lookup, err := orig.CMapTable.GetBest()
	if err != nil {
		return nil, err
	}

	fontInfo := orig.GetFontInfo()
	origFM := fontInfo.FontMatrix

	// Construct a new font with rescaled glyph outlines,
	// and set up font matrices to compensate for the rescaling.
	// The new font only contains .notdef, ' ', 'A'-'Z' and 'a'-'z'.

	var newGlyphs []*sfntcff.Glyph
	var GIDToCID []cid.CID
	cmapData := &cmap.File{
		Name: "TestCMap",
		ROS: &cid.SystemInfo{
			Registry: "seehuhn.de",
			Ordering: "test",
		},
		WMode: font.Horizontal,
		CodeSpaceRange: []charcode.Range{
			{Low: []byte{0x00}, High: []byte{0xFF}},
		},
	}
	private1 := clone(origOutlines.Private[0])
	private2 := clone(origOutlines.Private[0])

	// first group of glyphs (.notdef, ' ', 'A'-'Z'):
	// use 1000 design units horizontally and 2000 vertically
	qx := origFM[0] * 1000
	qy := origFM[3] * 2000

	GIDToCID = append(GIDToCID, 0)
	newGlyphs = append(newGlyphs, rescaleGlyph(origOutlines.Glyphs[0], qx, qy))

	newCID := cid.CID(len(newGlyphs))
	GIDToCID = append(GIDToCID, newCID)
	cmapData.CIDSingles = append(cmapData.CIDSingles, cmap.Single{Code: []byte{' '}, Value: newCID})
	gid := lookup.Lookup(' ')
	newGlyphs = append(newGlyphs, rescaleGlyph(origOutlines.Glyphs[gid], qx, qy))

	for c := 'A'; c <= 'Z'; c++ {
		newCID := cid.CID(len(newGlyphs))
		cmapData.CIDSingles = append(cmapData.CIDSingles, cmap.Single{Code: []byte{byte(c)}, Value: newCID})
		GIDToCID = append(GIDToCID, newCID)
		gid := lookup.Lookup(c)
		newGlyphs = append(newGlyphs, rescaleGlyph(origOutlines.Glyphs[gid], qx, qy))
	}

	blueValues := make([]funit.Int16, len(private1.BlueValues))
	for i, v := range private1.BlueValues {
		blueValues[i] = funit.Int16(math.Round(float64(v) * qy))
	}
	private1.BlueValues = blueValues

	cutoff := len(newGlyphs)

	// second group of glyphs ('a'-'z'):
	// use 2000 design units horizontally and 1000 vertically
	qx = origFM[0] * 2000
	qy = origFM[3] * 1000

	for c := 'a'; c <= 'z'; c++ {
		newCID := cid.CID(len(newGlyphs))
		cmapData.CIDSingles = append(cmapData.CIDSingles, cmap.Single{Code: []byte{byte(c)}, Value: newCID})
		GIDToCID = append(GIDToCID, newCID)
		gid := lookup.Lookup(c)
		newGlyphs = append(newGlyphs, rescaleGlyph(origOutlines.Glyphs[gid], qx, qy))
	}

	blueValues = make([]funit.Int16, len(private2.BlueValues))
	for i, v := range private2.BlueValues {
		blueValues[i] = funit.Int16(math.Round(float64(v) * qy))
	}

	// construct the new CFF font
	fontInfo.FontName = "Test"
	newOutlines := &sfntcff.Outlines{
		Glyphs: newGlyphs,
		Private: []*type1.PrivateDict{
			private1,
			private2,
		},
		FDSelect: func(gid glyph.ID) int {
			if gid < glyph.ID(cutoff) {
				return 0
			}
			return 1
		},
		ROS:      &cid.SystemInfo{Registry: "seehuhn.de", Ordering: "test"},
		GIDToCID: GIDToCID,
		FontMatrices: []matrix.Matrix{
			{0.001, 0, 0, 0.0005, 0, 0},
			{0.0005, 0, 0, 0.001, 0, 0},
		},
	}
	fontInfo.FontMatrix = matrix.Identity
	testCFF := &sfntcff.Font{
		FontInfo: fontInfo,
		Outlines: newOutlines,
	}

	// build the font dictionary
	q := orig.FontMatrix[3] * 1000
	fd := &font.Descriptor{
		FontName:   testCFF.FontName,
		IsSymbolic: true,
		FontBBox:   testCFF.FontBBoxPDF(),
		Ascent:     math.Round(orig.Ascent.AsFloat(q)),
		Descent:    math.Round(orig.Descent.AsFloat(q)),
		CapHeight:  math.Round(orig.CapHeight.AsFloat(q)),
		StemV:      80,
	}

	ww := make(map[cmap.CID]float64)
	for gid, cid := range testCFF.GIDToCID {
		w := testCFF.GlyphWidthPDF(glyph.ID(gid))
		ww[cid] = w
	}

	fontDict := &dict.CIDFontType0{
		PostScriptName:  testCFF.FontName,
		Descriptor:      fd,
		ROS:             cmapData.ROS,
		CMap:            cmapData,
		Width:           ww,
		DefaultWidth:    testCFF.GlyphWidthPDF(0),
		DefaultVMetrics: dict.DefaultVMetricsDefault,
		FontFile:        cffglyphs.ToStream(testCFF, glyphdata.CFF),
	}

	origFont, err := cff.NewSimple(orig, nil)
	if err != nil {
		return nil, err
	}

	return &testFonts{
		testFont: fontDict.MakeFont(),
		testCFF:  testCFF,
		origFont: origFont,
	}, nil
}

func rescaleGlyph(g *sfntcff.Glyph, xScale, yScale float64) *sfntcff.Glyph {
	new := &sfntcff.Glyph{
		Cmds:  make([]sfntcff.GlyphOp, len(g.Cmds)),
		Width: math.Round(g.Width * xScale),
	}
	for i, cmd := range g.Cmds {
		newCmd := sfntcff.GlyphOp{
			Op:   cmd.Op,
			Args: make([]float64, len(cmd.Args)),
		}
		for j, arg := range cmd.Args {
			if j%2 == 0 {
				newCmd.Args[j] = math.Round(arg * xScale)
			} else {
				newCmd.Args[j] = math.Round(arg * yScale)
			}
		}
		new.Cmds[i] = newCmd
	}
	return new
}

func clone[T any](x *T) *T {
	copy := *x
	return &copy
}
