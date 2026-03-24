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
	"seehuhn.de/go/pdf/font/standard"
	"seehuhn.de/go/pdf/font/type1"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/oc"
)

// This viewer test checks how viewers evaluate visibility expressions (VEs)
// that reference an OCG whose intent does not match the default configuration.
//
// Group D has Intent=Design; the default configuration has Intent=View.
// The spec (8.11.2.3) says D shall have "no effect on visibility" but does
// not define the semantics inside a VE that references D.
//
// Three test expressions distinguish three possible viewer behaviours:
//
//	D treated as ON   (or: viewer does not filter by intent)
//	D ignored         ("no opinion" — the group is transparent in VEs)
//	D treated as OFF
//
// The page shows both the raw test results and a self-diagnosing
// interpretation section.
func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	paper := &pdf.Rectangle{URx: 500, URy: 680}
	opt := &pdf.WriterOptions{HumanReadable: true}
	page, err := document.CreateSinglePage("test.pdf", paper, pdf.V1_7, opt)
	if err != nil {
		return err
	}

	F := standard.Helvetica.New()

	// --- OCG setup ---

	// A and B have Intent=View (the default), matching the configuration.
	// D has Intent=Design, which does NOT match.
	groupA := &oc.Group{Name: "A (View, ON)"}
	groupB := &oc.Group{Name: "B (View, OFF)"}
	groupD := &oc.Group{Name: "D (Design)", Intent: []pdf.Name{"Design"}}

	props := &oc.Properties{
		OCGs: []*oc.Group{groupA, groupB, groupD},
		D: &oc.Configuration{
			BaseState: oc.BaseStateON,
			OFF:       []*oc.Group{groupB},
			Order:     []oc.OrderItem{groupA, groupB, groupD},
			Locked:    []*oc.Group{groupA, groupB, groupD},
		},
	}
	ocRef, err := page.RM.Embed(props)
	if err != nil {
		return err
	}
	page.Out.GetMeta().Catalog.OCProperties = ocRef

	// --- test VEs ---
	//
	// Each expression produces a different visible/hidden pattern depending
	// on how the viewer treats D:
	//
	//                   D=ON     D=ignored  D=OFF
	//   Not(D)          hidden   visible    visible
	//   Or(B,D)         visible  hidden     hidden
	//   And(A,D)        visible  visible    hidden

	testNotD := newOCMD(&oc.VisibilityExpressionNot{
		Arg: &oc.VisibilityExpressionGroup{Group: groupD},
	})
	testOrBD := newOCMD(&oc.VisibilityExpressionOr{Args: []oc.VisibilityExpression{
		&oc.VisibilityExpressionGroup{Group: groupB},
		&oc.VisibilityExpressionGroup{Group: groupD},
	}})
	testAndAD := newOCMD(&oc.VisibilityExpressionAnd{Args: []oc.VisibilityExpression{
		&oc.VisibilityExpressionGroup{Group: groupA},
		&oc.VisibilityExpressionGroup{Group: groupD},
	}})

	// --- interpretation VEs ---
	//
	// Each is visible for exactly one interpretation of D.

	// Or(B,D): visible only when D is treated as ON (B is OFF).
	interpON := newOCMD(&oc.VisibilityExpressionOr{Args: []oc.VisibilityExpression{
		&oc.VisibilityExpressionGroup{Group: groupB},
		&oc.VisibilityExpressionGroup{Group: groupD},
	}})

	// D AND Not(D): always false in classical logic, but vacuously true
	// when both D and Not(D) have "no opinion."
	interpIgnored := newOCMD(&oc.VisibilityExpressionAnd{Args: []oc.VisibilityExpression{
		&oc.VisibilityExpressionGroup{Group: groupD},
		&oc.VisibilityExpressionNot{
			Arg: &oc.VisibilityExpressionGroup{Group: groupD},
		},
	}})

	// Not(And(A,D)): false when D=ON (And=true), false when D=ignored
	// (And=true, only A contributes), true only when D=OFF.
	interpOFF := newOCMD(&oc.VisibilityExpressionNot{
		Arg: &oc.VisibilityExpressionAnd{Args: []oc.VisibilityExpression{
			&oc.VisibilityExpressionGroup{Group: groupA},
			&oc.VisibilityExpressionGroup{Group: groupD},
		}},
	})

	// --- draw page ---

	const (
		left  = 50.0
		lineH = 16.0
		tcol0 = left + 12
		tcol1 = 220.0
		tcol2 = 295.0
		tcol3 = 375.0
	)

	y := 645.0

	// title
	drawText(page, F, left, y, 14, "Non-participating groups in visibility expressions")
	y -= 28

	// group descriptions
	drawText(page, F, left, y, 10, "Groups (all locked):")
	y -= lineH
	drawText(page, F, left+12, y, 10,
		"A \u2014 Intent: View, state: ON (matches the default configuration)")
	y -= lineH
	drawText(page, F, left+12, y, 10,
		"B \u2014 Intent: View, state: OFF (matches the default configuration)")
	y -= lineH
	drawText(page, F, left+12, y, 10,
		"D \u2014 Intent: Design, state: ON by BaseState (does NOT match)")
	y -= 1.5 * lineH

	// explanation
	drawText(page, F, left, y, 10,
		"The default configuration has Intent = View. Per spec \u00a78.11.2.3,")
	y -= lineH
	drawText(page, F, left, y, 10,
		"group D shall have \u201cno effect on visibility.\u201d The spec does not")
	y -= lineH
	drawText(page, F, left, y, 10,
		"define what this means inside a visibility expression.")
	y -= 2 * lineH

	// --- test results ---

	drawText(page, F, left, y, 12, "Test results")
	y -= lineH
	drawGray(page, F, left, y, 9,
		"(black = expression evaluates to visible, gray = hidden)")
	y -= 1.3 * lineH

	drawGray(page, F, tcol1, y, 9, "Not(D)")
	drawGray(page, F, tcol2, y, 9, "Or(B,D)")
	drawGray(page, F, tcol3, y, 9, "And(A,D)")
	y -= lineH
	drawText(page, F, tcol0, y, 9, "this viewer:")
	drawCondText(page, F, tcol1, y, 9, "visible", testNotD)
	drawCondText(page, F, tcol2, y, 9, "visible", testOrBD)
	drawCondText(page, F, tcol3, y, 9, "visible", testAndAD)
	y -= 2 * lineH

	// --- expected results table ---

	drawText(page, F, left, y, 12, "Expected results")
	y -= 1.3 * lineH

	drawGray(page, F, tcol1, y, 9, "Not(D)")
	drawGray(page, F, tcol2, y, 9, "Or(B,D)")
	drawGray(page, F, tcol3, y, 9, "And(A,D)")
	y -= lineH
	drawText(page, F, tcol0, y, 9, "D treated as ON:")
	drawText(page, F, tcol2, y, 9, "visible")
	drawText(page, F, tcol3, y, 9, "visible")
	y -= lineH
	drawText(page, F, tcol0, y, 9, "D ignored:")
	drawText(page, F, tcol1, y, 9, "visible")
	drawText(page, F, tcol3, y, 9, "visible")
	y -= lineH
	drawText(page, F, tcol0, y, 9, "D treated as OFF:")
	drawText(page, F, tcol1, y, 9, "visible")
	y -= 2 * lineH

	// --- self-diagnosing interpretation ---

	drawText(page, F, left, y, 12, "This viewer:")
	y -= 1.3 * lineH

	drawCondText(page, F, left+12, y, 10,
		"treats non-participating groups as ON in VEs (or ignores intent)",
		interpON)
	y -= lineH
	drawCondText(page, F, left+12, y, 10,
		"ignores non-participating groups in VEs (no opinion)",
		interpIgnored)
	y -= lineH
	drawCondText(page, F, left+12, y, 10,
		"treats non-participating groups as OFF in VEs",
		interpOFF)

	return page.Close()
}

