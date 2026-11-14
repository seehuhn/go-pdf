// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"fmt"
	"os"

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/geom/rect"

	"seehuhn.de/go/postscript/cid"
	"seehuhn.de/go/postscript/type1"

	"seehuhn.de/go/sfnt/cff"
	"seehuhn.de/go/sfnt/glyph"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/dict"
	"seehuhn.de/go/pdf/font/glyphdata"
	"seehuhn.de/go/pdf/font/glyphdata/cffglyphs"
	"seehuhn.de/go/pdf/font/standard"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/text"
	"seehuhn.de/go/pdf/viewer-tests/internal/notdefgrid"
)

func main() {
	err := createDocument("test.pdf")
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

const (
	margin    = 72.0
	smallskip = 8.0
)

func createDocument(filename string) error {
	page, err := document.CreateSinglePage(filename, document.A4, pdf.V2_0, nil)
	if err != nil {
		return err
	}

	titleFont := font.Must(standard.TimesBold.New())
	title := text.F{
		Font:  titleFont,
		Size:  10,
		Color: color.DeviceGray(0.2),
	}

	noteFont := font.Must(standard.TimesRoman.New())
	note := text.F{
		Font:  noteFont,
		Size:  10,
		Color: color.DeviceGray(0.1),
	}
	label := text.F{
		Font:  noteFont,
		Size:  8,
		Color: color.DeviceGray(0.5),
	}

	F, err := makeTestFont()
	if err != nil {
		return err
	}
	test := text.F{
		Font:  F,
		Size:  30,
		Color: color.DeviceRGB{0, 0, 0.8},
	}

	// -----------------------------------------------------------------------

	var y float64

	text.Show(page.Builder,
		text.M{X: margin, Y: 800},
		note,
		text.Wrap(340,
			"When decoding a PDF text string for a composite font, each code",
			"is mapped to a character identifier (CID),",
			"which is then used to look up the corresponding glyph.",
			"The decoder must handle the following cases:"),
		" • code mapped to a CID, glyph present in the font", text.NL,
		" • code mapped to a CID, no corresponding glyph in the font", text.NL,
		" • valid code, not mapped to a CID", text.NL,
		" • invalid code", text.NL,
		text.NL,
		text.Wrap(340,
			"If there is no glyph, no CID, or no valid code,",
			"usually the glyph for CID 0 is shown.",
			"This glyph must always exist.",
			"A “notdef range” in the CMap can be used to map ranges of codes",
			"to alternative notdef CIDs, which are used in place of CID 0.",
			"The decoder must handle these cases:"),
		" • no notdef CID", text.NL,
		" • notdef CID specified, glyph present in font", text.NL,
		" • notdef CID specified, no corresponding glyph", text.NL,
		text.NL,
		text.Wrap(340,
			"The PDF specification does not always clearly define which glyph width should be used in different cases.",
			"This document helps to determine how different PDF viewers handle glyph width selection and display.",
		),
		text.NL,
		title, "Control", text.NL,
		note, text.Wrap(340,
			"There should be six crosses, one in each green circle.",
			"Otherwise this test file is broken.",
		),
		text.RecordPos{UserX: nil, UserY: &y},
	)

	y -= 4
	notdefgrid.Row(page, test, margin, &y, 'A', 0, 0)
	notdefgrid.Row(page, test, margin, &y, 'B', 1, 1)
	notdefgrid.Row(page, test, margin, &y, 'C', 2, 2)
	y -= smallskip

	text.Show(page.Builder,
		text.M{X: 330, Y: y - 40},
		note,
		text.Wrap(200,
			"Crosses on the left indicate which glyph this PDF viewer shows,",
			"crosses on the right show which width is used."),
	)
	page.SetStrokeColor(color.Black)
	page.MoveTo(325, y-50)
	page.LineTo(240, y-50)
	page.MoveTo(245, y-47)
	page.LineTo(240, y-50)
	page.LineTo(245, y-53)
	page.Stroke()

	text.Show(page.Builder,
		text.M{X: margin, Y: y},
		title, "Test 1: glyph present", text.NL,
		note, text.Wrap(340,
			"The glyph should be shown, using its normal width.",
		),
		text.RecordPos{UserX: nil, UserY: &y},
	)
	y -= 4
	notdefgrid.Headers(page, label, margin, &y)
	notdefgrid.Row(page, test, margin, &y, 'C', 2, 2)
	y -= smallskip

	text.Show(page.Builder,
		text.M{X: margin, Y: y},
		title, "Test 2: missing glyph, no notdef CID", text.NL,
		note, text.Wrap(340,
			"The glyph for CID 0 should be shown,",
			"but using the original glyph width.",
		),
		text.RecordPos{UserX: nil, UserY: &y},
	)
	y -= 4
	notdefgrid.Row(page, test, margin, &y, 'E', 0, 2)
	y -= smallskip

	text.Show(page.Builder,
		text.M{X: margin, Y: y},
		title, "Test 3: missing glyph, notdef glyph present", text.NL,
		note, text.Wrap(340,
			"The custom notdef glyph should be shown.",
		),
		text.RecordPos{UserX: nil, UserY: &y},
	)
	y -= 4
	notdefgrid.Row(page, test, margin, &y, 'G', 1, -1)
	y -= smallskip

	text.Show(page.Builder,
		text.M{X: margin, Y: y},
		title, "Test 4: missing glyph, notdef glyph specified but missing", text.NL,
		note, text.Wrap(340,
			"CID 0 should be shown.",
		),
		text.RecordPos{UserX: nil, UserY: &y},
	)
	y -= 4
	notdefgrid.Row(page, test, margin, &y, 'H', 0, -1)
	y -= smallskip

	text.Show(page.Builder,
		text.M{X: margin, Y: y},
		title, "Test 5: unmapped code, no notdef CID", text.NL,
		note, text.Wrap(340,
			"The glyph for CID 0 should be shown.",
		),
		text.RecordPos{UserX: nil, UserY: &y},
	)
	y -= 4
	notdefgrid.Row(page, test, margin, &y, '0', 0, 0)
	y -= smallskip

	text.Show(page.Builder,
		text.M{X: margin, Y: y},
		title, "Test 6: unmapped code, notdef glyph present", text.NL,
		note, text.Wrap(340,
			"The custom notdef glyph should be shown.",
		),
		text.RecordPos{UserX: nil, UserY: &y},
	)
	y -= 4
	notdefgrid.Row(page, test, margin, &y, '1', 1, 1)
	y -= smallskip

	text.Show(page.Builder,
		text.M{X: margin, Y: y},
		title, "Test 7: unmapped code, notdef glyph specified but missing", text.NL,
		note, text.Wrap(340,
			"CID 0 should be shown.",
		),
		text.RecordPos{UserX: nil, UserY: &y},
	)
	y -= 4
	notdefgrid.Row(page, test, margin, &y, '2', 0, 1)
	y -= smallskip

	text.Show(page.Builder,
		text.M{X: margin, Y: y},
		title, "Test 8: invalid code", text.NL,
		note, text.Wrap(340,
			"CID 0 should be shown, and the width of CID 0 should be used.",
		),
		text.RecordPos{UserX: nil, UserY: &y},
	)
	y -= 4
	notdefgrid.Row(page, test, margin, &y, '!', 0, 0)

	// -----------------------------------------------------------------------

	err = page.Close()
	if err != nil {
		return err
	}

	return nil
}

func makeTestFont() (font.Instance, error) {
	glyphs := []*cff.Glyph{
		notdefgrid.CrossGlyph(250, notdefgrid.WidthCID0),   // CID 0
		notdefgrid.CrossGlyph(750, notdefgrid.WidthNotdef), // CID 1
		notdefgrid.CrossGlyph(1250, notdefgrid.WidthGlyph), // CID 2
		notdefgrid.CrossGlyph(-1, 500),                     // CID 3, blank
	}

	o := &cff.Outlines{
		Glyphs: glyphs,
		Private: []*type1.PrivateDict{
			{
				BlueValues: []float64{-10, 0, 290, 300},
				BlueScale:  0.039625,
				BlueShift:  7,
				BlueFuzz:   1,
				StdHW:      20,
				StdVW:      20,
				ForceBold:  false,
			},
		},
		FDSelect: func(glyph.ID) int {
			return 0
		},
		ROS: &cid.SystemInfo{
			Registry:   "seehuhn.de",
			Ordering:   "test",
			Supplement: 0,
		},
		GIDToCID:     []cid.CID{0, 1, 2, 3},
		FontMatrices: []matrix.Matrix{matrix.Identity},
	}
	fontCFF := &cff.Font{
		FontInfo: &type1.FontInfo{
			FontName:   "Test",
			FontMatrix: [6]float64{0.001, 0, 0, 0.001, 0, 0},
		},
		Outlines: o,
	}

	cmap := &cmap.File{
		Name:  "TestCMap",
		ROS:   o.ROS,
		WMode: font.Horizontal,
		CodeSpaceRange: []charcode.Range{
			{Low: []byte{'0'}, High: []byte{'Z'}},
		},
		CIDRanges: []cmap.Range{
			{First: []byte{'A'}, Last: []byte{'Z'}, Value: 0},
		},
		NotdefSingles: []cmap.Single{
			{Code: []byte{'1'}, Value: 1},
			{Code: []byte{'2'}, Value: 5},
		},
		NotdefRanges: []cmap.Range{
			{First: []byte{'G'}, Last: []byte{'G'}, Value: 1},
			{First: []byte{'H'}, Last: []byte{'H'}, Value: 5},
		},
	}

	widths := map[cid.CID]float64{
		0: 500,
		1: 1000,
		2: 1500, // glyph present in the font
		3: 500,
		4: 1500, // missing glyph
		5: 1000,
		6: 1500, // missing glyph, notdef CID 1
		7: 1500, // missing glyph, notdef CID 5
	}
	dw := 2000.0

	fd := &font.Descriptor{
		FontName:   fontCFF.FontName,
		IsSymbolic: true,
		FontBBox:   rect.Rect{LLx: 0, LLy: 0, URx: 1500, URy: 300},
		Ascent:     300,
		Descent:    0,
		Leading:    400,
		StemV:      0,
	}
	dict := &dict.CIDFontType0{
		PostScriptName:  fontCFF.FontName,
		Descriptor:      fd,
		ROS:             cmap.ROS,
		CMap:            cmap,
		Width:           widths,
		DefaultWidth:    dw,
		DefaultVMetrics: dict.DefaultVMetricsDefault,
		FontFile:        cffglyphs.ToStream(fontCFF, glyphdata.CFF),
	}

	return dict.MakeFont(), nil
}
