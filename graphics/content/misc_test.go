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
	"seehuhn.de/go/pdf/graphics"
)

func TestMiscOperators_XObject(t *testing.T) {
	state := &GraphicsState{}
	mockXObj := &mockXObject{}
	res := &Resources{
		XObject: map[pdf.Name]graphics.XObject{
			"Im1": mockXObj,
		},
	}

	op := Operator{Name: "Do", Args: []pdf.Native{pdf.Name("Im1")}}
	if err := state.Apply(res, op); err != nil {
		t.Fatalf("Do operator failed: %v", err)
	}
}

func TestMiscOperators_MarkedContent(t *testing.T) {
	state := &GraphicsState{}
	res := &Resources{}

	// BMC
	opBMC := Operator{Name: "BMC", Args: []pdf.Native{pdf.Name("Tag1")}}
	if err := state.Apply(res, opBMC); err != nil {
		t.Fatalf("BMC operator failed: %v", err)
	}

	// EMC
	opEMC := Operator{Name: "EMC", Args: nil}
	if err := state.Apply(res, opEMC); err != nil {
		t.Fatalf("EMC operator failed: %v", err)
	}
}

func TestMiscOperators_SpecialOperators(t *testing.T) {
	state := &GraphicsState{}
	res := &Resources{}

	// %raw%
	opRaw := Operator{Name: "%raw%", Args: []pdf.Native{pdf.String("  % comment\n")}}
	if err := state.Apply(res, opRaw); err != nil {
		t.Fatalf("%%raw%% operator failed: %v", err)
	}

	// %image%
	opImage := Operator{
		Name: "%image%",
		Args: []pdf.Native{
			pdf.Dict{"W": pdf.Integer(10), "H": pdf.Integer(10)},
			pdf.String("imagedata"),
		},
	}
	if err := state.Apply(res, opImage); err != nil {
		t.Fatalf("%%image%% operator failed: %v", err)
	}
}

// mockXObject for testing
type mockXObject struct{}

func (m *mockXObject) Embed(*pdf.EmbedHelper) (pdf.Native, error) { return nil, nil }
func (m *mockXObject) Subtype() pdf.Name                          { return "Form" }
