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
	"bytes"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

// testCases holds test cases for all function types, indexed by type
var testCases = map[int][]testCase{
	0: {
		{
			name: "basic Type0 8-bit",
			function: &Type0{
				Domain:        []float64{0, 1, 0, 1},
				Range:         []float64{0, 1, 0, 1, 0, 1},
				Size:          []int{2, 2},
				BitsPerSample: 8,
				Order:         1,
				Samples:       []byte{0, 128, 255, 64, 192, 32, 255, 0, 128, 128, 64, 96},
			},
		},
		{
			name: "Type0 with encode/decode",
			function: &Type0{
				Domain:        []float64{-1, 1},
				Range:         []float64{0, 1},
				Size:          []int{4},
				BitsPerSample: 8,
				Order:         1,
				Encode:        []float64{0, 3},
				Decode:        []float64{0, 1},
				Samples:       []byte{0, 85, 170, 255},
			},
		},
		{
			name: "Type0 16-bit samples",
			function: &Type0{
				Domain:        []float64{0, 1},
				Range:         []float64{0, 1},
				Size:          []int{3},
				BitsPerSample: 16,
				Order:         1,
				Samples:       []byte{0, 0, 128, 0, 255, 255},
			},
		},
		{
			name: "Type0 cubic interpolation",
			function: &Type0{
				Domain:        []float64{0, 1},
				Range:         []float64{0, 1},
				Size:          []int{5},
				BitsPerSample: 8,
				Order:         3,
				Samples:       []byte{0, 64, 128, 192, 255},
			},
		},
	},
	2: {
		{
			name: "basic Type2",
			function: &Type2{
				Domain: []float64{0, 1},
				C0:     []float64{0.0},
				C1:     []float64{1.0},
				N:      1.0,
			},
		},
		{
			name: "Type2 with range",
			function: &Type2{
				Domain: []float64{0, 1},
				Range:  []float64{0, 1, 0, 1, 0, 1},
				C0:     []float64{1, 0, 0},
				C1:     []float64{0, 1, 0},
				N:      2.0,
			},
		},
		{
			name: "Type2 exponential",
			function: &Type2{
				Domain: []float64{0, 1},
				C0:     []float64{0.1, 0.2},
				C1:     []float64{0.9, 0.8},
				N:      0.5,
			},
		},
		{
			name: "Type2 linear interpolation",
			function: &Type2{
				Domain: []float64{0, 1},
				C0:     []float64{0.2, 0.4, 0.6},
				C1:     []float64{0.8, 0.6, 0.4},
				N:      1.0,
			},
		},
		{
			name: "Type2 minimal",
			function: &Type2{
				Domain: []float64{0, 1},
				N:      1.0,
			},
		},
	},
	3: {
		{
			name: "basic Type3",
			function: &Type3{
				Domain: []float64{0, 1},
				Functions: []pdf.Function{
					&Type2{
						Domain: []float64{0, 1},
						C0:     []float64{1, 0, 0},
						C1:     []float64{0, 1, 0},
						N:      1.0,
					},
					&Type2{
						Domain: []float64{0, 1},
						C0:     []float64{0, 1, 0},
						C1:     []float64{0, 0, 1},
						N:      1.0,
					},
				},
				Bounds: []float64{0.5},
				Encode: []float64{0, 1, 0, 1},
			},
		},
		{
			name: "Type3 with range",
			function: &Type3{
				Domain: []float64{0, 2},
				Range:  []float64{0, 1},
				Functions: []pdf.Function{
					&Type2{
						Domain: []float64{0, 1},
						C0:     []float64{0.0},
						C1:     []float64{0.5},
						N:      1.0,
					},
					&Type2{
						Domain: []float64{0, 1},
						C0:     []float64{0.5},
						C1:     []float64{1.0},
						N:      1.0,
					},
				},
				Bounds: []float64{1.0},
				Encode: []float64{0, 1, 0, 1},
			},
		},
		{
			name: "Type3 three functions",
			function: &Type3{
				Domain: []float64{0, 3},
				Functions: []pdf.Function{
					&Type2{Domain: []float64{0, 1}, C0: []float64{0}, C1: []float64{1}, N: 1},
					&Type2{Domain: []float64{0, 1}, C0: []float64{1}, C1: []float64{0}, N: 1},
					&Type2{Domain: []float64{0, 1}, C0: []float64{0}, C1: []float64{1}, N: 2},
				},
				Bounds: []float64{1.0, 2.0},
				Encode: []float64{0, 1, 0, 1, 0, 1},
			},
		},
	},
	4: {
		{
			name: "basic Type4 add",
			function: &Type4{
				Domain:  []float64{0, 1, 0, 1},
				Range:   []float64{0, 2},
				Program: "add",
			},
		},
		{
			name: "Type4 multiply",
			function: &Type4{
				Domain:  []float64{0, 1, 0, 1},
				Range:   []float64{0, 1},
				Program: "mul",
			},
		},
		{
			name: "Type4 comparison",
			function: &Type4{
				Domain:  []float64{0, 1},
				Range:   []float64{0, 1},
				Program: "0.5 gt",
			},
		},
		{
			name: "Type4 arithmetic operations",
			function: &Type4{
				Domain:  []float64{0, 10},
				Range:   []float64{0, 100},
				Program: "dup mul",
			},
		},
		{
			name: "Type4 trigonometric",
			function: &Type4{
				Domain:  []float64{0, 6.28318},
				Range:   []float64{-1, 1},
				Program: "sin",
			},
		},
		{
			name: "Type4 stack operations",
			function: &Type4{
				Domain:  []float64{0, 1, 0, 1},
				Range:   []float64{0, 1, 0, 1},
				Program: "exch dup",
			},
		},
	},
}

