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
	"iter"
	"math"
	"os"

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/geom/rect"

	"seehuhn.de/go/postscript/cid"
	"seehuhn.de/go/postscript/funit"
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

	titleFont := standard.TimesBold.New()
	title := text.F{
		Font:  titleFont,
		Size:  10,
		Color: color.DeviceGray(0.2),
	}

	noteFont := standard.TimesRoman.New()
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

	testFont := makeTestFont()
	test := text.F{
		Font:  testFont,
		Size:  30,
		Color: color.DeviceRGB(0, 0, 0.8),
	}

	// -----------------------------------------------------------------------

	var y float64

	text.Show(page.Writer,
		text.M{X: margin, Y: 800},
		note,
		text.Wrap(340,
			"When decoding a PDF text string in a content stream, each code",
			"is mapped to character identifier (CID),",
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
	showRow(page, test, &y, 'A', 0, 0)
	showRow(page, test, &y, 'B', 1, 1)
	showRow(page, test, &y, 'C', 2, 2)
	y -= smallskip

	text.Show(page.Writer,
		text.M{X: 330, Y: y - 40},
		note,
		text.Wrap(200,
			"Crosses on the left indicate which glyph this PDF viewer shows,",
			"crosses on the right show which width is used."),
	)
	page.SetStrokeColor(color.DeviceGray(0))
	page.MoveTo(325, y-50)
	page.LineTo(240, y-50)
	page.MoveTo(245, y-47)
	page.LineTo(240, y-50)
	page.LineTo(245, y-53)
	page.Stroke()

	text.Show(page.Writer,
		text.M{X: margin, Y: y},
		title, "Test 1: glyph present", text.NL,
		note, text.Wrap(340,
			"The glyph should be shown, using its normal width.",
		),
		text.RecordPos{UserX: nil, UserY: &y},
	)
	y -= 4
	showHeaders(page, label, &y)
	showRow(page, test, &y, 'C', 2, 2)
	y -= smallskip

	text.Show(page.Writer,
		text.M{X: margin, Y: y},
		title, "Test 2: CID with no glyph, no notdef CID", text.NL,
		note, text.Wrap(340,
			"The glyph for CID 0 should be shown,",
			"but using the original glyph width.",
		),
		text.RecordPos{UserX: nil, UserY: &y},
	)
	y -= 4
	showRow(page, test, &y, 'E', 0, 2)
	y -= smallskip

	text.Show(page.Writer,
		text.M{X: margin, Y: y},
		title, "Test 3: CID with no glyph, notdef glyph present", text.NL,
		note, text.Wrap(340,
			"The custom notdef glyph should be shown.",
		),
		text.RecordPos{UserX: nil, UserY: &y},
	)
	y -= 4
	showRow(page, test, &y, 'e', 1, -1)
	y -= smallskip

	text.Show(page.Writer,
		text.M{X: margin, Y: y},
		title, "Test 4: CID with no glyph, notdef glyph specified but missing", text.NL,
		note, text.Wrap(340,
			"CID 0 should be shown.",
		),
		text.RecordPos{UserX: nil, UserY: &y},
	)
	y -= 4
	showRow(page, test, &y, 'x', 0, -1)
	y -= smallskip

	text.Show(page.Writer,
		text.M{X: margin, Y: y},
		title, "Test 5: unmapped code, no notdef CID", text.NL,
		note, text.Wrap(340,
			"The glyph for CID 0 should be shown.",
		),
		text.RecordPos{UserX: nil, UserY: &y},
	)
	y -= 4
	showRow(page, test, &y, '0', 0, 0)
	y -= smallskip

	text.Show(page.Writer,
		text.M{X: margin, Y: y},
		title, "Test 6: unmapped code, notdef glyph present", text.NL,
		note, text.Wrap(340,
			"The custom notdef glyph should be shown.",
		),
		text.RecordPos{UserX: nil, UserY: &y},
	)
	y -= 4
	showRow(page, test, &y, '1', 1, 1)
	y -= smallskip

	text.Show(page.Writer,
		text.M{X: margin, Y: y},
		title, "Test 7: unmapped code, notdef glyph specified but missing", text.NL,
		note, text.Wrap(340,
			"CID 0 should be shown.",
		),
		text.RecordPos{UserX: nil, UserY: &y},
	)
	y -= 4
	showRow(page, test, &y, '2', 0, 1)
	y -= smallskip

	text.Show(page.Writer,
		text.M{X: margin, Y: y},
		title, "Test 8: invalid code", text.NL,
		note, text.Wrap(340,
			"CID 0 should be shown, and the width of CID 0 should be used.",
		),
		text.RecordPos{UserX: nil, UserY: &y},
	)
	y -= 4
	showRow(page, test, &y, '!', 0, 0)

	// -----------------------------------------------------------------------

	err = page.Close()
	if err != nil {
		return err
	}

	return nil
}

func showHeaders(page *document.Page, font text.F, y *float64) {
	*y -= 32

	q := 30.0 / 1000.0

	M := matrix.Translate(margin+250*q+2, *y)
	M = matrix.RotateDeg(90).Mul(M)

	page.TextBegin()
	page.TextSetFont(font.Font, font.Size)
	page.SetFillColor(font.Color)
	page.TextSetMatrix(M)
	page.TextShow("CID 0")
	page.TextSecondLine(0, -500*q)
	page.TextShow("notdef")
	page.TextNextLine()
	page.TextShow("glyph")
	page.TextFirstLine(0, -1500*q)
	page.TextShow("width 0")
	page.TextNextLine()
	page.TextShow("CID 0 width")
	page.TextNextLine()
	page.TextShow("notdef wd")
	page.TextNextLine()
	page.TextShow("glyph width")
	page.TextNextLine()
	page.TextShow("default width")
	page.TextEnd()

	*y -= 11
}

func showRow(page *document.Page, test text.F, y *float64, code byte, gIdx, wIdx int) {
	for i := 0; i < 3; i++ {
		if i == gIdx {
			good(page, *y, i)
		} else {
			choice(page, *y, i)
		}
	}
	choice(page, *y, 5)
	for j := 0; j < 3; j++ {
		if j == wIdx {
			good(page, *y, j+6)
		} else {
			choice(page, *y, j+6)
		}
	}
	choice(page, *y, 9)
	text.Show(page.Writer,
		text.M{X: margin, Y: *y},
		test,
		pdf.String{code, 'D', 'D', 'D', 'D', 'D', 'A'},
	)

	*y -= 13
}

func choice(page *document.Page, y0 float64, idx int) {
	page.SetLineWidth(0.8)
	page.SetStrokeColor(color.DeviceGray(0.6))
	circle(page, y0, idx)
	page.Stroke()
}

func good(page *document.Page, y0 float64, idx int) {
	page.SetFillColor(color.DeviceRGB(0.3, 1, 0.3))
	circle(page, y0, idx)
	page.Fill()
}

func circle(page *document.Page, y0 float64, idx int) {
	q := 30.0 / 1000.0
	x := margin + (250+500*float64(idx))*q
	y := y0 + 150*q
	page.Circle(x, y, 130*q)
}

func makeTestFont() *testFont {
	var glyphs []*cff.Glyph

	g := &cff.Glyph{ // CID 0
		Width: 500,
	}
	drawCross(g, 250, 150, 140, 32)
	glyphs = append(glyphs, g)

	g = &cff.Glyph{ // CID 1
		Width: 1000,
	}
	drawCross(g, 750, 150, 140, 32)
	glyphs = append(glyphs, g)

	g = &cff.Glyph{ // CID 2
		Width: 1500,
	}
	drawCross(g, 1250, 150, 140, 32)
	glyphs = append(glyphs, g)

	g = &cff.Glyph{ // CID 3, blank
		Width: 500,
	}
	glyphs = append(glyphs, g)

	o := &cff.Outlines{
		Glyphs: glyphs,
		Private: []*type1.PrivateDict{
			{
				BlueValues: []funit.Int16{-10, 0, 290, 300},
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
			{Low: []byte{'A'}, High: []byte{'Z'}},
		},
		CIDRanges: []cmap.Range{
			{First: []byte{'A'}, Last: []byte{'Z'}, Value: 0},
		},
		NotdefSingles: []cmap.Single{
			{Code: []byte{'1'}, Value: 1},
			{Code: []byte{'2'}, Value: 5},
		},
		NotdefRanges: []cmap.Range{
			{First: []byte{'a'}, Last: []byte{'w'}, Value: 1},
			{First: []byte{'x'}, Last: []byte{'z'}, Value: 5},
		},
	}

	res := &testFont{
		data: fontCFF,
		cmap: cmap,
		widths: map[cid.CID]float64{
			0: 500,
			1: 1000,
			2: 1500, // glyph present in the font
			3: 500,
			4: 1500, // missing glyph
			5: 1000,
		},
		dw: 2000,
	}
	return res
}

var _ font.Font = (*testFont)(nil)

type testFont struct {
	data   *cff.Font
	cmap   *cmap.File
	widths map[cid.CID]float64
	dw     float64
}

func (f *testFont) PostScriptName() string {
	return f.data.FontName
}

func (f *testFont) Embed(rm *pdf.ResourceManager) (pdf.Native, font.Embedded, error) {
	fontDictRef := rm.Out.Alloc()

	fd := &font.Descriptor{
		FontName:   f.data.FontName,
		IsSymbolic: true,
		FontBBox:   rect.Rect{LLx: 0, LLy: 0, URx: 1500, URy: 300},
		Ascent:     300,
		Descent:    0,
		Leading:    400,
		StemV:      0,
	}
	dict := &dict.CIDFontType0{
		Ref:             fontDictRef,
		PostScriptName:  f.data.FontName,
		Descriptor:      fd,
		ROS:             f.cmap.ROS,
		CMap:            f.cmap,
		Width:           f.widths,
		DefaultWidth:    f.dw,
		DefaultVMetrics: dict.DefaultVMetricsDefault,
		FontType:        glyphdata.CFF,
		FontRef:         rm.Out.Alloc(),
	}
	err := dict.WriteToPDF(rm)
	if err != nil {
		return nil, nil, err
	}

	err = cffglyphs.Embed(rm.Out, dict.FontType, dict.FontRef, f.data)
	if err != nil {
		return nil, nil, err
	}

	e := &testFontEmbedded{
		ref:    fontDictRef,
		cmap:   f.cmap,
		widths: f.widths,
		dw:     f.dw,
	}
	return fontDictRef, e, nil
}

var _ font.Embedded = (*testFontEmbedded)(nil)

type testFontEmbedded struct {
	ref    pdf.Reference
	cmap   *cmap.File
	widths map[cmap.CID]float64
	dw     float64
}

func (e *testFontEmbedded) WritingMode() font.WritingMode {
	return font.Horizontal
}

func (e *testFontEmbedded) Codes(s pdf.String) iter.Seq[*font.Code] {
	return func(yield func(*font.Code) bool) {
		var code font.Code
		for i := range s {
			c := []byte{s[i]}
			cid := e.cmap.LookupCID(c)
			notdefCID := e.cmap.LookupNotdefCID(c)

			width, ok := e.widths[cid]
			if !ok {
				width = e.dw
			}

			code.CID = cid
			code.Notdef = notdefCID
			code.Width = width
			code.UseWordSpacing = (s[i] == 0x20)

			if !yield(&code) {
				break
			}
		}
	}
}

func drawCross(g *cff.Glyph, x, y, r, lw float64) {
	a := math.Round(r / math.Sqrt(2))
	b := math.Round(0.5 * lw / math.Sqrt(2))

	g.MoveTo(x-a-b, y-a+b)
	g.LineTo(x-a+b, y-a-b)
	g.LineTo(x, y-2*b)
	g.LineTo(x+a-b, y-a-b)
	g.LineTo(x+a+b, y-a+b)
	g.LineTo(x+2*b, y)
	g.LineTo(x+a+b, y+a-b)
	g.LineTo(x+a-b, y+a+b)
	g.LineTo(x, y+2*b)
	g.LineTo(x-a+b, y+a+b)
	g.LineTo(x-a-b, y+a-b)
	g.LineTo(x-2*b, y)
}
