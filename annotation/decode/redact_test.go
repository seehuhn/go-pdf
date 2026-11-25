// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package decode

import (
	"maps"
	"testing"

	"github.com/google/go-cmp/cmp"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

func readRedact(t *testing.T, w pdf.Getter, extra pdf.Dict) *annotation.Redact {
	t.Helper()
	dict := pdf.Dict{
		"Type":    pdf.Name("Annot"),
		"Subtype": pdf.Name("Redact"),
		"Rect":    pdf.Array{pdf.Integer(0), pdf.Integer(0), pdf.Integer(100), pdf.Integer(50)},
	}
	maps.Copy(dict, extra)
	a, err := pdf.CursorAt(pdf.NewExtractor(w), nil).Decode(dict, Annotation)
	if err != nil {
		t.Fatal(err)
	}
	r, ok := a.(*annotation.Redact)
	if !ok {
		t.Fatalf("unexpected type %T", a)
	}
	return r
}

// checkRedactRoundTrip writes r and reads it back; both reads must agree.
func checkRedactRoundTrip(t *testing.T, r *annotation.Redact) {
	t.Helper()
	w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
	rm := pdf.NewResourceManager(w)
	obj, err := r.Encode(rm)
	if err != nil {
		t.Fatal(err)
	}
	if err := rm.Close(); err != nil {
		t.Fatal(err)
	}
	dict, ok := obj.(pdf.Dict)
	if !ok {
		t.Fatalf("unexpected object %T", obj)
	}
	r2 := readRedact(t, w, dict)
	if diff := cmp.Diff(r, r2); diff != "" {
		t.Errorf("round trip failed (-want +got):\n%s", diff)
	}
}

// The malformed-input tests below check that the read succeeds and that a
// read-write-read cycle is stable.

func TestRedactEmptyIC(t *testing.T) {
	w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
	r := readRedact(t, w, pdf.Dict{"IC": pdf.Array{}})
	checkRedactRoundTrip(t, r)
}

func TestRedactOverlayTextWithoutDA(t *testing.T) {
	w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
	r := readRedact(t, w, pdf.Dict{"OverlayText": pdf.String("secret")})
	checkRedactRoundTrip(t, r)
}

func TestRedactIncompleteQuadPoints(t *testing.T) {
	w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
	var qp pdf.Array
	for i := range 10 {
		qp = append(qp, pdf.Integer(i))
	}
	r := readRedact(t, w, pdf.Dict{"QuadPoints": qp})
	checkRedactRoundTrip(t, r)
}

func TestRedactMalformedRO(t *testing.T) {
	for _, ro := range []pdf.Object{
		pdf.Integer(1),
		pdf.Dict{"Type": pdf.Name("XObject"), "Subtype": pdf.Name("Form")},
	} {
		w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
		r := readRedact(t, w, pdf.Dict{"RO": ro})
		checkRedactRoundTrip(t, r)
	}
}
