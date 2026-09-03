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

package page

import (
	"bytes"
	"fmt"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/standard"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

func TestAnnotsRoundTrip(t *testing.T) {
	for _, singleUse := range []bool{false, true} {
		t.Run(map[bool]string{false: "indirect", true: "inline"}[singleUse], func(t *testing.T) {
			w, f := memfile.NewPDFWriter(t, pdf.V1_7, nil)
			rm := pdf.NewResourceManager(w)
			rect := pdf.Rectangle{LLx: 10, LLy: 10, URx: 50, URy: 30}
			pg := &Page{
				MediaBox: &pdf.Rectangle{URx: 100, URy: 100},
				Parent:   w.GetMeta().Catalog.Pages,
				Annots: &Annots{
					List:      []annotation.Annotation{&annotation.Square{Common: annotation.Common{Rect: rect}}},
					SingleUse: singleUse,
				},
			}
			ref, err := rm.Store(pg)
			if err != nil {
				t.Fatal(err)
			}
			if err := rm.Close(); err != nil {
				t.Fatal(err)
			}
			if err := w.Close(); err != nil {
				t.Fatal(err)
			}

			r, err := pdf.NewReader(bytes.NewReader(f.Data), int64(len(f.Data)), nil)
			if err != nil {
				t.Fatal(err)
			}
			dict, err := pdf.NewCursor(r).Dict(ref)
			if err != nil {
				t.Fatal(err)
			}
			_, isRef := dict["Annots"].(pdf.Reference)
			if isRef == singleUse {
				t.Errorf("Annots indirect = %v, want %v", isRef, !singleUse)
			}
			got, err := pdf.Decode(pdf.NewCursor(r), ref, Decode)
			if err != nil {
				t.Fatal(err)
			}
			if got.Annots == nil || len(got.Annots.List) != 1 || got.Annots.SingleUse != singleUse {
				t.Errorf("decoded Annots = %+v", got.Annots)
			}
		})
	}
}

func TestAddAnnotsCreatesIndirectList(t *testing.T) {
	pg := &Page{}
	pg.AddAnnots(&annotation.Square{})
	if pg.Annots == nil || len(pg.Annots.List) != 1 || pg.Annots.SingleUse {
		t.Errorf("AddAnnots gave %+v", pg.Annots)
	}
	pg.Annots.Add(&annotation.Square{}, &annotation.Square{})
	if len(pg.Annots.List) != 3 {
		t.Errorf("Add gave %d annotations, want 3", len(pg.Annots.List))
	}
}

func TestEmptyAnnotsOmitted(t *testing.T) {
	w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	rm := pdf.NewResourceManager(w)
	pg := &Page{
		MediaBox: &pdf.Rectangle{URx: 100, URy: 100},
		Parent:   w.GetMeta().Catalog.Pages,
		Annots:   &Annots{},
	}
	native, err := pg.Encode(rm)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := native.(pdf.Dict)["Annots"]; ok {
		t.Error("empty annotation list was written")
	}
}

// objectHeader returns the token that starts the given object in the file.
func objectHeader(ref pdf.Reference) []byte {
	return fmt.Appendf(nil, "\n%d %d obj", ref.Number(), ref.Generation())
}

// writeBase writes a one-page file whose page carries an annotation list of
// the given kind, plus a Resources dictionary with a font and a content
// stream, and returns the file and the page reference.  The Font
// subdictionary gives the "/Font" sub-check in TestUpdateReplacesPageOnly
// something to catch: without it, "no /Font was written again" would hold
// trivially even if the page's resources were re-embedded.  Likewise the
// content stream gives the "stream" sub-check something to catch.
func writeBase(t *testing.T, singleUse bool) (*memfile.MemFile, pdf.Reference) {
	t.Helper()
	w, f := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	rm := pdf.NewResourceManager(w)
	pg := &Page{
		MediaBox: &pdf.Rectangle{URx: 100, URy: 100},
		Parent:   w.GetMeta().Catalog.Pages,
		// Resources is indirect (SingleUse false, the default) so that,
		// when a rewrite touches only /Annots, the decoded value keeps
		// its provenance and re-embeds as the same reference, without
		// rewriting the Font subdictionary.
		Resources: &content.Resources{
			Font: map[pdf.Name]font.Instance{
				"F1": font.Must(standard.TimesRoman.New()),
			},
		},
		Contents: []Segment{
			&content.Operators{Ops: []content.Operator{
				{Name: content.OpPushGraphicsState},
				{Name: content.OpPopGraphicsState},
			}},
		},
		Annots: &Annots{
			List:      []annotation.Annotation{&annotation.Square{Common: annotation.Common{Rect: pdf.Rectangle{URx: 10, URy: 10}}}},
			SingleUse: singleUse,
		},
	}
	ref, err := rm.Store(pg)
	if err != nil {
		t.Fatal(err)
	}
	if err := rm.Close(); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return f, ref
}

func TestUpdateReplacesPageOnly(t *testing.T) {
	f, pageRef := writeBase(t, true)
	orig := bytes.Clone(f.Data)

	w, err := pdf.NewUpdater(f, int64(len(orig)), nil)
	if err != nil {
		t.Fatal(err)
	}
	c := pdf.NewCursor(w)
	old, err := pdf.Decode(c, pageRef, Decode)
	if err != nil {
		t.Fatal(err)
	}
	rm := pdf.NewResourceManager(w)

	p := *old
	p.Annots = &Annots{List: append([]annotation.Annotation{}, old.Annots.List...), SingleUse: true}
	p.Annots.Add(&annotation.Square{Common: annotation.Common{Rect: pdf.Rectangle{URx: 20, URy: 20}}})
	if _, err := rm.Replace(old, &p); err != nil {
		t.Fatal(err)
	}
	if err := rm.Close(); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	appended := f.Data[len(orig):]
	if !bytes.Contains(appended, objectHeader(pageRef)) {
		t.Error("page was not rewritten")
	}
	if got := bytes.Count(appended, []byte("/Square")); got != 1 {
		t.Errorf("%d annotation objects appended, want 1 (the new one)", got)
	}
	// the update's own cross-reference stream always contains "stream";
	// only the newly written objects before it matter here.  /Font is the
	// meaningful check: writeBase gives the page an indirect Resources
	// dictionary with a font, so this catches Resources being re-embedded.
	body := appended
	if i := bytes.Index(body, []byte("/Type/XRef")); i >= 0 {
		body = body[:i]
	}
	if bytes.Contains(body, []byte("/Font")) || bytes.Contains(body, []byte("stream")) {
		t.Error("resources or content streams were written again")
	}

	r, err := pdf.NewReader(bytes.NewReader(f.Data), int64(len(f.Data)), nil)
	if err != nil {
		t.Fatal(err)
	}
	got, err := pdf.Decode(pdf.NewCursor(r), pageRef, Decode)
	if err != nil {
		t.Fatal(err)
	}
	if got.Annots == nil || len(got.Annots.List) != 2 {
		t.Errorf("page has %d annotations after update, want 2", len(got.Annots.List))
	}
}

// TestContentSegmentsHaveProvenance checks that a page's content-stream
// segments, decoded through an update Writer, carry provenance, and that
// replacing the page with one carrying an extra annotation appends no
// content stream.
func TestContentSegmentsHaveProvenance(t *testing.T) {
	f, pageRef := writeBase(t, true)
	orig := bytes.Clone(f.Data)

	w, err := pdf.NewUpdater(f, int64(len(orig)), nil)
	if err != nil {
		t.Fatal(err)
	}
	c := pdf.NewCursor(w)
	old, err := pdf.Decode(c, pageRef, Decode)
	if err != nil {
		t.Fatal(err)
	}
	if len(old.Contents) == 0 {
		t.Fatal("test setup: page has no content segments")
	}
	for i, seg := range old.Contents {
		src, ok := seg.(*Source)
		if !ok {
			t.Fatalf("Contents[%d] is %T, want *Source", i, seg)
		}
		if w.Origin(src) == 0 {
			t.Errorf("Contents[%d] has no provenance", i)
		}
	}

	rm := pdf.NewResourceManager(w)
	p := *old
	p.Annots = &Annots{List: append([]annotation.Annotation{}, old.Annots.List...), SingleUse: true}
	p.Annots.Add(&annotation.Square{Common: annotation.Common{Rect: pdf.Rectangle{URx: 20, URy: 20}}})
	if _, err := rm.Replace(old, &p); err != nil {
		t.Fatal(err)
	}
	if err := rm.Close(); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	appended := f.Data[len(orig):]
	body := appended
	if i := bytes.Index(body, []byte("/Type/XRef")); i >= 0 {
		body = body[:i]
	}
	if bytes.Contains(body, []byte("stream")) {
		t.Error("content stream was written again")
	}
}

func TestUpdateReplacesIndirectAnnotsOnly(t *testing.T) {
	f, pageRef := writeBase(t, false)
	orig := bytes.Clone(f.Data)

	w, err := pdf.NewUpdater(f, int64(len(orig)), nil)
	if err != nil {
		t.Fatal(err)
	}
	c := pdf.NewCursor(w)
	old, err := pdf.Decode(c, pageRef, Decode)
	if err != nil {
		t.Fatal(err)
	}
	annotsRef := w.Origin(old.Annots)
	if annotsRef == 0 {
		t.Fatal("indirect annotation list has no origin")
	}
	rm := pdf.NewResourceManager(w)

	list := &Annots{List: append([]annotation.Annotation{}, old.Annots.List...)}
	list.Add(&annotation.Square{Common: annotation.Common{Rect: pdf.Rectangle{URx: 20, URy: 20}}})
	if _, err := rm.Replace(old.Annots, list); err != nil {
		t.Fatal(err)
	}
	// replacing the page as well must write nothing further
	p := *old
	p.Annots = list
	if _, err := rm.Replace(old, &p); err != nil {
		t.Fatal(err)
	}
	if err := rm.Close(); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	appended := f.Data[len(orig):]
	if bytes.Contains(appended, objectHeader(pageRef)) {
		t.Error("page was rewritten although only the annotation list changed")
	}
	if !bytes.Contains(appended, objectHeader(annotsRef)) {
		t.Error("annotation list was not rewritten at its original number")
	}
	if got := bytes.Count(appended, []byte("/Square")); got != 1 {
		t.Errorf("%d annotation objects appended, want 1", got)
	}
}

// TestDecodeAnnotsEmptyOrMalformed verifies that an empty or malformed
// /Annots entry decodes as nil, matching what Annots.Embed writes for an
// empty list (nothing).  Without this, a read-write-read cycle would
// diverge: the first read gives a non-nil, empty *Annots, the second nil.
func TestDecodeAnnotsEmptyOrMalformed(t *testing.T) {
	for _, tc := range []struct {
		name  string
		value pdf.Object
	}{
		{"empty array", pdf.Array{}},
		{"wrong type", pdf.Name("bogus")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
			pagesRef := w.GetMeta().Catalog.Pages
			pageRef := w.Alloc()
			dict := pdf.Dict{
				"Type":     pdf.Name("Page"),
				"Parent":   pagesRef,
				"MediaBox": &pdf.Rectangle{URx: 100, URy: 100},
				"Annots":   tc.value,
			}
			if err := w.Put(pageRef, dict); err != nil {
				t.Fatal(err)
			}
			if err := w.Close(); err != nil {
				t.Fatal(err)
			}

			c := pdf.NewCursor(w)
			p, err := pdf.Decode(c, pageRef, Decode)
			if err != nil {
				t.Fatal(err)
			}
			if p.Annots != nil {
				t.Errorf("Annots = %+v, want nil", p.Annots)
			}

			rm := pdf.NewResourceManager(w)
			native, err := p.Encode(rm)
			if err != nil {
				t.Fatal(err)
			}
			if err := rm.Close(); err != nil {
				t.Fatal(err)
			}
			if _, ok := native.(pdf.Dict)["Annots"]; ok {
				t.Error("empty or malformed Annots was written back")
			}
		})
	}
}

