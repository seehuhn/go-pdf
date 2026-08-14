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

package action

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
	"seehuhn.de/go/pdf/oc"
)

// TestSetOCGStateEncodeInvalid checks that malformed state changes are
// rejected instead of reaching the file.
func TestSetOCGStateEncodeInvalid(t *testing.T) {
	group := &oc.Group{Name: "layer"}

	cases := []struct {
		name  string
		state []OCGStateChange
	}{
		{
			name:  "empty operation",
			state: []OCGStateChange{{Groups: []*oc.Group{group}}},
		},
		{
			name:  "no groups",
			state: []OCGStateChange{{Op: OCGOperationON}},
		},
		{
			name:  "nil group",
			state: []OCGStateChange{{Op: OCGOperationON, Groups: []*oc.Group{nil}}},
		},
		{
			name:  "operation outside the specification",
			state: []OCGStateChange{{Op: "Flip", Groups: []*oc.Group{group}}},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
			rm := pdf.NewResourceManager(w)

			_, err := (&SetOCGState{State: tc.state}).Encode(rm)
			if err == nil {
				t.Error("expected an error")
			}
		})
	}
}

// TestSetOCGStateEncodeEmpty checks that an action without state changes is
// written with an empty State array, so that it can be read back.  Decoding a
// file with a missing State entry yields such an action.
func TestSetOCGStateEncodeEmpty(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	rm := pdf.NewResourceManager(w)

	obj, err := (&SetOCGState{}).Encode(rm)
	if err != nil {
		t.Fatal(err)
	}
	dict, ok := obj.(pdf.Dict)
	if !ok {
		t.Fatalf("unexpected type %T", obj)
	}
	if state, ok := dict["State"].(pdf.Array); !ok || len(state) != 0 {
		t.Errorf("expected an empty State array, got %v", dict["State"])
	}
}

// TestSetOCGStateDecodeRepair checks that State arrays are read as far as
// they make sense: malformed entries are dropped, an operation name from
// outside the specification is dropped together with its groups, and what
// survives can be written again.
func TestSetOCGStateDecodeRepair(t *testing.T) {
	cases := []struct {
		name string
		// state builds the State array from the references of two groups
		state func(g1, g2 pdf.Object) pdf.Array
		want  func(g1, g2 *oc.Group) []OCGStateChange
	}{
		{
			name:  "missing State entry",
			state: nil,
			want:  func(g1, g2 *oc.Group) []OCGStateChange { return nil },
		},
		{
			name:  "empty array",
			state: func(g1, g2 pdf.Object) pdf.Array { return pdf.Array{} },
			want:  func(g1, g2 *oc.Group) []OCGStateChange { return nil },
		},
		{
			name: "groups before the first operation",
			state: func(g1, g2 pdf.Object) pdf.Array {
				return pdf.Array{g1, pdf.Name("ON"), g2}
			},
			want: func(g1, g2 *oc.Group) []OCGStateChange {
				return []OCGStateChange{{Op: OCGOperationON, Groups: []*oc.Group{g2}}}
			},
		},
		{
			name: "operation outside the specification is dropped",
			state: func(g1, g2 pdf.Object) pdf.Array {
				return pdf.Array{pdf.Name("Flip"), g1, pdf.Name("OFF"), g2}
			},
			want: func(g1, g2 *oc.Group) []OCGStateChange {
				return []OCGStateChange{{Op: OCGOperationOFF, Groups: []*oc.Group{g2}}}
			},
		},
		{
			// the unknown name still ends the preceding sequence, so its
			// groups do not fall to the previous operation
			name: "operation outside the specification delimits",
			state: func(g1, g2 pdf.Object) pdf.Array {
				return pdf.Array{pdf.Name("ON"), g1, pdf.Name("Flip"), g2}
			},
			want: func(g1, g2 *oc.Group) []OCGStateChange {
				return []OCGStateChange{{Op: OCGOperationON, Groups: []*oc.Group{g1}}}
			},
		},
		{
			name: "empty operation name drops its groups",
			state: func(g1, g2 pdf.Object) pdf.Array {
				return pdf.Array{pdf.Name(""), g1, pdf.Name("ON"), g2}
			},
			want: func(g1, g2 *oc.Group) []OCGStateChange {
				return []OCGStateChange{{Op: OCGOperationON, Groups: []*oc.Group{g2}}}
			},
		},
		{
			name: "operation without groups",
			state: func(g1, g2 pdf.Object) pdf.Array {
				return pdf.Array{pdf.Name("ON"), pdf.Name("OFF"), g1}
			},
			want: func(g1, g2 *oc.Group) []OCGStateChange {
				return []OCGStateChange{{Op: OCGOperationOFF, Groups: []*oc.Group{g1}}}
			},
		},
		{
			name: "entry which is not a group",
			state: func(g1, g2 pdf.Object) pdf.Array {
				return pdf.Array{pdf.Name("ON"), pdf.Integer(42), g1}
			},
			want: func(g1, g2 *oc.Group) []OCGStateChange {
				return []OCGStateChange{{Op: OCGOperationON, Groups: []*oc.Group{g1}}}
			},
		},
		{
			name: "no operation at all",
			state: func(g1, g2 pdf.Object) pdf.Array {
				return pdf.Array{pdf.Integer(7), g1, g2}
			},
			want: func(g1, g2 *oc.Group) []OCGStateChange { return nil },
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
			rm := pdf.NewResourceManager(w)

			g1 := &oc.Group{Name: "layer 1", Intent: []pdf.Name{"View"}}
			g2 := &oc.Group{Name: "layer 2", Intent: []pdf.Name{"View"}}
			ref1, err := rm.Embed(g1)
			if err != nil {
				t.Fatal(err)
			}
			ref2, err := rm.Embed(g2)
			if err != nil {
				t.Fatal(err)
			}

			dict := pdf.Dict{"S": pdf.Name("SetOCGState")}
			if tc.state != nil {
				dict["State"] = tc.state(ref1, ref2)
			}
			if err := rm.Close(); err != nil {
				t.Fatal(err)
			}
			if err := w.Close(); err != nil {
				t.Fatal(err)
			}

			c := pdf.NewCursor(w)
			decoded, err := decodeSetOCGState(c, dict)
			if err != nil {
				t.Fatal(err)
			}

			want := tc.want(g1, g2)
			if diff := cmp.Diff(want, decoded.State); diff != "" {
				t.Errorf("decode failed (-want +got):\n%s", diff)
			}

			// what survives must be writable again
			w2, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
			rm2 := pdf.NewResourceManager(w2)
			if _, err := decoded.Encode(rm2); err != nil {
				t.Errorf("re-encode failed: %v", err)
			}
		})
	}
}

