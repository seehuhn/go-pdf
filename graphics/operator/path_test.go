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

package operator

import (
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/resource"
)

func TestPathConstruction_MoveTo(t *testing.T) {
	state := &State{CurrentObject: ObjectType(1)} // objPage
	res := &resource.Resource{}

	op := Operator{
		Name: "m",
		Args: []pdf.Native{pdf.Real(10.0), pdf.Real(20.0)},
	}

	if err := state.Apply(res, op); err != nil {
		t.Fatalf("m operator failed: %v", err)
	}

	if state.Param.CurrentX != 10.0 || state.Param.CurrentY != 20.0 {
		t.Errorf("Current point = (%v, %v), want (10, 20)",
			state.Param.CurrentX, state.Param.CurrentY)
	}
	if state.Param.StartX != 10.0 || state.Param.StartY != 20.0 {
		t.Errorf("Start point = (%v, %v), want (10, 20)",
			state.Param.StartX, state.Param.StartY)
	}
	if state.CurrentObject != ObjectType(2) { // objPath
		t.Errorf("CurrentObject = %v, want objPath", state.CurrentObject)
	}
}

func TestPathConstruction_LineTo(t *testing.T) {
	state := &State{CurrentObject: ObjectType(2)} // objPath
	state.Param.CurrentX = 10.0
	state.Param.CurrentY = 20.0
	res := &resource.Resource{}

	op := Operator{
		Name: "l",
		Args: []pdf.Native{pdf.Real(30.0), pdf.Real(40.0)},
	}

	if err := state.Apply(res, op); err != nil {
		t.Fatalf("l operator failed: %v", err)
	}

	if state.Param.CurrentX != 30.0 || state.Param.CurrentY != 40.0 {
		t.Errorf("Current point = (%v, %v), want (30, 40)",
			state.Param.CurrentX, state.Param.CurrentY)
	}
}

func TestPathPainting_Stroke(t *testing.T) {
	state := &State{CurrentObject: ObjPath}
	state.Param.AllSubpathsClosed = true
	state.Param.ThisSubpathClosed = true // last subpath is closed
	state.Param.DashPattern = nil
	res := &resource.Resource{}

	op := Operator{Name: "S", Args: nil}
	if err := state.Apply(res, op); err != nil {
		t.Fatalf("S operator failed: %v", err)
	}

	// Should mark dependencies
	expected := graphics.StateLineWidth | graphics.StateLineJoin |
		graphics.StateLineDash | graphics.StateStrokeColor
	if state.In&expected != expected {
		t.Errorf("In = %v, want at least %v", state.In, expected)
	}

	// LineCap should NOT be marked for closed path without dashes
	if state.In&graphics.StateLineCap != 0 {
		t.Error("LineCap marked but path is closed and not dashed")
	}

	// Should reset to page context
	if state.CurrentObject != ObjPage {
		t.Errorf("CurrentObject = %v, want objPage", state.CurrentObject)
	}
}

func TestPathPainting_StrokeOpenPath(t *testing.T) {
	state := &State{CurrentObject: ObjPath}
	state.Param.AllSubpathsClosed = false
	state.Param.DashPattern = nil
	res := &resource.Resource{}

	op := Operator{Name: "S", Args: nil}
	if err := state.Apply(res, op); err != nil {
		t.Fatalf("S operator failed: %v", err)
	}

	// LineCap SHOULD be marked for open path
	if state.In&graphics.StateLineCap == 0 {
		t.Error("LineCap not marked for open path")
	}
}

func TestPathPainting_Fill(t *testing.T) {
	state := &State{CurrentObject: ObjPath}
	res := &resource.Resource{}

	op := Operator{Name: "f", Args: nil}
	if err := state.Apply(res, op); err != nil {
		t.Fatalf("f operator failed: %v", err)
	}

	if state.In&graphics.StateFillColor == 0 {
		t.Error("FillColor not marked in In")
	}
	if state.CurrentObject != ObjPage {
		t.Errorf("CurrentObject = %v, want objPage", state.CurrentObject)
	}
}
