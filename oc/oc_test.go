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

package oc

import (
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

// TestExtractConditionalInferType checks that an optional content dictionary
// with no /Type entry is classified by its contents.  /Type is required, but
// files omit it, and dropping such an /OC entry would leave the annotation or
// XObject visible under every configuration.
func TestExtractConditionalInferType(t *testing.T) {
	type testCase struct {
		name    string
		dict    pdf.Dict
		wantOCG bool
	}
	cases := []testCase{
		{
			name:    "named group",
			dict:    pdf.Dict{"Name": pdf.TextString("Layer")},
			wantOCG: true,
		},
		{
			name: "group with usage and intent",
			dict: pdf.Dict{
				"Name":   pdf.TextString("Layer"),
				"Intent": pdf.Name("Design"),
			},
			wantOCG: true,
		},
		{
			name: "membership via OCGs",
			dict: pdf.Dict{"OCGs": pdf.Array{}},
		},
		{
			name: "membership via P",
			dict: pdf.Dict{"P": pdf.Name("AllOn")},
		},
		{
			name: "membership via VE",
			dict: pdf.Dict{"VE": pdf.Array{pdf.Name("Not")}},
		},
		{
			// nothing points either way, so the commoner case wins
			name:    "empty dictionary",
			dict:    pdf.Dict{},
			wantOCG: true,
		},
		{
			// a /Type that is not a name carries no type information, so it is
			// treated like a missing one rather than dropping the whole entry
			name:    "Type is a string",
			dict:    pdf.Dict{"Type": pdf.String("OCG"), "Name": pdf.TextString("Layer")},
			wantOCG: true,
		},
		{
			name: "Type is a string, membership contents",
			dict: pdf.Dict{"Type": pdf.String("OCMD"), "VE": pdf.Array{pdf.Name("Not")}},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
			ref := w.Alloc()
			if err := w.Put(ref, tc.dict); err != nil {
				t.Fatal(err)
			}

			cond, err := pdf.Decode(pdf.NewCursor(w), ref, ExtractConditional)
			if err != nil {
				t.Fatalf("extract: %v", err)
			}
			switch cond.(type) {
			case *Group:
				if !tc.wantOCG {
					t.Error("got *Group, want *Membership")
				}
			case *Membership:
				if tc.wantOCG {
					t.Error("got *Membership, want *Group")
				}
			default:
				t.Errorf("unexpected type %T", cond)
			}
		})
	}
}

// TestExtractConditionalInferTypeIdentity checks that the inferred-type path
// keeps the pointer identity guarantee: a group whose /Type is missing still
// resolves to the value a [GroupStates] was built from.
func TestExtractConditionalInferTypeIdentity(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	ref := w.Alloc()
	err := w.Put(ref, pdf.Dict{"Name": pdf.TextString("Layer")})
	if err != nil {
		t.Fatal(err)
	}

	x := pdf.NewExtractor(w)
	group, err := pdf.Decode(pdf.CursorAt(x, nil), ref, ExtractGroup)
	if err != nil {
		t.Fatal(err)
	}
	cond, err := pdf.Decode(pdf.CursorAt(x, nil), ref, ExtractConditional)
	if err != nil {
		t.Fatal(err)
	}
	if cond != Conditional(group) {
		t.Fatal("conditional group pointer differs from directly extracted group")
	}

	state := (&Configuration{}).DefaultState([]*Group{group}, EventView, nil)
	state.SetState(group, false)
	if cond.IsVisible(state) {
		t.Error("conditional is visible although its group is off")
	}
}

// TestExtractConditionalBadType checks that a /Type entry naming something
// other than an optional content object is still rejected.  The inference
// applies to a missing entry, not to a contradictory one.
func TestExtractConditionalBadType(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	ref := w.Alloc()
	err := w.Put(ref, pdf.Dict{
		"Type": pdf.Name("Annot"),
		"Name": pdf.TextString("Layer"),
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = pdf.Decode(pdf.NewCursor(w), ref, ExtractConditional)
	if err == nil {
		t.Error("no error for a dictionary of the wrong type")
	}
}

// TestExtractConditionalInferTypeRoundTrip checks that a membership dictionary
// read without its /Type entry is written back with one, so the read-write-read
// cycle is stable.
func TestExtractConditionalInferTypeRoundTrip(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)

	ocgRef := w.Alloc()
	err := w.Put(ocgRef, pdf.Dict{
		"Type": pdf.Name("OCG"),
		"Name": pdf.TextString("Layer"),
	})
	if err != nil {
		t.Fatal(err)
	}
	mdRef := w.Alloc()
	err = w.Put(mdRef, pdf.Dict{
		"OCGs": pdf.Array{ocgRef},
		"P":    pdf.Name("AllOff"),
	})
	if err != nil {
		t.Fatal(err)
	}

	cond, err := pdf.Decode(pdf.NewCursor(w), mdRef, ExtractConditional)
	if err != nil {
		t.Fatal(err)
	}

	rm := pdf.NewResourceManager(w)
	embedded, err := rm.Embed(cond.(*Membership))
	if err != nil {
		t.Fatal(err)
	}
	if err := rm.Close(); err != nil {
		t.Fatal(err)
	}
	dict, err := pdf.NewCursor(w).Dict(embedded)
	if err != nil {
		t.Fatal(err)
	}
	if dict["Type"] != pdf.Name("OCMD") {
		t.Errorf("Type = %v, want OCMD", dict["Type"])
	}

	back, err := pdf.Decode(pdf.NewCursor(w), embedded, ExtractConditional)
	if err != nil {
		t.Fatal(err)
	}
	md, ok := back.(*Membership)
	if !ok {
		t.Fatalf("got %T, want *Membership", back)
	}
	if md.Policy != PolicyAllOff || len(md.OCGs) != 1 {
		t.Errorf("round trip changed the membership: %+v", md)
	}
}