// TestSetOCGStateDecodeIndirectOperation checks that an operation name stored
// as an indirect object is read.  Any PDF object may be indirect, so a State
// array may spell its names that way.
func TestSetOCGStateDecodeIndirectOperation(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	rm := pdf.NewResourceManager(w)

	group := &oc.Group{Name: "layer", Intent: []pdf.Name{"View"}}
	groupRef, err := rm.Embed(group)
	if err != nil {
		t.Fatal(err)
	}
	opRef := w.Alloc()
	if err := w.Put(opRef, pdf.Name("Toggle")); err != nil {
		t.Fatal(err)
	}
	if err := rm.Close(); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	dict := pdf.Dict{
		"S":     pdf.Name("SetOCGState"),
		"State": pdf.Array{opRef, groupRef},
	}
	decoded, err := decodeSetOCGState(pdf.NewCursor(w), dict)
	if err != nil {
		t.Fatal(err)
	}

	want := []OCGStateChange{{Op: OCGOperationToggle, Groups: []*oc.Group{group}}}
	if diff := cmp.Diff(want, decoded.State); diff != "" {
		t.Errorf("decode failed (-want +got):\n%s", diff)
	}
}

// TestSetOCGStateSharedGroup checks that a group used both by an annotation
// and by a SetOCGState action stays a single object in the file.  The typed
// State field relies on this: pdf.Decode caches by reference, so both uses
// yield the same *oc.Group, which rm.Embed then writes only once.
func TestSetOCGStateSharedGroup(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	rm := pdf.NewResourceManager(w)

	group := &oc.Group{Name: "layer", Intent: []pdf.Name{"View"}}
	groupRef, err := rm.Embed(group)
	if err != nil {
		t.Fatal(err)
	}

	a := &SetOCGState{
		State: []OCGStateChange{{Op: OCGOperationToggle, Groups: []*oc.Group{group}}},
	}
	encoded, err := a.Encode(rm)
	if err != nil {
		t.Fatal(err)
	}
	if err := rm.Close(); err != nil {
		t.Fatal(err)
	}

	// the action must point at the group which was embedded first
	state := encoded.(pdf.Dict)["State"].(pdf.Array)
	if len(state) != 2 || state[1] != groupRef {
		t.Fatalf("expected the shared group %v, got %v", groupRef, state)
	}

	// reading twice gives the same Go value, so writing it out again
	// produces a single object rather than a copy per use
	c := pdf.NewCursor(w)
	decoded, err := pdf.Decode(c, encoded, Decode)
	if err != nil {
		t.Fatal(err)
	}
	again, err := pdf.Decode(c, groupRef, oc.ExtractGroup)
	if err != nil {
		t.Fatal(err)
	}
	fromAction := decoded.(*SetOCGState).State[0].Groups[0]
	if fromAction != again {
		t.Error("the same reference decoded to two different groups")
	}
}
