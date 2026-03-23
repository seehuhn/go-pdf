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
	"math"
	"os"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/action"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font/standard"
	"seehuhn.de/go/pdf/font/type1"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/oc"
)

// This viewer test generates a PDF showing an interactive logic circuit.
//
// Three input squares (A, B, C) are clickable and toggle between black (1)
// and white (0). The output square (O) is computed via an OCMD visibility
// expression:
//
//	O = (A AND B) OR (NOT C)
//
// The gate shapes (AND, OR, NOT) and connecting wires are static graphics.
// Initial state: all inputs ON, so O = 1.
func main() {
	err := run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	paper := &pdf.Rectangle{URx: 600, URy: 400}
	opt := &pdf.WriterOptions{HumanReadable: true}
	page, err := document.CreateSinglePage("test.pdf", paper, pdf.V1_7, opt)
	if err != nil {
		return err
	}

	F := standard.Helvetica.New()

	// input OCGs
	groupA := &oc.Group{Name: "Input A"}
	groupB := &oc.Group{Name: "Input B"}
	groupC := &oc.Group{Name: "Input C"}

	// output OCMD: O = (A AND B) OR (NOT C)
	output := &oc.Membership{
		VE: &oc.VisibilityExpressionOr{Args: []oc.VisibilityExpression{
			&oc.VisibilityExpressionAnd{Args: []oc.VisibilityExpression{
				&oc.VisibilityExpressionGroup{Group: groupA},
				&oc.VisibilityExpressionGroup{Group: groupB},
			}},
			&oc.VisibilityExpressionNot{
				Arg: &oc.VisibilityExpressionGroup{Group: groupC},
			},
		}},
		SingleUse: true,
	}

	// register OCProperties
	props := &oc.Properties{
		OCGs: []*oc.Group{groupA, groupB, groupC},
		D: &oc.Configuration{
			BaseState: oc.BaseStateON,
			Order: []oc.OrderItem{
				groupA,
				groupB,
				groupC,
			},
		},
	}
	ocRef, err := page.RM.Embed(props)
	if err != nil {
		return err
	}
	page.Out.GetMeta().Catalog.OCProperties = ocRef

	// embed OCG references for SetOCGState actions
	refA, err := page.RM.Embed(groupA)
	if err != nil {
		return err
	}
	refB, err := page.RM.Embed(groupB)
	if err != nil {
		return err
	}
	refC, err := page.RM.Embed(groupC)
	if err != nil {
		return err
	}

	// layout constants
	const (
		sq   = 30.0 // input/output square size
		sqH  = sq / 2
		gw   = 50.0 // gate width
		gh   = 40.0 // gate height
		ghH  = gh / 2
		lw   = 1.5 // line width for gates and wires
		bubR = 5.0 // NOT bubble radius
	)

	// column x positions
	const (
		inX   = 50.0  // input square left edge
		col1X = 170.0 // first gate column left edge
		col2X = 330.0 // second gate column left edge
		outX  = 500.0 // output square left edge
	)

	// y positions
	const (
		yA   = 320.0 // input A centre y
		yB   = 260.0 // input B centre y
		yC   = 190.0 // input C centre y
		yAND = 290.0 // AND gate centre y
		yNOT = 190.0 // NOT gate centre y
		yOR  = 280.0 // OR gate centre y
		yO   = 280.0 // output centre y
	)

	page.SetLineWidth(lw)
	page.SetStrokeColor(color.DeviceGray(0))

	// --- draw static gates ---

	drawAND(page, col1X, yAND-ghH, gw, gh)
	drawNOT(page, col1X, yNOT-ghH, gw, gh, bubR)
	drawOR(page, col2X, yOR-ghH, gw, gh)

	// --- draw wires ---

	// A -> AND upper input
	drawWire(page, inX+sq, yA, col1X, yAND+ghH*0.4)
	// B -> AND lower input
	drawWire(page, inX+sq, yB, col1X, yAND-ghH*0.4)
	// C -> NOT
	drawWire(page, inX+sq, yC, col1X, yNOT)
	// AND -> OR upper input
	drawWire(page, col1X+gw, yAND, col2X, yOR+ghH*0.3)
	// NOT -> OR lower input
	drawWire(page, col1X+gw+bubR*2, yNOT, col2X, yOR-ghH*0.3)
	// OR -> O
	drawWire(page, col2X+gw, yOR, outX, yO)

	// --- draw input squares ---

	drawInputSquare(page, inX, yA-sqH, sq, groupA, refA)
	drawInputSquare(page, inX, yB-sqH, sq, groupB, refB)
	drawInputSquare(page, inX, yC-sqH, sq, groupC, refC)

	// --- draw output square ---

	drawOutputSquare(page, outX, yO-sqH, sq, output)

	// --- draw labels ---

	drawLabel(page, F, "A", inX+sqH, yA-sqH-12)
	drawLabel(page, F, "B", inX+sqH, yB-sqH-12)
	drawLabel(page, F, "C", inX+sqH, yC-sqH-12)
	drawLabel(page, F, "O", outX+sqH, yO-sqH-12)

	return page.Close()
}

