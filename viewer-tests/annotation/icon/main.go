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

// Command icon shows the icons the specification names for annotations
// displayed as an icon, one column per value of the /C entry.  §12.5.2 says
// /C is "the background of the annotation's icon when closed", so the
// coloured columns say whether a viewer reads it as the background or as
// the ink of the icon itself.
//
// Rubber stamps are left out: they are named by the same /Name entry but
// drawn as a label rather than as an icon.
package main

import (
	"fmt"
	"os"

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/annotation/colorenc"
	"seehuhn.de/go/pdf/annotation/fallback"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/standard"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/viewer-tests/internal/iconannot"
)

const (
	iconSize  = iconannot.Size
	cellPitch = 34.0
	rowPitch  = 38.0

	labelX     = 60.0
	leftGroup  = 150.0
	rightGroup = 340.0

	firstRowY = 730.0 // top edge of the first row of icons
)

// The icons are drawn over a hatched panel, so that an icon which fills its
// background white can be told from one which leaves it alone: the hatching
// shows through the second and not the first.
const (
	hatchPitch = 4.0 // between one hatch line and the next
	hatchPad   = 5.0 // between the panel's edge and the icons inside it
)

var hatchInk = color.DeviceGray(0.78)

// the three columns: what the annotation's /C entry says
var columns = []struct {
	head  string
	color color.Color
}{
	{"no /C", nil},
	{"/C []", colorenc.Transparent},
	// a pale blue: light enough to work as the background the specification
	// says /C is, so that an icon drawn in it instead stands out as wrong
	{"/C blue", color.DeviceRGB{0.80, 0.90, 0.98}},
	// dark enough that an icon has to be drawn in light ink to be seen
	{"/C dark", color.DeviceRGB{0.10, 0.18, 0.40}},
}

func main() {
	err := createDocument("test.pdf")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func createDocument(filename string) error {
	opt := &pdf.WriterOptions{
		HumanReadable: true,
	}
	page, err := document.CreateSinglePage(filename, document.A4, pdf.V1_7, opt)
	if err != nil {
		return err
	}

	style, err := fallback.NewStyle().New(pdf.V1_7)
	if err != nil {
		return err
	}

	w := &writer{
		page:  page,
		style: style,
		bold:  font.Must(standard.HelveticaBold.New()),
		body:  font.Must(standard.Helvetica.New()),
	}

	w.text(labelX, 800, w.bold, 13, "Annotation icons, and what /C colours")
	w.text(labelX, 782, w.body, 9,
		"Left: the annotations have no appearance stream, so your viewer draws them.  Right: the same ones, with Quire's.")

	w.text(leftGroup, 760, w.bold, 10, "your viewer")
	w.text(rightGroup, 760, w.bold, 10, "Quire")
	for i, c := range columns {
		w.text(leftGroup+float64(i)*cellPitch, 746, w.body, 7, c.head)
		w.text(rightGroup+float64(i)*cellPitch, 746, w.body, 7, c.head)
	}

	w.rowY = firstRowY
	for _, g := range iconannot.Groups {
		w.text(labelX, w.rowY-9, w.bold, 9, g.Kind)
		w.rowY -= 16

		// the panel the icons of this group sit on, one per side
		bottom := w.rowY - float64(len(g.Icons)-1)*rowPitch - iconSize
		for _, gx := range []float64{leftGroup, rightGroup} {
			w.hatch(pdf.Rectangle{
				LLx: gx - hatchPad,
				LLy: bottom - hatchPad,
				URx: gx + float64(len(columns)-1)*cellPitch + iconSize + hatchPad,
				URy: w.rowY + hatchPad,
			})
		}

		for _, icon := range g.Icons {
			if err := w.addRow(g.Kind, icon); err != nil {
				return err
			}
		}
	}

	w.text(labelX, w.rowY-6, w.body, 9,
		"Clause 12.5.2: /C is \"the background of the annotation's icon when closed\"; an empty array is \"no colour; transparent\".")

	return page.Close()
}

type writer struct {
	page  *document.Page
	style *fallback.Generator
	bold  font.Layouter
	body  font.Layouter
	rowY  float64
}

// addRow draws one icon's annotations, one per column, drawn by the viewer
// on the left and by Quire on the right.
func (w *writer) addRow(kind, icon string) error {
	w.text(labelX+8, w.rowY-16, w.body, 8, icon)

	for i, c := range columns {
		x := leftGroup + float64(i)*cellPitch
		w.page.Page.AddAnnots(iconannot.New(kind, icon, annotation.Common{Color: c.color}, x, w.rowY))

		x = rightGroup + float64(i)*cellPitch
		a := iconannot.New(kind, icon, annotation.Common{Color: c.color}, x, w.rowY)
		if err := w.style.AddAppearance(a); err != nil {
			return err
		}
		// The file attachment and sound generators force NoZoom and
		// NoRotate, so that an icon stays icon-sized however the page is
		// magnified.  Here the flags are cleared again, so that both
		// columns scale together and the icons can be looked at closely.
		// A text annotation scales in no viewer whatever the flags say:
		// §12.5.6.4 has it behave as if they were always set.
		a.GetCommon().Flags &^= annotation.FlagNoZoom | annotation.FlagNoRotate
		w.page.Page.AddAnnots(a)
	}

	w.rowY -= rowPitch
	return nil
}

// hatch fills rect with diagonal lines, clipped to it.
func (w *writer) hatch(rect pdf.Rectangle) {
	w.page.PushGraphicsState()

	w.page.Rectangle(rect.LLx, rect.LLy, rect.Dx(), rect.Dy())
	w.page.ClipNonZero()
	w.page.EndPath()

	w.page.SetStrokeColor(hatchInk)
	w.page.SetLineWidth(0.3)
	// lines at 45 degrees: each one starts on the bottom edge, or on the
	// left edge once the start has run past the right-hand corner
	for x := rect.LLx - rect.Dy(); x < rect.URx; x += hatchPitch {
		w.page.MoveTo(x, rect.LLy)
		w.page.LineTo(x+rect.Dy(), rect.URy)
	}
	w.page.Stroke()

	w.page.PopGraphicsState()
}

func (w *writer) text(x, y float64, f font.Layouter, size float64, s string) {
	w.page.SetFillColor(color.Black)
	w.page.TextBegin()
	w.page.TextSetFont(f, size)
	w.page.TextSetMatrix(matrix.Translate(x, y))
	w.page.TextShow(s)
	w.page.TextEnd()
}
