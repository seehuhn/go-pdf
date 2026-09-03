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

// writeBase writes a one-page file whose page carries an annotation list
// of the given kind and returns the file, the page reference and the
// resources reference.
func writeBase(t *testing.T, singleUse bool) (*memfile.MemFile, pdf.Reference) {
	t.Helper()
	w, f := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	rm := pdf.NewResourceManager(w)
	pg := &Page{
		MediaBox: &pdf.Rectangle{URx: 100, URy: 100},
		Parent:   w.GetMeta().Catalog.Pages,
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
	// only the newly written objects before it matter here
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
