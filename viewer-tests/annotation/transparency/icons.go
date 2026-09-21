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
	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
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
	leftGroup  = 200.0
	rightGroup = 330.0

	firstRowY = 730.0 // top edge of the first row of icons

	panelPad = (cellPitch - iconSize) / 2 // so that the two panels meet

	// the transparency every icon is shown at
	iconTransparency = 0.5
)

// the two columns: what the icons are drawn over, since a translucent icon
// is only as visible as what lies behind it
var columns = []struct {
	head string
	// back is the panel the icons sit on, or nil for the bare page
	back color.Color
}{
	{"on white", nil},
	{"on black", color.DeviceGray(0)},
}

// icons generates a file which shows every annotation icon at half opacity,
// as the viewer draws it beside the appearance this library writes for it.
//
// The icons carry no /C entry, so each gets the card colour the generator
// chooses rather than one the file asked for.
func icons(filename string) error {
	opt := &pdf.WriterOptions{
		HumanReadable: true,
	}
	// PDF 1.7, so that the annotations without an appearance stream are
	// legal: PDF 2.0 requires one for every annotation.
	page, err := document.CreateSinglePage(filename, document.A4, pdf.V1_7, opt)
	if err != nil {
		return err
	}

	style, err := fallback.NewStyle().New(pdf.V1_7)
	if err != nil {
		return err
	}

	w := &iconWriter{
		page:  page,
		style: style,
		bold:  font.Must(standard.HelveticaBold.New()),
		body:  font.Must(standard.Helvetica.New()),
	}

	w.text(labelX, 800, w.bold, 13, "Annotation icons at 50% opacity")
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

		// the panels this group's icons sit on: one per column, per side
		bottom := w.rowY - float64(len(g.Icons)-1)*rowPitch - iconSize
		for _, gx := range []float64{leftGroup, rightGroup} {
			for i, c := range columns {
				if c.back == nil {
					continue
				}
				x := gx + float64(i)*cellPitch
				w.page.SetFillColor(c.back)
				w.page.Rectangle(x-panelPad, bottom-panelPad,
					iconSize+2*panelPad, w.rowY+panelPad-(bottom-panelPad))
				w.page.Fill()
			}
		}

		for _, icon := range g.Icons {
			if err := w.addRow(g.Kind, icon); err != nil {
				return err
			}
		}
	}

	w.text(labelX, w.rowY-6, w.body, 9,
		"Clause 12.5.2: /CA \"shall not be used if the annotation has an appearance stream\", which then has to carry it.")

	return page.Close()
}

type iconWriter struct {
	page  *document.Page
	style *fallback.Generator
	bold  font.Layouter
	body  font.Layouter
	rowY  float64
}

// addRow draws one icon's four annotations: over white and over black, drawn
// by the viewer on the left and by Quire on the right.
func (w *iconWriter) addRow(kind, icon string) error {
	w.text(labelX+8, w.rowY-16, w.body, 8, icon)

	common := annotation.Common{
		StrokingTransparency:    iconTransparency,
		NonStrokingTransparency: iconTransparency,
	}
	for i := range columns {
		x := leftGroup + float64(i)*cellPitch
		w.page.Page.AddAnnots(iconannot.New(kind, icon, common, x, w.rowY))

		x = rightGroup + float64(i)*cellPitch
		a := iconannot.New(kind, icon, common, x, w.rowY)
		if err := w.style.AddAppearance(a); err != nil {
			return err
		}
		// the file attachment and sound generators force NoZoom and NoRotate;
		// clearing them again lets both sides scale together, so the icons
		// can be looked at closely
		a.GetCommon().Flags &^= annotation.FlagNoZoom | annotation.FlagNoRotate
		w.page.Page.AddAnnots(a)
	}

	w.rowY -= rowPitch
	return nil
}

func (w *iconWriter) text(x, y float64, f font.Layouter, size float64, s string) {
	w.page.SetFillColor(color.Black)
	w.page.TextBegin()
	w.page.TextSetFont(f, size)
	w.page.TextSetMatrix(matrix.Translate(x, y))
	w.page.TextShow(s)
	w.page.TextEnd()
}
