// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2026  Jochen Voss <voss@seehuhn.de>
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

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/geom/rect"

	"seehuhn.de/go/postscript/afm"
	"seehuhn.de/go/postscript/type1"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/standard"
	pdftype1 "seehuhn.de/go/pdf/font/type1"
	"seehuhn.de/go/pdf/font/type3"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/content/builder"
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
	wrapWidth = 450.0

	// The test glyph "a" is a box.  In the Type 3 font it declares a
	// bounding box of 1000 by boxHeight glyph units but paints only the
	// left marksFrac of it; the Type 1 font paints the same box.  A viewer
	// which clips to the painted marks, to the declared box, or not at all
	// can be told apart by how far a bar painted after ET reaches.
	fontSize  = 60.0
	marksFrac = 0.4

	boxHeight   = 250.0 // in glyph units
	glyphHeight = fontSize * boxHeight / 1000
	barHeight   = 10.0 // the bar runs through the middle of the box
	barY0       = (glyphHeight - barHeight) / 2
	barLength   = 180.0 // length of the probe bar, from the glyph origin

	// vertical layout: the recorded text position is one line below the
	// description, so a negative gap puts the box top just under it
	descGap = -4.0
	caseGap = 66.0
)

// answer is one possible outcome, marked by where the bar ends.
type answer struct {
	label string
	end   float64 // right end of the bar, relative to the glyph origin
}

var answers = []answer{
	{"marks", fontSize * marksFrac},
	{"bbox", fontSize},
	{"no clip", barLength},
}

func createDocument(filename string) error {
	page, err := document.CreateSinglePage(filename, document.A4, pdf.V2_0, nil)
	if err != nil {
		return err
	}

	titleFont := font.Must(standard.TimesBold.New())
	title := text.F{Font: titleFont, Size: 10, Color: color.DeviceGray(0.2)}
	noteFont := font.Must(standard.TimesRoman.New())
	note := text.F{Font: noteFont, Size: 10, Color: color.DeviceGray(0.1)}

	outlineFont := font.Must(standard.HelveticaBold.New())
	labelFont := font.Must(standard.Helvetica.New())
	type1Font, err := makeType1Font()
	if err != nil {
		return err
	}
	type3Font, err := makeType3Font()
	if err != nil {
		return err
	}

	var y float64
	text.Show(page.Builder,
		text.M{X: margin, Y: 812},
		note,
		text.Wrap(wrapWidth,
			"Text rendering modes 4 to 7 intersect the clipping region with",
			"the outlines of the text shown.  Special rules apply when no",
			"glyph with an outline is shown, and to Type 3 fonts.  Viewers",
			"disagree on these cases.",
		),
		text.NL,
		title, "How to read this page", text.NL,
		note, text.Wrap(wrapWidth,
			"Each case runs a text object in a clipping mode and then,",
			"after ET, paints a blue bar from the glyph origin to the right.",
			"The glyph is a box reaching to the “marks” tick; the Type 3",
			"version declares a bounding box reaching to the “bbox” tick.",
			"The bar is cut off by whatever the text object left in the",
			"clipping path, so the label under its right end says what the",
			"viewer did: “marks” if it clips to the painted glyph marks,",
			"“bbox” if it clips to the declared bounding box, “no clip” if",
			"the text object set no clip.  No bar at all means the viewer",
			"clipped to an empty path.  Black above and below the bar means",
			"the viewer painted the glyph.",
			"Green marks what Quire does.",
		),
		text.RecordPos{UserY: &y},
	)

	y -= 24
	text.Show(page.Builder,
		text.M{X: margin, Y: y},
		title, "Case 0: mode 7, Type 1 font", text.NL,
		note, text.Wrap(wrapWidth,
			"The regular case.  Quire: the bar ends under “marks”,",
			"and the glyph is not painted.",
		),
		text.RecordPos{UserY: &y},
	)
	y -= descGap + glyphHeight
	probe(page, y, type1Font, "a", graphics.TextRenderingModeClip)
	legend(page, y, labelFont, "marks")

	y -= caseGap
	text.Show(page.Builder,
		text.M{X: margin, Y: y},
		title, "Case 1: mode 7, only a space shown", text.NL,
		note, text.Wrap(wrapWidth,
			"Helvetica-Bold, one space character.  Quire: no clip,",
			"and no glyph marks.",
		),
		text.RecordPos{UserY: &y},
	)
	y -= descGap + glyphHeight
	probe(page, y, outlineFont, " ", graphics.TextRenderingModeClip)
	legend(page, y, labelFont, "no clip")

	y -= caseGap
	text.Show(page.Builder,
		text.M{X: margin, Y: y},
		title, "Case 2: mode 7, no text shown", text.NL,
		note, text.Wrap(wrapWidth,
			"BT, Tf and Tr, then ET without any text-showing operator.",
			"Quire: no clip.",
		),
		text.RecordPos{UserY: &y},
	)
	y -= descGap + glyphHeight
	probe(page, y, outlineFont, "", graphics.TextRenderingModeClip)
	legend(page, y, labelFont, "no clip")

	y -= caseGap
	text.Show(page.Builder,
		text.M{X: margin, Y: y},
		title, "Case 3: mode 7, Type 3 font", text.NL,
		note, text.Wrap(wrapWidth,
			"Quire: no clip, and the glyph is not painted.",
		),
		text.RecordPos{UserY: &y},
	)
	y -= descGap + glyphHeight
	probe(page, y, type3Font, "a", graphics.TextRenderingModeClip)
	legend(page, y, labelFont, "no clip")

	y -= caseGap
	text.Show(page.Builder,
		text.M{X: margin, Y: y},
		title, "Case 4: mode 4, Type 3 font", text.NL,
		note, text.Wrap(wrapWidth,
			"Quire: no clip, and the glyph is painted.",
		),
		text.RecordPos{UserY: &y},
	)
	y -= descGap + glyphHeight
	probe(page, y, type3Font, "a", graphics.TextRenderingModeFillClip)
	legend(page, y, labelFont, "no clip")

	return page.Close()
}

