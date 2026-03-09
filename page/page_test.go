// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2025  Jochen Voss <voss@seehuhn.de>
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
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/standard"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

type testCase struct {
	name string
	page *Page
}

var testCases = []testCase{
	{
		name: "minimal page",
		page: &Page{
			MediaBox: &pdf.Rectangle{LLx: 0, LLy: 0, URx: 612, URy: 792},
			Resources: &content.Resources{
				SingleUse: true,
			},
		},
	},
	{
		name: "page with simple content",
		page: &Page{
			MediaBox: &pdf.Rectangle{LLx: 0, LLy: 0, URx: 612, URy: 792},
			Resources: &content.Resources{
				SingleUse: true,
			},
			Contents: []content.Stream{
				content.Operators{
					{Name: content.OpMoveTo, Args: []pdf.Object{pdf.Number(100), pdf.Number(100)}},
					{Name: content.OpLineTo, Args: []pdf.Object{pdf.Number(200), pdf.Number(200)}},
					{Name: content.OpStroke},
				},
			},
		},
	},
	{
		name: "page with multiple content streams",
		page: &Page{
			MediaBox: &pdf.Rectangle{LLx: 0, LLy: 0, URx: 612, URy: 792},
			Resources: &content.Resources{
				SingleUse: true,
			},
			Contents: []content.Stream{
				// first stream: self-balanced graphics state
				content.Operators{
					{Name: content.OpPushGraphicsState},
					{Name: content.OpPopGraphicsState},
				},
				// second stream: draw operations
				content.Operators{
					{Name: content.OpMoveTo, Args: []pdf.Object{pdf.Number(50), pdf.Number(50)}},
					{Name: content.OpLineTo, Args: []pdf.Object{pdf.Number(100), pdf.Number(100)}},
					{Name: content.OpStroke},
				},
				// third stream: another self-balanced operation
				content.Operators{
					{Name: content.OpPushGraphicsState},
					{Name: content.OpPopGraphicsState},
				},
			},
		},
	},
	{
		name: "page with text",
		page: &Page{
			MediaBox: &pdf.Rectangle{URx: 612, URy: 792},
			Resources: &content.Resources{
				SingleUse: true,
				Font: map[pdf.Name]font.Instance{
					"F1": standard.TimesRoman.New(),
				},
			},
			Contents: []content.Stream{
				content.Operators{
					{Name: content.OpTextBegin},
					{Name: content.OpTextSetFont, Args: []pdf.Object{pdf.Name("F1"), pdf.Number(12)}},
					{Name: content.OpTextMoveOffset, Args: []pdf.Object{pdf.Number(72), pdf.Number(720)}},
					{Name: content.OpTextShow, Args: []pdf.Object{pdf.String("Hello")}},
					{Name: content.OpTextEnd},
				},
			},
		},
	},
	{
		name: "page with all boxes",
		page: &Page{
			MediaBox:  &pdf.Rectangle{LLx: 0, LLy: 0, URx: 612, URy: 792},
			CropBox:   &pdf.Rectangle{LLx: 10, LLy: 10, URx: 602, URy: 782},
			BleedBox:  &pdf.Rectangle{LLx: 5, LLy: 5, URx: 607, URy: 787},
			TrimBox:   &pdf.Rectangle{LLx: 20, LLy: 20, URx: 592, URy: 772},
			ArtBox:    &pdf.Rectangle{LLx: 30, LLy: 30, URx: 582, URy: 762},
			Resources: &content.Resources{SingleUse: true},
		},
	},
	{
		name: "page with rotation",
		page: &Page{
			MediaBox:  &pdf.Rectangle{LLx: 0, LLy: 0, URx: 612, URy: 792},
			Rotate:    Rotate270,
			Resources: &content.Resources{SingleUse: true},
		},
	},
	{
		name: "page with duration",
		page: &Page{
			MediaBox:  &pdf.Rectangle{LLx: 0, LLy: 0, URx: 612, URy: 792},
			Duration:  5.0,
			Resources: &content.Resources{SingleUse: true},
		},
	},
	{
		name: "page with tabs",
		page: &Page{
			MediaBox:  &pdf.Rectangle{LLx: 0, LLy: 0, URx: 612, URy: 792},
			Tabs:      "R",
			Resources: &content.Resources{SingleUse: true},
		},
	},
	{
		name: "page with user unit",
		page: &Page{
			MediaBox:  &pdf.Rectangle{LLx: 0, LLy: 0, URx: 612, URy: 792},
			UserUnit:  2.0,
			Resources: &content.Resources{SingleUse: true},
		},
	},
}

