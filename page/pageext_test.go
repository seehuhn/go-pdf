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

package page_test

import (
	"bytes"
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/internal/debug/memfile"
	pdfpage "seehuhn.de/go/pdf/page"
	"seehuhn.de/go/pdf/pagetree"
)

// TestPageRewriteKeepsInheritedMediaBox regresses a bug in
// examples/annotation/watermark: rewriting a page whose /MediaBox and
// /Rotate are inherited from the Pages tree, and whose existing /Annots is
// inline (so it has no provenance and the page dictionary itself must
// change), must not introduce explicit entries for the inherited values.
// The decoded page holds nil for an inherited box and [page.RotateInherit]
// for an inherited rotation, and Page.Encode omits both, so the values in
// force stay the ones from the Pages node.
func TestPageRewriteKeepsInheritedMediaBox(t *testing.T) {
	w, f := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	pagesRef := w.Alloc()
	pageRef := w.Alloc()

	// the Pages node carries a non-Letter MediaBox and a rotation, both
	// inherited by the page
	inherited := &pdf.Rectangle{URx: 400, URy: 600}
	pagesDict := pdf.Dict{
		"Type":     pdf.Name("Pages"),
		"Kids":     pdf.Array{pageRef},
		"Count":    pdf.Integer(1),
		"MediaBox": inherited,
		"Rotate":   pdf.Integer(90),
	}
	if err := w.Put(pagesRef, pagesDict); err != nil {
		t.Fatal(err)
	}

	annotRef := w.Alloc()
	annotDict := pdf.Dict{
		"Type":    pdf.Name("Annot"),
		"Subtype": pdf.Name("Square"),
		"Rect":    &pdf.Rectangle{URx: 10, URy: 10},
	}
	if err := w.Put(annotRef, annotDict); err != nil {
		t.Fatal(err)
	}
	pageDict := pdf.Dict{
		"Type":   pdf.Name("Page"),
		"Parent": pagesRef,
		// no /MediaBox and no /Rotate: both come from the Pages node
		"Annots": pdf.Array{annotRef}, // inline array: no provenance
	}
	if err := w.Put(pageRef, pageDict); err != nil {
		t.Fatal(err)
	}
	w.GetMeta().Catalog.Pages = pagesRef
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	upd, err := pdf.NewUpdater(f, int64(len(f.Data)), nil)
	if err != nil {
		t.Fatal(err)
	}
	c := pdf.NewCursor(upd)
	old, err := pdf.Decode(c, pageRef, pdfpage.Decode)
	if err != nil {
		t.Fatal(err)
	}
	if old.Annots == nil || upd.Origin(old.Annots) != 0 {
		t.Fatal("test setup: /Annots must be inline (no provenance)")
	}
	if old.MediaBox != nil {
		t.Errorf("MediaBox = %v, want nil", old.MediaBox)
	}
	if old.Rotate != pdfpage.RotateInherit {
		t.Errorf("Rotate = %v, want RotateInherit", old.Rotate)
	}

	// replicate examples/annotation/watermark's page-rewrite branch: the
	// decoded page is immutable, so a copy carrying the new annotation
	// list replaces it at its original object number
	annots := &pdfpage.Annots{SingleUse: old.Annots.SingleUse}
	annots.Add(old.Annots.List...)
	annots.Add(&annotation.Square{Common: annotation.Common{Rect: pdf.Rectangle{URx: 20, URy: 20}}})

	rm := pdf.NewResourceManager(upd)
	p := *old
	p.Annots = annots
	if _, err := rm.Replace(old, &p); err != nil {
		t.Fatal(err)
	}
	if err := rm.Close(); err != nil {
		t.Fatal(err)
	}
	if err := upd.Close(); err != nil {
		t.Fatal(err)
	}

	r, err := pdf.NewReader(bytes.NewReader(f.Data), int64(len(f.Data)), nil)
	if err != nil {
		t.Fatal(err)
	}
	rc := pdf.NewCursor(r)
	got, err := rc.Dict(pageRef)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := got["MediaBox"]; ok {
		t.Errorf("rewritten page gained an explicit /MediaBox: %v", got["MediaBox"])
	}
	if _, ok := got["Rotate"]; ok {
		t.Errorf("rewritten page gained an explicit /Rotate: %v", got["Rotate"])
	}
	if got["Parent"] != pagesRef {
		t.Errorf("Parent = %v, want %v", got["Parent"], pagesRef)
	}

	// the rewrite must have taken effect
	gotAnnots, err := rc.Array(got["Annots"])
	if err != nil {
		t.Fatal(err)
	}
	if len(gotAnnots) != 2 {
		t.Errorf("got %d annotations, want 2", len(gotAnnots))
	}

	// the values in force for the page are still the inherited ones
	_, view, err := pagetree.GetPage(r, 0)
	if err != nil {
		t.Fatal(err)
	}
	viewBox, err := rc.Rectangle(view["MediaBox"])
	if err != nil {
		t.Fatal(err)
	}
	if d := cmp.Diff(inherited, viewBox); d != "" {
		t.Errorf("unexpected MediaBox in force (-want +got):\n%s", d)
	}
	if view["Rotate"] != pdf.Integer(90) {
		t.Errorf("Rotate in force = %v, want 90", view["Rotate"])
	}
}
