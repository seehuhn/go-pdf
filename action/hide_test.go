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
)

// TestHideSelfReference checks that a Hide action whose T entry names the
// object being read, as for a button hiding itself, keeps its T entry.
func TestHideSelfReference(t *testing.T) {
	w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	ref := w.Alloc()
	if err := w.Put(ref, pdf.Dict{"S": pdf.Name("Hide"), "T": ref}); err != nil {
		t.Fatal(err)
	}

	a, err := pdf.CursorAt(pdf.NewExtractor(w), nil).Decode(ref, Decode)
	if err != nil {
		t.Fatal(err)
	}
	if h, ok := a.(*Hide); !ok || h.T != ref {
		t.Errorf("got %#v, want Hide action with T = %v", a, ref)
	}
}

// TestHideTarget checks that the T entry of a Hide action is read as a
// string, a dictionary or an array of these, that invalid array elements are
// dropped, and that the result survives a read-write-read cycle.
func TestHideTarget(t *testing.T) {
	w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	annot := pdf.Dict{"Type": pdf.Name("Annot"), "Subtype": pdf.Name("Widget")}
	annotRef := w.Alloc()
	if err := w.Put(annotRef, annot); err != nil {
		t.Fatal(err)
	}
	textRef := w.Alloc()
	if err := w.Put(textRef, pdf.String("field")); err != nil {
		t.Fatal(err)
	}
	dangling := pdf.NewReference(99, 0)
	c := pdf.NewCursor(w)

	valid := []struct {
		name string
		t    pdf.Object
		want pdf.Object
	}{
		{"string", pdf.String("field"), pdf.String("field")},
		{"string reference", textRef, pdf.String("field")},
		{"dictionary", annot, annot},
		{"dictionary reference", annotRef, annotRef},
		{
			"array",
			pdf.Array{pdf.String("a"), annotRef, textRef, annot},
			pdf.Array{pdf.String("a"), annotRef, pdf.String("field"), annot},
		},
		{
			"array with invalid elements",
			pdf.Array{
				pdf.Name("x"), pdf.String("a"), nil, pdf.Integer(1), dangling,
				pdf.Array{pdf.String("nested")}, annotRef,
			},
			pdf.Array{pdf.String("a"), annotRef},
		},
	}
	for _, tc := range valid {
		t.Run(tc.name, func(t *testing.T) {
			a, err := Decode(c, pdf.Dict{"S": pdf.Name("Hide"), "T": tc.t}, true)
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(tc.want, a.(*Hide).T); diff != "" {
				t.Errorf("wrong T (-want +got):\n%s", diff)
			}

			// references stay valid, so write back to the same file
			encoded, err := a.Encode(pdf.NewResourceManager(w))
			if err != nil {
				t.Fatal(err)
			}
			b, err := Decode(c, encoded, true)
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(a, b); diff != "" {
				t.Errorf("round trip failed (-first +second):\n%s", diff)
			}
		})
	}

	invalid := map[string]pdf.Object{
		"name":                  pdf.Name("x"),
		"integer":               pdf.Integer(1),
		"dangling":              dangling,
		"null":                  nil,
		"empty array":           pdf.Array{},
		"array without targets": pdf.Array{pdf.Name("x"), pdf.Integer(1), dangling, nil},
		"nested array":          pdf.Array{pdf.Array{pdf.String("a")}},
	}
	for name, value := range invalid {
		t.Run("invalid "+name, func(t *testing.T) {
			dict := pdf.Dict{"S": pdf.Name("Hide"), "T": value}
			if _, err := Decode(c, dict, true); !pdf.IsMalformed(err) {
				t.Errorf("got %v, want malformed error", err)
			}
		})
	}
}

// TestHideTargetEncodeInvalid checks that a T entry of the wrong type is
// rejected on write.
func TestHideTargetEncodeInvalid(t *testing.T) {
	w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	rm := pdf.NewResourceManager(w)
	for name, value := range map[string]pdf.Object{
		"missing":      nil,
		"name":         pdf.Name("x"),
		"integer":      pdf.Integer(1),
		"empty array":  pdf.Array{},
		"bad element":  pdf.Array{pdf.String("a"), pdf.Name("x")},
		"nested array": pdf.Array{pdf.Array{pdf.String("a")}},
		"null element": pdf.Array{nil},
	} {
		if _, err := (&Hide{T: value, H: true}).Encode(rm); err == nil {
			t.Errorf("%s: encode succeeded", name)
		}
	}
}
