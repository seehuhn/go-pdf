// Copyright 2024 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package pdf_test

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font/gofont"
	"seehuhn.de/go/pdf/internal/debug/memfile"
	"seehuhn.de/go/pdf/internal/debug/mock"
	"seehuhn.de/go/pdf/pagetree"
)

// TestResourceManager creates a document which uses a font on two different
// pages, and checks that the font is only embedded once.
func TestResourceManager(t *testing.T) {
	buf := memfile.New()

	// part 1: create a document with a font

	doc, err := document.WriteMultiPage(buf, document.A4, pdf.V2_0, nil)
	if err != nil {
		t.Fatal(err)
	}

	F, err := gofont.Regular.NewSimple(nil)
	if err != nil {
		t.Fatal(err)
	}

	page1 := doc.AddPage()
	page1.SetFontNameInternal(F, "X")
	page1.TextBegin()
	page1.TextSetFont(F, 12)
	page1.TextFirstLine(100, 600)
	page1.TextShow("Hello, world!")
	page1.TextEnd()
	err = page1.Close()
	if err != nil {
		t.Fatal(err)
	}

	page2 := doc.AddPage()
	page2.SetFontNameInternal(F, "X")
	page2.TextBegin()
	page2.TextSetFont(F, 12)
	page2.TextFirstLine(100, 600)
	page2.TextShow("second page")
	page2.TextEnd()
	err = page2.Close()
	if err != nil {
		t.Fatal(err)
	}

	err = doc.Close()
	if err != nil {
		t.Fatal(err)
	}

	// part 2: read the document and check that the font is only embedded once

	r, err := pdf.NewReader(buf, int64(len(buf.Data)), nil)
	if err != nil {
		t.Fatal(err)
	}

	_, dict1, err := pagetree.GetPage(r, 0)
	if err != nil {
		t.Fatal(err)
	}
	resources1, err := pdf.NewCursor(r).Dict(dict1["Resources"])
	if err != nil {
		t.Fatal(err)
	}
	font1, err := pdf.NewCursor(r).Dict(resources1["Font"])
	if err != nil {
		t.Fatal(err)
	}

	_, dict2, err := pagetree.GetPage(r, 1)
	if err != nil {
		t.Fatal(err)
	}
	resources2, err := pdf.NewCursor(r).Dict(dict2["Resources"])
	if err != nil {
		t.Fatal(err)
	}
	font2, err := pdf.NewCursor(r).Dict(resources2["Font"])
	if err != nil {
		t.Fatal(err)
	}

	if font1["X"] != font2["X"] {
		t.Errorf("%s != %s", pdf.AsString(font1), pdf.AsString(font2))
	}
}

// TestExtractorResolveCycle tests that Extractor.Resolve detects reference cycles.
func TestExtractorResolveCycle(t *testing.T) {
	// create a PDF with a cycle: A -> B -> A
	w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)

	refA := w.Alloc()
	refB := w.Alloc()

	err := w.Put(refA, refB)
	if err != nil {
		t.Fatal(err)
	}

	err = w.Put(refB, refA)
	if err != nil {
		t.Fatal(err)
	}

	err = w.Close()
	if err != nil {
		t.Fatal(err)
	}

	// use the writer as a getter and try to resolve the cycle
	x := pdf.NewExtractor(w)
	_, err = pdf.CursorAt(x, nil).Resolve(refA)
	if err == nil {
		t.Fatal("expected cycle error, got nil")
	}
	if !errors.Is(err, pdf.ErrCycle) {
		t.Errorf("expected cycle error, got %v", err)
	}
}

// TestCursorArrayCycle tests that Cursor.Array detects cycles.
func TestCursorArrayCycle(t *testing.T) {
	// create a PDF with a cycle in an array
	w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)

	refA := w.Alloc()
	err := w.Put(refA, pdf.Array{pdf.Integer(1), refA})
	if err != nil {
		t.Fatal(err)
	}

	err = w.Close()
	if err != nil {
		t.Fatal(err)
	}

	// use the writer as a getter and try to resolve
	x := pdf.NewExtractor(w)

	extract := func(c pdf.Cursor, obj pdf.Object, _ bool) (pdf.Array, error) {
		arr, err := c.Array(obj.AsPDF(0))
		if err != nil {
			return nil, err
		}
		for i := range arr {
			arr[i], err = c.Resolve(arr[i])
			if err != nil {
				return nil, err
			}
		}
		return arr, nil
	}

	_, err = pdf.CursorAt(x, nil).Decode(refA, extract)
	if !errors.Is(err, pdf.ErrCycle) {
		t.Errorf("expected cycle error, got %v", err)
	}
}

// TestEmbedFuncCopier checks that an embed function passed to
// [pdf.ResourceManager.EmbedFunc] can copy objects from another file.
func TestEmbedFuncCopier(t *testing.T) {
	src, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
	ref := src.Alloc()
	if err := src.Put(ref, pdf.Dict{"A": pdf.Integer(1)}); err != nil {
		t.Fatal(err)
	}
	x := pdf.NewExtractor(src)

	dst, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
	rm := pdf.NewResourceManager(dst)
	embed := func(e *pdf.EmbedHelper, r pdf.Reference) (pdf.Native, error) {
		return e.CopierFrom(x).Copy(r)
	}
	got, err := rm.EmbedFunc(embed, ref)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := got.(pdf.Reference); !ok {
		t.Errorf("expected a reference, got %T", got)
	}
	if err := rm.Close(); err != nil {
		t.Fatal(err)
	}
}

