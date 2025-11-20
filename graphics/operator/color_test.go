package operator

import (
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/resource"
)

func TestColorOperators_DeviceGray(t *testing.T) {
	state := &State{}
	res := &resource.Resource{}

	// Set stroke gray
	opG := Operator{Name: "G", Args: []pdf.Native{pdf.Real(0.5)}}
	if err := ApplyOperator(state, opG, res); err != nil {
		t.Fatalf("G operator failed: %v", err)
	}

	if state.Out&graphics.StateStrokeColor == 0 {
		t.Error("StateStrokeColor not marked in Out")
	}

	// Set fill gray
	opg := Operator{Name: "g", Args: []pdf.Native{pdf.Real(0.75)}}
	if err := ApplyOperator(state, opg, res); err != nil {
		t.Fatalf("g operator failed: %v", err)
	}

	if state.Out&graphics.StateFillColor == 0 {
		t.Error("StateFillColor not marked in Out")
	}
}

func TestColorOperators_DeviceRGB(t *testing.T) {
	state := &State{}
	res := &resource.Resource{}

	op := Operator{
		Name: "rg",
		Args: []pdf.Native{pdf.Real(1.0), pdf.Real(0.0), pdf.Real(0.0)},
	}

	if err := ApplyOperator(state, op, res); err != nil {
		t.Fatalf("rg operator failed: %v", err)
	}

	if state.Out&graphics.StateFillColor == 0 {
		t.Error("StateFillColor not marked in Out")
	}
}

func TestColorOperators_SetColorSpace(t *testing.T) {
	state := &State{}
	res := &resource.Resource{
		ColorSpace: map[pdf.Name]color.Space{
			"CS1": color.SpaceDeviceGray,
		},
	}

	// Set stroke color space
	opCS := Operator{Name: "CS", Args: []pdf.Native{pdf.Name("CS1")}}
	if err := ApplyOperator(state, opCS, res); err != nil {
		t.Fatalf("CS operator failed: %v", err)
	}

	if state.Out&graphics.StateStrokeColor == 0 {
		t.Error("StateStrokeColor not marked in Out")
	}
}
