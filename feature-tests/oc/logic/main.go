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
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/standard"
	"seehuhn.de/go/pdf/font/type1"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/oc"
	pdfpage "seehuhn.de/go/pdf/page"
)

// This viewer test generates a PDF showing an interactive logic circuit.
//
// Three input squares (A, B, C) are clickable and toggle between dark (1)
// and light (0). The output square (O) is computed via an OCMD visibility
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

	F := font.Must(standard.Helvetica.New())

	// input OCGs
	groupA := &oc.Group{Name: "Input A"}
	groupB := &oc.Group{Name: "Input B"}
	groupC := &oc.Group{Name: "Input C"}

	// intermediate OCMDs
	andResult := &oc.Membership{
		VE: &oc.VisibilityExpressionAnd{Args: []oc.VisibilityExpression{
			&oc.VisibilityExpressionGroup{Group: groupA},
			&oc.VisibilityExpressionGroup{Group: groupB},
		}},
		SingleUse: true,
	}
	notResult := &oc.Membership{
		VE: &oc.VisibilityExpressionNot{
			Arg: &oc.VisibilityExpressionGroup{Group: groupC},
		},
		SingleUse: true,
	}

	// output OCMD: O = (A AND B) OR (NOT C)
	// The sub-expressions are repeated here because VE can only reference
	// OCGs, not OCMDs, so we cannot reuse andResult/notResult directly.
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
		isq  = 20.0 // intermediate square size
		isqH = isq / 2
		gw   = 50.0 // gate width
		gh   = 40.0 // gate height
		ghH  = gh / 2
		lw   = 1.5 // line width for gates and wires
		bubR = 5.0 // NOT bubble radius
	)

	// column x positions
	const (
		inX   = 80.0                   // input square left edge
		col1X = 200.0                  // first gate column left edge
		intX  = 270.0                  // intermediate square left edge
		col2X = 360.0                  // second gate column left edge
		outX  = 460.0                  // output square left edge
		mid1X = (inX + sq + col1X) / 2 // wire bend between inputs and first gates
	)

	// y positions
	const (
		yA   = 320.0         // input A centre y
		yB   = 260.0         // input B centre y
		yC   = 200.0         // input C centre y
		yOR  = 285.0         // OR gate centre y
		yAND = yOR + ghH*0.3 // AND gate centre y, aligned with OR upper input
		yNOT = yC            // NOT gate centre y
		yO   = yOR           // output centre y
	)

	// OR gate input x: where the left concave arc is at the input y positions
	s32 := gh * math.Sqrt(3) / 2
	dy := ghH * 0.3
	orInputX := pdf.Round(col2X+gw-2*s32+math.Sqrt(gh*gh-dy*dy), 1)
	mid2X := pdf.Round((intX+isq+orInputX)/2, 1)

	// --- draw input background box ---

	const (
		boxPad    = 12.0
		boxRadius = 8.0
	)
	boxX := inX - boxPad
	boxY := yC - sqH - 12 - 8 - boxPad
	boxW := sq + 2*boxPad
	boxH := (yA + sqH + boxPad) - boxY
	page.SetLineWidth(0.5)
	page.SetStrokeColor(color.DeviceGray(0.5))
	page.SetFillColor(color.DeviceGray(0.95))
	drawRoundedRect(page, boxX, boxY, boxW, boxH, boxRadius)
	page.FillAndStroke()

	// label underneath the box
	page.SetFillColor(color.DeviceGray(0.3))
	page.TextBegin()
	page.TextSetFont(F, 9)
	page.TextFirstLine(boxX+boxW/2, boxY-12)
	page.TextShowAligned("inputs", 0, 0.5)
	page.TextFirstLine(0, -11)
	page.TextShowAligned("(click to toggle)", 0, 0.5)
	page.TextEnd()

	page.SetLineWidth(lw)
	page.SetStrokeColor(color.DeviceGray(0))

	// --- draw static gates ---

	drawAND(page, col1X, yAND-ghH, gw, gh)
	drawNOT(page, col1X, yNOT-ghH, gw, gh, bubR)
	drawOR(page, col2X, yOR-ghH, gw, gh)

	// --- draw wires (Manhattan routing) ---

	// A -> AND upper input
	drawWireHVH(page, inX+sq, yA, col1X, yAND+ghH*0.4, mid1X)
	// B -> AND lower input
	drawWireHVH(page, inX+sq, yB, col1X, yAND-ghH*0.4, mid1X)
	// C -> NOT
	drawWireH(page, inX+sq, col1X, yC)
	// AND -> intermediate -> OR upper input
	drawWireH(page, col1X+gw, intX, yAND)
	drawWireH(page, intX+isq, orInputX, yAND)
	// NOT -> intermediate -> OR lower input
	drawWireH(page, col1X+gw, intX, yNOT)
	drawWireHVH(page, intX+isq, yNOT, orInputX, yOR-ghH*0.3, mid2X)
	// OR -> O
	drawWireH(page, col2X+gw, outX, yO)

	// --- draw input squares ---

	page.SetLineWidth(1)
	drawSquare(page, F, inX, yA-sqH, sq, groupA)
	addToggleLink(page, inX, yA-sqH, sq, refA)
	drawSquare(page, F, inX, yB-sqH, sq, groupB)
	addToggleLink(page, inX, yB-sqH, sq, refB)
	drawSquare(page, F, inX, yC-sqH, sq, groupC)
	addToggleLink(page, inX, yC-sqH, sq, refC)

	// --- draw intermediate squares ---

	drawSquare(page, F, intX, yAND-isqH, isq, andResult)
	drawSquare(page, F, intX, yNOT-isqH, isq, notResult)

	// --- draw output square ---

	drawSquare(page, F, outX, yO-sqH, sq, output)

	// --- draw labels ---

	drawLabel(page, F, "A", inX+sqH, yA-sqH-12)
	drawLabel(page, F, "B", inX+sqH, yB-sqH-12)
	drawLabel(page, F, "C", inX+sqH, yC-sqH-12)
	drawLabel(page, F, "O", outX+sqH, yO-sqH-12)

	return page.Close()
}

