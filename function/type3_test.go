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

package function

import (
	"math"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
	"seehuhn.de/go/pdf/internal/limits"
)

func TestType3BoundaryHandling(t *testing.T) {
	// These cases exercise the half-open interval logic of the stitching
	// function (PDF 32000-1:2008, section 7.10.4), including the special case
	// where XMin equals Bounds[0] and the first interval degenerates to a
	// single point.  Each sub-function is a probe with two outputs that report,
	// through the public Apply: out[0] is the selected function index (a
	// constant per function, so it jumps discontinuously at a boundary and
	// pins down the selection even there) and out[1] is the encoded input (the
	// position of x within its subdomain, mapped to [0, 1]).  Comparing these
	// against the independently known boundary rules verifies that Apply
	// selects the right function and subdomain.
	tests := []struct {
		name   string
		xMin   float64
		xMax   float64
		bounds []float64
		inputs []struct {
			input          float64
			expectedFunc   int        // which function should be selected (0-indexed)
			expectedDomain [2]float64 // expected subdomain boundaries
		}
	}{
		{
			name:   "k=2, XMin < Bounds[0] < XMax",
			xMin:   0,
			xMax:   2,
			bounds: []float64{1.0},
			inputs: []struct {
				input          float64
				expectedFunc   int
				expectedDomain [2]float64
			}{
				{0.0, 0, [2]float64{0, 1}},   // left boundary of first interval [0, 1)
				{0.5, 0, [2]float64{0, 1}},   // inside first interval
				{0.999, 0, [2]float64{0, 1}}, // just before boundary (still first interval)
				{1.0, 1, [2]float64{1, 2}},   // exactly at boundary -> second interval [1, 2]
				{1.5, 1, [2]float64{1, 2}},   // inside second interval
				{2.0, 1, [2]float64{1, 2}},   // right boundary of last interval (included)
			},
		},
		{
			name:   "k=2, XMin = Bounds[0]",
			xMin:   0,
			xMax:   2,
			bounds: []float64{0.0},
			inputs: []struct {
				input          float64
				expectedFunc   int
				expectedDomain [2]float64
			}{
				{0.0, 0, [2]float64{0, 0}},   // first interval [0, 0] (single point)
				{0.001, 1, [2]float64{0, 2}}, // second interval (0, 2] (open on left)
				{1.0, 1, [2]float64{0, 2}},   // inside second interval
				{2.0, 1, [2]float64{0, 2}},   // right boundary included in last interval
			},
		},
		{
			name:   "k=3, normal boundaries",
			xMin:   0,
			xMax:   3,
			bounds: []float64{1.0, 2.0},
			inputs: []struct {
				input          float64
				expectedFunc   int
				expectedDomain [2]float64
			}{
				{0.0, 0, [2]float64{0, 1}},   // first interval [0, 1)
				{0.999, 0, [2]float64{0, 1}}, // just before first boundary
				{1.0, 1, [2]float64{1, 2}},   // exactly at first boundary -> second interval [1, 2)
				{1.5, 1, [2]float64{1, 2}},   // inside second interval
				{1.999, 1, [2]float64{1, 2}}, // just before second boundary
				{2.0, 2, [2]float64{2, 3}},   // exactly at second boundary -> third interval [2, 3]
				{2.5, 2, [2]float64{2, 3}},   // inside third interval
				{3.0, 2, [2]float64{2, 3}},   // right boundary of last interval (included)
			},
		},
		{
			name:   "k=1, no bounds",
			xMin:   0,
			xMax:   1,
			bounds: []float64{},
			inputs: []struct {
				input          float64
				expectedFunc   int
				expectedDomain [2]float64
			}{
				{0.0, 0, [2]float64{0, 1}}, // left boundary
				{0.5, 0, [2]float64{0, 1}}, // middle
				{1.0, 0, [2]float64{0, 1}}, // right boundary
			},
		},
		{
			name:   "k=3, XMin = Bounds[0]",
			xMin:   0,
			xMax:   3,
			bounds: []float64{0.0, 2.0},
			inputs: []struct {
				input          float64
				expectedFunc   int
				expectedDomain [2]float64
			}{
				{0.0, 0, [2]float64{0, 0}},   // first interval [0, 0]
				{0.001, 1, [2]float64{0, 2}}, // second interval (0, 2)
				{2.0, 2, [2]float64{2, 3}},   // third interval [2, 3]
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// build k probe sub-functions: out[0] = function index (constant),
			// out[1] = encoded input (Encode maps each subdomain to [0, 1])
			k := len(tt.bounds) + 1
			funcs := make([]pdf.Function, k)
			encode := make([]float64, 0, 2*k)
			for i := range funcs {
				funcs[i] = &Type2{
					XMin: 0, XMax: 1,
					C0: []float64{float64(i), 0},
					C1: []float64{float64(i), 1},
					N:  1,
				}
				encode = append(encode, 0, 1)
			}
			f := &Type3{
				XMin:      tt.xMin,
				XMax:      tt.xMax,
				Functions: funcs,
				Bounds:    tt.bounds,
				Encode:    encode,
			}

			for _, tc := range tt.inputs {
				out := make([]float64, 2)
				f.Apply(out, tc.input)

				if got := int(out[0]); got != tc.expectedFunc {
					t.Errorf("input %.3f: selected function %d, want %d",
						tc.input, got, tc.expectedFunc)
				}

				// expected position of x within its subdomain, mapped to [0, 1];
				// a degenerate single-point subdomain encodes to the low end
				a, b := tc.expectedDomain[0], tc.expectedDomain[1]
				var wantEncoded float64
				if b > a {
					wantEncoded = (tc.input - a) / (b - a)
				}
				if math.Abs(out[1]-wantEncoded) > 1e-9 {
					t.Errorf("input %.3f: encoded input %.6f, want %.6f (subdomain [%.3f, %.3f])",
						tc.input, out[1], wantEncoded, a, b)
				}
			}
		})
	}
}