// TestPageRewriteKeepsInheritedMediaBox regresses a bug in
// examples/annotation/watermark: rewriting a page whose /MediaBox is
// inherited from the Pages tree, and whose existing /Annots is inline (so
// it has no provenance and the page dictionary itself must change), must
// not introduce an explicit /MediaBox.  page.Decode substitutes US Letter
// for a missing /MediaBox; going through the typed Page.Encode route on
// such a decoded value would write that substitute back as if it had been
// the page's real, inherited box.  The fix edits the stored page
// dictionary directly instead, touching only /Annots.
func TestPageRewriteKeepsInheritedMediaBox(t *testing.T) {
	w, f := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	pagesRef := w.Alloc()
	pageRef := w.Alloc()

	// the Pages node carries a non-Letter MediaBox, inherited by the page
	inherited := &pdf.Rectangle{URx: 400, URy: 600}
	pagesDict := pdf.Dict{
		"Type":     pdf.Name("Pages"),
		"Kids":     pdf.Array{pageRef},
		"Count":    pdf.Integer(1),
		"MediaBox": inherited,
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
		// no /MediaBox: it is inherited from the Pages node
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
	old, err := pdf.Decode(c, pageRef, Decode)
	if err != nil {
		t.Fatal(err)
	}
	if old.Annots == nil || upd.Origin(old.Annots) != 0 {
		t.Fatal("test setup: /Annots must be inline (no provenance)")
	}

	// replicate examples/annotation/watermark's page-rewrite branch: edit
	// the stored page dictionary directly instead of going through
	// page.Encode, so page.Decode's MediaBox fallback is not written back
	// as an explicit value
	annots := &Annots{SingleUse: old.Annots.SingleUse}
	annots.Add(old.Annots.List...)
	annots.Add(&annotation.Square{Common: annotation.Common{Rect: pdf.Rectangle{URx: 20, URy: 20}}})

	rm := pdf.NewResourceManager(upd)
	pageDict2, err := c.Dict(pageRef)
	if err != nil {
		t.Fatal(err)
	}
	annotsObj, err := rm.Embed(annots)
	if err != nil {
		t.Fatal(err)
	}
	if annotsObj != nil {
		pageDict2["Annots"] = annotsObj
	} else {
		delete(pageDict2, "Annots")
	}
	if err := upd.Put(pageRef, pageDict2); err != nil {
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
	got, err := pdf.NewCursor(r).Dict(pageRef)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := got["MediaBox"]; ok {
		t.Errorf("rewritten page gained an explicit /MediaBox: %v", got["MediaBox"])
	}
}