// drawSquare draws a "0"/"1" indicator square controlled by an optional content layer.
func drawSquare(page *document.Page, F *type1.Instance, x, y, size float64, cond oc.Conditional) {
	cx := x + size/2
	cy := y + size/2
	fontSize := size * 0.55

	// light background with "0" label (always visible)
	page.SetFillColor(color.DeviceGray(0.9))
	page.SetStrokeColor(color.DeviceGray(0))
	page.SetLineWidth(1)
	page.Rectangle(x, y, size, size)
	page.FillAndStroke()
	page.SetFillColor(color.DeviceGray(0))
	page.TextBegin()
	page.TextSetFont(F, fontSize)
	page.TextFirstLine(cx, cy-fontSize*0.3)
	page.TextShowAligned("0", 0, 0.5)
	page.TextEnd()

	// dark overlay with "1" label (visible when OC layer is ON)
	page.MarkedContentStart(&graphics.MarkedContent{
		Tag:        "OC",
		Properties: cond,
	})
	page.SetFillColor(color.DeviceGray(0.35))
	page.Rectangle(x, y, size, size)
	page.FillAndStroke()
	page.SetFillColor(color.DeviceGray(1))
	page.TextBegin()
	page.TextSetFont(F, fontSize)
	page.TextFirstLine(cx, cy-fontSize*0.3)
	page.TextShowAligned("1", 0, 0.5)
	page.TextEnd()
	page.MarkedContentEnd()
}

// addToggleLink adds a link annotation that toggles an OCG on click.
func addToggleLink(page *document.Page, x, y, size float64, ref pdf.Native) {
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
			State: pdf.Array{pdf.Name("Toggle"), ref},
		},
		Highlight: annotation.LinkHighlightNone,
	}
	page.Page.Annots = append(page.Page.Annots, pdfpage.AnnotInfo{Annot: link, Ref: page.Out.Alloc()})
}

// drawAND draws an AND gate (D-shape): flat left + semicircular right.
func drawAND(page *document.Page, x, y, w, h float64) {
	midY := y + h/2
	halfH := h / 2
	arcCx := x + w - halfH // arc centre, so right edge reaches x+w

	// flat left side and top/bottom
	page.MoveTo(x, y)
	page.LineTo(arcCx, y)
	// semicircle on the right from bottom to top
	page.LineToArc(arcCx, midY, halfH, -math.Pi/2, math.Pi/2)
	page.LineTo(x, y+h)
	page.ClosePath()
	page.Stroke()
}

// drawOR draws an OR gate using the IEEE/ANSI equilateral triangle construction.
// Three 60° circular arcs of radius h form the gate shape.
func drawOR(page *document.Page, x, y, w, h float64) {
	s32 := h * math.Sqrt(3) / 2 // h * sin(60°)
	baseX := x + w - s32

	// equilateral triangle vertices (side length = h)
	p1x, p1y := baseX, y   // bottom-left
	p2x, p2y := baseX, y+h // top-left
	// output tip at (x+w, y+h/2)

	// centre for left concave arc (reflection of output tip across the base)
	p4x, p4y := baseX-s32, y+h/2

	// bottom output arc: P1 → output, centred at P2
	page.MoveToArc(p2x, p2y, h, -math.Pi/2, -math.Pi/6)
	// top output arc: output → P2, centred at P1
	page.LineToArc(p1x, p1y, h, math.Pi/6, math.Pi/2)
	// left concave arc: P2 → P1, centred at P4 (clockwise)
	page.LineToArc(p4x, p4y, h, math.Pi/6, -math.Pi/6)
	page.ClosePath()
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

// drawWireH draws a horizontal wire.
func drawWireH(page *document.Page, x1, x2, y float64) {
	page.MoveTo(x1, y)
	page.LineTo(x2, y)
	page.Stroke()
}

// drawWireHVH draws a wire with horizontal-vertical-horizontal routing.
func drawWireHVH(page *document.Page, x1, y1, x2, y2, midX float64) {
	page.MoveTo(x1, y1)
	page.LineTo(midX, y1)
	page.LineTo(midX, y2)
	page.LineTo(x2, y2)
	page.Stroke()
}

// drawRoundedRect adds a rounded rectangle path to the current page.
// The arc centres are inset by r from each corner.
func drawRoundedRect(page *document.Page, x, y, w, h, r float64) {
	page.MoveTo(x+r, y)
	page.LineTo(x+w-r, y)                             // bottom edge
	page.LineToArc(x+w-r, y+r, r, -math.Pi/2, 0)      // bottom-right corner
	page.LineTo(x+w, y+h-r)                           // right edge
	page.LineToArc(x+w-r, y+h-r, r, 0, math.Pi/2)     // top-right corner
	page.LineTo(x+r, y+h)                             // top edge
	page.LineToArc(x+r, y+h-r, r, math.Pi/2, math.Pi) // top-left corner
	page.LineTo(x, y+r)                               // left edge
	page.LineToArc(x+r, y+r, r, math.Pi, 3*math.Pi/2) // bottom-left corner
	page.ClosePath()
}

// drawLabel draws a centred text label at the given position.
func drawLabel(page *document.Page, F *type1.Instance, text string, cx, y float64) {
	page.TextBegin()
	page.TextSetFont(F, 10)
	page.TextFirstLine(cx, y)
	page.TextShowAligned(text, 0, 0.5)
	page.TextEnd()
}
