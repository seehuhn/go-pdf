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
	"math"
	"strings"
	"testing"

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/internal/debug/memfile"
	"seehuhn.de/go/pdf/pagetree"
)

// apForm writes a simple form-XObject appearance stream and returns its ref.
func apForm(t *testing.T, w *pdf.Writer, body string) pdf.Reference {
	t.Helper()
	ref := w.Alloc()
	stm, err := w.OpenStream(ref, pdf.Dict{
		"Type":    pdf.Name("XObject"),
		"Subtype": pdf.Name("Form"),
		"BBox":    pdf.Array{pdf.Integer(0), pdf.Integer(0), pdf.Integer(40), pdf.Integer(40)},
	})
	if err != nil {
		t.Fatal(err)
	}
	io.WriteString(stm, body)
	if err := stm.Close(); err != nil {
		t.Fatal(err)
	}
	return ref
}

// TestFlattenNormalizesSourceAppearance checks that a source appearance stream
// is routed through the same content normalization as page content: its
// marked-content operators are stripped rather than embedded verbatim.
func TestFlattenNormalizesSourceAppearance(t *testing.T) {
	w, buf := memfile.NewPDFWriter(pdf.V1_7, nil)

	// appearance content wraps its drawing in a marked-content region
	ap := apForm(t, w, "/Artifact BMC\n1 0 0 rg 0 0 40 40 re f\nEMC\n")

	square := w.Alloc()
	w.Put(square, pdf.Dict{
		"Type": pdf.Name("Annot"), "Subtype": pdf.Name("Square"),
		"Rect": pdf.Array{pdf.Integer(10), pdf.Integer(10), pdf.Integer(50), pdf.Integer(50)},
		"F":    pdf.Integer(4), // Print
		"AP":   pdf.Dict{"N": ap},
	})

	contentRef := w.Alloc()
	cstm, _ := w.OpenStream(contentRef, pdf.Dict{})
	io.WriteString(cstm, "q 0 0 1 rg 5 5 10 10 re f Q\n")
	cstm.Close()

	pageRef := w.Alloc()
	pagesRef := w.Alloc()
	w.Put(pageRef, pdf.Dict{
		"Type": pdf.Name("Page"), "Parent": pagesRef,
		"MediaBox": pdf.Array{pdf.Integer(0), pdf.Integer(0), pdf.Integer(200), pdf.Integer(200)},
		"Contents": contentRef,
		"Annots":   pdf.Array{square},
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

	got := formContent(t, out.Bytes(), "PPAnnot0")
	if strings.Contains(got, "BMC") || strings.Contains(got, "EMC") {
		t.Errorf("marked content not stripped from flattened appearance:\n%s", got)
	}
	if !strings.Contains(got, "0 0 40 40 re") {
		t.Errorf("appearance drawing missing from flattened form:\n%s", got)
	}
}

func TestFlattenAnnots(t *testing.T) {
	w, buf := memfile.NewPDFWriter(pdf.V1_7, nil)

	printAP := apForm(t, w, "1 0 0 rg 0 0 40 40 re f\n")
	hiddenAP := apForm(t, w, "0 1 0 rg 0 0 40 40 re f\n")

	printSquare := w.Alloc()
	w.Put(printSquare, pdf.Dict{
		"Type": pdf.Name("Annot"), "Subtype": pdf.Name("Square"),
		"Rect": pdf.Array{pdf.Integer(10), pdf.Integer(10), pdf.Integer(50), pdf.Integer(50)},
		"F":    pdf.Integer(4), // Print
		"AP":   pdf.Dict{"N": printAP},
	})
	link := w.Alloc()
	w.Put(link, pdf.Dict{
		"Type": pdf.Name("Annot"), "Subtype": pdf.Name("Link"),
		"Rect": pdf.Array{pdf.Integer(60), pdf.Integer(60), pdf.Integer(80), pdf.Integer(80)},
		"F":    pdf.Integer(4),
	})
	hiddenSquare := w.Alloc()
	w.Put(hiddenSquare, pdf.Dict{
		"Type": pdf.Name("Annot"), "Subtype": pdf.Name("Square"),
		"Rect": pdf.Array{pdf.Integer(10), pdf.Integer(60), pdf.Integer(50), pdf.Integer(90)},
		"F":    pdf.Integer(2), // Hidden
		"AP":   pdf.Dict{"N": hiddenAP},
	})

	contentRef := w.Alloc()
	cstm, _ := w.OpenStream(contentRef, pdf.Dict{})
	io.WriteString(cstm, "q 0 0 1 rg 5 5 10 10 re f Q\n")
	cstm.Close()

	pageRef := w.Alloc()
	pagesRef := w.Alloc()
	w.Put(pageRef, pdf.Dict{
		"Type": pdf.Name("Page"), "Parent": pagesRef,
		"MediaBox": pdf.Array{pdf.Integer(0), pdf.Integer(0), pdf.Integer(200), pdf.Integer(200)},
		"Contents": contentRef,
		"Annots":   pdf.Array{printSquare, link, hiddenSquare},
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
	res := out.Bytes()
	rr, err := pdf.NewReader(bytes.NewReader(res), int64(len(res)), nil)
	if err != nil {
		t.Fatal(err)
	}
	_, page, err := pagetree.GetPage(rr, 0)
	if err != nil {
		t.Fatal(err)
	}

	// the interactive annotation layer is gone
	if page["Annots"] != nil {
		t.Error("Annots survived on the flattened page")
	}

	// exactly one annotation (the printable square) was flattened into content
	got := pageContent(t, res)
	if n := strings.Count(got, " Do"); n != 1 {
		t.Errorf("want 1 flattened annotation Do, got %d:\n%s", n, got)
	}
	if !strings.Contains(got, "PPAnnot0") {
		t.Errorf("printable square not flattened:\n%s", got)
	}
	// the page's own mark is still there
	if !strings.Contains(got, "5 5 10 10 re") {
		t.Errorf("page content missing:\n%s", got)
	}

	// the flattened form is present in resources
	cur := pdf.NewCursor(rr)
	resDict, _ := cur.Dict(page["Resources"])
	xobjs, _ := cur.Dict(resDict["XObject"])
	if _, ok := xobjs["PPAnnot0"]; !ok {
		t.Error("flattened annotation form not in resources")
	}
}

// TestFlattenSynthesizesFallback checks that a printable annotation without an
// appearance stream has a fallback synthesized and flattened into the page.
// The FreeText fallback shows text, so its synthesized form carries a /Font
// resource: reopening the output confirms that resource was rebuilt in the new
// document rather than left pointing at the source.
func TestFlattenSynthesizesFallback(t *testing.T) {
	w, buf := memfile.NewPDFWriter(pdf.V1_7, nil)

	// a printable FreeText with no /AP: the fallback must synthesize one
	freeText := w.Alloc()
	w.Put(freeText, pdf.Dict{
		"Type": pdf.Name("Annot"), "Subtype": pdf.Name("FreeText"),
		"Rect":     pdf.Array{pdf.Integer(20), pdf.Integer(20), pdf.Integer(160), pdf.Integer(60)},
		"F":        pdf.Integer(4), // Print
		"Contents": pdf.String("hello"),
		"DA":       pdf.String("0 0 0 rg /Helv 12 Tf"),
	})

	contentRef := w.Alloc()
	cstm, _ := w.OpenStream(contentRef, pdf.Dict{})
	io.WriteString(cstm, "q 0 0 1 rg 5 5 10 10 re f Q\n")
	cstm.Close()

	pageRef := w.Alloc()
	pagesRef := w.Alloc()
	w.Put(pageRef, pdf.Dict{
		"Type": pdf.Name("Page"), "Parent": pagesRef,
		"MediaBox": pdf.Array{pdf.Integer(0), pdf.Integer(0), pdf.Integer(200), pdf.Integer(200)},
		"Contents": contentRef,
		"Annots":   pdf.Array{freeText},
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
	res := out.Bytes()

	// property 1: the synthesized appearance was flattened into page content
	got := pageContent(t, res)
	if n := strings.Count(got, " Do"); n != 1 {
		t.Errorf("want 1 flattened annotation Do, got %d:\n%s", n, got)
	}

	// property 2: the flattened form, and the /Font resource its content needs,
	// resolve entirely within the output; a dangling source reference would
	// fail one of these lookups
	rr, err := pdf.NewReader(bytes.NewReader(res), int64(len(res)), nil)
	if err != nil {
		t.Fatal(err)
	}
	_, page, err := pagetree.GetPage(rr, 0)
	if err != nil {
		t.Fatal(err)
	}
	cur := pdf.NewCursor(rr)
	resDict, _ := cur.Dict(page["Resources"])
	xobjs, err := cur.Dict(resDict["XObject"])
	if err != nil {
		t.Fatal(err)
	}
	formStm, err := cur.Stream(xobjs["PPAnnot0"])
	if err != nil {
		t.Fatalf("flattened form does not resolve in output: %v", err)
	}
	formRes, err := cur.Dict(formStm.Dict["Resources"])
	if err != nil {
		t.Fatalf("flattened form resources do not resolve: %v", err)
	}
	fonts, err := cur.Dict(formRes["Font"])
	if err != nil {
		t.Fatalf("flattened form /Font does not resolve: %v", err)
	}
	if len(fonts) == 0 {
		t.Fatal("synthesized FreeText appearance has no font resource")
	}
	for name, fref := range fonts {
		if _, err := cur.Dict(fref); err != nil {
			t.Errorf("font %q in flattened form does not resolve in output: %v", name, err)
		}
	}
}

// TestFlattenAvoidsResourceNameCollision checks that a flattened annotation
// does not shadow a page XObject that already uses the overlay naming scheme.
// The page owns an XObject named /PPAnnot0 (a 20x20 form) that its content
// draws; a printable annotation is flattened on top.  The annotation must get a
// different name, and the page's own /PPAnnot0 must survive unchanged.
func TestFlattenAvoidsResourceNameCollision(t *testing.T) {
	w, buf := memfile.NewPDFWriter(pdf.V1_7, nil)

	pageXObj := apForm(t, w, "0 0 1 rg 0 0 20 20 re f\n") // page's own form, 20x20
	annotAP := apForm(t, w, "1 0 0 rg 0 0 40 40 re f\n")  // annotation form, 40x40

	square := w.Alloc()
	w.Put(square, pdf.Dict{
		"Type": pdf.Name("Annot"), "Subtype": pdf.Name("Square"),
		"Rect": pdf.Array{pdf.Integer(10), pdf.Integer(10), pdf.Integer(50), pdf.Integer(50)},
		"F":    pdf.Integer(4), // Print
		"AP":   pdf.Dict{"N": annotAP},
	})

	contentRef := w.Alloc()
	cstm, _ := w.OpenStream(contentRef, pdf.Dict{})
	io.WriteString(cstm, "q /PPAnnot0 Do Q\n") // the page draws its own PPAnnot0
	cstm.Close()

	pageRef := w.Alloc()
	pagesRef := w.Alloc()
	w.Put(pageRef, pdf.Dict{
		"Type": pdf.Name("Page"), "Parent": pagesRef,
		"MediaBox":  pdf.Array{pdf.Integer(0), pdf.Integer(0), pdf.Integer(200), pdf.Integer(200)},
		"Contents":  contentRef,
		"Resources": pdf.Dict{"XObject": pdf.Dict{"PPAnnot0": pageXObj}},
		"Annots":    pdf.Array{square},
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
	res := out.Bytes()

	// both the page's own draw and the flattened annotation are present, under
	// distinct names
	got := pageContent(t, res)
	if n := strings.Count(got, " Do"); n != 2 {
		t.Errorf("want 2 XObject invocations, got %d:\n%s", n, got)
	}
	if !strings.Contains(got, "/PPAnnot0 Do") {
		t.Errorf("page's own /PPAnnot0 invocation missing:\n%s", got)
	}

	rr, err := pdf.NewReader(bytes.NewReader(res), int64(len(res)), nil)
	if err != nil {
		t.Fatal(err)
	}
	_, page, err := pagetree.GetPage(rr, 0)
	if err != nil {
		t.Fatal(err)
	}
	cur := pdf.NewCursor(rr)
	resDict, _ := cur.Dict(page["Resources"])
	xobjs, _ := cur.Dict(resDict["XObject"])
	if len(xobjs) != 2 {
		t.Fatalf("want 2 XObjects (page form + flattened annotation), got %d", len(xobjs))
	}

	// the page's own PPAnnot0 was not overwritten by the annotation: its stream
	// still draws the page form's 20x20 rectangle, not the annotation's 40x40
	rc, err := cur.StreamReader(xobjs["PPAnnot0"])
	if err != nil {
		t.Fatalf("page's own PPAnnot0 XObject missing from output: %v", err)
	}
	form0, err := io.ReadAll(rc)
	rc.Close()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(form0), "0 0 20 20 re") {
		t.Errorf("page's own PPAnnot0 was overwritten by the annotation:\n%s", form0)
	}
}

// TestFlattenIsolatesGraphicsState checks that graphics state left over from
// the page marks does not leak into the flattened annotation overlay.  The
// page content leaves a 2x CTM active with no closing "Q"; the appearance box
// matches the annotation rectangle, so the appearance-to-rect matrix is unit
// scale.  If the page's CTM leaked, the annotation would be drawn at 2x.
func TestFlattenIsolatesGraphicsState(t *testing.T) {
	w, buf := memfile.NewPDFWriter(pdf.V1_7, nil)

	ap := apForm(t, w, "1 0 0 rg 0 0 40 40 re f\n") // 40x40 bbox

	square := w.Alloc()
	w.Put(square, pdf.Dict{
		"Type": pdf.Name("Annot"), "Subtype": pdf.Name("Square"),
		"Rect": pdf.Array{pdf.Integer(10), pdf.Integer(10), pdf.Integer(50), pdf.Integer(50)}, // 40x40
		"F":    pdf.Integer(4),                                                                // Print
		"AP":   pdf.Dict{"N": ap},
	})

	// page content installs a 2x CTM and never restores it
	contentRef := w.Alloc()
	cstm, _ := w.OpenStream(contentRef, pdf.Dict{})
	io.WriteString(cstm, "2 0 0 2 0 0 cm 0 0 1 rg 5 5 10 10 re f\n")
	cstm.Close()

	pageRef := w.Alloc()
	pagesRef := w.Alloc()
	w.Put(pageRef, pdf.Dict{
		"Type": pdf.Name("Page"), "Parent": pagesRef,
		"MediaBox": pdf.Array{pdf.Integer(0), pdf.Integer(0), pdf.Integer(200), pdf.Integer(200)},
		"Contents": contentRef,
		"Annots":   pdf.Array{square},
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

	ctm := ctmAtFirstXObject(t, out.Bytes())
	if math.Abs(ctm[0]-1) > 1e-6 || math.Abs(ctm[3]-1) > 1e-6 {
		t.Errorf("annotation drawn under leaked CTM %v; want unit scale", ctm)
	}
}

// ctmAtFirstXObject returns the current transformation matrix in effect at the
// first XObject invocation ("Do") in the first page's content stream.
func ctmAtFirstXObject(t *testing.T, data []byte) matrix.Matrix {
	t.Helper()
	rr, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)), nil)
	if err != nil {
		t.Fatal(err)
	}
	_, page, err := pagetree.GetPage(rr, 0)
	if err != nil {
		t.Fatal(err)
	}
	rc, err := pdf.NewCursor(rr).StreamReader(page["Contents"])
	if err != nil {
		t.Fatal(err)
	}
	raw, err := io.ReadAll(rc)
	rc.Close()
	if err != nil {
		t.Fatal(err)
	}

	ctm := matrix.Identity
	var stack []matrix.Matrix
	open := func() (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(raw)), nil }
	for name, args := range content.NewScanner(open).NewIter().All() {
		switch name {
		case content.OpPushGraphicsState:
			stack = append(stack, ctm)
		case content.OpPopGraphicsState:
			if n := len(stack); n > 0 {
				ctm = stack[n-1]
				stack = stack[:n-1]
			}
		case content.OpTransform:
			if len(args) == 6 {
				var m matrix.Matrix
				for i := range m {
					m[i] = asFloat(args[i])
				}
				ctm = m.Mul(ctm)
			}
		case content.OpXObject:
			return ctm
		}
	}
	t.Fatal("no XObject Do operator found")
	return ctm
}

func asFloat(o pdf.Object) float64 {
	switch v := o.(type) {
	case pdf.Real:
		return float64(v)
	case pdf.Integer:
		return float64(v)
	}
	return 0
}

// TestFlattenMergedFieldValue checks that a form field merged with its single
// widget keeps its field context while being flattened.  Such a dictionary is
// both the annotation and the field, so the annotation decoder has to recover
// the reference it was called for to link the two; a widget that reaches the
// fallback generator with no field attached draws only empty chrome, silently
// dropping the field value from the printed page.
func TestFlattenMergedFieldValue(t *testing.T) {
	w, buf := memfile.NewPDFWriter(pdf.V1_7, nil)

	// one object that is both a text field and its widget, with a value but no
	// appearance stream
	mergedRef := w.Alloc()
	w.Put(mergedRef, pdf.Dict{
		"Type": pdf.Name("Annot"), "Subtype": pdf.Name("Widget"),
		"Rect": pdf.Array{pdf.Integer(20), pdf.Integer(20), pdf.Integer(160), pdf.Integer(60)},
		"F":    pdf.Integer(4), // Print
		"FT":   pdf.Name("Tx"),
		"T":    pdf.TextString("merged"),
		"V":    pdf.TextString("hello"),
		"DA":   pdf.String("0 0 0 rg /Helv 12 Tf"),
	})

	formRef := w.Alloc()
	w.Put(formRef, pdf.Dict{"Fields": pdf.Array{mergedRef}})

	contentRef := w.Alloc()
	cstm, _ := w.OpenStream(contentRef, pdf.Dict{})
	io.WriteString(cstm, "q 0 0 1 rg 5 5 10 10 re f Q\n")
	cstm.Close()

	pageRef := w.Alloc()
	pagesRef := w.Alloc()
	w.Put(pageRef, pdf.Dict{
		"Type": pdf.Name("Page"), "Parent": pagesRef,
		"MediaBox": pdf.Array{pdf.Integer(0), pdf.Integer(0), pdf.Integer(200), pdf.Integer(200)},
		"Contents": contentRef,
		"Annots":   pdf.Array{mergedRef},
	})
	w.Put(pagesRef, pdf.Dict{"Type": pdf.Name("Pages"), "Kids": pdf.Array{pageRef}, "Count": pdf.Integer(1)})
	w.GetMeta().Catalog.Pages = pagesRef
	w.GetMeta().Catalog.AcroForm = formRef
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
	res := out.Bytes()

	rr, err := pdf.NewReader(bytes.NewReader(res), int64(len(res)), nil)
	if err != nil {
		t.Fatal(err)
	}
	_, page, err := pagetree.GetPage(rr, 0)
	if err != nil {
		t.Fatal(err)
	}
	cur := pdf.NewCursor(rr)
	resDict, _ := cur.Dict(page["Resources"])
	xobjs, err := cur.Dict(resDict["XObject"])
	if err != nil {
		t.Fatal(err)
	}
	body, err := cur.ReadAll(xobjs["PPAnnot0"], 1<<20)
	if err != nil {
		t.Fatalf("flattened form does not resolve in output: %v", err)
	}

	// the field value is drawn, so the flattened form shows text
	if !strings.Contains(string(body), "Tj") && !strings.Contains(string(body), "TJ") {
		t.Errorf("flattened merged field draws no text, so the value was lost:\n%s", body)
	}
}

// TestFlattenDropsReplies checks that an annotation replying to another one is
// not flattened into the print content.  §12.5.6.2 has replies shown as part
// of their parent's thread rather than individually, so a reply's appearance
// is drawn nowhere on screen and must not appear on paper either.  The "Group"
// relationship is not a reply and is flattened as usual.
func TestFlattenDropsReplies(t *testing.T) {
	w, buf := memfile.NewPDFWriter(pdf.V1_7, nil)

	ap := apForm(t, w, "1 0 0 rg 0 0 40 40 re f\n")
	rect := pdf.Array{pdf.Integer(10), pdf.Integer(10), pdf.Integer(50), pdf.Integer(50)}

	parent := w.Alloc()
	w.Put(parent, pdf.Dict{
		"Type": pdf.Name("Annot"), "Subtype": pdf.Name("Square"),
		"Rect": rect, "F": pdf.Integer(4), "AP": pdf.Dict{"N": ap},
	})
	reply := w.Alloc()
	w.Put(reply, pdf.Dict{
		"Type": pdf.Name("Annot"), "Subtype": pdf.Name("Square"),
		"Rect": rect, "F": pdf.Integer(4), "AP": pdf.Dict{"N": ap},
		"IRT": parent, "RT": pdf.Name("R"),
	})
	// a reply which leaves out /RT: "R" is the default, so it is one too
	bareReply := w.Alloc()
	w.Put(bareReply, pdf.Dict{
		"Type": pdf.Name("Annot"), "Subtype": pdf.Name("Square"),
		"Rect": rect, "F": pdf.Integer(4), "AP": pdf.Dict{"N": ap},
		"IRT": parent,
	})
	grouped := w.Alloc()
	w.Put(grouped, pdf.Dict{
		"Type": pdf.Name("Annot"), "Subtype": pdf.Name("Square"),
		"Rect": rect, "F": pdf.Integer(4), "AP": pdf.Dict{"N": ap},
		"IRT": parent, "RT": pdf.Name("Group"),
	})

	pageRef := w.Alloc()
	pagesRef := w.Alloc()
	w.Put(pageRef, pdf.Dict{
		"Type": pdf.Name("Page"), "Parent": pagesRef,
		"MediaBox": pdf.Array{pdf.Integer(0), pdf.Integer(0), pdf.Integer(200), pdf.Integer(200)},
		"Annots":   pdf.Array{parent, reply, bareReply, grouped},
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

	// the parent and the group member are flattened, the two replies are not
	got := pageContent(t, out.Bytes())
	if n := strings.Count(got, " Do"); n != 2 {
		t.Errorf("want 2 flattened annotations (parent and group member), got %d:\n%s", n, got)
	}
}

// TestFlattenSplitFieldValue checks that a text field whose value lives in a
// separate field dictionary (the widget as a /Kids entry, /V on the field) is
// flattened with its value drawn.  The synthesized appearance reads the value
// through the widget's form field, which reading the page's annotations links
// up; a widget decoded on its own would print an empty box.
func TestFlattenSplitFieldValue(t *testing.T) {
	w, buf := memfile.NewPDFWriter(pdf.V1_7, nil)

	fieldRef := w.Alloc()
	widgetRef := w.Alloc()
	w.Put(fieldRef, pdf.Dict{
		"FT": pdf.Name("Tx"), "T": pdf.TextString("split"),
		"V": pdf.TextString("hello"), "DA": pdf.String("0 0 0 rg /Helv 12 Tf"),
		"Kids": pdf.Array{widgetRef},
	})
	w.Put(widgetRef, pdf.Dict{
		"Type": pdf.Name("Annot"), "Subtype": pdf.Name("Widget"),
		"Rect":   pdf.Array{pdf.Integer(20), pdf.Integer(20), pdf.Integer(160), pdf.Integer(60)},
		"F":      pdf.Integer(4), // Print
		"Parent": fieldRef,
	})
	formRef := w.Alloc()
	w.Put(formRef, pdf.Dict{"Fields": pdf.Array{fieldRef}})

	pageRef := w.Alloc()
	pagesRef := w.Alloc()
	w.Put(pageRef, pdf.Dict{
		"Type": pdf.Name("Page"), "Parent": pagesRef,
		"MediaBox": pdf.Array{pdf.Integer(0), pdf.Integer(0), pdf.Integer(200), pdf.Integer(200)},
		"Annots":   pdf.Array{widgetRef},
	})
	w.Put(pagesRef, pdf.Dict{"Type": pdf.Name("Pages"), "Kids": pdf.Array{pageRef}, "Count": pdf.Integer(1)})
	w.GetMeta().Catalog.Pages = pagesRef
	w.GetMeta().Catalog.AcroForm = formRef
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
	res := out.Bytes()

	rr, err := pdf.NewReader(bytes.NewReader(res), int64(len(res)), nil)
	if err != nil {
		t.Fatal(err)
	}
	_, page, err := pagetree.GetPage(rr, 0)
	if err != nil {
		t.Fatal(err)
	}
	cur := pdf.NewCursor(rr)
	resDict, _ := cur.Dict(page["Resources"])
	xobjs, err := cur.Dict(resDict["XObject"])
	if err != nil {
		t.Fatal(err)
	}
	body, err := cur.ReadAll(xobjs["PPAnnot0"], 1<<20)
	if err != nil {
		t.Fatalf("flattened widget does not resolve in output: %v", err)
	}

	// the field value is drawn, so the flattened form shows text
	if !strings.Contains(string(body), "Tj") && !strings.Contains(string(body), "TJ") {
		t.Errorf("flattened split field draws no text, so the value was lost:\n%s", body)
	}
}

// TestFlattenDanglingReply checks that an annotation whose IRT entry names no
// annotation on the page is flattened as an ordinary one.  Reading the page's
// annotations clears such an entry (table 172 requires the target on the same
// page), the same repair a screen render sees, so the printed page agrees
// with the screen instead of dropping the mark over a reference that leads
// nowhere.
func TestFlattenDanglingReply(t *testing.T) {
	w, buf := memfile.NewPDFWriter(pdf.V1_7, nil)

	ap := apForm(t, w, "1 0 0 rg 0 0 40 40 re f\n")
	a := w.Alloc()
	w.Put(a, pdf.Dict{
		"Type": pdf.Name("Annot"), "Subtype": pdf.Name("Square"),
		"Rect": pdf.Array{pdf.Integer(10), pdf.Integer(10), pdf.Integer(50), pdf.Integer(50)},
		"F":    pdf.Integer(4), "AP": pdf.Dict{"N": ap},
		"IRT": pdf.NewReference(999, 0),
	})

	pageRef := w.Alloc()
	pagesRef := w.Alloc()
	w.Put(pageRef, pdf.Dict{
		"Type": pdf.Name("Page"), "Parent": pagesRef,
		"MediaBox": pdf.Array{pdf.Integer(0), pdf.Integer(0), pdf.Integer(200), pdf.Integer(200)},
		"Annots":   pdf.Array{a},
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

	got := pageContent(t, out.Bytes())
	if n := strings.Count(got, " Do"); n != 1 {
		t.Errorf("want the annotation flattened despite the dangling IRT, got %d forms:\n%s", n, got)
	}
}
