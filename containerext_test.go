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

package pdf_test

import (
	"errors"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

// putJBIG2Stream writes a JBIG2Decode stream at ref whose /JBIG2Globals
// points at globals.  Body is empty; only filter resolution is exercised.
func putJBIG2Stream(t *testing.T, w *pdf.Writer, ref pdf.Reference, globals pdf.Object) {
	t.Helper()
	dict := pdf.Dict{
		"Filter": pdf.Name("JBIG2Decode"),
	}
	if globals != nil {
		dict["DecodeParms"] = pdf.Dict{"JBIG2Globals": globals}
	}
	stm := pdf.NewStream(dict, nil)
	if err := w.Put(ref, stm); err != nil {
		t.Fatal(err)
	}
}

// TestGetFiltersJBIG2GlobalsCycleSelf checks that a JBIG2Globals reference
// pointing at its own stream is detected as a cycle rather than recursing
// until the goroutine stack is exhausted.
func TestGetFiltersJBIG2GlobalsCycleSelf(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	refA := w.Alloc()
	putJBIG2Stream(t, w, refA, refA)
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	stream, err := pdf.GetStream(w, refA)
	if err != nil {
		t.Fatal(err)
	}
	_, err = pdf.GetFilters(w, nil, stream.Dict)
	if !errors.Is(err, pdf.ErrCycle) {
		t.Errorf("expected cycle error, got %v", err)
	}
}

// TestGetFiltersJBIG2GlobalsCycleMutual checks that two JBIG2Decode streams
// whose /JBIG2Globals entries point at each other are detected as a cycle.
func TestGetFiltersJBIG2GlobalsCycleMutual(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	refA := w.Alloc()
	refB := w.Alloc()
	putJBIG2Stream(t, w, refA, refB)
	putJBIG2Stream(t, w, refB, refA)
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	stream, err := pdf.GetStream(w, refA)
	if err != nil {
		t.Fatal(err)
	}
	_, err = pdf.GetFilters(w, nil, stream.Dict)
	if !errors.Is(err, pdf.ErrCycle) {
		t.Errorf("expected cycle error, got %v", err)
	}
}

// TestGetFiltersJBIG2GlobalsValid confirms the non-cyclic path still works:
// stream A's /JBIG2Globals points at a plain stream B with no further
// indirection, and GetFilters returns a populated FilterJBIG2 without
// flagging a cycle.
func TestGetFiltersJBIG2GlobalsValid(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	refA := w.Alloc()
	refGlobals := w.Alloc()

	putJBIG2Stream(t, w, refA, refGlobals)
	globalsStm := pdf.NewStream(pdf.Dict{}, []byte("globals body"))
	if err := w.Put(refGlobals, globalsStm); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	stream, err := pdf.GetStream(w, refA)
	if err != nil {
		t.Fatal(err)
	}
	filters, err := pdf.GetFilters(w, nil, stream.Dict)
	if err != nil {
		t.Fatalf("GetFilters: %v", err)
	}
	if len(filters) != 1 {
		t.Fatalf("expected 1 filter, got %d", len(filters))
	}
	jf, ok := filters[0].(*pdf.FilterJBIG2)
	if !ok {
		t.Fatalf("expected *FilterJBIG2, got %T", filters[0])
	}
	if string(jf.Globals) != "globals body" {
		t.Errorf("globals = %q, want %q", jf.Globals, "globals body")
	}
}

// TestGetFiltersChainTooLong verifies that GetFilters rejects a /Filter
// array longer than the per-stream cap, preventing attackers from
// stacking many decoder wrappers on a single read.
func TestGetFiltersChainTooLong(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	ref := w.Alloc()

	chain := make(pdf.Array, 17)
	for i := range chain {
		chain[i] = pdf.Name("FlateDecode")
	}
	stm := pdf.NewStream(pdf.Dict{"Filter": chain}, nil)
	if err := w.Put(ref, stm); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	stream, err := pdf.GetStream(w, ref)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pdf.GetFilters(w, nil, stream.Dict); err == nil {
		t.Fatal("expected error for oversized filter chain, got nil")
	}
}
