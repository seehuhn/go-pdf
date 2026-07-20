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

package printprep

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
	"seehuhn.de/go/pdf/pagetree"
)

// makeLayeredSource builds a one-page document with three drawings: a visible
// blue rectangle, a red rectangle inside an optional-content region, and a
// green rectangle inside an Artifact marked-content region.  It returns a
// reader and the reference of the optional-content group.
func makeLayeredSource(t *testing.T) (*pdf.Reader, pdf.Reference) {
	t.Helper()
	w, buf := memfile.NewPDFWriter(pdf.V1_7, nil)

	ocgRef := w.Alloc()
	if err := w.Put(ocgRef, pdf.Dict{"Type": pdf.Name("OCG"), "Name": pdf.String("Layer 1")}); err != nil {
		t.Fatal(err)
	}

	const body = "q 0 0 1 rg 10 10 30 30 re f Q\n" +
		"/OC /oc0 BDC\n" +
		"q 1 0 0 rg 60 60 30 30 re f Q\n" +
		"EMC\n" +
		"/Artifact BMC\n" +
		"q 0 1 0 rg 10 60 30 30 re f Q\n" +
		"EMC\n"
	contentRef := w.Alloc()
	stm, err := w.OpenStream(contentRef, pdf.Dict{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(stm, body); err != nil {
		t.Fatal(err)
	}
	if err := stm.Close(); err != nil {
		t.Fatal(err)
	}

	pageRef := w.Alloc()
	pagesRef := w.Alloc()
	page := pdf.Dict{
		"Type":      pdf.Name("Page"),
		"Parent":    pagesRef,
		"MediaBox":  pdf.Array{pdf.Integer(0), pdf.Integer(0), pdf.Integer(200), pdf.Integer(200)},
		"Contents":  contentRef,
		"Resources": pdf.Dict{"Properties": pdf.Dict{"oc0": ocgRef}},
	}
	if err := w.Put(pageRef, page); err != nil {
		t.Fatal(err)
	}
	if err := w.Put(pagesRef, pdf.Dict{
		"Type":  pdf.Name("Pages"),
		"Kids":  pdf.Array{pageRef},
		"Count": pdf.Integer(1),
	}); err != nil {
		t.Fatal(err)
	}

	w.GetMeta().Catalog.Pages = pagesRef
	w.GetMeta().Catalog.OCProperties = pdf.Dict{
		"OCGs": pdf.Array{ocgRef},
		"D":    pdf.Dict{},
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	r, err := pdf.NewReader(buf, int64(len(buf.Data)), nil)
	if err != nil {
		t.Fatal(err)
	}
	return r, ocgRef
}

// pageContent returns the decoded content stream of the first page.
func pageContent(t *testing.T, data []byte) string {
	t.Helper()
	rr, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)), nil)
	if err != nil {
		t.Fatal(err)
	}
	_, dict, err := pagetree.GetPage(rr, 0)
	if err != nil {
		t.Fatal(err)
	}
	rc, err := pdf.NewCursor(rr).StreamReader(dict["Contents"])
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close()
	raw, err := io.ReadAll(rc)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestStripMarkedContentOnly(t *testing.T) {
	r, _ := makeLayeredSource(t)

	var out bytes.Buffer
	if err := Write(&out, r, nil); err != nil { // no hidden layers
		t.Fatal(err)
	}
	got := pageContent(t, out.Bytes())

	for _, op := range []string{"BDC", "BMC", "EMC"} {
		if strings.Contains(got, op) {
			t.Errorf("marked-content operator %q not stripped:\n%s", op, got)
		}
	}
	// with no hidden layers, all three drawings survive
	for _, marks := range []string{"10 10 30 30 re", "60 60 30 30 re", "10 60 30 30 re"} {
		if !strings.Contains(got, marks) {
			t.Errorf("drawing %q missing:\n%s", marks, got)
		}
	}
}

// formContent returns the decoded content of the named form XObject on the
// first page of the document.
func formContent(t *testing.T, data []byte, name pdf.Name) string {
	t.Helper()
	rr, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)), nil)
	if err != nil {
		t.Fatal(err)
	}
	cur := pdf.NewCursor(rr)
	_, page, err := pagetree.GetPage(rr, 0)
	if err != nil {
		t.Fatal(err)
	}
	res, err := cur.Dict(page["Resources"])
	if err != nil {
		t.Fatal(err)
	}
	xobjs, err := cur.Dict(res["XObject"])
	if err != nil {
		t.Fatal(err)
	}
	rc, err := cur.StreamReader(xobjs[name])
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close()
	raw, err := io.ReadAll(rc)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestFormRecursion(t *testing.T) {
	w, buf := memfile.NewPDFWriter(pdf.V1_7, nil)

	formRef := w.Alloc()
	fstm, err := w.OpenStream(formRef, pdf.Dict{
		"Type":         pdf.Name("XObject"),
		"Subtype":      pdf.Name("Form"),
		"BBox":         pdf.Array{pdf.Integer(0), pdf.Integer(0), pdf.Integer(100), pdf.Integer(100)},
		"StructParent": pdf.Integer(3), // must be dropped
	})
	if err != nil {
		t.Fatal(err)
	}
	io.WriteString(fstm, "/Artifact BMC\nq 1 0 0 rg 5 5 20 20 re f Q\nEMC\n")
	if err := fstm.Close(); err != nil {
		t.Fatal(err)
	}

	contentRef := w.Alloc()
	cstm, err := w.OpenStream(contentRef, pdf.Dict{})
	if err != nil {
		t.Fatal(err)
	}
	io.WriteString(cstm, "q /Fm0 Do Q\n")
	cstm.Close()

	pageRef := w.Alloc()
	pagesRef := w.Alloc()
	w.Put(pageRef, pdf.Dict{
		"Type":      pdf.Name("Page"),
		"Parent":    pagesRef,
		"MediaBox":  pdf.Array{pdf.Integer(0), pdf.Integer(0), pdf.Integer(100), pdf.Integer(100)},
		"Contents":  contentRef,
		"Resources": pdf.Dict{"XObject": pdf.Dict{"Fm0": formRef}},
	})
	w.Put(pagesRef, pdf.Dict{"Type": pdf.Name("Pages"), "Kids": pdf.Array{pageRef}, "Count": pdf.Integer(1)})
	w.GetMeta().Catalog.Pages = pagesRef
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	r, err := pdf.NewReader(buf, int64(len(buf.Data)), nil)
	if err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	if err := Write(&out, r, nil); err != nil {
		t.Fatal(err)
	}

	got := formContent(t, out.Bytes(), "Fm0")
	if strings.Contains(got, "BMC") || strings.Contains(got, "EMC") {
		t.Errorf("marked content not stripped inside form:\n%s", got)
	}
	if !strings.Contains(got, "5 5 20 20 re") {
		t.Errorf("form drawing missing:\n%s", got)
	}
}

func TestStripHiddenLayer(t *testing.T) {
	r, ocgRef := makeLayeredSource(t)

	var out bytes.Buffer
	if err := Write(&out, r, &Options{HiddenLayers: []pdf.Reference{ocgRef}}); err != nil {
		t.Fatal(err)
	}
	got := pageContent(t, out.Bytes())

	if strings.Contains(got, "BDC") || strings.Contains(got, "EMC") || strings.Contains(got, "BMC") {
		t.Errorf("marked-content operators not stripped:\n%s", got)
	}
	// the optional-content drawing is gone
	if strings.Contains(got, "60 60 30 30 re") {
		t.Errorf("hidden-layer drawing survived:\n%s", got)
	}
	// the visible and artifact drawings remain
	if !strings.Contains(got, "10 10 30 30 re") {
		t.Errorf("visible drawing missing:\n%s", got)
	}
	if !strings.Contains(got, "10 60 30 30 re") {
		t.Errorf("artifact drawing missing:\n%s", got)
	}
}
