// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package halftone

import (
	"bytes"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

// TestType1SpotFunctionProvenance regresses a bug where a Type 1
// halftone's indirect /SpotFunction lost its provenance on decode, so
// re-embedding the halftone wrote a second copy of the function instead
// of reusing the original object.
func TestType1SpotFunctionProvenance(t *testing.T) {
	w0, f := memfile.NewPDFWriter(t, pdf.V1_7, nil)

	funcRef := w0.Alloc()
	funcBody, err := w0.OpenStream(funcRef, pdf.Dict{
		"FunctionType": pdf.Integer(4),
		"Domain":       pdf.Array{pdf.Number(-1), pdf.Number(1), pdf.Number(-1), pdf.Number(1)},
		"Range":        pdf.Array{pdf.Number(-1), pdf.Number(1)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := funcBody.Write([]byte("{ pop pop 0 }")); err != nil {
		t.Fatal(err)
	}
	if err := funcBody.Close(); err != nil {
		t.Fatal(err)
	}

	htRef := w0.Alloc()
	if err := w0.Put(htRef, pdf.Dict{
		"HalftoneType": pdf.Integer(1),
		"Frequency":    pdf.Number(60),
		"Angle":        pdf.Number(45),
		"SpotFunction": funcRef,
	}); err != nil {
		t.Fatal(err)
	}
	if err := w0.Close(); err != nil {
		t.Fatal(err)
	}

	orig := bytes.Clone(f.Data)
	w, err := pdf.NewUpdater(f, int64(len(orig)), nil)
	if err != nil {
		t.Fatal(err)
	}
	c := pdf.NewCursor(w)
	ht, err := c.Decode(htRef, Extract)
	if err != nil {
		t.Fatal(err)
	}
	h1, ok := ht.(*Type1)
	if !ok {
		t.Fatalf("decoded %T, want *Type1", ht)
	}
	if got := w.Origin(h1.SpotFunction); got == 0 {
		t.Fatal("decoded SpotFunction has no provenance")
	}

	// Embedding a copy with no provenance entry of its own forces the
	// halftone's Embed method to run.  The function keeps whatever
	// provenance the decoder gave it.
	cp := *h1
	rm := pdf.NewResourceManager(w)
	if _, err := rm.Embed(&cp); err != nil {
		t.Fatal(err)
	}
	if err := rm.Close(); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	if n := bytes.Count(f.Data[len(orig):], []byte("/FunctionType 4")); n != 0 {
		t.Errorf("spot function written again: found %d copies in the update", n)
	}
}
