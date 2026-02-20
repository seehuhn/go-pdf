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
	"testing"

	"seehuhn.de/go/pdf"
)

func TestType3BoundaryHandling(t *testing.T) {
	// Test cases based on PDF specification Section 7.10.4
	tests := []struct {
		name       string
		function   *Type3
		testInputs []struct {
			input          float64
			expectedFunc   int        // which function should be selected (0-indexed)
			expectedDomain [2]float64 // expected subdomain boundaries
		}
	}{
		{
			name: "Normal case: k=2, XMin < Bounds[0] < XMax",
			function: &Type3{
				XMin: 0,
				XMax: 2,
				Functions: []pdf.Function{
					&Type2{XMin: 0, XMax: 1, C0: []float64{0}, C1: []float64{1}, N: 1},
					&Type2{XMin: 0, XMax: 1, C0: []float64{1}, C1: []float64{0}, N: 1},
				},
				Bounds: []float64{1.0},
				Encode: []float64{0, 1, 0, 1},
			},
			testInputs: []struct {
				input          float64
				expectedFunc   int
				expectedDomain [2]float64
			}{
				{0.0, 0, [2]float64{0, 1}},   // left boundary of first interval [0, 1)
				{0.5, 0, [2]float64{0, 1}},   // inside first interval
				{0.999, 0, [2]float64{0, 1}}, // just before boundary (should be in first interval)
				{1.0, 1, [2]float64{1, 2}},   // exactly at boundary (should be in second interval) [1, 2]
				{1.5, 1, [2]float64{1, 2}},   // inside second interval
				{2.0, 1, [2]float64{1, 2}},   // right boundary of last interval (should be included)
			},
		},
		{
			name: "Special case: k=2, XMin = Bounds[0]",
			function: &Type3{
				XMin: 0,
				XMax: 2,
				Functions: []pdf.Function{
					&Type2{XMin: 0, XMax: 1, C0: []float64{0}, C1: []float64{1}, N: 1},
					&Type2{XMin: 0, XMax: 1, C0: []float64{1}, C1: []float64{0}, N: 1},
				},
				Bounds: []float64{0.0}, // XMin = Bounds[0]
				Encode: []float64{0, 1, 0, 1},
			},
			testInputs: []struct {
				input          float64
				expectedFunc   int
				expectedDomain [2]float64
			}{
				{0.0, 0, [2]float64{0, 0}},   // Special case: first interval [0, 0] (closed on both sides)
				{0.001, 1, [2]float64{0, 2}}, // Second interval (0, 2] (open on left)
				{1.0, 1, [2]float64{0, 2}},   // Inside second interval
				{2.0, 1, [2]float64{0, 2}},   // Right boundary included in last interval
			},
		},
		{
			name: "Three functions: k=3, normal boundaries",
			function: &Type3{
				XMin: 0,
				XMax: 3,
				Functions: []pdf.Function{
					&Type2{XMin: 0, XMax: 1, C0: []float64{0}, C1: []float64{1}, N: 1},
					&Type2{XMin: 0, XMax: 1, C0: []float64{1}, C1: []float64{0}, N: 1},
					&Type2{XMin: 0, XMax: 1, C0: []float64{0}, C1: []float64{1}, N: 2},
				},
				Bounds: []float64{1.0, 2.0},
				Encode: []float64{0, 1, 0, 1, 0, 1},
			},
			testInputs: []struct {
				input          float64
				expectedFunc   int
				expectedDomain [2]float64
			}{
				{0.0, 0, [2]float64{0, 1}},   // First interval [0, 1)
				{0.999, 0, [2]float64{0, 1}}, // Just before first boundary
				{1.0, 1, [2]float64{1, 2}},   // Exactly at first boundary -> second interval [1, 2)
				{1.5, 1, [2]float64{1, 2}},   // Inside second interval
				{1.999, 1, [2]float64{1, 2}}, // Just before second boundary
				{2.0, 2, [2]float64{2, 3}},   // Exactly at second boundary -> third interval [2, 3]
				{2.5, 2, [2]float64{2, 3}},   // Inside third interval
				{3.0, 2, [2]float64{2, 3}},   // Right boundary of last interval (included)
			},
		},
		{
			name: "Single function: k=1, no bounds",
			function: &Type3{
				XMin: 0,
				XMax: 1,
				Functions: []pdf.Function{
					&Type2{XMin: 0, XMax: 1, C0: []float64{0}, C1: []float64{1}, N: 1},
				},
				Bounds: []float64{}, // No bounds for single function
				Encode: []float64{0, 1},
			},
			testInputs: []struct {
				input          float64
				expectedFunc   int
				expectedDomain [2]float64
			}{
				{0.0, 0, [2]float64{0, 1}}, // Left boundary
				{0.5, 0, [2]float64{0, 1}}, // Middle
				{1.0, 0, [2]float64{0, 1}}, // Right boundary
			},
		},
		{
			name: "Special case: k=3, XMin = Bounds[0]",
			function: &Type3{
				XMin: 0,
				XMax: 3,
				Functions: []pdf.Function{
					&Type2{XMin: 0, XMax: 1, C0: []float64{0.2}, C1: []float64{0.2}, N: 1},
					&Type2{XMin: 0, XMax: 1, C0: []float64{0.5}, C1: []float64{0.5}, N: 1},
					&Type2{XMin: 0, XMax: 1, C0: []float64{0.9}, C1: []float64{0.9}, N: 1}},
				Bounds: []float64{0.0, 2.0}, // XMin = Bounds[0]
				Encode: []float64{0, 1, 0, 1, 0, 1},
			},
			testInputs: []struct {
				input          float64
				expectedFunc   int
				expectedDomain [2]float64
			}{
				{0.0, 0, [2]float64{0, 0}},   // First interval [0, 0]
				{0.001, 1, [2]float64{0, 2}}, // Second interval (0, 2)
				{2.0, 2, [2]float64{2, 3}},   // Third interval [2, 3]
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, test := range tt.testInputs {
				actualFunc, a, b := tt.function.findSubdomain(test.input)

				if actualFunc != test.expectedFunc {
					t.Errorf("input %.3f: expected function %d, got %d",
						test.input, test.expectedFunc, actualFunc)
				}

				if [2]float64{a, b} != test.expectedDomain {
					t.Errorf("input %.3f: expected domain [%.3f, %.3f], got [%.3f, %.3f]",
						test.input, test.expectedDomain[0], test.expectedDomain[1],
						a, b)
				}
			}
		})
	}
}

