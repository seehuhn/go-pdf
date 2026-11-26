// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package redact

// FuzzWrite, which checks the invariants for arbitrary input files, and
// the seed documents for it.

import (
	"bytes"
	"errors"
	"fmt"
	"sync"
	"testing"

	"seehuhn.de/go/pdf"
)

// sourceBytes maps the documents made by the test builders to the file
// contents.
var sourceBytes sync.Map

// openSource opens the PDF file data, and records the data for [fuzzSeeds].
func openSource(t testing.TB, data []byte) *pdf.Reader {
	t.Helper()
	r := openPDF(t, data)
	sourceBytes.Store(r, bytes.Clone(data))
	return r
}

// fuzzSeeds builds the seed documents for [FuzzWrite].
func fuzzSeeds(t testing.TB) [][]byte {
	var docs []*pdf.Reader
	region := func() []pdf.Dict { return []pdf.Dict{{"Rect": rect(0, 0, 300, 120)}} }

	// text, an image and a redaction annotation
	docs = append(docs, nestDoc(t, region(), func(b *nestBuilder) []nestPage {
		ref, err := b.rm.Embed(rgbDict(10, 10, secretRGB(10, 10, nil)))
		if err != nil {
			t.Fatal(err)
		}
		return []nestPage{{
			content: secretText + "\nq 100 0 0 100 100 100 cm /Im1 Do Q\n",
			res: pdf.Dict{
				"Font":    helv()["Font"],
				"XObject": pdf.Dict{"Im1": ref},
			},
		}}
	}))

	// a form XObject
	docs = append(docs, singleForm(t, "q 1 0 0 1 20 100 cm /Fm1 Do Q", nil, secretText)(region()))

	// a tiling pattern
	docs = append(docs, patternDoc(t, []pdf.Dict{{"Rect": rect(41, 25, 47, 31)}}, "0 0 200 200 re", nil))

	// a tagged structure tree
	docs = append(docs, structDoc(t, func(p *structParts) {
		p.form = pdf.Dict{"StructParent": pdf.Integer(5)}
	}))

	// an interactive form
	in, _ := formDoc(t, false)
	docs = append(docs, in)

	res := [][]byte{textSeed()}
	for _, r := range docs {
		data, ok := sourceBytes.Load(r)
		if !ok {
			t.Fatal("document has no recorded source")
		}
		res = append(res, data.([]byte))
	}
	return res
}