// collectOps collects all operators from a content.Stream into an Operators slice.
func collectOps(s content.Stream) content.Operators {
	if s == nil {
		return nil
	}
	var ops content.Operators
	for name, args := range s.All() {
		ops = append(ops, content.Operator{Name: name, Args: slices.Clone(args)})
	}
	return ops
}

// roundTripTest encodes a page, decodes it back, and verifies the result.
func roundTripTest(t *testing.T, v pdf.Version, p1 *Page) {
	t.Helper()

	w, _ := memfile.NewPDFWriter(v, nil)

	// Allocate a parent reference
	parentRef := w.Alloc()
	p1.Parent = parentRef

	rm := pdf.NewResourceManager(w)

	// Encode the page
	dict, err := p1.Encode(rm)
	if pdf.IsWrongVersion(err) {
		t.Skip("version not supported")
	} else if err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	err = rm.Close()
	if err != nil {
		t.Fatalf("rm.Close failed: %v", err)
	}

	// Write a dummy parent dict
	if err := w.Put(parentRef, pdf.Dict{"Type": pdf.Name("Pages")}); err != nil {
		t.Fatalf("Put parent failed: %v", err)
	}

	err = w.Close()
	if err != nil {
		t.Fatalf("w.Close failed: %v", err)
	}

	// Decode using extractor directly from writer
	x := pdf.NewExtractor(w)
	p2, err := Decode(x, dict)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	// Normalize UserUnit: 0 is shorthand for 1.0
	if p1.UserUnit == 0 {
		p1.UserUnit = 1.0
	}

	// Collect operators from both pages for comparison.
	// The original may have multiple content streams; the decoded page
	// preserves per-stream identity, so compare the combined content.
	var wantOps content.Operators
	for _, s := range p1.Contents {
		wantOps = append(wantOps, collectOps(s)...)
	}
	gotOps := collectOps(p2.ContentStream())

	if !wantOps.Equal(gotOps) {
		t.Errorf("content stream mismatch:\nwant: %v\n got: %v", wantOps, gotOps)
	}

	// Compare all fields except Parent and Contents (already compared above)
	opts := []cmp.Option{
		cmpopts.IgnoreFields(Page{}, "Parent", "Contents"),
		cmp.Comparer(func(a, b *content.Resources) bool {
			if a == nil && b == nil {
				return true
			}
			if a == nil || b == nil {
				return false
			}
			return a.Equal(b)
		}),
	}

	if diff := cmp.Diff(p1, p2, opts...); diff != "" {
		t.Errorf("round-trip mismatch (-want +got):\n%s", diff)
	}
}

func TestRoundTrip(t *testing.T) {
	versions := []pdf.Version{pdf.V1_7, pdf.V2_0}

	for _, v := range versions {
		for _, tc := range testCases {
			t.Run(tc.name+"-"+v.String(), func(t *testing.T) {
				// Make a copy to avoid modifying the original
				p := *tc.page
				roundTripTest(t, v, &p)
			})
		}
	}
}