func TestType3ApplyWithBoundaries(t *testing.T) {
	// Test that Apply method correctly handles boundary values
	// This tests the complete workflow including encoding

	tests := []struct {
		name     string
		function *Type3
		input    float64
		// We'll verify the function selection is correct by checking
		// which underlying Type2 function gets used
	}{
		{
			name: "Boundary value at 1.0 should use second function",
			function: &Type3{
				XMin: 0,
				XMax: 2,
				Functions: []pdf.Function{
					&Type2{XMin: 0, XMax: 1, C0: []float64{0}, C1: []float64{0}, N: 1}, // Always returns 0
					&Type2{XMin: 0, XMax: 1, C0: []float64{1}, C1: []float64{1}, N: 1}, // Always returns 1
				},
				Bounds: []float64{1.0},
				Encode: []float64{0, 1, 0, 1},
			},
			input: 1.0,
			// If findSubdomain is correct, x=1.0 should select function 1 (returns 1.0)
			// If incorrect, it might select function 0 (returns 0.0)
		},
		{
			name: "Special case XMin = Bounds[0], test boundary",
			function: &Type3{
				XMin: 0,
				XMax: 1,
				Functions: []pdf.Function{
					&Type2{XMin: 0, XMax: 1, C0: []float64{0.5}, C1: []float64{0.5}, N: 1}, // Always returns 0.5
					&Type2{XMin: 0, XMax: 1, C0: []float64{0.8}, C1: []float64{0.8}, N: 1}, // Always returns 0.8
				},
				Bounds: []float64{0.0}, // XMin = Bounds[0]
				Encode: []float64{0, 1, 0, 1},
			},
			input: 0.0,
			// In special case, x=0.0 should be in first interval [0,0] and use function 0
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := make([]float64, 1)
			tt.function.Apply(result, tt.input)

			// For the first test case, we expect result[0] = 1.0 if boundary handling is correct
			// For the second test case, we expect result[0] = 0.5 if special case is handled correctly
			expectedResult := map[string]float64{
				"Boundary value at 1.0 should use second function": 1.0,
				"Special case XMin = Bounds[0], test boundary":     0.5,
			}[tt.name]

			if len(result) != 1 {
				t.Fatalf("expected 1 output, got %d", len(result))
			}

			if result[0] != expectedResult {
				t.Errorf("input %.3f: expected %.3f, got %.3f (this indicates incorrect function selection)",
					tt.input, expectedResult, result[0])
			}
		})
	}
}
