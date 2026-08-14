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
	"testing"

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
	"seehuhn.de/go/pdf/pagetree"
)

// outResStream returns the dictionary of the named stream in the given
// resource category of the first output page, together with a cursor for
// resolving its entries.
func outResStream(t *testing.T, data []byte, category, name pdf.Name) (pdf.Dict, pdf.Cursor) {
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
	sub, err := cur.Dict(res[category])
	if err != nil {
		t.Fatal(err)
	}
	stm, err := cur.Stream(sub[name])
	if err != nil {
		t.Fatal(err)
	}
	if stm == nil {
		t.Fatalf("%s %s not in output", category, name)
	}
	return stm.Dict, cur
}

// TestFormEntriesNormalized checks that the /BBox and /Matrix of a rewritten
// form XObject are written as decode reads them, not copied verbatim.  The
// flattened-annotation overlay is placed with the decoded values, so a value
// the decoder repairs -- a malformed matrix read as the identity, a reversed
// bounding box put in order -- must reach the output in repaired form, or the
// output would draw the appearance elsewhere than the placement assumed.
func TestFormEntriesNormalized(t *testing.T) {
	rect := pdf.Rectangle{LLx: 100, LLy: 200, URx: 140, URy: 220}
	quarter := pdf.Array{
		pdf.Integer(0), pdf.Integer(1), pdf.Integer(-1),
		pdf.Integer(0), pdf.Integer(0), pdf.Integer(0),
	}

	cases := []struct {
		name       string
		bbox       pdf.Object
		mtx        pdf.Object     // /Matrix entry of the source form, nil to omit
		rect       pdf.Rectangle  // annotation rectangle
		wantMatrix *matrix.Matrix // expected output entry, nil for absent
		wantBBox   []float64      // expected raw output /BBox values
	}{
		{
			name: "degenerateMatrix",
			bbox: pdf.Array{pdf.Integer(0), pdf.Integer(0), pdf.Integer(40), pdf.Integer(20)},
			mtx: pdf.Array{
				pdf.Integer(0), pdf.Integer(0), pdf.Integer(0),
				pdf.Integer(0), pdf.Integer(0), pdf.Integer(0),
			},
			rect:     rect,
			wantBBox: []float64{0, 0, 40, 20},
		},
		{
			name:     "malformedMatrix",
			bbox:     pdf.Array{pdf.Integer(0), pdf.Integer(0), pdf.Integer(40), pdf.Integer(20)},
			mtx:      pdf.Array{pdf.Integer(1), pdf.Integer(0)},
			rect:     rect,
			wantBBox: []float64{0, 0, 40, 20},
		},
		{
			name:       "matrixKept",
			bbox:       pdf.Array{pdf.Integer(0), pdf.Integer(0), pdf.Integer(40), pdf.Integer(20)},
			mtx:        quarter,
			rect:       pdf.Rectangle{LLx: 100, LLy: 200, URx: 120, URy: 240},
			wantMatrix: &matrix.Matrix{0, 1, -1, 0, 0, 0},
			wantBBox:   []float64{0, 0, 40, 20},
		},
		{
			name:     "reversedBBox",
			bbox:     pdf.Array{pdf.Integer(40), pdf.Integer(20), pdf.Integer(0), pdf.Integer(0)},
			rect:     rect,
			wantBBox: []float64{0, 0, 40, 20},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w, buf := memfile.NewPDFWriter(pdf.V1_7, nil)

			apDict := pdf.Dict{
				"Type":    pdf.Name("XObject"),
				"Subtype": pdf.Name("Form"),
				"BBox":    tc.bbox,
			}
			if tc.mtx != nil {
				apDict["Matrix"] = tc.mtx
			}
			apRef := w.Alloc()
			stm, err := w.OpenStream(apRef, apDict)
			if err != nil {
				t.Fatal(err)
			}
			io.WriteString(stm, "1 0 0 rg 0 0 40 20 re f\n")
			if err := stm.Close(); err != nil {
				t.Fatal(err)
			}

			annot := w.Alloc()
			w.Put(annot, pdf.Dict{
				"Type": pdf.Name("Annot"), "Subtype": pdf.Name("Square"),
				"Rect": pdf.Array{
					pdf.Number(tc.rect.LLx), pdf.Number(tc.rect.LLy),
					pdf.Number(tc.rect.URx), pdf.Number(tc.rect.URy),
				},
				"F":  pdf.Integer(4), // Print
				"AP": pdf.Dict{"N": apRef},
			})

			contentRef := w.Alloc()
			cstm, _ := w.OpenStream(contentRef, pdf.Dict{})
			io.WriteString(cstm, "q 0 0 1 rg 5 5 10 10 re f Q\n")
			cstm.Close()

			pageRef, pagesRef := w.Alloc(), w.Alloc()
			w.Put(pageRef, pdf.Dict{
				"Type": pdf.Name("Page"), "Parent": pagesRef,
				"MediaBox": pdf.Array{pdf.Integer(0), pdf.Integer(0), pdf.Integer(300), pdf.Integer(300)},
				"Contents": contentRef,
				"Annots":   pdf.Array{annot},
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

			dict, cur := outResStream(t, out.Bytes(), "XObject", "PPAnnot0")

			if tc.wantMatrix == nil {
				if dict["Matrix"] != nil {
					t.Errorf("output /Matrix = %v, want absent", dict["Matrix"])
				}
			} else {
				got, err := cur.Matrix(dict["Matrix"])
				if err != nil {
					t.Fatal(err)
				}
				if got != *tc.wantMatrix {
					t.Errorf("output /Matrix = %v, want %v", got, *tc.wantMatrix)
				}
			}

			raw, err := cur.FloatArray(dict["BBox"])
			if err != nil {
				t.Fatal(err)
			}
			if len(raw) != 4 {
				t.Fatalf("output /BBox has %d elements, want 4", len(raw))
			}
			for i, v := range tc.wantBBox {
				if raw[i] != v {
					t.Errorf("output /BBox = %v, want %v", raw, tc.wantBBox)
					break
				}
			}

			// the output must draw the appearance where the placement put it:
			// over the annotation rectangle
			effective := matrix.Identity
			if tc.wantMatrix != nil {
				effective = *tc.wantMatrix
			}
			effective = effective.Mul(ctmAtFirstXObject(t, out.Bytes()))
			got := drawnBounds(t, pdf.Rectangle{URx: 40, URy: 20}, effective)
			if !got.NearlyEqual(&tc.rect, 1e-6) {
				t.Errorf("appearance drawn over %s, want %s", &got, &tc.rect)
			}
		})
	}
}

// TestFormEntriesNormalizedInResources checks that the same normalization
// applies to form XObjects reached through page resources, and that an
// unreadable /BBox is copied verbatim there rather than dropped.
func TestFormEntriesNormalizedInResources(t *testing.T) {
	cases := []struct {
		name     string
		bbox     pdf.Object
		mtx      pdf.Object
		wantBBox pdf.Object // nil to check the normalized [0 0 40 20]
	}{
		{
			name: "repaired",
			bbox: pdf.Array{pdf.Integer(40), pdf.Integer(20), pdf.Integer(0), pdf.Integer(0)},
			mtx: pdf.Array{
				pdf.Integer(0), pdf.Integer(0), pdf.Integer(0),
				pdf.Integer(0), pdf.Integer(0), pdf.Integer(0),
			},
		},
		{
			name:     "unreadableBBoxKept",
			bbox:     pdf.Name("junk"),
			wantBBox: pdf.Name("junk"),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w, buf := memfile.NewPDFWriter(pdf.V1_7, nil)

			formDict := pdf.Dict{
				"Type":    pdf.Name("XObject"),
				"Subtype": pdf.Name("Form"),
				"BBox":    tc.bbox,
			}
			if tc.mtx != nil {
				formDict["Matrix"] = tc.mtx
			}
			formRef := w.Alloc()
			stm, err := w.OpenStream(formRef, formDict)
			if err != nil {
				t.Fatal(err)
			}
			io.WriteString(stm, "1 0 0 rg 0 0 40 20 re f\n")
			if err := stm.Close(); err != nil {
				t.Fatal(err)
			}

			contentRef := w.Alloc()
			cstm, _ := w.OpenStream(contentRef, pdf.Dict{})
			io.WriteString(cstm, "q /Fm0 Do Q\n")
			cstm.Close()

			pageRef, pagesRef := w.Alloc(), w.Alloc()
			w.Put(pageRef, pdf.Dict{
				"Type": pdf.Name("Page"), "Parent": pagesRef,
				"MediaBox":  pdf.Array{pdf.Integer(0), pdf.Integer(0), pdf.Integer(300), pdf.Integer(300)},
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

			dict, cur := outResStream(t, out.Bytes(), "XObject", "Fm0")

			if dict["Matrix"] != nil {
				t.Errorf("output /Matrix = %v, want absent", dict["Matrix"])
			}
			if tc.wantBBox != nil {
				if got := dict["BBox"]; got != tc.wantBBox {
					t.Errorf("output /BBox = %v, want %v", got, tc.wantBBox)
				}
			} else {
				raw, err := cur.FloatArray(dict["BBox"])
				if err != nil {
					t.Fatal(err)
				}
				want := []float64{0, 0, 40, 20}
				if len(raw) != 4 {
					t.Fatalf("output /BBox has %d elements, want 4", len(raw))
				}
				for i, v := range want {
					if raw[i] != v {
						t.Errorf("output /BBox = %v, want %v", raw, want)
						break
					}
				}
			}
		})
	}
}

// TestPatternEntriesNormalized checks that the /BBox and /Matrix of a
// rewritten tiling pattern are written as decode reads them, like those of a
// form XObject: a renderer draws the pattern from the decoded values, so a
// repaired value must reach the printed output as well.  XStep and YStep have
// no read repair -- decoding fails without them -- and stay verbatim.
func TestPatternEntriesNormalized(t *testing.T) {
	quarter := pdf.Array{
		pdf.Integer(0), pdf.Integer(1), pdf.Integer(-1),
		pdf.Integer(0), pdf.Integer(0), pdf.Integer(0),
	}

	cases := []struct {
		name       string
		bbox       pdf.Object
		mtx        pdf.Object     // /Matrix entry of the source pattern, nil to omit
		wantMatrix *matrix.Matrix // expected output entry, nil for absent
		wantBBox   pdf.Object     // expected verbatim entry, nil for normalized [0 0 40 20]
	}{
		{
			name: "repaired",
			bbox: pdf.Array{pdf.Integer(40), pdf.Integer(20), pdf.Integer(0), pdf.Integer(0)},
			mtx: pdf.Array{
				pdf.Integer(0), pdf.Integer(0), pdf.Integer(0),
				pdf.Integer(0), pdf.Integer(0), pdf.Integer(0),
			},
		},
		{
			name:       "matrixKept",
			bbox:       pdf.Array{pdf.Integer(0), pdf.Integer(0), pdf.Integer(40), pdf.Integer(20)},
			mtx:        quarter,
			wantMatrix: &matrix.Matrix{0, 1, -1, 0, 0, 0},
		},
		{
			name:     "unreadableBBoxKept",
			bbox:     pdf.Name("junk"),
			wantBBox: pdf.Name("junk"),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w, buf := memfile.NewPDFWriter(pdf.V1_7, nil)

			patDict := pdf.Dict{
				"Type":        pdf.Name("Pattern"),
				"PatternType": pdf.Integer(1),
				"PaintType":   pdf.Integer(1),
				"TilingType":  pdf.Integer(1),
				"BBox":        tc.bbox,
				"XStep":       pdf.Integer(40),
				"YStep":       pdf.Integer(20),
			}
			if tc.mtx != nil {
				patDict["Matrix"] = tc.mtx
			}
			patRef := w.Alloc()
			stm, err := w.OpenStream(patRef, patDict)
			if err != nil {
				t.Fatal(err)
			}
			io.WriteString(stm, "1 0 0 rg 0 0 40 20 re f\n")
			if err := stm.Close(); err != nil {
				t.Fatal(err)
			}

			contentRef := w.Alloc()
			cstm, _ := w.OpenStream(contentRef, pdf.Dict{})
			io.WriteString(cstm, "q /Pattern cs /P0 scn 0 0 100 100 re f Q\n")
			cstm.Close()

			pageRef, pagesRef := w.Alloc(), w.Alloc()
			w.Put(pageRef, pdf.Dict{
				"Type": pdf.Name("Page"), "Parent": pagesRef,
				"MediaBox":  pdf.Array{pdf.Integer(0), pdf.Integer(0), pdf.Integer(300), pdf.Integer(300)},
				"Contents":  contentRef,
				"Resources": pdf.Dict{"Pattern": pdf.Dict{"P0": patRef}},
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

			dict, cur := outResStream(t, out.Bytes(), "Pattern", "P0")

			if tc.wantMatrix == nil {
				if dict["Matrix"] != nil {
					t.Errorf("output /Matrix = %v, want absent", dict["Matrix"])
				}
			} else {
				got, err := cur.Matrix(dict["Matrix"])
				if err != nil {
					t.Fatal(err)
				}
				if got != *tc.wantMatrix {
					t.Errorf("output /Matrix = %v, want %v", got, *tc.wantMatrix)
				}
			}

			if tc.wantBBox != nil {
				if got := dict["BBox"]; got != tc.wantBBox {
					t.Errorf("output /BBox = %v, want %v", got, tc.wantBBox)
				}
			} else {
				raw, err := cur.FloatArray(dict["BBox"])
				if err != nil {
					t.Fatal(err)
				}
				want := []float64{0, 0, 40, 20}
				if len(raw) != 4 {
					t.Fatalf("output /BBox has %d elements, want 4", len(raw))
				}
				for i, v := range want {
					if raw[i] != v {
						t.Errorf("output /BBox = %v, want %v", raw, want)
						break
					}
				}
			}

			// XStep and YStep stay verbatim
			if got := dict["XStep"]; got != pdf.Integer(40) {
				t.Errorf("output /XStep = %v, want 40", got)
			}
			if got := dict["YStep"]; got != pdf.Integer(20) {
				t.Errorf("output /YStep = %v, want 20", got)
			}
		})
	}
}