// probe shows s in the given rendering mode with the glyph origin at
// (margin, y) and then paints the bar.  If s is empty, the text object
// contains no text-showing operator.
func probe(page *document.Page, y float64, F font.Instance, s string, mode graphics.TextRenderingMode) {
	page.PushGraphicsState()
	page.SetFillColor(color.DeviceGray(0))
	page.TextBegin()
	page.TextSetFont(F, fontSize)
	page.TextSetRenderingMode(mode)
	page.TextFirstLine(margin, y)
	if s != "" {
		page.TextShow(s)
	}
	page.TextEnd()

	page.SetFillColor(color.DeviceRGB{0.2, 0.4, 1})
	page.Rectangle(margin, y+barY0, barLength, barHeight)
	page.Fill()
	page.PopGraphicsState()
}

// legend draws the answer scale under the bar of the row at y.  The
// answer matching Quire is printed in green.
func legend(page *document.Page, y float64, labelFont font.Instance, quire string) {
	y0 := y - 2
	page.PushGraphicsState()
	defer page.PopGraphicsState()

	page.SetStrokeColor(color.DeviceGray(0.5))
	page.SetLineWidth(0.5)
	page.MoveTo(margin, y0)
	page.LineTo(margin, y0-4)
	for _, a := range answers {
		page.MoveTo(margin+a.end, y0)
		page.LineTo(margin+a.end, y0-4)
	}
	page.MoveTo(margin, y0)
	page.LineTo(margin+barLength, y0)
	page.Stroke()

	for _, a := range answers {
		var col color.Color = color.DeviceGray(0.5)
		if a.label == quire {
			col = color.DeviceRGB{0, 0.6, 0}
		}
		// written downwards from the tick which ends the interval
		M := matrix.Matrix{0, -1, 1, 0, margin + a.end - 2.5, y0 - 5}
		page.TextBegin()
		page.TextSetFont(labelFont, 7)
		page.SetFillColor(col)
		page.TextSetMatrix(M)
		page.TextShow(a.label)
		page.TextEnd()
	}
}

// makeType3Font returns a Type 3 font whose glyph "a" declares a bounding
// box of 1000 by boxHeight units but paints only the left marksFrac of it.
func makeType3Font() (font.Instance, error) {
	b := builder.New(content.Glyph, nil, pdf.V2_0)
	b.Type3UncoloredGlyph(1000, 0, 0, 0, 1000, boxHeight)
	b.Rectangle(0, 0, 1000*marksFrac, boxHeight)
	b.Fill()
	stream, err := b.Harvest()
	if err != nil {
		return nil, err
	}
	F := &type3.Font{
		Glyphs: []*type3.Glyph{
			{}, // .notdef
			{Name: "a", Content: stream},
		},
		FontMatrix: matrix.Matrix{0.001, 0, 0, 0.001, 0, 0},
	}
	return F.New()
}

// makeType1Font returns a Type 1 font whose glyph "a" paints the same box
// as the Type 3 glyph.
func makeType1Font() (font.Instance, error) {
	encoding := make([]string, 256)
	for i := range encoding {
		encoding[i] = ".notdef"
	}
	encoding['a'] = "a"

	box := &type1.Glyph{WidthX: 1000}
	box.MoveTo(0, 0)
	box.LineTo(1000*marksFrac, 0)
	box.LineTo(1000*marksFrac, boxHeight)
	box.LineTo(0, boxHeight)
	box.ClosePath()

	F := &type1.Font{
		FontInfo: &type1.FontInfo{
			FontName:   "BoxFont",
			FullName:   "Box Font",
			FamilyName: "BoxFont",
			Weight:     "Medium",
			Version:    "1.0",
			FontMatrix: matrix.Matrix{0.001, 0, 0, 0.001, 0, 0},
		},
		Outlines: &type1.Outlines{
			Glyphs: map[string]*type1.Glyph{
				".notdef": {WidthX: 1000},
				"a":       box,
			},
			Private:  &type1.PrivateDict{BlueValues: []float64{0, 0}},
			Encoding: encoding,
		},
	}
	M := &afm.Metrics{
		Glyphs: map[string]*afm.GlyphInfo{
			".notdef": {WidthX: 1000},
			"a": {
				WidthX: 1000,
				BBox:   rect.Rect{URx: 1000 * marksFrac, URy: boxHeight},
			},
		},
		Encoding:  encoding,
		FontName:  "BoxFont",
		FullName:  "Box Font",
		Version:   "1.0",
		Ascent:    boxHeight,
		CapHeight: boxHeight,
	}
	return pdftype1.New(F, M)
}
