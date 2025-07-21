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
)

// TestType4NewOperators demonstrates all the newly implemented PostScript operators.
func TestType4NewOperators(t *testing.T) {
	tests := []struct {
		name     string
		program  string
		inputs   []float64
		expected []float64
	}{
		{
			name:     "atan - arctangent",
			program:  "1 atan",
			inputs:   []float64{1.0},
			expected: []float64{math.Atan(1.0) * 180.0 / math.Pi}, // atan(1) converted to degrees
		},
		{
			name:     "bitshift left",
			program:  "cvi 3 bitshift",
			inputs:   []float64{7},
			expected: []float64{float64(7 << 3)}, // 7 << 3
		},
		{
			name:     "bitshift right",
			program:  "cvi -3 bitshift",
			inputs:   []float64{142},
			expected: []float64{float64(142 >> 3)}, // 142 >> 3
		},
		{
			name:     "ceiling",
			program:  "ceiling",
			inputs:   []float64{3.2},
			expected: []float64{math.Ceil(3.2)},
		},
		{
			name:     "cos - cosine",
			program:  "cos",
			inputs:   []float64{0},
			expected: []float64{math.Cos(0 * math.Pi / 180.0)}, // cos(0°) converted from degrees
		},
		{
			name:     "cvi - convert to integer",
			program:  "cvi",
			inputs:   []float64{3.7},
			expected: []float64{math.Trunc(3.7)}, // truncate to integer part
		},
		{
			name:     "cvr - convert to real",
			program:  "cvr",
			inputs:   []float64{42},
			expected: []float64{float64(42)}, // convert to real
		},
		{
			name:     "div - real division",
			program:  "div",
			inputs:   []float64{3, 2},
			expected: []float64{3.0 / 2.0},
		},
		{
			name:     "exp - exponentiation",
			program:  "0.5 exp",
			inputs:   []float64{9},
			expected: []float64{math.Pow(9, 0.5)}, // 9^0.5
		},
		{
			name:     "floor",
			program:  "floor",
			inputs:   []float64{3.7},
			expected: []float64{math.Floor(3.7)},
		},
		{
			name:    "ge - greater than or equal",
			program: "4 ge",
			inputs:  []float64{4.2},
			expected: func() []float64 {
				if 4.2 >= 4 {
					return []float64{1}
				} else {
					return []float64{0}
				}
			}(),
		},
		{
			name:    "gt - greater than",
			program: "4 gt",
			inputs:  []float64{5},
			expected: func() []float64 {
				if 5 > 4 {
					return []float64{1}
				} else {
					return []float64{0}
				}
			}(),
		},
		{
			name:     "idiv - integer division",
			program:  "cvi 2 idiv",
			inputs:   []float64{5},
			expected: []float64{float64(int(5) / 2)}, // integer division
		},
		{
			name:    "le - less than or equal",
			program: "4 le",
			inputs:  []float64{3},
			expected: func() []float64 {
				if 3 <= 4 {
					return []float64{1}
				} else {
					return []float64{0}
				}
			}(),
		},
		{
			name:     "ln - natural logarithm",
			program:  "ln",
			inputs:   []float64{math.E},
			expected: []float64{math.Log(math.E)}, // ln(e) = 1
		},
		{
			name:     "log - base 10 logarithm",
			program:  "log",
			inputs:   []float64{100},
			expected: []float64{math.Log10(100)}, // log₁₀(100) = 2
		},
		{
			name:    "lt - less than",
			program: "4 lt",
			inputs:  []float64{3},
			expected: func() []float64 {
				if 3 < 4 {
					return []float64{1}
				} else {
					return []float64{0}
				}
			}(),
		},
		{
			name:     "mod - modulo",
			program:  "cvi 3 mod",
			inputs:   []float64{5},
			expected: []float64{float64(5 % 3)}, // 5 mod 3 = 2
		},
		{
			name:     "neg - negation",
			program:  "neg",
			inputs:   []float64{4.5},
			expected: []float64{-4.5},
		},
		{
			name:     "round",
			program:  "round",
			inputs:   []float64{3.7},
			expected: []float64{math.Round(3.7)},
		},
		{
			name:     "sin - sine",
			program:  "sin",
			inputs:   []float64{90},
			expected: []float64{math.Sin(90 * math.Pi / 180.0)}, // sin(90°) converted from degrees
		},
		{
			name:     "sqrt - square root",
			program:  "sqrt",
			inputs:   []float64{16},
			expected: []float64{math.Sqrt(16)},
		},
		{
			name:     "truncate",
			program:  "truncate",
			inputs:   []float64{3.7},
			expected: []float64{math.Trunc(3.7)},
		},
		{
			name:     "xor - exclusive or",
			program:  "cvi 3 xor",
			inputs:   []float64{7},
			expected: []float64{float64(7 ^ 3)}, // 7 XOR 3
		},
		{
			name:     "complex expression",
			program:  "dup dup mul exch 2 mul add sqrt", // sqrt(x^2 + 2x) where x=3 gives sqrt(15)
			inputs:   []float64{3},
			expected: []float64{math.Sqrt(15)},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fn := &Type4{
				Domain:  make([]float64, len(tt.inputs)*2),
				Range:   make([]float64, len(tt.expected)*2),
				Program: tt.program,
			}

			// Set domain to accommodate inputs
			for i := range tt.inputs {
				fn.Domain[2*i] = -1000
				fn.Domain[2*i+1] = 1000
			}

			// Set range to accommodate outputs
			for i := range tt.expected {
				fn.Range[2*i] = -1000
				fn.Range[2*i+1] = 1000
			}

			result := fn.Apply(tt.inputs...)
			if len(result) != len(tt.expected) {
				t.Fatalf("expected %d outputs, got %d", len(tt.expected), len(result))
			}

			for i, expected := range tt.expected {
				tolerance := 1e-10
				if tt.name == "complex expression" {
					tolerance = 1e-10
				}
				if math.Abs(result[i]-expected) > tolerance {
					t.Errorf("output[%d]: expected %f, got %f (diff: %e)",
						i, expected, result[i], math.Abs(result[i]-expected))
				}
			}
		})
	}
}

