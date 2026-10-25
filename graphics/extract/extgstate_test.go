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

package extract

import (
	"errors"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/function"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/internal/debug/memfile"
	"seehuhn.de/go/pdf/internal/debug/mock"
)

// TestExtGStateBooleanAbsent checks that boolean entries which are dangling,
// null or of the wrong type read as absent rather than as false.
func TestExtGStateBooleanAbsent(t *testing.T) {
	c := pdf.CursorAt(pdf.NewExtractor(mock.Getter), nil)
	keys := []pdf.Name{"TK", "SA", "AIS", "OP", "op"}
	bits := graphics.StateTextKnockout | graphics.StateStrokeAdjustment |
		graphics.StateAlphaSourceFlag | graphics.StateOverprint

	for _, tc := range []struct {
		name string
		obj  pdf.Object
		want graphics.Bits
		val  bool
	}{
		{"dangling", pdf.NewReference(9999, 0), 0, false},
		{"null", nil, 0, false},
		{"wrong type", pdf.Integer(1), 0, false},
		{"false", pdf.Boolean(false), bits, false},
		{"true", pdf.Boolean(true), bits, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dict := pdf.Dict{"Type": pdf.Name("ExtGState")}
			for _, key := range keys {
				dict[key] = tc.obj
			}
			res, err := ExtGState(c, dict, true)
			if err != nil {
				t.Fatal(err)
			}
			if res.Set != tc.want {
				t.Errorf("wrong Set: got %v, want %v", res.Set, tc.want)
			}
			got := []bool{res.TextKnockout, res.StrokeAdjustment,
				res.AlphaSourceFlag, res.OverprintStroke, res.OverprintFill}
			for i, g := range got {
				if g != tc.val {
					t.Errorf("%s: got %v, want %v", keys[i], g, tc.val)
				}
			}
		})
	}
}

// TestExtGStateNameAbsent checks that name entries which are dangling, null
// or of the wrong type read as absent rather than as the empty name.
func TestExtGStateNameAbsent(t *testing.T) {
	c := pdf.CursorAt(pdf.NewExtractor(mock.Getter), nil)
	for _, obj := range []pdf.Object{pdf.NewReference(9999, 0), nil, pdf.Integer(1)} {
		dict := pdf.Dict{
			"Type":           pdf.Name("ExtGState"),
			"RI":             obj,
			"UseBlackPtComp": obj,
		}
		res, err := ExtGState(c, dict, true)
		if err != nil {
			t.Fatal(err)
		}
		if res.Set != 0 {
			t.Errorf("%v: wrong Set: got %v, want 0", obj, res.Set)
		}
	}
}

// failingGetter wraps a [pdf.Getter] and fails to read the object bad with
// an I/O error, which is not a sign of a malformed file.
type failingGetter struct {
	pdf.Getter
	bad pdf.Reference
}

func (g failingGetter) Get(ref pdf.Reference, canObjStm bool) (pdf.Native, error) {
	if ref == g.bad {
		return nil, errors.New("I/O error")
	}
	return g.Getter.Get(ref, canObjStm)
}

func unaryFunction(c1 float64) pdf.Dict {
	return pdf.Dict{
		"FunctionType": pdf.Integer(2),
		"Domain":       pdf.Array{pdf.Integer(0), pdf.Integer(1)},
		"C0":           pdf.Array{pdf.Integer(0)},
		"C1":           pdf.Array{pdf.Number(c1)},
		"N":            pdf.Integer(1),
	}
}

// malformedFunctions are objects which are not a function with one input
// and one output.
func malformedFunctions() map[string]pdf.Object {
	return map[string]pdf.Object{
		"dangling":   pdf.NewReference(9999, 0),
		"null":       nil,
		"wrong type": pdf.Integer(5),
		"bad dict":   pdf.Dict{"FunctionType": pdf.Integer(99)},
		"wrong shape": pdf.Dict{
			"FunctionType": pdf.Integer(2),
			"Domain":       pdf.Array{pdf.Integer(0), pdf.Integer(1)},
			"C0":           pdf.Array{pdf.Integer(0), pdf.Integer(0)},
			"C1":           pdf.Array{pdf.Integer(1), pdf.Integer(1)},
			"N":            pdf.Integer(1),
		},
	}
}

