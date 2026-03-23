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
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/oc"
)

// This viewer test demonstrates an edge case in intent filtering with
// visibility expressions.
//
// Setup:
//   - One View-intent group (participating, toggleable)
//   - One Design-intent group (non-participating under default View config)
//   - Content gated by Not(Design group) via a VE expression
//
// Expected (per spec 8.11.2.3): the Design group has "no effect on
// visibility" under a View config, so the Not(Design) content should
// be visible.
//
// The page shows four rows, top to bottom:
//
//	green  — no OC (always visible, reference)
//	blue   — View group (toggleable in layer panel)
//	red    — Not(Design) VE (should be visible)
//	orange — Design group directly (should be visible)
//
// If all four rectangles are visible, the viewer handles intent
// filtering correctly.  If the red rectangle is missing, the viewer
// evaluates Not(non-participating) as false.
func main() {
	err := run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	paper := &pdf.Rectangle{URx: 400, URy: 400}
	opt := &pdf.WriterOptions{HumanReadable: true}
	page, err := document.CreateSinglePage("test.pdf", paper, pdf.V1_7, opt)
	if err != nil {
		return err
	}

	viewGroup := &oc.Group{
		Name:   "View Layer",
		Intent: []pdf.Name{"View"},
	}
	designGroup := &oc.Group{
		Name:   "Design Layer",
		Intent: []pdf.Name{"Design"},
	}

	notDesign := &oc.Membership{
		VE: &oc.VisibilityExpressionNot{
			Arg: &oc.VisibilityExpressionGroup{Group: designGroup},
		},
	}

	// register OCProperties in the catalog
	props := &oc.Properties{
		OCGs: []*oc.Group{viewGroup, designGroup},
		D: &oc.Configuration{
			BaseState: oc.BaseStateON,
			Order: []oc.OrderItem{
				viewGroup,
				designGroup,
			},
		},
	}
	ocRef, err := page.RM.Embed(props)
	if err != nil {
		return err
	}
	page.Out.GetMeta().Catalog.OCProperties = ocRef

	const (
		x = 50.0
		w = 300.0
		h = 60.0
	)
	y := 310.0

	// row 1: green rectangle, no OC
	page.SetFillColor(color.DeviceRGB{0, 0.7, 0})
	page.Rectangle(x, y, w, h)
	page.Fill()

	y -= 80

	// row 2: blue rectangle, View group
	page.MarkedContentStart(&graphics.MarkedContent{
		Tag:        "OC",
		Properties: viewGroup,
	})
	page.SetFillColor(color.DeviceRGB{0, 0, 0.8})
	page.Rectangle(x, y, w, h)
	page.Fill()
	page.MarkedContentEnd()

	y -= 80

	// row 3: red rectangle, Not(Design group)
	page.MarkedContentStart(&graphics.MarkedContent{
		Tag:        "OC",
		Properties: notDesign,
	})
	page.SetFillColor(color.DeviceRGB{0.8, 0, 0})
	page.Rectangle(x, y, w, h)
	page.Fill()
	page.MarkedContentEnd()

	y -= 80

	// row 4: orange rectangle, Design group directly
	page.MarkedContentStart(&graphics.MarkedContent{
		Tag:        "OC",
		Properties: designGroup,
	})
	page.SetFillColor(color.DeviceRGB{0.9, 0.5, 0})
	page.Rectangle(x, y, w, h)
	page.Fill()
	page.MarkedContentEnd()

	return page.Close()
}
