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

package graphics

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
	if err := state.Apply(res, opG); err != nil {
		t.Fatalf("G operator failed: %v", err)
	}

	if state.Out&graphics.StateStrokeColor == 0 {
		t.Error("StateStrokeColor not marked in Out")
	}

	// Set fill gray
	opg := Operator{Name: "g", Args: []pdf.Native{pdf.Real(0.75)}}
	if err := state.Apply(res, opg); err != nil {
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

	if err := state.Apply(res, op); err != nil {
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
	if err := state.Apply(res, opCS); err != nil {
		t.Fatalf("CS operator failed: %v", err)
	}

	if state.Out&graphics.StateStrokeColor == 0 {
		t.Error("StateStrokeColor not marked in Out")
	}
}