// textSeed returns a document with uncompressed streams, which the fuzzer
// can mutate more easily than the output of the builders.  It combines text,
// an image, a form, a pattern, a soft mask, a structure tree and a form
// field.
func textSeed() []byte {
	var buf bytes.Buffer
	buf.WriteString("%PDF-1.7\n")
	obj := func(n int, body string) {
		fmt.Fprintf(&buf, "%d 0 obj\n%s\nendobj\n", n, body)
	}
	stream := func(n int, dict, body string) {
		obj(n, fmt.Sprintf("<<%s/Length %d>>\nstream\n%s\nendstream", dict, len(body), body))
	}
	obj(1, "<</Type/Catalog/Pages 2 0 R/StructTreeRoot 9 0 R/AcroForm<</Fields[12 0 R]>>>>")
	obj(2, "<</Type/Pages/Kids[3 0 R]/Count 1>>")
	obj(3, "<</Type/Page/Parent 2 0 R/MediaBox[0 0 200 200]/Contents 4 0 R/Resources 5 0 R"+
		"/Annots[6 0 R 12 0 R]/StructParents 0>>")
	stream(4, "", "/P <</MCID 0>> BDC BT /F1 12 Tf 20 100 Td (Secret text) Tj [(a) -50 (b)] TJ ET EMC\n"+
		"q 50 0 0 50 100 100 cm /Im1 Do Q\n"+
		"q /G1 gs /Pattern cs /P1 scn 0 0 100 100 re f Q\n"+
		"q 1 0 0 1 10 10 cm /Fm1 Do Q\n"+
		"0 0 m 50 50 l 100 0 l S\n"+
		"BI /W 2 /H 2 /BPC 8 /CS /G /F /AHx ID 00ff80ff> EI\n")
	obj(5, "<</Font<</F1 7 0 R>>/XObject<</Im1 8 0 R/Fm1 10 0 R>>/Pattern<</P1 11 0 R>>"+
		"/ExtGState<</G1<</SMask<</S/Luminosity/G 13 0 R>>>>>>>>")
	obj(6, "<</Type/Annot/Subtype/Redact/Rect[10 90 120 120]/OverlayText(X)>>")
	obj(7, "<</Type/Font/Subtype/Type1/BaseFont/Helvetica>>")
	stream(8, "/Type/XObject/Subtype/Image/Width 2/Height 2/BitsPerComponent 8"+
		"/ColorSpace/DeviceGray/Filter/ASCIIHexDecode", "00 7f ff 80>")
	obj(9, "<</Type/StructTreeRoot/K 14 0 R/ParentTree 15 0 R>>")
	stream(10, "/Type/XObject/Subtype/Form/BBox[0 0 100 100]/Resources<</Font<</F1 7 0 R>>>>"+
		"/StructParents 1/StructParent 3",
		"/P <</MCID 0>> BDC BT /F1 10 Tf 5 5 Td (inner) Tj ET EMC")
	stream(11, "/Type/Pattern/PatternType 1/PaintType 1/TilingType 1/BBox[0 0 20 20]"+
		"/XStep 20/YStep 20/Resources<</Font<</F1 7 0 R>>>>",
		"BT /F1 8 Tf 2 2 Td (tile) Tj ET")
	obj(12, "<</Type/Annot/Subtype/Widget/FT/Tx/T(f)/V(secret)/Rect[20 90 80 110]/P 3 0 R>>")
	stream(13, "/Type/XObject/Subtype/Form/BBox[0 0 200 200]/Group<</S/Transparency/CS/DeviceGray>>",
		"0.5 g 0 0 100 100 re f")
	obj(14, "<</Type/StructElem/S/Document/P 9 0 R/K[16 0 R]/Alt(doc)>>")
	obj(15, "<</Nums[0[16 0 R]1[16 0 R]]>>")
	obj(16, "<</Type/StructElem/S/P/P 14 0 R/K 0/ActualText(Secret text)>>")
	buf.WriteString("trailer\n<</Root 1 0 R/Size 17>>\n")
	return buf.Bytes()
}

// expectedWriteError reports whether Write may fail with err on fuzzed
// input.
func expectedWriteError(err error) bool {
	var unusable *UnusableRedactionError
	return errors.Is(err, ErrTooComplex) || errors.Is(err, ErrEncrypted) ||
		errors.Is(err, ErrUnsupportedVersion) ||
		errors.As(err, &unusable) || pdf.IsMalformed(err)
}

// FuzzWrite checks that Write never panics or hangs on malformed input, that
// it fails only with the documented errors, and that its output can be read.
// The output must have no redaction annotations, and no glyph or image of it
// may lie under the regions of the input (see [checkNoLeak]).  Redacting the
// output again must keep its glyphs and images.
func FuzzWrite(f *testing.F) {
	for _, data := range fuzzSeeds(f) {
		f.Add(data)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		r, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)), nil)
		if err != nil {
			t.Skip()
		}
		var out bytes.Buffer
		err = Write(&out, r, nil)
		if err != nil {
			if expectedWriteError(err) {
				t.Skip()
			}
			t.Fatalf("write failed: %v", err)
		}
		r2 := openPDF(t, out.Bytes())
		checkNoRedactAnnots(t, r2)
		pc2, ok := pageContents(t, r2)
		if ok {
			checkNoLeak(t, inputRegions(t, r), pc2)
		}

		var out2 bytes.Buffer
		err = Write(&out2, r2, nil)
		if errors.Is(err, ErrTooComplex) {
			return
		} else if err != nil {
			t.Fatalf("second Write failed: %v", err)
		}
		r3 := openPDF(t, out2.Bytes())
		checkNoRedactAnnots(t, r3)
		if ok {
			pc3, ok3 := pageContents(t, r3)
			if !ok3 {
				t.Fatal("second output paints more than the first")
			}
			checkSameContent(t, pc2, pc3)
		}
	})
}
