// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2024  Jochen Voss <voss@seehuhn.de>
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

package pattern_test

import (
	"bytes"
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/function"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/content/builder"
	"seehuhn.de/go/pdf/graphics/extract"
	"seehuhn.de/go/pdf/graphics/pattern"
	"seehuhn.de/go/pdf/graphics/shading"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

type testCase struct {
	Name    string
	Pattern color.Pattern
}

func makeType1Colored() *pattern.Type1 {
	b := builder.New(content.PatternColored, nil)
	b.SetFillColor(color.DeviceGray(0.5))
	b.Rectangle(0, 0, 5, 5)
	b.Fill()
	return &pattern.Type1{
		TilingType: 1,
		BBox:       pdf.Rectangle{LLx: 0, LLy: 0, URx: 10, URy: 10},
		XStep:      10,
		YStep:      10,
		Color:      true,
		Content:    b.Stream,
		Res:        b.Resources,
	}
}

func makeType1Uncolored() *pattern.Type1 {
	b := builder.New(content.PatternUncolored, nil)
	b.Rectangle(5, 5, 10, 10)
	b.Fill()
	return &pattern.Type1{
		TilingType: 2,
		BBox:       pdf.Rectangle{LLx: 0, LLy: 0, URx: 20, URy: 20},
		XStep:      20,
		YStep:      20,
		Color:      false,
		Content:    b.Stream,
		Res:        b.Resources,
	}
}

var testCases = []testCase{
	{
		Name:    "Type1-Colored",
		Pattern: makeType1Colored(),
	},
	{
		Name:    "Type1-Uncolored",
		Pattern: makeType1Uncolored(),
	},
	{
		Name: "Type2-Shading",
		Pattern: &pattern.Type2{
			Shading: &shading.Type1{
				ColorSpace: color.SpaceDeviceGray,
				F: &function.Type4{
					Domain:  []float64{0, 100, 0, 100},
					Range:   []float64{0, 1},
					Program: "add 200 div",
				},
				Domain: []float64{0, 100, 0, 100},
			},
			SingleUse: false,
		},
	},
	{
		Name: "Type2-SingleUse-True",
		Pattern: &pattern.Type2{
			Shading: &shading.Type1{
				ColorSpace: color.SpaceDeviceGray,
				F: &function.Type4{
					Domain:  []float64{0, 100, 0, 100},
					Range:   []float64{0, 1},
					Program: "add 200 div",
				},
				Domain: []float64{0, 100, 0, 100},
			},
			SingleUse: true,
		},
	},
	{
		Name: "Type2-SingleUse-False",
		Pattern: &pattern.Type2{
			Shading: &shading.Type1{
				ColorSpace: color.SpaceDeviceGray,
				F: &function.Type4{
					Domain:  []float64{0, 100, 0, 100},
					Range:   []float64{0, 1},
					Program: "add 200 div",
				},
				Domain: []float64{0, 100, 0, 100},
			},
			SingleUse: false,
		},
	},
}

// testRoundTrip embeds a pattern, extracts it back, embeds again, extracts again,
// and verifies the two extracted results match using cmp.Diff.
func testRoundTrip(t *testing.T, pat color.Pattern) {
	t.Helper()

	// first round: embed and extract
	w1, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	rm1 := pdf.NewResourceManager(w1)

	ref1, err := rm1.Embed(pat)
	if err != nil {
		t.Fatalf("first embed failed: %v", err)
	}

	err = rm1.Close()
	if err != nil {
		t.Fatalf("rm1.Close failed: %v", err)
	}

	err = w1.Close()
	if err != nil {
		t.Fatalf("w1.Close failed: %v", err)
	}

	x1 := pdf.NewExtractor(w1)
	pat1, err := extract.Pattern(x1, ref1)
	if err != nil {
		t.Fatalf("first extract failed: %v", err)
	}

	// second round: embed and extract
	w2, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	rm2 := pdf.NewResourceManager(w2)

	ref2, err := rm2.Embed(pat1)
	if err != nil {
		t.Fatalf("second embed failed: %v", err)
	}

	err = rm2.Close()
	if err != nil {
		t.Fatalf("rm2.Close failed: %v", err)
	}

	err = w2.Close()
	if err != nil {
		t.Fatalf("w2.Close failed: %v", err)
	}

	x2 := pdf.NewExtractor(w2)
	pat2, err := extract.Pattern(x2, ref2)
	if err != nil {
		t.Fatalf("second extract failed: %v", err)
	}

	// compare extracted patterns
	// For Type1, we compare Content streams semantically
	opts := cmp.Options{
		cmp.Comparer(func(a, b content.Stream) bool {
			return content.StreamsEqual(a, b)
		}),
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
	if diff := cmp.Diff(pat1, pat2, opts); diff != "" {
		t.Errorf("round-trip mismatch (-want +got):\n%s", diff)
	}
}

func TestRoundTrip(t *testing.T) {
	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			testRoundTrip(t, tc.Pattern)
		})
	}
}

func FuzzRoundTrip(f *testing.F) {
	// seed corpus with test cases
	opt := &pdf.WriterOptions{
		HumanReadable: true,
	}

	for _, tc := range testCases {
		w, buf := memfile.NewPDFWriter(pdf.V2_0, opt)

		err := memfile.AddBlankPage(w)
		if err != nil {
			continue
		}

		rm := pdf.NewResourceManager(w)

		ref, err := rm.Embed(tc.Pattern)
		if err != nil {
			continue
		}

		err = rm.Close()
		if err != nil {
			continue
		}

		w.GetMeta().Trailer["Quir:E"] = ref
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
			t.Skip("missing pattern object")
		}

		x := pdf.NewExtractor(r)
		pat, err := extract.Pattern(x, obj)
		if err != nil {
			t.Skip("malformed pattern")
		}

		// round-trip test
		testRoundTrip(t, pat)
	})
}
