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
	"bytes"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

// TestXRefOffsetsSingleSection checks that a freshly written file has one
// cross-reference section, at its startxref position.
func TestXRefOffsetsSingleSection(t *testing.T) {
	w, f := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	r := reopen(t, f.Data)
	got := r.XRefOffsets()
	want := []int64{lastStartXRef(t, f.Data)}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("XRefOffsets() mismatch (-want +got):\n%s", diff)
	}
}

// TestXRefOffsetsAfterUpdate checks that an incremental update adds a
// second, newer offset, and that the older offset is unchanged.
func TestXRefOffsetsAfterUpdate(t *testing.T) {
	f, oldRef := newBaseFile(t, pdf.V1_7, nil)
	baseXRef := lastStartXRef(t, f.Data)

	w, err := pdf.NewUpdater(f, int64(len(f.Data)), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Put(oldRef, pdf.Name("changed")); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	r := reopen(t, f.Data)
	got := r.XRefOffsets()
	want := []int64{lastStartXRef(t, f.Data), baseXRef}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("XRefOffsets() mismatch (-want +got):\n%s", diff)
	}
	if len(got) == 2 && got[0] <= got[1] {
		t.Errorf("offsets not newest-first: %v", got)
	}
}

// TestXRefOffsetsHybrid checks that a hybrid-reference section (a classic
// table with an /XRefStm) counts as a single section, listed at the
// table's offset.
func TestXRefOffsetsHybrid(t *testing.T) {
	base, _ := newBaseFile(t, pdf.V1_4, nil)
	data := base.Data
	prev := lastStartXRef(t, data)
	size := trailerSize(t, data)
	hidden := size     // only listed in the cross-reference stream
	stmNum := size + 1 // the cross-reference stream object

	root, ok := reopen(t, data).GetMeta().Trailer["Root"].(pdf.Reference)
	if !ok {
		t.Fatal("base file has no direct Root reference")
	}

	buf := bytes.NewBuffer(bytes.Clone(data))
	hiddenPos := buf.Len()
	fmt.Fprintf(buf, "%d 0 obj\n/hidden\nendobj\n", hidden)

	// one type-1 entry for hidden: W [1 4 2]
	entry := []byte{1,
		byte(hiddenPos >> 24), byte(hiddenPos >> 16), byte(hiddenPos >> 8), byte(hiddenPos),
		0, 0}
	stmPos := buf.Len()
	fmt.Fprintf(buf, "%d 0 obj\n<< /Type /XRef /Size %d /W [1 4 2] /Index [%d 1] /Length %d >>\nstream\n",
		stmNum, size+2, hidden, len(entry))
	buf.Write(entry)
	buf.WriteString("\nendstream\nendobj\n")

	xrefPos := buf.Len()
	fmt.Fprintf(buf, "xref\n0 0\ntrailer\n<< /Size %d /Root %d 0 R /Prev %d /XRefStm %d >>\nstartxref\n%d\n%%%%EOF\n",
		size+2, root.Number(), prev, stmPos, xrefPos)
	data = buf.Bytes()

	r := reopen(t, data)
	got := r.XRefOffsets()
	want := []int64{int64(xrefPos), int64(prev)}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("XRefOffsets() mismatch (-want +got):\n%s", diff)
	}
}

// TestXRefOffsetsIsCopy checks that mutating the returned slice does not
// affect the Reader.
func TestXRefOffsetsIsCopy(t *testing.T) {
	w, f := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	r := reopen(t, f.Data)
	got := r.XRefOffsets()
	if len(got) == 0 {
		t.Fatal("no offsets")
	}
	got[0] = -1

	again := r.XRefOffsets()
	if again[0] == -1 {
		t.Error("XRefOffsets returned a slice sharing storage with the reader")
	}
}
