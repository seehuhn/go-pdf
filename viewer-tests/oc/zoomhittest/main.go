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

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/action"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/annotation/appearance"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/standard"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/content/builder"
	"seehuhn.de/go/pdf/graphics/form"
	"seehuhn.de/go/pdf/oc"
)

// This viewer test checks zoom-dependent optional content (§8.11.4.4)
// against hit testing and rendering.  The single hyperlink sits in a layer
// whose usage dictionary shows it only between 200% and 400% magnification,
// applied through the default configuration's View-event AS entry.
//
// Expected behaviour: the frame is drawn, the link cursor appears and a
// click activates, exactly while the displayed magnification is in
// [200%, 400%); below and above, the area is empty and dead.
//
// Two known failure shapes: a viewer that caches clickable regions ignores
// the zoom for hit testing entirely; a viewer that renders tiles at
// power-of-two levels evaluates the usage at the tile scale, so near a
// range boundary the pixels can disagree with the cursor.
func main() {
	if err := createDocument("test.pdf"); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// linkFrame draws a labelled green frame with a pale fill, so the link says
// what it does and is visible exactly while the zoom keeps its layer on.
func linkFrame(w, h float64, F font.Instance, label string) *form.Form {
	b := builder.New(content.Form, nil, pdf.V1_7)
	b.SetFillColor(color.DeviceRGB{0.88, 1, 0.88})
	b.Rectangle(0, 0, w, h)
	b.Fill()
	stroke := color.DeviceRGB{0, 0.5, 0}
	b.SetStrokeColor(stroke)
	b.SetLineWidth(1)
	b.Rectangle(0.5, 0.5, w-1, h-1)
	b.Stroke()

	const size = 10.0
	b.SetFillColor(stroke)
	b.TextBegin()
	b.TextSetFont(F, size)
	b.TextFirstLine(0, (h-0.7*size)/2)
	b.TextShowAligned(label, w, 0.5)
	b.TextEnd()

	return &form.Form{
		Content: &content.Operators{Ops: b.Stream},
		Res:     b.Resources,
		BBox:    pdf.Rectangle{URx: w, URy: h},
	}
}

// drawCorners marks a rectangle with an L-shaped tick at each corner, drawn
// in the page content rather than in an appearance stream.  The ticks are
// outside the optional content group, so they stay visible at magnifications
// which hide the link and show where to aim the pointer.
func drawCorners(page *document.Page, r pdf.Rectangle) {
	const (
		arm = 6.0 // length of each arm of the L
		gap = 3.0 // clearance between tick and rectangle
	)
	corners := []struct{ x, y, dx, dy float64 }{
		{r.LLx - gap, r.LLy - gap, 1, 1},
		{r.URx + gap, r.LLy - gap, -1, 1},
		{r.URx + gap, r.URy + gap, -1, -1},
		{r.LLx - gap, r.URy + gap, 1, -1},
	}

	page.SetLineWidth(0.5)
	page.SetStrokeColor(color.DeviceGray(0.6))
	for _, c := range corners {
		page.MoveTo(c.x, c.y+c.dy*arm)
		page.LineTo(c.x, c.y)
		page.LineTo(c.x+c.dx*arm, c.y)
	}
	page.Stroke()
}

func createDocument(filename string) error {
	page, err := document.CreateSinglePage(filename, document.A4, pdf.V1_7, nil)
	if err != nil {
		return err
	}

	F := font.Must(standard.Helvetica.New())
	page.TextBegin()
	page.TextSetFont(F, 12)
	page.TextFirstLine(72, 760)
	page.TextShow("The framed link below is in a layer shown only between")
	page.TextSecondLine(0, -16)
	page.TextShow("200% and 400% magnification.  It must be clickable, with")
	page.TextNextLine()
	page.TextShow("the pointing hand, exactly while its frame is visible.  The")
	page.TextNextLine()
	page.TextShow("corner ticks are page content and mark where the link sits.")
	page.TextEnd()

	group := &oc.Group{
		Name:  "Zoomed detail",
		Usage: &oc.Usage{Zoom: &oc.UsageZoom{Min: 2, Max: 4}},
	}

	linkRect := pdf.Rectangle{LLx: 72, LLy: 640, URx: 340, URy: 670}
	link := &annotation.Link{
		Common: annotation.Common{
			Rect:            linkRect,
			Contents:        "example.com, between 200% and 400%",
			OptionalContent: group,
			Appearance: &appearance.Dict{
				Normal: linkFrame(linkRect.URx-linkRect.LLx,
					linkRect.URy-linkRect.LLy,
					F, "open example.com, at 200% to 400%"),
			},
		},
		Action: &action.URI{URI: "https://example.com/zoom"},
	}
	drawCorners(page, link.Rect)
	page.Page.Annots = append(page.Page.Annots, link)

	// no Order entry: the group is managed by the zoom level, so it is kept
	// out of the layers panel (§8.11.4.3).  A manual switch there would count
	// as a user override and stop the usage application from being applied.
	props := &oc.Properties{
		OCGs: []*oc.Group{group},
		D: &oc.Configuration{
			Name: "Default",
			AS: []*oc.UsageApplication{{
				Event:     oc.EventView,
				OCGs:      []*oc.Group{group},
				Category:  []oc.Category{oc.CategoryZoom},
				SingleUse: true,
			}},
		},
	}
	ocRef, err := page.RM.Embed(props)
	if err != nil {
		return err
	}
	page.Out.GetMeta().Catalog.OCProperties = ocRef

	return page.Close()
}
