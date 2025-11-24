// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2025  Jochen Voss <voss@seehuhn.de>
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

package content

import (
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/graphics"
)

func TestIntegration_PathWithStroke(t *testing.T) {
	state := &GraphicsState{CurrentObject: ObjPage}
	res := &Resources{}

	ops := []Operator{
		{Name: "w", Args: []pdf.Native{pdf.Real(2.0)}},
		{Name: "m", Args: []pdf.Native{pdf.Real(10.0), pdf.Real(10.0)}},
		{Name: "l", Args: []pdf.Native{pdf.Real(100.0), pdf.Real(10.0)}},
		{Name: "l", Args: []pdf.Native{pdf.Real(100.0), pdf.Real(100.0)}},
		{Name: "h", Args: nil},
		{Name: "S", Args: nil},
	}

	for i, op := range ops {
		if err := state.Apply(res, op); err != nil {
			t.Fatalf("operator %d (%s) failed: %v", i, op.Name, err)
		}
	}

	// Verify dependencies: LineJoin, LineDash, and StrokeColor should be in In.
	// LineWidth was set before the path, so it's in Out and not added to In.
	expected := graphics.StateLineJoin | graphics.StateLineDash | graphics.StateStrokeColor
	if state.In&expected != expected {
		t.Errorf("missing expected dependencies, In = %v", state.In)
	}

	// LineCap should not be needed (closed path, no dashes)
	if state.In&graphics.StateLineCap != 0 {
		t.Error("LineCap marked but not needed")
	}

	// Verify outputs: LineWidth was explicitly set
	if state.Out&graphics.StateLineWidth == 0 {
		t.Error("LineWidth not in Out")
	}
}

func TestIntegration_TextRenderingDependencies(t *testing.T) {
	state := &GraphicsState{CurrentObject: ObjPage}
	mockFont := &mockFontInstance{}
	res := &Resources{
		Font: map[pdf.Name]font.Instance{
			"F1": mockFont,
		},
	}

	ops := []Operator{
		{Name: "BT", Args: nil},
		{Name: "Tf", Args: []pdf.Native{pdf.Name("F1"), pdf.Real(12.0)}},
		{Name: "Tr", Args: []pdf.Native{pdf.Integer(1)}}, // Stroke mode
		{Name: "Tj", Args: []pdf.Native{pdf.String("Hello")}},
		{Name: "ET", Args: nil},
	}

	for i, op := range ops {
		if err := state.Apply(res, op); err != nil {
			t.Fatalf("operator %d (%s) failed: %v", i, op.Name, err)
		}
	}

	// Text stroke mode should require stroke color and line parameters
	if state.In&graphics.StateStrokeColor == 0 {
		t.Error("StrokeColor not marked for stroke rendering mode")
	}
	if state.In&graphics.StateLineWidth == 0 {
		t.Error("LineWidth not marked for stroke rendering mode")
	}
}

func TestIntegration_GraphicsStateStack(t *testing.T) {
	state := &GraphicsState{CurrentObject: ObjPage}
	res := &Resources{}

	// Set line width
	op1 := Operator{Name: "w", Args: []pdf.Native{pdf.Real(2.0)}}
	if err := state.Apply(res, op1); err != nil {
		t.Fatalf("w failed: %v", err)
	}

	// Push state
	opQ := Operator{Name: "q", Args: nil}
	if err := state.Apply(res, opQ); err != nil {
		t.Fatalf("q failed: %v", err)
	}

	savedOut := state.Out

	// Modify state
	op2 := Operator{Name: "w", Args: []pdf.Native{pdf.Real(5.0)}}
	if err := state.Apply(res, op2); err != nil {
		t.Fatalf("second w failed: %v", err)
	}

	// Pop state
	opPop := Operator{Name: "Q", Args: nil}
	if err := state.Apply(res, opPop); err != nil {
		t.Fatalf("Q failed: %v", err)
	}

	// Verify Out was restored
	if state.Out != savedOut {
		t.Errorf("Out not restored: got %v, want %v", state.Out, savedOut)
	}

	// Verify LineWidth was restored
	if state.Param.LineWidth != 2.0 {
		t.Errorf("LineWidth = %v, want 2.0", state.Param.LineWidth)
	}
}
