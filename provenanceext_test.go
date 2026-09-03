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
	"errors"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

func TestTwoManagersShareObjects(t *testing.T) {
	w, f := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	rm1 := pdf.NewResourceManager(w)
	rm2 := pdf.NewResourceManager(w)

	info := &pdf.Info{Title: "shared"}
	ref1, err := rm1.Embed(info)
	if err != nil {
		t.Fatal(err)
	}
	ref2, err := rm2.Embed(info)
	if err != nil {
		t.Fatal(err)
	}
	if ref1 != ref2 {
		t.Errorf("same value embedded as %v and %v", ref1, ref2)
	}
	if err := rm1.Close(); err != nil {
		t.Fatal(err)
	}
	if err := rm2.Close(); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if n := bytes.Count(f.Data, []byte("(shared)")); n != 1 {
		t.Errorf("title written %d times, want 1", n)
	}
}

// newUpdateBase writes a one-page file with an Info dictionary and one
// extra object, and returns it.
func newUpdateBase(t *testing.T) *memfile.MemFile {
	t.Helper()
	w, f := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	w.GetMeta().Info.Title = "base"
	if err := w.Put(w.Alloc(), pdf.Name("extra")); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return f
}

func TestOriginOfDecodedValue(t *testing.T) {
	f := newUpdateBase(t)
	w, err := pdf.NewUpdater(f, int64(len(f.Data)), nil)
	if err != nil {
		t.Fatal(err)
	}
	c := pdf.NewCursor(w)
	pagesRef := w.GetMeta().Catalog.Pages
	pg, err := pdf.Decode(c, pagesRef, func(c pdf.Cursor, obj pdf.Object, _ bool) (*pdf.Dict, error) {
		d, err := c.Dict(obj)
		return &d, err
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := w.Origin(pg); got != pagesRef {
		t.Errorf("Origin = %v, want %v", got, pagesRef)
	}
	if got := w.Origin(&pdf.Dict{}); got != 0 {
		t.Errorf("Origin of unrelated value = %v, want 0", got)
	}
}

// TestOriginOfNonComparableValue regresses a panic in Origin for a value
// whose dynamic type is not comparable (e.g. pdf.Dict, a map type), which
// cannot be used as a map key.
func TestOriginOfNonComparableValue(t *testing.T) {
	f := newUpdateBase(t)
	w, err := pdf.NewUpdater(f, int64(len(f.Data)), nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := w.Origin(pdf.Dict{"A": pdf.Integer(1)}); got != 0 {
		t.Errorf("Origin = %v for a non-comparable value, want 0", got)
	}
}

func TestDecodedValueEmbedsAsOrigin(t *testing.T) {
	f := newUpdateBase(t)
	orig := bytes.Clone(f.Data)
	w, err := pdf.NewUpdater(f, int64(len(orig)), nil)
	if err != nil {
		t.Fatal(err)
	}
	c := pdf.NewCursor(w)
	infoRef := w.GetMeta().Trailer["Info"].(pdf.Reference)
	info, err := pdf.Decode(c, infoRef, pdf.ExtractInfo)
	if err != nil {
		t.Fatal(err)
	}
	rm := pdf.NewResourceManager(w)
	got, err := rm.Embed(info)
	if err != nil {
		t.Fatal(err)
	}
	if got != infoRef {
		t.Errorf("embedded as %v, want %v", got, infoRef)
	}
	if err := rm.Close(); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(f.Data[len(orig):], []byte("(base)")) {
		t.Error("unchanged Info was written again")
	}
}

func TestValueFromOtherGetterHasNoOrigin(t *testing.T) {
	f := newUpdateBase(t)
	r, err := pdf.NewReader(bytes.NewReader(f.Data), int64(len(f.Data)), nil)
	if err != nil {
		t.Fatal(err)
	}
	infoRef := r.GetMeta().Trailer["Info"].(pdf.Reference)
	info, err := pdf.Decode(pdf.NewCursor(r), infoRef, pdf.ExtractInfo)
	if err != nil {
		t.Fatal(err)
	}

	w, err := pdf.NewUpdater(f, int64(len(f.Data)), nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := w.Origin(info); got != 0 {
		t.Errorf("Origin = %v for a value from another Getter", got)
	}
	rm := pdf.NewResourceManager(w)
	got, err := rm.Embed(info)
	if err != nil {
		t.Fatal(err)
	}
	if got == infoRef {
		t.Error("value from another Getter reused the original reference")
	}
}

func TestInlineValueHasNoOrigin(t *testing.T) {
	f := newUpdateBase(t)
	w, err := pdf.NewUpdater(f, int64(len(f.Data)), nil)
	if err != nil {
		t.Fatal(err)
	}
	c := pdf.NewCursor(w)
	v, err := pdf.Decode(c, pdf.Dict{"A": pdf.Integer(1)}, func(c pdf.Cursor, obj pdf.Object, _ bool) (*pdf.Dict, error) {
		d, err := c.Dict(obj)
		return &d, err
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := w.Origin(v); got != 0 {
		t.Errorf("Origin = %v for an inline value", got)
	}
}

func TestOriginFollowsReferenceChain(t *testing.T) {
	w0, f := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	target := w0.Alloc()
	middle := w0.Alloc()
	if err := w0.Put(target, pdf.Dict{"Kind": pdf.Name("target")}); err != nil {
		t.Fatal(err)
	}
	if err := w0.Put(middle, target); err != nil {
		t.Fatal(err)
	}
	if err := w0.Close(); err != nil {
		t.Fatal(err)
	}

	w, err := pdf.NewUpdater(f, int64(len(f.Data)), nil)
	if err != nil {
		t.Fatal(err)
	}
	v, err := pdf.Decode(pdf.NewCursor(w), middle, func(c pdf.Cursor, obj pdf.Object, _ bool) (*pdf.Dict, error) {
		d, err := c.Dict(obj)
		return &d, err
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := w.Origin(v); got != target {
		t.Errorf("Origin = %v, want the last reference %v", got, target)
	}
}

func TestReplaceWritesInPlace(t *testing.T) {
	f := newUpdateBase(t)
	orig := bytes.Clone(f.Data)
	w, err := pdf.NewUpdater(f, int64(len(orig)), nil)
	if err != nil {
		t.Fatal(err)
	}
	c := pdf.NewCursor(w)
	infoRef := w.GetMeta().Trailer["Info"].(pdf.Reference)
	old, err := pdf.Decode(c, infoRef, pdf.ExtractInfo)
	if err != nil {
		t.Fatal(err)
	}

	repl := *old
	repl.Title = "replaced"
	rm := pdf.NewResourceManager(w)
	ref, err := rm.Replace(old, &repl)
	if err != nil {
		t.Fatal(err)
	}
	if ref != infoRef {
		t.Errorf("Replace wrote at %v, want %v", ref, infoRef)
	}
	if got, _ := rm.Embed(old); got != infoRef {
		t.Errorf("original embeds as %v after Replace", got)
	}
	if got, _ := rm.Embed(&repl); got != infoRef {
		t.Errorf("replacement embeds as %v after Replace", got)
	}
	if _, err := rm.Replace(old, &repl); err == nil {
		t.Error("second Replace of the same object succeeded")
	}
	if _, err := rm.Replace(old, "not an encoder"); err == nil {
		t.Error("Replace with a non-Encoder succeeded")
	}
	if _, err := rm.Replace(&pdf.Info{}, &repl); err == nil {
		t.Error("Replace of a value without provenance succeeded")
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
	if r.GetMeta().Info == nil || r.GetMeta().Info.Title != "replaced" {
		t.Errorf("Info after update: %+v", r.GetMeta().Info)
	}
}

func TestReplaceSkipsEqualEncoding(t *testing.T) {
	w0, f := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	ref0 := w0.Alloc()
	if err := w0.Put(ref0, pdf.Dict{"Type": pdf.Name("Catalog"), "Pages": w0.GetMeta().Catalog.Pages}); err != nil {
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
	old, err := pdf.Decode(pdf.NewCursor(w), ref0, pdf.DecodeCatalog)
	if err != nil {
		t.Fatal(err)
	}
	repl := *old
	rm := pdf.NewResourceManager(w)
	if _, err := rm.Replace(old, &repl); err != nil {
		t.Fatal(err)
	}
	if err := rm.Close(); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(f.Data[len(orig):], []byte("/Catalog")) {
		t.Error("an equal replacement was written")
	}
}

// selfRefEncoder embeds its own reference in its dictionary, as outline
// nodes do for their children's /Parent entries.
type selfRefEncoder struct{ Label pdf.Name }

func (s *selfRefEncoder) Encode(rm *pdf.ResourceManager) (pdf.Native, error) {
	return pdf.Dict{"Label": s.Label, "Self": rm.GetReference(s)}, nil
}

func decodeSelfRef(c pdf.Cursor, obj pdf.Object, _ bool) (*selfRefEncoder, error) {
	dict, err := c.Dict(obj)
	if err != nil {
		return nil, err
	}
	label, err := c.Name(dict["Label"])
	if err != nil {
		return nil, err
	}
	return &selfRefEncoder{Label: label}, nil
}

func TestReplaceSelfReferencingEncoder(t *testing.T) {
	w0, f := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	selfRef := w0.Alloc()
	if err := w0.Put(selfRef, pdf.Dict{"Label": pdf.Name("old"), "Self": selfRef}); err != nil {
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
	old, err := pdf.Decode(pdf.NewCursor(w), selfRef, decodeSelfRef)
	if err != nil {
		t.Fatal(err)
	}

	repl := &selfRefEncoder{Label: "new"}
	rm := pdf.NewResourceManager(w)
	ref, err := rm.Replace(old, repl)
	if err != nil {
		t.Fatal(err)
	}
	if ref != selfRef {
		t.Errorf("Replace wrote at %v, want %v", ref, selfRef)
	}
	if _, err := rm.Replace(old, repl); err == nil {
		t.Error("second Replace of the same object succeeded")
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
	dict, err := pdf.NewCursor(r).Dict(selfRef)
	if err != nil {
		t.Fatal(err)
	}
	if dict["Label"] != pdf.Name("new") {
		t.Errorf("Label = %v, want new", dict["Label"])
	}
	if dict["Self"] != pdf.Object(selfRef) {
		t.Errorf("Self = %v, want %v", dict["Self"], selfRef)
	}
}

// toggleEmbedder succeeds the first time it is embedded and fails
// thereafter; used to give a value a prior, successful entry before a later,
// failing Replace attempt.
type toggleEmbedder struct{ fail bool }

func (e *toggleEmbedder) Embed(h *pdf.EmbedHelper) (pdf.Native, error) {
	if e.fail {
		return nil, errors.New("boom")
	}
	ref := h.AllocSelf()
	if err := h.Out().Put(ref, pdf.Dict{"Kind": pdf.Name("toggle")}); err != nil {
		return nil, err
	}
	return ref, nil
}

// TestReplaceEmbedderErrorRestoresPreviousEntry regresses a bug where a
// failing Embedder in Replace left repl's provenance entry deleted, even
// though repl already had a valid entry from an earlier, successful embed.
func TestReplaceEmbedderErrorRestoresPreviousEntry(t *testing.T) {
	f := newUpdateBase(t)
	w, err := pdf.NewUpdater(f, int64(len(f.Data)), nil)
	if err != nil {
		t.Fatal(err)
	}
	c := pdf.NewCursor(w)
	infoRef := w.GetMeta().Trailer["Info"].(pdf.Reference)
	old, err := pdf.Decode(c, infoRef, pdf.ExtractInfo)
	if err != nil {
		t.Fatal(err)
	}

	rm := pdf.NewResourceManager(w)
	repl := &toggleEmbedder{}
	prevNative, err := rm.Embed(repl)
	if err != nil {
		t.Fatal(err)
	}
	prevRef, ok := prevNative.(pdf.Reference)
	if !ok {
		t.Fatalf("Embed returned %T, want pdf.Reference", prevNative)
	}

	repl.fail = true
	if _, err := rm.Replace(old, repl); err == nil {
		t.Fatal("Replace with a failing Embedder succeeded")
	}
	if got := w.Origin(repl); got != prevRef {
		t.Errorf("Origin(repl) = %v after failed Replace, want the previous reference %v", got, prevRef)
	}
}

// inlineEmbedder always embeds inline, ignoring any requested reference.
type inlineEmbedder struct{}

func (inlineEmbedder) Embed(h *pdf.EmbedHelper) (pdf.Native, error) {
	return pdf.Dict{"Inline": pdf.Boolean(true)}, nil
}

// TestReplaceEmbedderInlineRejected verifies that Replace rejects an
// Embedder which does not embed itself as an indirect object at the
// requested reference: Replace's contract requires an object at orig's
// reference, which an inline embedding cannot provide.
func TestReplaceEmbedderInlineRejected(t *testing.T) {
	f := newUpdateBase(t)
	w, err := pdf.NewUpdater(f, int64(len(f.Data)), nil)
	if err != nil {
		t.Fatal(err)
	}
	c := pdf.NewCursor(w)
	infoRef := w.GetMeta().Trailer["Info"].(pdf.Reference)
	old, err := pdf.Decode(c, infoRef, pdf.ExtractInfo)
	if err != nil {
		t.Fatal(err)
	}

	rm := pdf.NewResourceManager(w)
	repl := &inlineEmbedder{}
	if _, err := rm.Replace(old, repl); err == nil {
		t.Fatal("Replace accepted an Embedder that embeds inline")
	}
	if got := w.Origin(repl); got != 0 {
		t.Errorf("Origin(repl) = %v after rejected Replace, want 0", got)
	}
}

// sameKindEncoder always encodes to the same dictionary content, so a
// Replace using it as the replacement compares equal to a matching original
// and is skipped.
type sameKindEncoder struct{}

func (*sameKindEncoder) Encode(rm *pdf.ResourceManager) (pdf.Native, error) {
	return pdf.Dict{"Kind": pdf.Name("thing")}, nil
}

// TestReplaceEqualEncodingKeepsUnrelatedReservation regresses a bug where
// Replace, on finding the replacement's encoding equal to the stored
// original (so nothing is written), unconditionally cleared any pending
// reservation for the replacement value — even one made earlier for an
// entirely different, still-unwritten reference.  That reservation must
// stay in place so Close still reports it as dangling.
func TestReplaceEqualEncodingKeepsUnrelatedReservation(t *testing.T) {
	w0, f := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	thingRef := w0.Alloc()
	if err := w0.Put(thingRef, pdf.Dict{"Kind": pdf.Name("thing")}); err != nil {
		t.Fatal(err)
	}
	if err := w0.Close(); err != nil {
		t.Fatal(err)
	}

	w, err := pdf.NewUpdater(f, int64(len(f.Data)), nil)
	if err != nil {
		t.Fatal(err)
	}
	old, err := pdf.Decode(pdf.NewCursor(w), thingRef, func(c pdf.Cursor, obj pdf.Object, _ bool) (*pdf.Dict, error) {
		d, err := c.Dict(obj)
		return &d, err
	})
	if err != nil {
		t.Fatal(err)
	}

	rm := pdf.NewResourceManager(w)
	repl := &sameKindEncoder{}
	otherRef := rm.GetReference(repl) // reserved for an unrelated write that never happens
	if otherRef == thingRef {
		t.Fatal("test setup: reserved reference collides with the original's")
	}

	gotRef, err := rm.Replace(old, repl)
	if err != nil {
		t.Fatal(err)
	}
	if gotRef != thingRef {
		t.Errorf("Replace returned %v, want the original's reference %v", gotRef, thingRef)
	}

	if err := rm.Close(); err == nil {
		t.Error("Close succeeded despite a dangling reservation for an unrelated reference")
	}
}

func TestOriginVoidAfterFree(t *testing.T) {
	f := newUpdateBase(t)
	w, err := pdf.NewUpdater(f, int64(len(f.Data)), nil)
	if err != nil {
		t.Fatal(err)
	}
	c := pdf.NewCursor(w)
	infoRef := w.GetMeta().Trailer["Info"].(pdf.Reference)
	info, err := pdf.Decode(c, infoRef, pdf.ExtractInfo)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Free(infoRef); err != nil {
		t.Fatal(err)
	}
	if got := w.Origin(info); got != 0 {
		t.Errorf("Origin = %v after Free, want 0", got)
	}
	rm := pdf.NewResourceManager(w)
	got, err := rm.Embed(info)
	if err != nil {
		t.Fatal(err)
	}
	if got == infoRef {
		t.Error("embedding after Free reused the freed reference")
	}

	// the same must hold on the Store path (an Encoder, unlike the
	// Embedder above): after Free, Store must allocate a fresh object
	// instead of failing to write at the freed reference
	rootRef := w.GetMeta().Trailer["Root"].(pdf.Reference)
	if err := w.Free(rootRef); err != nil {
		t.Fatal(err)
	}
	if got := w.Origin(w.GetMeta().Catalog); got != 0 {
		t.Errorf("Catalog Origin = %v after Free, want 0", got)
	}
	catRef, err := rm.Store(w.GetMeta().Catalog)
	if err != nil {
		t.Fatal(err)
	}
	if catRef == rootRef {
		t.Error("Store after Free reused the freed reference")
	}
}

// simpleEncoder is a minimal Encoder for tests that do not need
// self-referencing behaviour.
type simpleEncoder struct{ Label pdf.Name }

func (s *simpleEncoder) Encode(rm *pdf.ResourceManager) (pdf.Native, error) {
	return pdf.Dict{"Label": s.Label}, nil
}

// TestGetReferenceAcrossManagers regresses a bug where a reservation made by
// one ResourceManager on a Writer was not honoured by Store on a different
// ResourceManager on the same Writer: the value would embed as the reserved
// reference without the object ever being written, and the reserving
// manager's Close would report the reservation as dangling.
func TestGetReferenceAcrossManagers(t *testing.T) {
	w, f := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	rm1 := pdf.NewResourceManager(w)
	rm2 := pdf.NewResourceManager(w)

	v := &simpleEncoder{Label: "cross-manager"}
	ref1 := rm1.GetReference(v)

	ref2, err := rm2.Store(v)
	if err != nil {
		t.Fatal(err)
	}
	if ref2 != ref1 {
		t.Errorf("Store wrote at %v, want the reserved reference %v", ref2, ref1)
	}

	if err := rm1.Close(); err != nil {
		t.Fatal(err)
	}
	if err := rm2.Close(); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	r, err := pdf.NewReader(bytes.NewReader(f.Data), int64(len(f.Data)), nil)
	if err != nil {
		t.Fatal(err)
	}
	dict, err := pdf.NewCursor(r).Dict(ref1)
	if err != nil {
		t.Fatal(err)
	}
	if dict["Label"] != pdf.Name("cross-manager") {
		t.Errorf("Label = %v, want cross-manager", dict["Label"])
	}
}