// TestExtGStateFunctionsMalformed checks that malformed BG, BG2, UCR, UCR2,
// TR and TR2 entries are dropped, and that a malformed second entry falls
// back to a valid first one.
func TestExtGStateFunctionsMalformed(t *testing.T) {
	w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	c := pdf.CursorAt(pdf.NewExtractor(w), nil)

	// a reference to the name Identity
	identityRef := w.Alloc()
	if err := w.Put(identityRef, pdf.Name("Identity")); err != nil {
		t.Fatal(err)
	}

	pairs := []struct {
		first, second pdf.Name
		bit           graphics.Bits
	}{
		{"BG", "BG2", graphics.StateBlackGeneration},
		{"UCR", "UCR2", graphics.StateUndercolorRemoval},
		{"TR", "TR2", graphics.StateTransferFunction},
	}
	for _, p := range pairs {
		for name, bad := range malformedFunctions() {
			t.Run(string(p.second)+" "+name, func(t *testing.T) {
				for _, withFirst := range []bool{false, true} {
					dict := pdf.Dict{"Type": pdf.Name("ExtGState")}
					dict[p.second] = bad
					if withFirst {
						dict[p.first] = unaryFunction(1)
					}
					res, err := ExtGState(c, dict, true)
					if err != nil {
						t.Fatal(err)
					}
					if got := res.Set&p.bit != 0; got != withFirst {
						t.Errorf("first entry %t: bit set = %t", withFirst, got)
					}
				}
			})
			t.Run(string(p.first)+" "+name, func(t *testing.T) {
				dict := pdf.Dict{"Type": pdf.Name("ExtGState")}
				dict[p.first] = bad
				res, err := ExtGState(c, dict, true)
				if err != nil {
					t.Fatal(err)
				}
				if res.Set&p.bit != 0 {
					t.Error("malformed entry not dropped")
				}
			})
		}
	}

	t.Run("TR arrays", func(t *testing.T) {
		good := unaryFunction(1)
		for name, arr := range map[string]pdf.Array{
			"too short":   {good, good, good},
			"bad element": {good, good, pdf.Integer(5), good},
			"wrong shape": {good, good, good, malformedFunctions()["wrong shape"]},
		} {
			dict := pdf.Dict{"Type": pdf.Name("ExtGState"), "TR": arr}
			res, err := ExtGState(c, dict, true)
			if err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			if res.Set&graphics.StateTransferFunction != 0 {
				t.Errorf("%s: malformed array not dropped", name)
			}
		}
	})

	t.Run("indirect name", func(t *testing.T) {
		dict := pdf.Dict{
			"Type": pdf.Name("ExtGState"),
			"TR2":  identityRef,
			"TR":   unaryFunction(1),
		}
		res, err := ExtGState(c, dict, true)
		if err != nil {
			t.Fatal(err)
		}
		if res.Set&graphics.StateTransferFunction == 0 {
			t.Fatal("transfer function not set")
		}
		if res.TransferFunctions.Gray != function.Identity {
			t.Errorf("got %v, want the identity function", res.TransferFunctions.Gray)
		}

		dict = pdf.Dict{"Type": pdf.Name("ExtGState"), "BG2": identityRef, "BG": unaryFunction(1)}
		res, err = ExtGState(c, dict, true)
		if err != nil {
			t.Fatal(err)
		}
		if res.BlackGeneration == nil {
			t.Error("BG not used after BG2 which is not /Default")
		}

		defaultRef := w.Alloc()
		if err := w.Put(defaultRef, pdf.Name("Default")); err != nil {
			t.Fatal(err)
		}
		dict = pdf.Dict{"Type": pdf.Name("ExtGState"), "UCR2": defaultRef, "UCR": unaryFunction(1)}
		res, err = ExtGState(c, dict, true)
		if err != nil {
			t.Fatal(err)
		}
		if res.Set&graphics.StateUndercolorRemoval == 0 || res.UndercolorRemoval != nil {
			t.Error("indirect /Default not handled")
		}
	})
}

// TestExtGStateFunctionsReadError checks that I/O errors are propagated.
func TestExtGStateFunctionsReadError(t *testing.T) {
	w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	bad := w.Alloc()
	if err := w.Put(bad, unaryFunction(1)); err != nil {
		t.Fatal(err)
	}
	c := pdf.CursorAt(pdf.NewExtractor(failingGetter{Getter: w, bad: bad}), nil)

	for _, key := range []pdf.Name{"BG", "BG2", "UCR", "UCR2", "TR", "TR2"} {
		dict := pdf.Dict{"Type": pdf.Name("ExtGState"), key: bad}
		if _, err := ExtGState(c, dict, true); !pdf.IsReadError(err) {
			t.Errorf("%s: got %v, want read error", key, err)
		}
	}
	dict := pdf.Dict{"Type": pdf.Name("ExtGState"), "TR": pdf.Array{bad, bad, bad, bad}}
	if _, err := ExtGState(c, dict, true); !pdf.IsReadError(err) {
		t.Errorf("TR array: got %v, want read error", err)
	}
}
