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
	"math"

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/sfnt"

	"seehuhn.de/go/postscript/cid"
	"seehuhn.de/go/postscript/funit"
	"seehuhn.de/go/postscript/type1"

	"seehuhn.de/go/sfnt/cff"
	"seehuhn.de/go/sfnt/glyph"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/examples/feature-tests/cffcidbbox/text"
	"seehuhn.de/go/pdf/font"
	pdfcff "seehuhn.de/go/pdf/font/cff"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/cidfont"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/standard"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/internal/debug/makefont"
)

func main() {
	err := create("test.pdf")
	if err != nil {
		panic(err)
	}
}

func create(fname string) error {
	data, err := makeTestFonts()
	if err != nil {
		return err
	}

	// colors

	black := color.DeviceGray(0)
	red := color.DeviceRGB(0.9, 0, 0)
	blue := color.DeviceRGB(0, 0, 0.9)

	// fonts

	const fontSize = 12

	noteFont, err := standard.TimesRoman.New(nil)
	if err != nil {
		return err
	}
	note := text.F{Font: noteFont, Size: fontSize}

	origFont, err := pdfcff.New(data.origFont, nil)
	if err != nil {
		return err
	}
	orig := text.F{Font: origFont, Size: fontSize}

	test := text.F{Font: data.testFont, Size: fontSize}

	opt := &pdf.WriterOptions{
		HumanReadable: true,
	}
	w, err := document.CreateSinglePage(fname, document.A5, pdf.V2_0, opt)
	if err != nil {
		return err
	}
	w.SetFontNameInternal(origFont, "Orig")
	w.SetFontNameInternal(data.testFont, "Test")

	text.Show(w.Writer,
		text.M{X: 36, Y: 500},
		note, black, "The blue text is rendered using a regular font,",
		text.NewLine,
		"while the red text is rendered using the test font:",
		text.NewLine,
		orig, blue, "These two lines should look the same",
		text.NewLine,
		test, red, pdf.String("These two lines should look the same"),
		text.NewLine,
		text.NewLine,
		note, black, "The boxes are meant to tightly enclose the following glyphs:",
	)

	return w.Close()
}

type testFonts struct {
	testFont *testFont
	origFont *sfnt.Font
}