// drawInputSquare draws a white background square with a black OCG-controlled
// overlay and a link annotation for toggling.
func drawInputSquare(page *document.Page, x, y, size float64, group *oc.Group, ref pdf.Native) {
	// white background (always visible)
	page.SetFillColor(color.DeviceGray(1))
	page.SetStrokeColor(color.DeviceGray(0))
	page.SetLineWidth(1)
	page.Rectangle(x, y, size, size)
	page.FillAndStroke()

	// black overlay (visible when OCG is ON)
	page.MarkedContentStart(&graphics.MarkedContent{
		Tag:        "OC",
		Properties: group,
	})
	page.SetFillColor(color.DeviceGray(0))
	page.Rectangle(x+1, y+1, size-2, size-2)
	page.Fill()
	page.MarkedContentEnd()

	// link annotation for toggling
	link := &annotation.Link{
		Common: annotation.Common{
			Rect: pdf.Rectangle{
				LLx: x,
				LLy: y,
				URx: x + size,
				URy: y + size,
			},
		},
		Action: &action.SetOCGState{
			State:      pdf.Array{pdf.Name("Toggle"), ref},
			PreserveRB: true,
		},
		Highlight: annotation.LinkHighlightNone,
	}
	page.Page.Annots = append(page.Page.Annots, link)

	// restore line width for gates
	page.SetLineWidth(1.5)
}

// drawOutputSquare draws a white background square with a black OCMD-controlled overlay.
func drawOutputSquare(page *document.Page, x, y, size float64, md *oc.Membership) {
	// white background
	page.SetFillColor(color.DeviceGray(1))
	page.SetStrokeColor(color.DeviceGray(0))
	page.SetLineWidth(1)
	page.Rectangle(x, y, size, size)
	page.FillAndStroke()

	// black overlay (visible when OCMD expression is true)
	page.MarkedContentStart(&graphics.MarkedContent{
		Tag:        "OC",
		Properties: md,
	})
	page.SetFillColor(color.DeviceGray(0))
	page.Rectangle(x+1, y+1, size-2, size-2)
	page.Fill()
	page.MarkedContentEnd()

	// restore line width
	page.SetLineWidth(1.5)
}

// drawAND draws an AND gate (D-shape): flat left + semicircular right.
func drawAND(page *document.Page, x, y, w, h float64) {
	midY := y + h/2
	halfH := h / 2

	// flat left side and top/bottom
	page.MoveTo(x, y)
	page.LineTo(x+w/2, y)
	// semicircle on the right from bottom to top
	page.LineToArc(x+w/2, midY, halfH, -math.Pi/2, math.Pi/2)
	page.LineTo(x, y+h)
	page.ClosePath()
	page.Stroke()
}

// drawOR draws an OR gate: concave left curve + two convex curves meeting at a point.
func drawOR(page *document.Page, x, y, w, h float64) {
	midY := y + h/2

	// left side: concave curve
	page.MoveTo(x, y)
	// control point for left curve pushed to the right
	cx := x + w*0.3
	page.CurveTo(cx, y+h*0.2, cx, y+h*0.8, x, y+h)

	// top curve from top-left to right tip
	page.CurveTo(x+w*0.4, y+h, x+w*0.8, y+h*0.8, x+w, midY)

	// bottom curve from right tip to bottom-left
	page.CurveTo(x+w*0.8, y+h*0.2, x+w*0.4, y, x, y)

	page.Stroke()
}

// drawNOT draws a NOT gate: triangle pointing right + bubble at output.
func drawNOT(page *document.Page, x, y, w, h float64, bubR float64) {
	midY := y + h/2
	tipX := x + w - bubR*2

	// triangle
	page.MoveTo(x, y)
	page.LineTo(tipX, midY)
	page.LineTo(x, y+h)
	page.ClosePath()
	page.Stroke()

	// bubble
	page.Circle(tipX+bubR, midY, bubR)
	page.Stroke()
}

// drawWire draws a straight line between two points.
func drawWire(page *document.Page, x1, y1, x2, y2 float64) {
	page.MoveTo(x1, y1)
	page.LineTo(x2, y2)
	page.Stroke()
}

// drawLabel draws a centred text label at the given position.
func drawLabel(page *document.Page, F *type1.Instance, text string, cx, y float64) {
	page.TextBegin()
	page.TextSetFont(F, 10)
	page.TextFirstLine(cx, y)
	page.TextShowAligned(text, 0, 0.5)
	page.TextEnd()
}
