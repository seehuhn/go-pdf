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

package pattern

import (
	"bytes"
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/function"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/shading"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

type testCase struct {
	Name    string
	Pattern color.Pattern
}

var testCases = []testCase{
	{
		Name: "Type1-Colored",
		Pattern: &Type1{
			TilingType: 1,
			BBox:       &pdf.Rectangle{LLx: 0, LLy: 0, URx: 10, URy: 10},
			XStep:      10,
			YStep:      10,
			Color:      true,
			Draw: func(w *graphics.Writer) error {
				w.SetFillColor(color.DeviceGray(0.5))
				w.Rectangle(0, 0, 5, 5)
				w.Fill()
				return nil
			},
		},
	},
	{
		Name: "Type1-Uncolored",
		Pattern: &Type1{
			TilingType: 2,
			BBox:       &pdf.Rectangle{LLx: 0, LLy: 0, URx: 20, URy: 20},
			XStep:      20,
			YStep:      20,
			Color:      false,
			Draw: func(w *graphics.Writer) error {
				w.Rectangle(5, 5, 10, 10)
				w.Fill()
				return nil
			},
		},
	},
	{
		Name: "Type2-Shading",
		Pattern: &Type2{
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
	pat1, err := Extract(x1, ref1)
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
	pat2, err := Extract(x2, ref2)
	if err != nil {
		t.Fatalf("second extract failed: %v", err)
	}

	// compare extracted patterns (excluding Draw function for Type1)
	opts := cmp.Options{
		cmp.FilterPath(func(p cmp.Path) bool {
			return p.String() == "Draw"
		}, cmp.Ignore()),
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

		w.GetMeta().Trailer["Quir:Pat"] = ref
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

		obj := r.GetMeta().Trailer["Quir:Pat"]
		if obj == nil {
			t.Skip("missing pattern object")
		}

		x := pdf.NewExtractor(r)
		pat, err := Extract(x, obj)
		if err != nil {
			t.Skip("malformed pattern")
		}

		// round-trip test
		testRoundTrip(t, pat)
	})
}