// newOCMD creates an inline OCMD with the given visibility expression.
func newOCMD(ve oc.VisibilityExpression) *oc.Membership {
	return &oc.Membership{VE: ve, SingleUse: true}
}

// drawText draws black text at the given position.
func drawText(page *document.Page, F *type1.Instance, x, y, size float64, s string) {
	page.SetFillColor(color.DeviceGray(0))
	page.TextBegin()
	page.TextSetFont(F, size)
	page.TextFirstLine(x, y)
	page.TextShow(s)
	page.TextEnd()
}

// drawGray draws gray text at the given position.
func drawGray(page *document.Page, F *type1.Instance, x, y, size float64, s string) {
	page.SetFillColor(color.DeviceGray(0.5))
	page.TextBegin()
	page.TextSetFont(F, size)
	page.TextFirstLine(x, y)
	page.TextShow(s)
	page.TextEnd()
}

// drawCondText draws text that appears gray when cond is false and black
// when cond is true. The gray version is always drawn underneath; a black
// overlay controlled by the OC condition is drawn on top.
func drawCondText(page *document.Page, F *type1.Instance, x, y, size float64, s string, cond oc.Conditional) {
	// gray background (always visible)
	page.SetFillColor(color.DeviceGray(0.82))
	page.TextBegin()
	page.TextSetFont(F, size)
	page.TextFirstLine(x, y)
	page.TextShow(s)
	page.TextEnd()

	// black overlay (visible when condition is true)
	page.MarkedContentStart(&graphics.MarkedContent{
		Tag:        "OC",
		Properties: cond,
	})
	page.SetFillColor(color.DeviceGray(0))
	page.TextBegin()
	page.TextSetFont(F, size)
	page.TextFirstLine(x, y)
	page.TextShow(s)
	page.TextEnd()
	page.MarkedContentEnd()
}
