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

// This program generates a PDF file for checking how a viewer switches
// between an annotation's three appearance states (PDF 32000-2 §12.5.5):
// the normal appearance, the rollover appearance shown while the pointer is
// over the annotation, and the down appearance shown while the mouse button
// is held down inside it.
//
// The three appearances of each annotation paint disjoint shapes: a square on
// the left, a circle in the middle, a triangle on the right.  A state
// therefore does not cover the one it replaces, and a viewer which composites
// the states instead of substituting them shows two or three shapes at once.
package main

import (
	"fmt"
	"log"

	"seehuhn.de/go/geom/matrix"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/acroform"
	"seehuhn.de/go/pdf/action"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/annotation/appearance"
	"seehuhn.de/go/pdf/destination"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/standard"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/content/builder"
	"seehuhn.de/go/pdf/graphics/form"
)

const (
	annotWidth  = 210.0
	annotHeight = 54.0

	labelX = 72.0
	rectX  = 220.0
	firstY = 610.0
	rowGap = 84.0

	fontSize = 11.0
)

// the three states, each in its own colour and its own third of the
// annotation rectangle
var (
	normalColor   = color.DeviceRGB{0.25, 0.35, 0.75}
	rollOverColor = color.DeviceRGB{0.90, 0.50, 0.10}
	downColor     = color.DeviceRGB{0.10, 0.55, 0.25}
	guideColor    = color.DeviceGray(0.75)
	inkColor      = color.DeviceGray(0)
)

// state is one of the three appearance states an annotation can show.
type state int

const (
	stateNormal state = iota
	stateRollOver
	stateDown
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	doc, err := document.CreateSinglePage("test.pdf", document.A4, pdf.V2_0, nil)
	if err != nil {
		return err
	}

	// the link's action needs a reference to the page it targets, which is
	// this page itself, so clicking the link changes nothing
	pageRef := doc.Out.Alloc()
	doc.Ref = pageRef

	F := font.Must(standard.Helvetica.New())
	B := font.Must(standard.HelveticaBold.New())

	// The free text annotation's /DA and the push button both name the font
	// "Helv"; define it in the form's default resources (/DR) so the name
	// resolves.  The form also carries the push button's field.
	acro := &acroform.InteractiveForm{
		DefaultResources: &content.Resources{
			Font: map[pdf.Name]font.Instance{"Helv": F},
		},
	}

	doc.TextBegin()
	doc.TextSetFont(B, 18)
	doc.TextFirstLine(labelX, 780)
	doc.TextShow("Appearance states")
	doc.TextEnd()

	writeIntro(doc, F)

	y := firstY

	// row 1: link annotation
	rect := rowRect(y)
	writeRowLabel(doc, F, B, y, "Link", "/AP is often ignored for links.")
	link := &annotation.Link{
		Common: annotation.Common{
			Rect:       rect,
			Flags:      annotation.FlagPrint,
			Appearance: states(rect, F),
		},
		// A link's Push mode (§12.5.6.5) is described as a push-down effect
		// and does not mention /D, so the down appearance is not promised
		// here.  The default is Invert.
		Highlight: annotation.HighlightPush,
		Action:    &action.GoTo{Dest: &destination.Fit{Page: pageRef}},
	}
	doc.Page.Annots = append(doc.Page.Annots, link)
	drawGuide(doc, rect)
	y -= rowGap

	// row 2: free text annotation
	rect = rowRect(y)
	writeRowLabel(doc, F, B, y, "FreeText", "Markup: hidden by hide-markup.")
	freeText := &annotation.FreeText{
		Common: annotation.Common{
			Rect:  rect,
			Flags: annotation.FlagPrint | annotation.FlagLocked | annotation.FlagLockedContents,
			// Locked so that a viewer does not offer to edit the annotation:
			// editing regenerates the normal appearance and discards the
			// other two.
			Contents:   "Appearance states",
			Appearance: states(rect, F),
		},
		DefaultAppearance: fmt.Sprintf("/Helv %g Tf 0 g", fontSize),
	}
	doc.Page.Annots = append(doc.Page.Annots, freeText)
	drawGuide(doc, rect)
	y -= rowGap

	// row 3: push button widget
	rect = rowRect(y)
	writeRowLabel(doc, F, B, y, "Push button", "The usual consumer of /D.")
	button := acroform.NewButtonField("states")
	button.Flags = acroform.FieldPushbutton
	acro.Fields = append(acro.Fields, button)
	widget := annotation.AddWidget(button, rect)
	widget.Common.Flags = annotation.FlagPrint
	widget.Common.Appearance = states(rect, F)
	// a highlighting mode other than Push overrides the down appearance
	// (§12.5.6.19); the default is Invert
	widget.Highlight = annotation.HighlightPush
	doc.Page.Annots = append(doc.Page.Annots, widget)
	drawGuide(doc, rect)

	// the form is stored before the page is closed, so that the merged
	// field/widget object is in place when the page writes the widget
	doc.Out.GetMeta().Catalog.AcroForm = doc.RM.StoreDeferred(acro)

	return doc.Close()
}

