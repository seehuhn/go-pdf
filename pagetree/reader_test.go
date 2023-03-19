package pagetree_test

import (
	"bytes"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/pagetree"
)

func TestReader(t *testing.T) {
	buf := &bytes.Buffer{}
	w, err := document.WriteMultiPage(buf, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 300; i++ {
		page := w.AddPage()
		page.PageDict["Test"] = pdf.Integer(99 - 2*i)
		err = page.Close()
		if err != nil {
			t.Fatal(err)
		}
	}
	err = w.Close()
	if err != nil {
		t.Fatal(err)
	}

	r, err := pdf.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()), nil)
	if err != nil {
		t.Fatal(err)
	}
	pages, err := pagetree.NewReader(r)
	if err != nil {
		t.Fatal(err)
	}
	n, err := pages.NumPages()
	if err != nil {
		t.Fatal(err)
	}
	if n != 300 {
		t.Fatalf("expected 300 pages, got %d", n)
	}
	for i := 0; i < 300; i++ {
		page, err := pages.Get(pdf.Integer(i))
		if err != nil {
			t.Fatal(err)
		}
		v, err := r.GetInt(page["Test"])
		if err != nil {
			t.Fatal(err)
		}
		if v != pdf.Integer(99-2*i) {
			t.Fatalf("expected %d, got %d", 99-2*i, v)
		}
	}
}
