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

// Command hover writes a PDF file with a free text annotation that draws a grey
// frame around its text normally and a blue frame when the pointer hovers over
// it.
//
// The hover effect uses the annotation's rollover appearance (the /R entry of
// the appearance dictionary, PDF 2.0 section 12.5.5).  The library's fallback
// appearance generator only produces normal (/N) appearances, so the rollover
// appearance has to be supplied by hand, as shown here.
//
// A free text annotation requires a default appearance string (/DA); the font
// it names is resolved through the document's interactive form default
// resources (/DR), so the document defines a minimal AcroForm for that purpose.
//
// The annotation is locked so that viewers do not let the user edit the text
// and regenerate the appearance: a regenerated appearance cannot reproduce the
// rollover, the frame colour, or the vertical centring of the text.
package main

import (
	"fmt"
	"log"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/acroform"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/annotation/appearance"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/standard"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/content/builder"
	"seehuhn.de/go/pdf/graphics/form"
)

const (
	message    = "Hover over me"
	fontSize   = 14.0
	lineWidth  = 1.5
	frameLevel = 0.6 // grey level of the normal-appearance frame
)

// frameGray is the colour of the normal-appearance frame.
var frameGray = color.DeviceGray(frameLevel)

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

	F := font.Must(standard.Helvetica.New())

	// The free text annotation's /DA names the font "Helv"; define it in the
	// form's default resources (/DR) so the name resolves.
	doc.Out.GetMeta().Catalog.AcroForm = doc.RM.StoreDeferred(&acroform.InteractiveForm{
		DefaultResources: &content.Resources{
			Font: map[pdf.Name]font.Instance{"Helv": F},
		},
	})

	// page heading
	doc.TextBegin()
	doc.TextSetFont(F, 20)
	doc.TextFirstLine(72, 760)
	doc.TextShow("Rollover appearance")
	doc.TextSetFont(F, 12)
	doc.TextSecondLine(0, -28)
	doc.TextShow("Move the pointer over the box below; the frame turns blue.")
	doc.TextEnd()

	rect := pdf.Rectangle{LLx: 72, LLy: 690, URx: 232, URy: 720}

	ft := &annotation.FreeText{
		Common: annotation.Common{
			Rect:     rect,
			Contents: message,
			// Lock the annotation so viewers do not let the user edit the text.
			// Editing would make the viewer regenerate the appearance, which
			// cannot reproduce this annotation's frame colour or vertical
			// centring and would discard the rollover appearance entirely.
			Flags: annotation.FlagLocked | annotation.FlagLockedContents,
			Appearance: &appearance.Dict{
				Normal:   box(rect, F, frameGray),
				RollOver: box(rect, F, color.DeviceRGB{0, 0, 1}),
			},
		},
		DefaultAppearance: fmt.Sprintf("/Helv %g Tf 0 g", fontSize),
	}
	doc.Page.Annots = append(doc.Page.Annots, ft)

	return doc.Close()
}

// box builds the annotation's appearance: the message in black, framed by a
// rectangle in the given colour.  The form's bounding box matches the size of
// the annotation rectangle, so the annotation maps it onto rect.
func box(rect pdf.Rectangle, F font.Instance, frameColor color.Color) *form.Form {
	w := rect.URx - rect.LLx
	h := rect.URy - rect.LLy

	b := builder.New(content.Form, nil, pdf.V2_0)

	b.TextBegin()
	b.TextSetFont(F, fontSize)
	b.TextFirstLine(8, (h-fontSize)/2+2) // approximate vertical centring
	b.TextShow(message)
	b.TextEnd()

	b.SetStrokeColor(frameColor)
	b.SetLineWidth(lineWidth)
	b.Rectangle(lineWidth/2, lineWidth/2, w-lineWidth, h-lineWidth)
	b.Stroke()

	return &form.Form{
		Content: builder.Must(b.Harvest()),
		Res:     b.Resources,
		BBox:    pdf.Rectangle{URx: w, URy: h},
	}
}