// rowRect is the annotation rectangle of the row whose top edge is at y.
func rowRect(y float64) pdf.Rectangle {
	return pdf.Rectangle{
		LLx: rectX, LLy: y - annotHeight,
		URx: rectX + annotWidth, URy: y,
	}
}

func writeIntro(doc *document.Page, F font.Instance) {
	lines := []string{
		"Each annotation below carries three appearances which paint disjoint shapes:",
		"",
		"    N   blue square, left third        the normal appearance",
		"    R   orange circle, middle third    while the pointer is over the annotation",
		"    D   green triangle, right third    while the mouse button is held down inside it",
		"",
		"Exactly one shape should be visible at any time.  Two or three at once mean the viewer",
		"paints a state over the one it replaces instead of in place of it.  The grey dashes are",
		"page content marking the annotation rectangle, and stay visible throughout.",
	}
	doc.TextBegin()
	doc.TextSetFont(F, 10)
	doc.TextFirstLine(labelX, 752)
	for i, line := range lines {
		if i > 0 {
			doc.TextSecondLine(0, -13)
		}
		doc.TextShow(line)
	}
	doc.TextEnd()
}

func writeRowLabel(doc *document.Page, F, B font.Instance, y float64, name, note string) {
	doc.TextBegin()
	doc.TextSetFont(B, 10)
	doc.TextSetMatrix(matrix.Translate(labelX, y-16))
	doc.TextShow(name)
	doc.TextSetFont(F, 8)
	doc.TextSetMatrix(matrix.Translate(labelX, y-30))
	doc.TextShow(note)
	doc.TextEnd()
}

// drawGuide marks the annotation rectangle in the page content, so that the
// extent of the annotation stays visible whichever appearance is shown.
func drawGuide(doc *document.Page, rect pdf.Rectangle) {
	doc.PushGraphicsState()
	doc.SetStrokeColor(guideColor)
	doc.SetLineWidth(0.5)
	doc.SetLineDash([]float64{2, 2}, 0)
	doc.Rectangle(rect.LLx, rect.LLy, rect.URx-rect.LLx, rect.URy-rect.LLy)
	doc.Stroke()
	doc.PopGraphicsState()
}

// states builds the three appearances of one annotation.
func states(rect pdf.Rectangle, F font.Instance) *appearance.Dict {
	return &appearance.Dict{
		Normal:   shape(rect, F, stateNormal),
		RollOver: shape(rect, F, stateRollOver),
		Down:     shape(rect, F, stateDown),
	}
}

// shape builds the appearance of one state: a filled shape in its own third
// of the annotation rectangle, labelled with the state's letter.  All three
// forms share the bounding box, so the annotation maps them onto its
// rectangle identically and the shapes land where they are drawn.
func shape(rect pdf.Rectangle, F font.Instance, s state) *form.Form {
	w := rect.URx - rect.LLx
	h := rect.URy - rect.LLy
	third := w / 3

	b := builder.New(content.Form, nil, pdf.V2_0)

	// the sizes are whole numbers so the appearance streams hold exact
	// coordinates rather than the tail of a floating-point product
	const (
		squareSide   = 32.0
		circleRadius = 17.0
		triangleSide = 34.0
	)

	var letter string
	var cx float64
	switch s {
	case stateNormal:
		letter, cx = "N", third/2
		b.SetFillColor(normalColor)
		b.Rectangle(cx-squareSide/2, (h-squareSide)/2, squareSide, squareSide)
		b.Fill()

	case stateRollOver:
		letter, cx = "R", third*1.5
		b.SetFillColor(rollOverColor)
		b.Circle(cx, h/2, circleRadius)
		b.Fill()

	case stateDown:
		letter, cx = "D", third*2.5
		b.SetFillColor(downColor)
		b.MoveTo(cx, h/2+triangleSide/2)
		b.LineTo(cx-triangleSide/2, h/2-triangleSide/2)
		b.LineTo(cx+triangleSide/2, h/2-triangleSide/2)
		b.ClosePath()
		b.Fill()
	}

	// the state's letter, below its shape
	b.SetFillColor(inkColor)
	b.TextBegin()
	b.TextSetFont(F, 8)
	b.TextFirstLine(cx-2, 4)
	b.TextShow(letter)
	b.TextEnd()

	return &form.Form{
		Content: builder.Must(b.Harvest()),
		Res:     b.Resources,
		BBox:    pdf.Rectangle{URx: w, URy: h},
	}
}
