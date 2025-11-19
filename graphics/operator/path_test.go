package operator

import (
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/resource"
)

func TestPathConstruction_MoveTo(t *testing.T) {
	state := &State{CurrentObject: graphics.ObjectType(1)} // objPage
	res := &resource.Resource{}

	op := Operator{
		Name: "m",
		Args: []pdf.Native{pdf.Real(10.0), pdf.Real(20.0)},
	}

	if err := ApplyOperator(state, op, res); err != nil {
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
	if state.CurrentObject != graphics.ObjectType(2) { // objPath
		t.Errorf("CurrentObject = %v, want objPath", state.CurrentObject)
	}
}

func TestPathConstruction_LineTo(t *testing.T) {
	state := &State{CurrentObject: graphics.ObjectType(2)} // objPath
	state.Param.CurrentX = 10.0
	state.Param.CurrentY = 20.0
	res := &resource.Resource{}

	op := Operator{
		Name: "l",
		Args: []pdf.Native{pdf.Real(30.0), pdf.Real(40.0)},
	}

	if err := ApplyOperator(state, op, res); err != nil {
		t.Fatalf("l operator failed: %v", err)
	}

	if state.Param.CurrentX != 30.0 || state.Param.CurrentY != 40.0 {
		t.Errorf("Current point = (%v, %v), want (30, 40)",
			state.Param.CurrentX, state.Param.CurrentY)
	}
}