// TestExtractType3DeepChainBounded guards against a stack-overflow DoS: a
// chain of distinct Type 3 stitching functions, each whose /Functions holds
// the next, is acyclic, so the cycle guard never trips, yet recursing one
// frame per level would exhaust the Go stack. The Decode depth cap must
// turn this into a malformed-file error rather than a crash.
func TestExtractType3DeepChainBounded(t *testing.T) {
	depth := limits.MaxExtractDepth + 10
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)

	refs := make([]pdf.Reference, depth)
	for i := range refs {
		refs[i] = w.Alloc()
	}
	for i, ref := range refs {
		var obj pdf.Object
		if i+1 < depth {
			obj = pdf.Dict{
				"FunctionType": pdf.Integer(3),
				"Domain":       pdf.Array{pdf.Real(0), pdf.Real(1)},
				"Functions":    pdf.Array{refs[i+1]},
				"Bounds":       pdf.Array{},
				"Encode":       pdf.Array{pdf.Real(0), pdf.Real(1)},
				"Range":        pdf.Array{pdf.Real(0), pdf.Real(1)},
			}
		} else {
			obj = pdf.Dict{
				"FunctionType": pdf.Integer(2),
				"Domain":       pdf.Array{pdf.Real(0), pdf.Real(1)},
				"C0":           pdf.Array{pdf.Real(0)},
				"C1":           pdf.Array{pdf.Real(1)},
				"N":            pdf.Real(1),
			}
		}
		if err := w.Put(ref, obj); err != nil {
			t.Fatal(err)
		}
	}

	x := pdf.NewExtractor(w)
	if _, err := Extract(pdf.CursorAt(x, nil), refs[0], false); !pdf.IsMalformed(err) {
		t.Errorf("err = %v, want malformed", err)
	}
}
