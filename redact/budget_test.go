// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package redact

// Tests of the work budget: inputs which are expensive to redact are
// rejected with ErrTooComplex or processed in bounded time.

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/type3"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/content/builder"
)

// checkBoundedWrite redacts in.  The work must be bounded: the write either
// fails with [ErrTooComplex], or it succeeds and the output leaves no
// redaction annotation and nothing under the regions.
func checkBoundedWrite(t *testing.T, in *pdf.Reader) {
	t.Helper()
	var out bytes.Buffer
	err := Write(&out, in, nil)
	if errors.Is(err, ErrTooComplex) {
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	r := openPDF(t, out.Bytes())
	checkNoRedactAnnots(t, r)
	if pc, ok := pageContents(t, r); ok {
		checkNoLeak(t, inputRegions(t, in), pc)
	}
}

// bombImageDoc builds a document whose image has the given size.  The image
// data is the byte string data, which is repeated rep times.
func bombImageDoc(t *testing.T, w, h int, data []byte, rep int) *pdf.Reader {
	t.Helper()
	return nestDoc(t, []pdf.Dict{{"Rect": rect(120, 120, 140, 140)}}, func(b *nestBuilder) []nestPage {
		ref := b.w.Alloc()
		stm, err := b.w.OpenStream(ref, pdf.Dict{
			"Type": pdf.Name("XObject"), "Subtype": pdf.Name("Image"),
			"Width": pdf.Integer(w), "Height": pdf.Integer(h),
			"ColorSpace": pdf.Name("DeviceGray"), "BitsPerComponent": pdf.Integer(8),
		}, pdf.FilterCompress{})
		if err != nil {
			t.Fatal(err)
		}
		for range rep {
			if _, err := stm.Write(data); err != nil {
				t.Fatal(err)
			}
		}
		if err := stm.Close(); err != nil {
			t.Fatal(err)
		}
		return []nestPage{{
			content: placeAt100,
			res:     pdf.Dict{"XObject": pdf.Dict{"Im1": ref}},
		}}
	})
}

// TestBudgetImageRows checks that an image which compresses to a small
// stream, but has many samples, is rejected.
func TestBudgetImageRows(t *testing.T) {
	const w, h = 8192, 8192
	in := bombImageDoc(t, w, h, make([]byte, w), h)
	if err := Write(io.Discard, in, nil); !errors.Is(err, ErrTooComplex) {
		t.Errorf("got error %v, want ErrTooComplex", err)
	}
}

// TestBudgetImageSize checks that an image with a huge size and a short
// data stream is processed without exhausting memory or time.
func TestBudgetImageSize(t *testing.T) {
	in := bombImageDoc(t, 16384, 8192, []byte{1, 2, 3}, 1)
	checkBoundedWrite(t, in)
}

// TestBudgetImageModerate checks that an image of moderate size is
// not rejected.
func TestBudgetImageModerate(t *testing.T) {
	in := bombImageDoc(t, 512, 512, make([]byte, 512), 512)
	if err := Write(io.Discard, in, nil); err != nil {
		t.Error(err)
	}
}

// manyRegions returns n redaction annotations.
func manyRegions(n int) []pdf.Dict {
	regs := make([]pdf.Dict, n)
	for i := range regs {
		x := i % 250
		regs[i] = pdf.Dict{"Rect": rect(x, 0, x+1, 1)}
	}
	return regs
}

// TestBudgetRegionsTimesGlyphs checks that the work for testing every glyph
// against every region counts towards the budget.
func TestBudgetRegionsTimesGlyphs(t *testing.T) {
	text := "BT /F1 1 Tf 0 100 Td (" + strings.Repeat("a", 200000) + ") Tj ET"
	build := func(n int) *pdf.Reader {
		return nestDoc(t, manyRegions(n), func(b *nestBuilder) []nestPage {
			return []nestPage{{content: text, res: helv()}}
		})
	}
	if err := Write(io.Discard, build(2), nil); err != nil {
		t.Errorf("small number of regions: %v", err)
	}
	if err := Write(io.Discard, build(2000), nil); !errors.Is(err, ErrTooComplex) {
		t.Errorf("got error %v, want ErrTooComplex", err)
	}
}

// TestBudgetReplyChain checks that a long chain of replies, listed in the
// order which is worst for a repeated scan, is processed.
func TestBudgetReplyChain(t *testing.T) {
	const n = 20000
	in := nestDoc(t, []pdf.Dict{{"Rect": rect(0, 0, 300, 100)}}, func(b *nestBuilder) []nestPage {
		refs := make([]pdf.Reference, n)
		for i := range refs {
			refs[i] = b.w.Alloc()
		}
		var annots []pdf.Object
		// reply i refers to reply i+1, and the last one to the region
		for i := n - 1; i >= 0; i-- {
			d := pdf.Dict{
				"Type": pdf.Name("Annot"), "Subtype": pdf.Name("Text"),
				"Rect": rect(0, 200, 10, 210),
			}
			if i > 0 {
				d["IRT"] = refs[i-1]
			} else {
				d["Rect"] = rect(0, 0, 10, 10)
			}
			if err := b.w.Put(refs[i], d); err != nil {
				t.Fatal(err)
			}
			annots = append(annots, refs[i])
		}
		return []nestPage{{content: "q Q", res: pdf.Dict{}, annots: annots}}
	})
	var buf bytes.Buffer
	if err := Write(&buf, in, nil); err != nil {
		t.Fatal(err)
	}
	out := openPDF(t, buf.Bytes())
	_, dicts := pageDicts(t, out)
	if got := annotSubtypes(t, out, dicts[0]); len(got) != 0 {
		t.Errorf("%d annotations survive", len(got))
	}
}

// TestBudgetStructureTree checks that a wide structure tree which loses its
// text entries is processed in work proportional to its size.
func TestBudgetStructureTree(t *testing.T) {
	const n = 3000
	in := structDoc(t, func(p *structParts) {
		kids := pdf.Array{p.pRef, p.qRef, p.aRef, p.fRef, p.gRef}
		p.nums = nil // forces the fallback
		for i := range n {
			d := pdf.Dict{
				"Type": pdf.Name("StructElem"), "S": pdf.Name("P"), "P": p.docRef,
				"ActualText": pdf.String(fmt.Sprintf("bulk%d", i)),
			}
			ref := p.rootRef + pdf.Reference(1000+i)
			p.elems[ref] = d
			kids = append(kids, ref)
		}
		p.elems[p.docRef]["K"] = kids
	})
	_, data := redactStruct(t, in)
	if bytes.Contains(data, []byte("bulk")) {
		t.Error("fallback left text entries")
	}
}

// fieldChainDoc builds a document with a form whose field tree is a chain of
// the given depth.  The partial names have the given length.
func fieldChainDoc(t *testing.T, depth, nameLen int) *pdf.Reader {
	t.Helper()
	return nestDoc(t, []pdf.Dict{{"Rect": rect(0, 0, 10, 10)}}, func(b *nestBuilder) []nestPage {
		refs := make([]pdf.Reference, depth)
		for i := range refs {
			refs[i] = b.w.Alloc()
		}
		name := pdf.String(strings.Repeat("n", nameLen))
		for i, ref := range refs {
			d := pdf.Dict{"T": name, "FT": pdf.Name("Tx")}
			if i+1 < depth {
				d["Kids"] = pdf.Array{refs[i+1]}
			}
			if i > 0 {
				d["Parent"] = refs[i-1]
			}
			if err := b.w.Put(ref, d); err != nil {
				t.Fatal(err)
			}
		}
		acro := b.put(pdf.Dict{"Fields": pdf.Array{refs[0]}})
		b.w.GetMeta().Catalog.AcroForm = acro
		return []nestPage{{content: "q Q", res: pdf.Dict{}}}
	})
}

// TestBudgetFieldTree checks that the qualified names of a deep field tree,
// which are longer than the file, count towards the budget.
func TestBudgetFieldTree(t *testing.T) {
	if err := Write(io.Discard, fieldChainDoc(t, 20, 20), nil); err != nil {
		t.Errorf("small tree: %v", err)
	}
	if err := Write(io.Discard, fieldChainDoc(t, 250, 4000), nil); !errors.Is(err, ErrTooComplex) {
		t.Errorf("got error %v, want ErrTooComplex", err)
	}
}

// TestBudgetSameNameFields checks that many terminal fields with one name
// are processed.
func TestBudgetSameNameFields(t *testing.T) {
	const n = 20000
	in := nestDoc(t, []pdf.Dict{{"Rect": rect(0, 0, 10, 10)}}, func(b *nestBuilder) []nestPage {
		var fields, annots pdf.Array
		for i := range n {
			w := pdf.Dict{
				"Type": pdf.Name("Annot"), "Subtype": pdf.Name("Widget"),
				"FT": pdf.Name("Tx"), "T": pdf.String("same"), "V": pdf.String("v"),
				"Rect": rect(0, 200, 10, 210),
			}
			if i == 0 {
				w["Rect"] = rect(0, 0, 10, 10)
			}
			ref := b.put(w)
			fields = append(fields, ref)
			annots = append(annots, ref)
		}
		b.w.GetMeta().Catalog.AcroForm = b.put(pdf.Dict{"Fields": fields})
		return []nestPage{{content: "q Q", res: pdf.Dict{}, annots: annots}}
	})
	checkBoundedWrite(t, in)
}

// bigType3Font returns a Type 3 font with four glyphs A to D whose glyph
// procedures have n path segments each.
func bigType3Font(t *testing.T, n int) font.Layouter {
	t.Helper()
	F := &type3.Font{FontMatrix: matrix.Matrix{0.001, 0, 0, 0.001, 0, 0}}
	F.Glyphs = append(F.Glyphs, &type3.Glyph{})
	for _, name := range []string{"A", "B", "C", "D"} {
		b := builder.New(content.Glyph, nil, pdf.V2_0)
		b.Type3UncoloredGlyph(500, 0, 0, 0, 500, 700)
		for range n {
			b.MoveTo(0, 0)
			b.LineTo(500, 700)
			b.Stroke()
		}
		stream, err := b.Harvest()
		if err != nil {
			t.Fatal(err)
		}
		F.Glyphs = append(F.Glyphs, &type3.Glyph{Name: name, Content: stream})
	}
	inst, err := F.New()
	if err != nil {
		t.Fatal(err)
	}
	return inst
}

// TestBudgetType3Glyphs checks that the analysis of large glyph procedures
// counts towards the budget.
func TestBudgetType3Glyphs(t *testing.T) {
	build := func(n int) *pdf.Reader {
		inst := bigType3Font(t, n)
		data := fmt.Sprintf("BT /T3 12 Tf 50 100 Td %s Tj ET", hexShow(t, inst, "ABCD"))
		o := &srcOpts{fonts: map[pdf.Name]font.Instance{"T3": inst}}
		return textSourceOpt(t, data, o, pdf.Dict{"Rect": rect(0, 0, 300, 300)})
	}
	if err := Write(io.Discard, build(10), nil); err != nil {
		t.Errorf("small glyphs: %v", err)
	}
	if err := Write(io.Discard, build(300000), nil); !errors.Is(err, ErrTooComplex) {
		t.Errorf("got error %v, want ErrTooComplex", err)
	}
}

// TestBudgetRepeatedContents checks that the bytes of page content which
// holds no operators count towards the budget, also when the /Contents array
// repeats one stream many times.
func TestBudgetRepeatedContents(t *testing.T) {
	filler := []byte(strings.Repeat("%"+strings.Repeat(" ", 1022)+"\n", 64))
	build := func(n int) *pdf.Reader {
		return pagesDoc(t, 1, func(w *pdf.Writer, refs []pdf.Reference, pages []pdf.Dict) {
			ref := w.Alloc()
			stm, err := w.OpenStream(ref, pdf.Dict{}, pdf.FilterCompress{})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := stm.Write(filler); err != nil {
				t.Fatal(err)
			}
			if err := stm.Close(); err != nil {
				t.Fatal(err)
			}
			contents := make(pdf.Array, n)
			for i := range contents {
				contents[i] = ref
			}
			pages[0]["Contents"] = contents
			addRedaction(t, w, refs[0], pages[0], rect(100, 100, 200, 200))
		})
	}
	if err := Write(io.Discard, build(2), nil); err != nil {
		t.Errorf("short content: %v", err)
	}
	if err := Write(io.Discard, build(1000), nil); !errors.Is(err, ErrTooComplex) {
		t.Errorf("got error %v, want ErrTooComplex", err)
	}
}

// TestBudgetSharedRedaction checks that a redaction annotation with many
// quads, which many pages list, is rejected before any output is written.
func TestBudgetSharedRedaction(t *testing.T) {
	const pages, quads = 500, 2000
	var qp, kids strings.Builder
	for i := range quads {
		x := i % 100
		fmt.Fprintf(&qp, "%d 10 %d 10 %d 0 %d 0 ", x, x+1, x, x+1)
	}
	objs := []string{
		"<</Type/Catalog/Pages 2 0 R>>",
		"", // the page tree root
		"<</Type/Annot/Subtype/Redact/Rect[0 0 200 200]/QuadPoints[" + qp.String() + "]>>",
	}
	for range pages {
		fmt.Fprintf(&kids, "%d 0 R ", len(objs)+1)
		objs = append(objs, "<</Type/Page/Parent 2 0 R/Annots[3 0 R]>>")
	}
	objs[1] = fmt.Sprintf("<</Type/Pages/MediaBox[0 0 200 200]/Kids[%s]/Count %d>>", kids.String(), pages)
	in := openPDF(t, rawDoc("1.7", "", objs...))

	var buf bytes.Buffer
	if err := Write(&buf, in, nil); !errors.Is(err, ErrTooComplex) {
		t.Errorf("got error %v, want ErrTooComplex", err)
	}
	if buf.Len() > 0 {
		t.Errorf("%d bytes were written", buf.Len())
	}
}