type testCase struct {
	name     string
	function pdf.Function
}

func TestRoundTrip(t *testing.T) {
	for functionType, cases := range testCases {
		for _, tc := range cases {
			t.Run(fmt.Sprintf("Type%d-%s", functionType, tc.name), func(t *testing.T) {
				roundTripTest(t, tc.function)
			})
		}
	}
}

// roundTripTest performs a round-trip test for any function type
func roundTripTest(t *testing.T, originalFunction pdf.Function) {
	buf, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	rm := pdf.NewResourceManager(buf)

	// Embed the function
	embedded, _, err := originalFunction.Embed(rm)
	if err != nil {
		t.Fatal(err)
	}

	ref := buf.Alloc()
	err = buf.Put(ref, embedded)
	if err != nil {
		t.Fatal(err)
	}

	err = buf.Close()
	if err != nil {
		t.Fatal(err)
	}

	// Read the function back
	readFunction, err := Read(buf, ref)
	if err != nil {
		t.Fatal(err)
	}

	// Check that the types match
	if readFunction.FunctionType() != originalFunction.FunctionType() {
		t.Fatalf("function type mismatch: expected %d, got %d",
			originalFunction.FunctionType(), readFunction.FunctionType())
	}

	// Check that shapes match
	origM, origN := originalFunction.Shape()
	readM, readN := readFunction.Shape()
	if origM != readM || origN != readN {
		t.Fatalf("function shape mismatch: expected (%d,%d), got (%d,%d)",
			origM, origN, readM, readN)
	}

	// Use cmp.Diff to compare the original and read function
	if diff := cmp.Diff(originalFunction, readFunction); diff != "" {
		t.Errorf("round trip failed (-want +got):\n%s", diff)
	}
}

func TestFunctionEvaluation(t *testing.T) {
	tests := []struct {
		name      string
		function  pdf.Function
		inputs    []float64
		expected  []float64
		tolerance float64
	}{
		{
			name: "Type2 linear",
			function: &Type2{
				Domain: []float64{0, 1},
				C0:     []float64{0.0},
				C1:     []float64{1.0},
				N:      1.0,
			},
			inputs:    []float64{0.5},
			expected:  []float64{0.5},
			tolerance: 1e-10,
		},
		{
			name: "Type2 quadratic",
			function: &Type2{
				Domain: []float64{0, 1},
				C0:     []float64{0.0},
				C1:     []float64{1.0},
				N:      2.0,
			},
			inputs:    []float64{0.5},
			expected:  []float64{0.25},
			tolerance: 1e-10,
		},
		{
			name: "Type2 multi-output",
			function: &Type2{
				Domain: []float64{0, 1},
				C0:     []float64{1.0, 0.0, 0.0},
				C1:     []float64{0.0, 1.0, 0.0},
				N:      1.0,
			},
			inputs:    []float64{0.5},
			expected:  []float64{0.5, 0.5, 0.0},
			tolerance: 1e-10,
		},
		{
			name: "Type4 add",
			function: &Type4{
				Domain:  []float64{0, 1, 0, 1},
				Range:   []float64{0, 2},
				Program: "add",
			},
			inputs:    []float64{0.3, 0.7},
			expected:  []float64{1.0},
			tolerance: 1e-10,
		},
		{
			name: "Type4 multiply",
			function: &Type4{
				Domain:  []float64{0, 1, 0, 1},
				Range:   []float64{0, 1},
				Program: "mul",
			},
			inputs:    []float64{0.5, 0.8},
			expected:  []float64{0.4},
			tolerance: 1e-10,
		},
		{
			name: "Type4 simple greater than",
			function: &Type4{
				Domain:  []float64{0, 1},
				Range:   []float64{0, 1},
				Program: "0.5 gt",
			},
			inputs:    []float64{0.7},
			expected:  []float64{1.0},
			tolerance: 1e-10,
		},
		{
			name: "Type4 simple less than",
			function: &Type4{
				Domain:  []float64{0, 1},
				Range:   []float64{0, 1},
				Program: "0.5 gt",
			},
			inputs:    []float64{0.3},
			expected:  []float64{0.0},
			tolerance: 1e-10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.function.Apply(tt.inputs...)
			if len(result) != len(tt.expected) {
				t.Fatalf("expected %d outputs, got %d", len(tt.expected), len(result))
			}
			for i, expected := range tt.expected {
				if abs(result[i]-expected) > tt.tolerance {
					t.Errorf("output[%d]: expected %f, got %f (diff: %e)",
						i, expected, result[i], abs(result[i]-expected))
				}
			}
		})
	}
}

