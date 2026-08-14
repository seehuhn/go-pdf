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
	"io"
	"os"
	"time"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/action"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/annotation/appearance"
	"seehuhn.de/go/pdf/annotation/fallback"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/standard"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/content/builder"
	"seehuhn.de/go/pdf/graphics/form"
	"seehuhn.de/go/pdf/oc"
	pdfsound "seehuhn.de/go/pdf/sound"
)

// This viewer test checks whether annotation hit testing follows the
// optional-content state (§8.11, §12.5).  A comment thread and a hyperlink
// live in the layer "Notes layer"; a sound annotation and a second link that
// toggles the layer (a SetOCGState action, §12.6.4.13) live outside it.
//
// Expected behaviour, whether the layer is switched in the viewer's layers
// panel or through the toggle link:
//
//   - with the layer off, the comment and the framed link neither draw nor
//     react to the pointer: no popup, no link cursor, no activation;
//   - with the layer back on, both work again;
//   - the sound annotation is unaffected by the layer, staying visible and
//     clickable; only hiding annotations (if the viewer offers it) removes it;
//   - the toggle link always works, and a viewer with a comments sidebar
//     keeps listing the comment while it is hidden.
//
// A viewer that caches its clickable regions when the file is opened fails
// the first point: the hidden annotations stay clickable.
func main() {
	if err := createDocument("test.pdf"); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// linkFrame draws a labelled frame over a pale fill, so a link annotation
// says what it does and is visible exactly while it is shown.
func linkFrame(w, h float64, fill, stroke color.DeviceRGB, F font.Instance, label string) *form.Form {
	b := builder.New(content.Form, nil, pdf.V1_7)
	b.SetFillColor(fill)
	b.Rectangle(0, 0, w, h)
	b.Fill()
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
// outside the optional content group, so they stay visible when the layer
// hides the annotation and show where to aim the pointer.
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

	style, err := fallback.NewStyle().New(pdf.V1_7)
	if err != nil {
		return err
	}

	F := font.Must(standard.Helvetica.New())
	page.TextBegin()
	page.TextSetFont(F, 12)
	page.TextFirstLine(72, 760)
	page.TextShow("The comment below and the blue framed link are in the layer")
	page.TextSecondLine(0, -16)
	page.TextShow("\"Notes layer\".  With the layer off, neither may draw nor react")
	page.TextNextLine()
	page.TextShow("to the pointer.  The speaker sits outside the layer and stays")
	page.TextNextLine()
	page.TextShow("clickable; only hiding annotations removes it.  The gray link")
	page.TextNextLine()
	page.TextShow("toggles the layer and must always work.  The corner ticks are")
	page.TextNextLine()
	page.TextShow("page content and mark where the layered annotations sit.")
	page.TextEnd()

	group := &oc.Group{Name: "Notes layer"}
	base := time.Date(2026, 8, 13, 9, 0, 0, 0, time.UTC)

	// comment thread in the layer
	commentRect := pdf.Rectangle{LLx: 72, LLy: 640, URx: 92, URy: 660}
	parent := &annotation.Text{
		Common: annotation.Common{
			Rect:            commentRect,
			Contents:        "Only while the layer is on.",
			OptionalContent: group,
		},
		Markup: annotation.Markup{
			User:         "Alice",
			Subject:      "Layered note",
			CreationDate: base,
		},
		Icon: annotation.TextIconComment,
	}
	parentRef := page.RM.GetReference(parent)
	if err := style.AddAppearance(parent); err != nil {
		return err
	}
	reply := &annotation.Text{
		Common: annotation.Common{
			Rect:            commentRect,
			Contents:        "A reply inside the same layer.",
			OptionalContent: group,
		},
		Markup: annotation.Markup{
			User:         "Bob",
			InReplyTo:    parentRef,
			RT:           "R",
			CreationDate: base.Add(30 * time.Minute),
		},
		Icon: annotation.TextIconComment,
	}
	if err := style.AddAppearance(reply); err != nil {
		return err
	}

	// hyperlink in the layer
	linkRect := pdf.Rectangle{LLx: 72, LLy: 560, URx: 340, URy: 585}
	link := &annotation.Link{
		Common: annotation.Common{
			Rect:            linkRect,
			Contents:        "example.com, in the layer",
			OptionalContent: group,
			Appearance: &appearance.Dict{
				Normal: linkFrame(linkRect.URx-linkRect.LLx,
					linkRect.URy-linkRect.LLy,
					color.DeviceRGB{0.85, 0.92, 1},
					color.DeviceRGB{0, 0, 1},
					F, "open example.com, in the Notes layer"),
			},
		},
		Action: &action.URI{URI: "https://example.com/layer"},
	}

	// sound annotation outside the layer
	soundRect := pdf.Rectangle{LLx: 72, LLy: 480, URx: 92, URy: 500}
	samples := make([]byte, 8000)
	for i := range samples {
		// a quiet 250 Hz square wave, one second
		if (i/16)%2 == 0 {
			samples[i] = 0x70
		} else {
			samples[i] = 0x90
		}
	}
	sound := &annotation.Sound{
		Common: annotation.Common{
			Rect:     soundRect,
			Contents: "A sound outside the layer.",
		},
		Markup: annotation.Markup{
			User:         "Carol",
			CreationDate: base.Add(time.Hour),
		},
		Sound: &pdfsound.Sound{
			SampleRate:    8000,
			Channels:      1,
			BitsPerSample: 8,
			Encoding:      pdfsound.EncodingRaw,
			Data: &pdfsound.InlineSource{
				WriteData: func(w io.Writer) error {
					_, err := w.Write(samples)
					return err
				},
			},
		},
		Icon: annotation.SoundIconSpeaker,
	}
	if err := style.AddAppearance(sound); err != nil {
		return err
	}

	// a link outside the layer which toggles it (§12.6.4.13)
	toggleRect := pdf.Rectangle{LLx: 72, LLy: 400, URx: 340, URy: 425}
	toggle := &annotation.Link{
		Common: annotation.Common{
			Rect:     toggleRect,
			Contents: "toggle the Notes layer",
			Appearance: &appearance.Dict{
				Normal: linkFrame(toggleRect.URx-toggleRect.LLx,
					toggleRect.URy-toggleRect.LLy,
					color.DeviceRGB{0.93, 0.93, 0.93},
					color.DeviceRGB{0.35, 0.35, 0.35},
					F, "toggle the Notes layer"),
			},
		},
		Action: &action.SetOCGState{
			State: []action.OCGStateChange{
				{Op: action.OCGOperationToggle, Groups: []*oc.Group{group}},
			},
		},
	}

	// The comment's rectangle is adjusted while its appearance is built, so
	// the ticks have to be drawn from the final value.
	drawCorners(page, parent.Rect)
	drawCorners(page, link.Rect)

	page.Page.Annots = append(page.Page.Annots, parent, reply, link, sound, toggle)

	// Order is what puts the group in the viewer's layers panel: groups left
	// out of it "shall not be presented in any user interface" (§8.11.4.3).
	props := &oc.Properties{
		OCGs: []*oc.Group{group},
		D: &oc.Configuration{
			Name:  "Default",
			Order: []oc.OrderItem{group},
		},
	}
	ocRef, err := page.RM.Embed(props)
	if err != nil {
		return err
	}
	page.Out.GetMeta().Catalog.OCProperties = ocRef

	return page.Close()
}