func TestContentStream_Embed(t *testing.T) {
	ops := content.Operators{
		{Name: content.OpMoveTo, Args: []pdf.Object{pdf.Number(100), pdf.Number(100)}},
		{Name: content.OpLineTo, Args: []pdf.Object{pdf.Number(200), pdf.Number(200)}},
		{Name: content.OpStroke},
	}

	buf := &bytes.Buffer{}
	w, err := pdf.NewWriter(buf, pdf.V1_7, nil)
	if err != nil {
		t.Fatal(err)
	}

	rm := pdf.NewResourceManager(w)
	ref, err := embedPageContent(rm, ops)
	if err != nil {
		t.Fatal(err)
	}

	if ref == nil {
		t.Fatal("expected reference, got nil")
	}

	if _, ok := ref.(pdf.Reference); !ok {
		t.Errorf("expected pdf.Reference, got %T", ref)
	}

	if err := rm.Close(); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestContentSegment_Deduplication(t *testing.T) {
	// encode a page with content to get a contentSegment via round-trip
	w1, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	parentRef := w1.Alloc()
	p := &Page{
		Parent:   parentRef,
		MediaBox: &pdf.Rectangle{URx: 100, URy: 100},
		Resources: &content.Resources{
			SingleUse: true,
		},
		Contents: []content.Stream{
			content.Operators{
				{Name: content.OpMoveTo, Args: []pdf.Object{pdf.Number(0), pdf.Number(0)}},
				{Name: content.OpLineTo, Args: []pdf.Object{pdf.Number(100), pdf.Number(100)}},
				{Name: content.OpStroke},
			},
		},
	}
	rm1 := pdf.NewResourceManager(w1)
	dict, err := p.Encode(rm1)
	if err != nil {
		t.Fatal(err)
	}
	rm1.Close()
	w1.Put(parentRef, pdf.Dict{"Type": pdf.Name("Pages")})
	w1.Close()

	// decode back to get a contentSegment
	decoded, err := Decode(pdf.NewExtractor(w1), dict)
	if err != nil {
		t.Fatal(err)
	}
	if len(decoded.Contents) == 0 {
		t.Skip("no content streams")
	}
	seg := decoded.Contents[0]

	// embedding the same contentSegment twice should produce the same reference
	w2, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	rm2 := pdf.NewResourceManager(w2)

	ref1, err := embedPageContent(rm2, seg)
	if err != nil {
		t.Fatal(err)
	}
	ref2, err := embedPageContent(rm2, seg)
	if err != nil {
		t.Fatal(err)
	}

	if ref1 != ref2 {
		t.Errorf("expected same reference for deduplicated content, got %v and %v", ref1, ref2)
	}

	rm2.Close()
	w2.Close()
}

func TestPage_Encode_ValidationError(t *testing.T) {
	buf := &bytes.Buffer{}
	w, err := pdf.NewWriter(buf, pdf.V1_7, nil)
	if err != nil {
		t.Fatal(err)
	}

	parentRef := w.Alloc()

	// page with unbalanced q/Q
	page := &Page{
		Parent:   parentRef,
		MediaBox: &pdf.Rectangle{LLx: 0, LLy: 0, URx: 612, URy: 792},
		Resources: &content.Resources{
			SingleUse: true,
		},
		Contents: []content.Stream{
			content.Operators{
				{Name: content.OpPushGraphicsState},
				// missing Q
			},
		},
	}

	rm := pdf.NewResourceManager(w)
	_, err = page.Encode(rm)
	if err == nil {
		t.Fatal("expected validation error for unbalanced q/Q")
	}
}

func TestPage_VersionChecks(t *testing.T) {
	tests := []struct {
		name       string
		version    pdf.Version
		page       *Page
		shouldFail bool
	}{
		{
			name:    "BleedBox requires 1.3",
			version: pdf.V1_2,
			page: &Page{
				MediaBox:  &pdf.Rectangle{LLx: 0, LLy: 0, URx: 612, URy: 792},
				BleedBox:  &pdf.Rectangle{LLx: 0, LLy: 0, URx: 612, URy: 792},
				Resources: &content.Resources{SingleUse: true},
			},
			shouldFail: true,
		},
		{
			name:    "BleedBox works in 1.3",
			version: pdf.V1_3,
			page: &Page{
				MediaBox:  &pdf.Rectangle{LLx: 0, LLy: 0, URx: 612, URy: 792},
				BleedBox:  &pdf.Rectangle{LLx: 0, LLy: 0, URx: 612, URy: 792},
				Resources: &content.Resources{SingleUse: true},
			},
			shouldFail: false,
		},
		{
			name:    "Tabs requires 1.5",
			version: pdf.V1_4,
			page: &Page{
				MediaBox:  &pdf.Rectangle{LLx: 0, LLy: 0, URx: 612, URy: 792},
				Tabs:      "R",
				Resources: &content.Resources{SingleUse: true},
			},
			shouldFail: true,
		},
		{
			name:    "UserUnit requires 1.6",
			version: pdf.V1_5,
			page: &Page{
				MediaBox:  &pdf.Rectangle{LLx: 0, LLy: 0, URx: 612, URy: 792},
				UserUnit:  2.0,
				Resources: &content.Resources{SingleUse: true},
			},
			shouldFail: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w, _ := memfile.NewPDFWriter(tc.version, nil)
			parentRef := w.Alloc()
			tc.page.Parent = parentRef

			rm := pdf.NewResourceManager(w)
			_, err := tc.page.Encode(rm)

			if tc.shouldFail && err == nil {
				t.Error("expected version error, got nil")
			}
			if !tc.shouldFail && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestPage_Encode_InvalidBoxCoords(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	parentRef := w.Alloc()

	page := &Page{
		Parent:   parentRef,
		MediaBox: &pdf.Rectangle{LLx: 100, LLy: 0, URx: 50, URy: 792}, // LLx > URx
		Resources: &content.Resources{
			SingleUse: true,
		},
	}
	rm := pdf.NewResourceManager(w)
	_, err := page.Encode(rm)
	if err == nil {
		t.Fatal("expected error for inverted MediaBox coordinates")
	}
}

func TestPage_Encode_BoxOutsideMediaBox(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	parentRef := w.Alloc()

	page := &Page{
		Parent:   parentRef,
		MediaBox: &pdf.Rectangle{LLx: 0, LLy: 0, URx: 612, URy: 792},
		CropBox:  &pdf.Rectangle{LLx: -10, LLy: 0, URx: 612, URy: 792},
		Resources: &content.Resources{
			SingleUse: true,
		},
	}
	rm := pdf.NewResourceManager(w)
	_, err := page.Encode(rm)
	if err == nil {
		t.Fatal("expected error for CropBox extending beyond MediaBox")
	}
}

func TestPage_Decode_ClipsBoxes(t *testing.T) {
	// write a page dict directly with boxes extending beyond MediaBox
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	parentRef := w.Alloc()

	mediaBox := &pdf.Rectangle{LLx: 0, LLy: 0, URx: 612, URy: 792}
	cropBox := &pdf.Rectangle{LLx: -10, LLy: -10, URx: 620, URy: 800}

	dict := pdf.Dict{
		"Type":      pdf.Name("Page"),
		"Parent":    parentRef,
		"MediaBox":  mediaBox,
		"CropBox":   cropBox,
		"Resources": pdf.Dict{},
	}

	if err := w.Put(parentRef, pdf.Dict{"Type": pdf.Name("Pages")}); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	x := pdf.NewExtractor(w)
	p, err := Decode(x, dict)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	// CropBox should be clipped to MediaBox
	want := &pdf.Rectangle{LLx: 0, LLy: 0, URx: 612, URy: 792}
	if p.CropBox == nil {
		t.Fatal("CropBox is nil after clipping")
	}
	if !p.CropBox.NearlyEqual(want, 1e-9) {
		t.Errorf("CropBox = %v, want %v", p.CropBox, want)
	}
}

func TestPage_Decode_ClipsToNil(t *testing.T) {
	// write a page dict with a TrimBox completely outside MediaBox
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	parentRef := w.Alloc()

	dict := pdf.Dict{
		"Type":      pdf.Name("Page"),
		"Parent":    parentRef,
		"MediaBox":  &pdf.Rectangle{LLx: 0, LLy: 0, URx: 100, URy: 100},
		"TrimBox":   &pdf.Rectangle{LLx: 200, LLy: 200, URx: 300, URy: 300},
		"Resources": pdf.Dict{},
	}

	if err := w.Put(parentRef, pdf.Dict{"Type": pdf.Name("Pages")}); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	x := pdf.NewExtractor(w)
	p, err := Decode(x, dict)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	if p.TrimBox != nil {
		t.Errorf("TrimBox should be nil when completely outside MediaBox, got %v", p.TrimBox)
	}
}

func FuzzRoundTrip(f *testing.F) {
	// Seed the fuzzer with valid test cases
	opt := &pdf.WriterOptions{
		HumanReadable: true,
	}

	for _, tc := range testCases {
		w, buf := memfile.NewPDFWriter(pdf.V1_7, opt)
		rm := pdf.NewResourceManager(w)

		// allocate page tree references
		pageRef := w.Alloc()
		pagesRef := w.Alloc()

		p := *tc.page
		p.Parent = pagesRef

		obj, err := p.Encode(rm)
		if err != nil {
			continue
		}

		err = rm.Close()
		if err != nil {
			continue
		}

		// write the page object
		if err := w.Put(pageRef, obj); err != nil {
			continue
		}

		// write a proper Pages tree
		pagesDict := pdf.Dict{
			"Type":  pdf.Name("Pages"),
			"Kids":  pdf.Array{pageRef},
			"Count": pdf.Integer(1),
		}
		if err := w.Put(pagesRef, pagesDict); err != nil {
			continue
		}

		// set up catalog
		w.GetMeta().Catalog.Pages = pagesRef
		w.GetMeta().Trailer["Quir:E"] = obj

		err = w.Close()
		if err != nil {
			continue
		}

		f.Add(buf.Data)
	}

	f.Fuzz(func(t *testing.T, fileData []byte) {
		r, err := pdf.NewReader(bytes.NewReader(fileData), nil)
		if err != nil {
			t.Skip("invalid PDF")
		}

		obj := r.GetMeta().Trailer["Quir:E"]
		if obj == nil {
			t.Skip("missing test object")
		}

		x := pdf.NewExtractor(r)
		p, err := Decode(x, obj)
		if err != nil {
			t.Skip("malformed page")
		}

		// Round-trip test
		roundTripTest(t, pdf.V1_7, p)
	})
}