func TestFunctionValidation(t *testing.T) {
	tests := []struct {
		name     string
		function interface{ validate() error }
		wantErr  bool
	}{
		{
			name: "valid Type0",
			function: &Type0{
				Domain:        []float64{0, 1},
				Range:         []float64{0, 1},
				Size:          []int{2},
				BitsPerSample: 8,
				Order:         1, // Add valid order
				Samples:       []byte{0, 255},
			},
			wantErr: false,
		},
		{
			name: "Type0 invalid bits per sample",
			function: &Type0{
				Domain:        []float64{0, 1},
				Range:         []float64{0, 1},
				Size:          []int{2},
				BitsPerSample: 7, // Invalid
			},
			wantErr: true,
		},
		{
			name: "Type0 size mismatch",
			function: &Type0{
				Domain: []float64{0, 1, 0, 1}, // 2 inputs
				Range:  []float64{0, 1},       // 1 output
				Size:   []int{2},              // Only 1 dimension specified
			},
			wantErr: true,
		},
		{
			name: "valid Type2",
			function: &Type2{
				Domain: []float64{0, 1},
				C0:     []float64{0.0},
				C1:     []float64{1.0},
				N:      1.0,
			},
			wantErr: false,
		},
		{
			name: "Type2 C0/C1 length mismatch",
			function: &Type2{
				Domain: []float64{0, 1},
				C0:     []float64{0.0, 0.0},
				C1:     []float64{1.0},
				N:      1.0,
			},
			wantErr: true,
		},
		{
			name: "Type2 negative domain with non-integer N",
			function: &Type2{
				Domain: []float64{-1, 1},
				C0:     []float64{0.0},
				C1:     []float64{1.0},
				N:      0.5, // Non-integer with negative domain
			},
			wantErr: true,
		},
		{
			name: "valid Type3",
			function: &Type3{
				Domain: []float64{0, 1},
				Functions: []pdf.Function{
					&Type2{Domain: []float64{0, 1}, C0: []float64{0}, C1: []float64{1}, N: 1},
				},
				Bounds: []float64{},
				Encode: []float64{0, 1},
			},
			wantErr: false,
		},
		{
			name: "Type3 bounds count mismatch",
			function: &Type3{
				Domain: []float64{0, 1},
				Functions: []pdf.Function{
					&Type2{Domain: []float64{0, 1}, C0: []float64{0}, C1: []float64{1}, N: 1},
					&Type2{Domain: []float64{0, 1}, C0: []float64{0}, C1: []float64{1}, N: 1},
				},
				Bounds: []float64{}, // Should have 1 bound for 2 functions
				Encode: []float64{0, 1, 0, 1},
			},
			wantErr: true,
		},
		{
			name: "valid Type4",
			function: &Type4{
				Domain:  []float64{0, 1},
				Range:   []float64{0, 1},
				Program: "dup mul",
			},
			wantErr: false,
		},
		{
			name: "Type4 empty program",
			function: &Type4{
				Domain:  []float64{0, 1},
				Range:   []float64{0, 1},
				Program: "",
			},
			wantErr: true,
		},
		{
			name: "Type4 unbalanced braces",
			function: &Type4{
				Domain:  []float64{0, 1},
				Range:   []float64{0, 1},
				Program: "{ dup mul",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.function.validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDomainRangeClipping(t *testing.T) {
	tests := []struct {
		name     string
		function pdf.Function
		inputs   []float64
		expected []float64
	}{
		{
			name: "Type2 input clipping",
			function: &Type2{
				Domain: []float64{0, 1},
				Range:  []float64{0, 1},
				C0:     []float64{0.0},
				C1:     []float64{1.0},
				N:      1.0,
			},
			inputs:   []float64{-0.5}, // Below domain
			expected: []float64{0.0},  // Should clip to domain min and evaluate
		},
		{
			name: "Type2 input clipping upper",
			function: &Type2{
				Domain: []float64{0, 1},
				Range:  []float64{0, 1},
				C0:     []float64{0.0},
				C1:     []float64{1.0},
				N:      1.0,
			},
			inputs:   []float64{1.5}, // Above domain
			expected: []float64{1.0}, // Should clip to domain max and evaluate
		},
		{
			name: "Type2 output clipping",
			function: &Type2{
				Domain: []float64{0, 1},
				Range:  []float64{0.2, 0.8}, // Restricted output range
				C0:     []float64{0.0},      // Would normally give 0.0
				C1:     []float64{1.0},      // Would normally give 1.0
				N:      1.0,
			},
			inputs:   []float64{0.0},
			expected: []float64{0.2}, // Should clip to range min
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.function.Apply(tt.inputs...)
			if len(result) != len(tt.expected) {
				t.Fatalf("expected %d outputs, got %d", len(tt.expected), len(result))
			}
			for i, expected := range tt.expected {
				if abs(result[i]-expected) > 1e-10 {
					t.Errorf("output[%d]: expected %f, got %f", i, expected, result[i])
				}
			}
		})
	}
}

func FuzzRead(f *testing.F) {
	// Seed the fuzzer with valid test cases from all function types
	for _, cases := range testCases {
		for _, tc := range cases {
			out := memfile.New()
			opt := &pdf.WriterOptions{
				HumanReadable: true,
			}
			w, err := pdf.NewWriter(out, pdf.V2_0, opt)
			if err != nil {
				f.Fatal(err)
			}
			rm := pdf.NewResourceManager(w)

			ref := w.Alloc()

			embedded, _, err := tc.function.Embed(rm)
			if err != nil {
				continue
			}

			err = w.Put(ref, embedded)
			if err != nil {
				continue
			}

			err = rm.Close()
			if err != nil {
				continue
			}

			w.GetMeta().Trailer["Quir:X"] = ref

			err = w.Close()
			if err != nil {
				continue
			}

			f.Add(out.Data)
		}
	}

	f.Fuzz(func(t *testing.T, fileData []byte) {
		// Get a "random" function from the PDF file.

		// Make sure we don't panic on random input.
		opt := &pdf.ReaderOptions{
			ErrorHandling: pdf.ErrorHandlingReport,
		}
		r, err := pdf.NewReader(bytes.NewReader(fileData), opt)
		if err != nil {
			t.Skip("broken PDF: " + err.Error())
		}
		obj := r.GetMeta().Trailer["Quir:X"]
		if obj == nil {
			t.Skip("broken reference")
		}
		function, err := Read(r, obj)
		if err != nil {
			t.Skip("broken function")
		}

		// Make sure we can write the function, and read it back.
		roundTripTest(t, function)

		// Test function evaluation doesn't panic
		m, n := function.Shape()
		if m > 0 && m <= 10 { // Reasonable input size
			inputs := make([]float64, m)
			for i := range inputs {
				inputs[i] = 0.5 // Use middle values
			}
			outputs := function.Apply(inputs...)
			if len(outputs) != n {
				t.Errorf("expected %d outputs, got %d", n, len(outputs))
			}
		}
	})
}

func FuzzApply(f *testing.F) {
	// Seed the fuzzer with known function types and test inputs
	f.Add(2, 0.5) // Type2 with single input
	f.Add(4, 0.3) // Type4 with single input
	f.Add(2, 0.0) // Edge case: domain minimum
	f.Add(2, 1.0) // Edge case: domain maximum

	f.Fuzz(func(t *testing.T, functionType int, input1 float64) {
		// Create a simple function of the specified type
		var fn pdf.Function
		switch functionType {
		case 2:
			fn = &Type2{
				Domain: []float64{0, 1},
				C0:     []float64{0},
				C1:     []float64{1},
				N:      1.0,
			}
		case 4:
			fn = &Type4{
				Domain:  []float64{0, 1},
				Range:   []float64{0, 1},
				Program: "0.5", // Simple safe program that just pushes a constant
			}
		default:
			t.Skip("unsupported function type for fuzzing")
		}

		// Test that Apply doesn't panic and returns correct number of outputs
		m, n := fn.Shape()
		inputs := []float64{input1}
		if m != 1 {
			t.Skip("function doesn't have single input")
		}

		outputs := fn.Apply(inputs...)
		if len(outputs) != n {
			t.Errorf("expected %d outputs, got %d", n, len(outputs))
		}

		// Test that all outputs are finite numbers
		for i, output := range outputs {
			if !isFinite(output) {
				t.Errorf("output[%d] is not finite: %v", i, output)
			}
		}
	})
}

// Helper functions
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

func isFinite(x float64) bool {
	return !((x != x) || (x > 1e100) || (x < -1e100))
}