// TestDeferCopier checks that a function registered with
// [pdf.EmbedHelper.Defer] can use [pdf.EmbedHelper.CopierFrom].
func TestDeferCopier(t *testing.T) {
	src, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
	ref := src.Alloc()
	if err := src.Put(ref, pdf.Dict{"A": pdf.Integer(1)}); err != nil {
		t.Fatal(err)
	}
	x := pdf.NewExtractor(src)

	dst, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
	rm := pdf.NewResourceManager(dst)
	var got pdf.Native
	embed := func(e *pdf.EmbedHelper, r pdf.Reference) (pdf.Native, error) {
		e.Defer(func(e *pdf.EmbedHelper) error {
			var err error
			got, err = e.CopierFrom(x).Copy(r)
			return err
		})
		return pdf.Integer(0), nil
	}
	if _, err := rm.EmbedFunc(embed, ref); err != nil {
		t.Fatal(err)
	}
	if err := rm.Close(); err != nil {
		t.Fatal(err)
	}
	if _, ok := got.(pdf.Reference); !ok {
		t.Errorf("expected a reference, got %T", got)
	}
}

// TestExtractorOnce checks that [pdf.Extractor.Once] runs its function once per
// result type, shares the result between concurrent callers, and does not
// cache a failure.
func TestExtractorOnce(t *testing.T) {
	type docValue struct{ n int }
	type otherValue struct{ n int }

	x := pdf.NewExtractor(mock.Getter)

	var calls atomic.Int32
	run := func(pdf.Cursor) (*docValue, error) {
		n := calls.Add(1)
		return &docValue{n: int(n)}, nil
	}

	var wg sync.WaitGroup
	results := make([]*docValue, 8)
	for i := range results {
		wg.Go(func() {
			v, err := x.Once(run)
			if err != nil {
				t.Error(err)
			}
			results[i] = v
		})
	}
	wg.Wait()
	if n := calls.Load(); n != 1 {
		t.Errorf("function ran %d times, want 1", n)
	}
	for i, v := range results {
		if v != results[0] {
			t.Errorf("caller %d got a different value", i)
		}
	}

	// another type is another value
	other, err := x.Once(func(pdf.Cursor) (otherValue, error) {
		return otherValue{n: 7}, nil
	})
	if err != nil || other.n != 7 {
		t.Errorf("other type: %v, %v", other, err)
	}

	// a failure is not cached
	if _, err := x.Once(func(pdf.Cursor) (int, error) {
		return 0, errors.New("fail")
	}); err == nil {
		t.Fatal("missing error")
	}
	v, err := x.Once(func(pdf.Cursor) (int, error) { return 1, nil })
	if err != nil || v != 1 {
		t.Errorf("after a failure: got %d, %v, want 1", v, err)
	}
	v, err = x.Once(func(pdf.Cursor) (int, error) { return 2, nil })
	if err != nil || v != 1 {
		t.Errorf("cached value: got %d, %v, want 1", v, err)
	}
}

// TestStoreOrLoad checks that a published value is what a later decode of the
// same reference returns, and that a second publication adopts the first.
func TestStoreOrLoad(t *testing.T) {
	x := pdf.NewExtractor(mock.Getter)
	ref := pdf.NewReference(1, 0)
	first, second := new(int), new(int)

	if got := x.StoreOrLoad(ref, first); got != first {
		t.Error("first publication not stored")
	}
	if got := x.StoreOrLoad(ref, second); got != first {
		t.Error("second publication did not adopt the first")
	}

	got, err := pdf.CursorAt(x, nil).Decode(ref, func(pdf.Cursor, pdf.Object, bool) (*int, error) {
		t.Error("decode function called for a published reference")
		return nil, nil
	})
	if err != nil || got != first {
		t.Errorf("decode returned %v, %v, want the published value", got, err)
	}
}

// TestStoreOrLoadChain checks that a value published under a reference which
// leads to an object through a chain is found by decoding the object.
func TestStoreOrLoadChain(t *testing.T) {
	w, obj, a, b := writeChain(t)
	x := pdf.NewExtractor(w)
	noDecode := func(pdf.Cursor, pdf.Object, bool) (*int, error) {
		t.Error("decode function called for a published object")
		return nil, nil
	}

	v := new(int)
	if got := x.StoreOrLoad(b, v); got != v {
		t.Error("first publication not stored")
	}
	if got := x.StoreOrLoad(a, new(int)); got != v {
		t.Error("publication via another reference did not adopt the first")
	}
	for _, ref := range []pdf.Reference{obj, a, b} {
		got, err := pdf.CursorAt(x, nil).Decode(ref, noDecode)
		if err != nil || got != v {
			t.Errorf("decode of %s returned %v, %v, want the published value", ref, got, err)
		}
	}

	s := new(string)
	gotV, gotS := x.StoreOrLoadPair(a, new(int), s)
	if gotV != v || gotS != s {
		t.Error("pair publication did not adopt the existing value")
	}
	gotS, err := pdf.CursorAt(x, nil).Decode(obj, func(pdf.Cursor, pdf.Object, bool) (*string, error) {
		t.Error("decode function called for a published object")
		return nil, nil
	})
	if err != nil || gotS != s {
		t.Errorf("decode returned %v, %v, want the published value", gotS, err)
	}
}

// TestStoreOrLoadCycle checks that a reference which cannot be resolved is
// not published.
func TestStoreOrLoadCycle(t *testing.T) {
	w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
	refA, refB := w.Alloc(), w.Alloc()
	if err := w.Put(refA, refB); err != nil {
		t.Fatal(err)
	}
	if err := w.Put(refB, refA); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	x := pdf.NewExtractor(w)

	v := new(int)
	if got := x.StoreOrLoad(refA, v); got != v {
		t.Error("value not returned")
	}
	if got := x.StoreOrLoad(refA, new(int)); got == v {
		t.Error("value was published under a reference cycle")
	}
}
