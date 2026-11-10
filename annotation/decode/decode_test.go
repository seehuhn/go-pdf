// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package decode

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/acroform"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

// TestFitExportValues checks that a button field's /Opt array is fitted to
// its widgets on read: surplus entries are dropped, and a missing entry
// takes the widget's on-state name.
func TestFitExportValues(t *testing.T) {
	tests := []struct {
		name string
		opt  pdf.Array
		want []string
	}{
		{"exact", pdf.Array{pdf.String("x"), pdf.String("y")}, []string{"x", "y"}},
		{"too long", pdf.Array{pdf.String("x"), pdf.String("y"), pdf.String("z")}, []string{"x", "y"}},
		{"too short", pdf.Array{pdf.String("x")}, []string{"x", "b"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
			x := pdf.NewExtractor(w)

			fieldRef := w.Alloc()
			var kids pdf.Array
			for i, on := range []pdf.Name{"a", "b"} {
				stm := writeFormStream(t, w, nil)
				ref := w.Alloc()
				kid := pdf.Dict{
					"Subtype": pdf.Name("Widget"),
					"Rect":    &pdf.Rectangle{LLx: float64(30 * i), URx: float64(30*i + 20), URy: 20},
					"AP":      pdf.Dict{"N": pdf.Dict{on: stm, "Off": stm}},
					"AS":      pdf.Name("Off"),
					"Parent":  fieldRef,
				}
				if err := w.Put(ref, kid); err != nil {
					t.Fatal(err)
				}
				kids = append(kids, ref)
			}
			field := pdf.Dict{
				"FT":   pdf.Name("Btn"),
				"Ff":   pdf.Integer(acroform.FieldRadio),
				"T":    pdf.String("r"),
				"Opt":  tc.opt,
				"Kids": kids,
			}
			if err := w.Put(fieldRef, field); err != nil {
				t.Fatal(err)
			}

			node, err := decodeRootField(x, fieldRef)
			if err != nil {
				t.Fatalf("decode failed: %v", err)
			}
			f, ok := node.(*acroform.ButtonField)
			if !ok {
				t.Fatalf("expected a *acroform.ButtonField, got %T", node)
			}
			if diff := cmp.Diff(tc.want, f.ExportValues); diff != "" {
				t.Errorf("export values mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// TestAbsentButtonValue checks that a check box or radio button without /V
// and /DV reads as "Off".
func TestAbsentButtonValue(t *testing.T) {
	tests := []struct {
		name string
		ff   acroform.FieldFlags
		want pdf.Name
	}{
		{"check box", 0, "Off"},
		{"radio button", acroform.FieldRadio, "Off"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
			x := pdf.NewExtractor(w)

			ref := w.Alloc()
			field := pdf.Dict{
				"FT": pdf.Name("Btn"),
				"Ff": pdf.Integer(tc.ff),
				"T":  pdf.String("b"),
			}
			if err := w.Put(ref, field); err != nil {
				t.Fatal(err)
			}

			node, err := decodeRootField(x, ref)
			if err != nil {
				t.Fatalf("decode failed: %v", err)
			}
			f, ok := node.(*acroform.ButtonField)
			if !ok {
				t.Fatalf("expected a *acroform.ButtonField, got %T", node)
			}
			if f.State != tc.want || f.DefaultState != tc.want {
				t.Errorf("State, DefaultState = %q, %q, want %q", f.State, f.DefaultState, tc.want)
			}
		})
	}
}

// TestButtonStateRepair checks that a /V or /DV which names no on-state of
// the field is repaired on read, so that the field can be written again.
func TestButtonStateRepair(t *testing.T) {
	w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	x := pdf.NewExtractor(w)

	stm := writeFormStream(t, w, nil)
	fieldRef := w.Alloc()
	var kids pdf.Array
	for i := range 2 {
		ref := w.Alloc()
		kid := pdf.Dict{
			"Subtype": pdf.Name("Widget"),
			"Rect":    &pdf.Rectangle{LLx: float64(30 * i), URx: float64(30*i + 20), URy: 20},
			"AP":      pdf.Dict{"N": pdf.Dict{pdf.Name([]string{"a", "b"}[i]): stm, "Off": stm}},
			"AS":      pdf.Name("Off"),
			"Parent":  fieldRef,
		}
		if err := w.Put(ref, kid); err != nil {
			t.Fatal(err)
		}
		kids = append(kids, ref)
	}
	field := pdf.Dict{
		"FT":   pdf.Name("Btn"),
		"Ff":   pdf.Integer(acroform.FieldRadio),
		"T":    pdf.String("r"),
		"DV":   pdf.Name("c"),
		"Kids": kids,
	}
	if err := w.Put(fieldRef, field); err != nil {
		t.Fatal(err)
	}

	node, err := decodeRootField(x, fieldRef)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	f := node.(*acroform.ButtonField)
	if f.DefaultState != "Off" {
		t.Errorf("DefaultState = %q, want Off", f.DefaultState)
	}
	if f.State != "Off" {
		t.Errorf("State = %q, want Off", f.State)
	}
}

// TestButtonStateRepairUnnamed checks a radio button whose widgets have no
// appearance states: a /V naming no on-state is dropped.
func TestButtonStateRepairUnnamed(t *testing.T) {
	w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	x := pdf.NewExtractor(w)

	stm := writeFormStream(t, w, nil)
	fieldRef := w.Alloc()
	kidRef := w.Alloc()
	kid := pdf.Dict{
		"Subtype": pdf.Name("Widget"),
		"Rect":    &pdf.Rectangle{URx: 20, URy: 20},
		"AP":      pdf.Dict{"N": stm},
		"Parent":  fieldRef,
	}
	if err := w.Put(kidRef, kid); err != nil {
		t.Fatal(err)
	}
	field := pdf.Dict{
		"FT":   pdf.Name("Btn"),
		"Ff":   pdf.Integer(acroform.FieldRadio),
		"T":    pdf.String("r"),
		"V":    pdf.Name("Foo"),
		"Kids": pdf.Array{kidRef},
	}
	if err := w.Put(fieldRef, field); err != nil {
		t.Fatal(err)
	}

	node, err := decodeRootField(x, fieldRef)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if f := node.(*acroform.ButtonField); f.State != "Off" {
		t.Errorf("State = %q, want Off", f.State)
	}
}