func makeTestFonts() (*testFonts, error) {
	orig := makefont.OpenType()

	// disable kerning and ligatures for the test
	orig.Gdef = nil
	orig.Gsub = nil
	orig.Gpos = nil

	origOutlines := orig.Outlines.(*cff.Outlines)
	if origOutlines.IsCIDKeyed() {
		panic("expected non-CID-keyed font")
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

	var newGlyphs []*cff.Glyph
	var GIDToCID []cid.CID
	cmapData := &cmap.InfoNew{
		Name: "TestCMap",
		ROS: &cmap.CIDSystemInfo{
			Registry: "seehuhn.de",
			Ordering: "test",
		},
		WMode: cmap.Horizontal,
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
	cmapData.CIDSingles = append(cmapData.CIDSingles, cmap.SingleNew{Code: []byte{' '}, Value: cmap.CID(newCID)})
	gid := lookup.Lookup(' ')
	newGlyphs = append(newGlyphs, rescaleGlyph(origOutlines.Glyphs[gid], qx, qy))

	for c := 'A'; c <= 'Z'; c++ {
		newCID := cid.CID(len(newGlyphs))
		cmapData.CIDSingles = append(cmapData.CIDSingles, cmap.SingleNew{Code: []byte{byte(c)}, Value: cmap.CID(newCID)})
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
		cmapData.CIDSingles = append(cmapData.CIDSingles, cmap.SingleNew{Code: []byte{byte(c)}, Value: cmap.CID(newCID)})
		GIDToCID = append(GIDToCID, newCID)
		gid := lookup.Lookup(c)
		newGlyphs = append(newGlyphs, rescaleGlyph(origOutlines.Glyphs[gid], qx, qy))
	}

	blueValues = make([]funit.Int16, len(private2.BlueValues))
	for i, v := range private2.BlueValues {
		blueValues[i] = funit.Int16(math.Round(float64(v) * qy))
	}

	// construct the new font

	fontInfo.FontName = "Test"
	newOutlines := &cff.Outlines{
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
		ROS:      &cff.CIDSystemInfo{Registry: "seehuhn.de", Ordering: "test"},
		GIDToCID: GIDToCID,
		FontMatrices: []matrix.Matrix{
			{0.001, 0, 0, 0.0005, 0, 0},
			{0.0005, 0, 0, 0.001, 0, 0},
		},
	}
	fontInfo.FontMatrix = matrix.Identity
	testCFF := &cff.Font{
		FontInfo: fontInfo,
		Outlines: newOutlines,
	}

	q := orig.FontMatrix[3] * 1000
	res := &testFonts{
		testFont: &testFont{
			data:      testCFF,
			ascent:    math.Round(orig.Ascent.AsFloat(q)),
			descent:   math.Round(orig.Descent.AsFloat(q)),
			capHeight: math.Round(orig.CapHeight.AsFloat(q)),
			cmap:      cmapData,
		},
		origFont: orig,
	}
	return res, nil
}

func rescaleGlyph(g *cff.Glyph, xScale, yScale float64) *cff.Glyph {
	new := &cff.Glyph{
		Cmds:  make([]cff.GlyphOp, len(g.Cmds)),
		Width: math.Round(g.Width * xScale),
	}
	for i, cmd := range g.Cmds {
		newCmd := cff.GlyphOp{
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

var _ font.Font = (*testFont)(nil)

type testFont struct {
	data      *cff.Font
	ascent    float64 // PDF glyph space units
	descent   float64 // PDF glyph space units, negative
	capHeight float64
	cmap      *cmap.InfoNew
}

func (f *testFont) PostScriptName() string {
	return f.data.FontName
}

func (f *testFont) Embed(rm *pdf.ResourceManager) (pdf.Native, font.Embedded, error) {
	fontDictRef := rm.Out.Alloc()

	fd := &font.Descriptor{
		FontName:   f.data.FontName,
		IsSymbolic: true,
		FontBBox:   f.data.FontBBoxPDF(),
		Ascent:     f.ascent,
		Descent:    f.descent,
		CapHeight:  f.capHeight,
		StemV:      80,
	}

	ww := make(map[cmap.CID]float64)
	for gid, cid := range f.data.GIDToCID {
		w := f.data.GlyphWidthPDF(glyph.ID(gid))
		ww[cmap.CID(cid)] = w
	}

	dicts := &cidfont.Type0Dict{
		Ref:            fontDictRef,
		PostScriptName: f.data.FontName,
		Descriptor:     fd,
		Encoding:       f.cmap,
		Width:          ww,
		DefaultWidth:   f.data.GlyphWidthPDF(0),
		GetFont: func() (cidfont.Type0FontData, error) {
			return f.data, nil
		},
	}
	err := dicts.Finish(rm)
	if err != nil {
		return nil, nil, err
	}

	e := &testFontEmbedded{
		ref:  fontDictRef,
		cmap: f.cmap,
		ww:   ww,
		dw:   f.data.GlyphWidthPDF(0),
	}
	return fontDictRef, e, nil
}

type testFontEmbedded struct {
	ref  pdf.Reference
	cmap *cmap.InfoNew
	ww   map[cmap.CID]float64
	dw   float64
}

func (e *testFontEmbedded) WritingMode() cmap.WritingMode {
	return cmap.Horizontal
}

func (e *testFontEmbedded) DecodeWidth(s pdf.String) (float64, int) {
	if len(s) == 0 {
		return 0, 0
	}

	cid := e.cmap.LookupCID(s[:1])
	w, ok := e.ww[cid]
	if !ok {
		return e.dw, 1
	}
	return w, 1
}

func clone[T any](x *T) *T {
	y := *x
	return &y
}