// TestType4DoubleDotSpotFunction demonstrates the DoubleDot spot function from the PDF spec.
func TestType4DoubleDotSpotFunction(t *testing.T) {
	// This is the exact example from PDF specification section 7.10.5.3
	fn := &Type4{
		Domain:  []float64{-1.0, 1.0, -1.0, 1.0},
		Range:   []float64{-1.0, 1.0},
		Program: "360 mul sin 2 div exch 360 mul sin 2 div add",
	}

	// Test a few points - note that PDF Type 4 sin works in degrees
	// Function: sin(360*x)/2 + sin(360*y)/2
	testCases := []struct {
		x, y     float64
		expected float64
	}{
		{
			x: 0.0, y: 0.0,
			expected: math.Sin(360*0.0*math.Pi/180.0)/2 + math.Sin(360*0.0*math.Pi/180.0)/2,
		},
		{
			x: 0.5, y: 0.0,
			expected: math.Sin(360*0.5*math.Pi/180.0)/2 + math.Sin(360*0.0*math.Pi/180.0)/2,
		},
		{
			x: 1.0, y: 1.0,
			expected: math.Sin(360*1.0*math.Pi/180.0)/2 + math.Sin(360*1.0*math.Pi/180.0)/2,
		},
	}

	for _, tc := range testCases {
		result := fn.Apply(tc.x, tc.y)
		if len(result) != 1 {
			t.Fatalf("expected 1 output, got %d", len(result))
		}

		tolerance := 1e-10
		if math.Abs(result[0]-tc.expected) > tolerance {
			t.Errorf("DoubleDot(%f, %f): expected %f, got %f",
				tc.x, tc.y, tc.expected, result[0])
		}
	}
}

// TestType4PDFSpecExamples tests examples from the PDF specification.
func TestType4PDFSpecExamples(t *testing.T) {
	tests := []struct {
		name     string
		function *Type4
		inputs   []float64
		expected []float64
	}{
		{
			name: "Simple addition from spec",
			function: &Type4{
				Domain:  []float64{0, 1, 0, 1},
				Range:   []float64{0, 2},
				Program: "add",
			},
			inputs:   []float64{0.3, 0.7},
			expected: []float64{0.3 + 0.7}, // simple addition
		},
		{
			name: "Conditional operator example",
			function: &Type4{
				Domain:  []float64{0, 1},
				Range:   []float64{0, 1},
				Program: "dup 0.5 gt { pop 1 } { pop 0 } ifelse",
			},
			inputs: []float64{0.7},
			expected: func() []float64 {
				if 0.7 > 0.5 {
					return []float64{1.0}
				} else {
					return []float64{0.0}
				}
			}(),
		},
		{
			name: "Stack manipulation example",
			function: &Type4{
				Domain:  []float64{0, 1, 0, 1},
				Range:   []float64{0, 1, 0, 1},
				Program: "exch", // x y -> y x (exchange top two elements)
			},
			inputs:   []float64{0.3, 0.7},
			expected: []float64{0.7, 0.3}, // After exch: 0.7, 0.3
		},
		{
			name: "Arithmetic expression from spec",
			function: &Type4{
				Domain:  []float64{0, 10},
				Range:   []float64{0, 100},
				Program: "dup mul", // square the input
			},
			inputs:   []float64{5},
			expected: []float64{5 * 5}, // 5²
		},
		{
			name: "Complex trig example from spec notes",
			function: &Type4{
				Domain:  []float64{0, 6.28318}, // 0 to 2π
				Range:   []float64{-1, 1},
				Program: "57.2958 mul sin", // Convert radians to degrees (57.2958 ≈ 180/π) then sin
			},
			inputs:   []float64{math.Pi / 2}, // π/2 radians
			expected: []float64{1.0},         // sin(90°) = 1
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.function.Apply(tt.inputs...)
			if len(result) != len(tt.expected) {
				t.Fatalf("expected %d outputs, got %d", len(tt.expected), len(result))
			}

			for i, expected := range tt.expected {
				tolerance := 1e-10
				if tt.name == "Complex trig example from spec notes" {
					tolerance = 1e-4 // More lenient for floating point trig
				}
				if math.Abs(result[i]-expected) > tolerance {
					t.Errorf("output[%d]: expected %f, got %f (diff: %e)",
						i, expected, result[i], math.Abs(result[i]-expected))
				}
			}
		})
	}
}

// TestType4Empty tests that Type4 functions with no inputs or outputs
// are handled correctly.
func TestType4Empty(t *testing.T) {
	fn := &Type4{}

	result := fn.Apply()
	if len(result) != 0 {
		t.Fatalf("expected no outputs, got %d", len(result))
	}
}

// TestType4Constant tests that a Type4 function with no inputs and one output
// returns the constant value.
func TestType4Constant(t *testing.T) {
	fn := &Type4{
		Domain:  []float64{},
		Range:   []float64{0, 100},
		Program: "42", // Constant function
	}

	result := fn.Apply()
	if len(result) != 1 {
		t.Fatalf("expected 1 output, got %d", len(result))
	}
	if result[0] != 42 {
		t.Errorf("expected output 42, got %f", result[0])
	}
}
