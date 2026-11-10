// Copyright 2024 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package memfile

import (
	"testing"

	"seehuhn.de/go/pdf"
)

// NewPDFWriter creates a new PDF writer which writes to a MemFile.
// The returned writer can still be used as a reader after it has been closed.
//
// A minimal page tree is added automatically so the resulting document
// has a valid catalog without further setup.  Tests that need a different
// page tree can overwrite [Catalog.Pages] themselves.
//
// After the test finishes, the generated file is checked for real numbers
// with an implausibly long fractional part (see [CheckNumbers]), and the
// test fails if any are found.
func NewPDFWriter(t testing.TB, v pdf.Version, opt *pdf.WriterOptions) (*pdf.Writer, *MemFile) {
	tmpFile := New()
	w, err := pdf.NewWriter(tmpFile, v, opt)
	if err != nil {
		t.Fatalf("cannot create PDF writer: %v", err)
	}
	if err := AddBlankPage(w); err != nil {
		t.Fatalf("cannot add blank page: %v", err)
	}
	t.Cleanup(func() {
		for _, s := range CheckNumbers(tmpFile.Data) {
			t.Errorf("unrounded number in generated PDF: %s", s)
		}
	})
	return w, tmpFile
}

// AddBlankPage adds a minimal page tree with a single blank page to the writer.
// This creates the minimum structure required for a valid PDF document that can
// be read by pdf.NewReader.
func AddBlankPage(w *pdf.Writer) error {
	pageRef := w.Alloc()
	pagesRef := w.Alloc()

	pageDict := pdf.Dict{
		"Type":      pdf.Name("Page"),
		"Parent":    pagesRef,
		"Resources": pdf.Dict{},
		"MediaBox":  &pdf.Rectangle{URx: 100, URy: 100},
	}
	err := w.Put(pageRef, pageDict)
	if err != nil {
		return err
	}

	pagesDict := pdf.Dict{
		"Type":  pdf.Name("Pages"),
		"Kids":  pdf.Array{pageRef},
		"Count": pdf.Integer(1),
	}
	err = w.Put(pagesRef, pagesDict)
	if err != nil {
		return err
	}

	w.GetMeta().Catalog.Pages = pagesRef
	return nil
}
