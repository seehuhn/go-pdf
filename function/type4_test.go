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
	"fmt"
	"math"
	"math/rand"
	"testing"

	"seehuhn.de/go/postscript"
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

			result := make([]float64, len(tt.expected))
			fn.Apply(result, tt.inputs...)

			for i, expected := range tt.expected {
				tolerance := 1e-10
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
		result := make([]float64, 1)
		fn.Apply(result, tc.x, tc.y)

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
			result := make([]float64, len(tt.expected))
			tt.function.Apply(result, tt.inputs...)

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

// TestType4Constant tests that a Type4 function with no inputs and one output
// returns the constant value.
func TestType4Constant(t *testing.T) {
	fn := &Type4{
		Domain:  []float64{},
		Range:   []float64{0, 100},
		Program: "42", // Constant function
	}

	result := make([]float64, 1)
	fn.Apply(result)
	if result[0] != 42 {
		t.Errorf("expected output 42, got %f", result[0])
	}
}

func TestType4Repair(t *testing.T) {
	tests := []struct {
		name           string
		inputDomain    []float64
		inputRange     []float64
		expectedDomain []float64
		expectedRange  []float64
	}{
		{
			name:           "empty domain and range",
			inputDomain:    []float64{},
			inputRange:     []float64{},
			expectedDomain: []float64{0.0, 1.0}, // default domain
			expectedRange:  []float64{0, 1},     // default range (required)
		},
		{
			name:           "odd length domain",
			inputDomain:    []float64{0.0, 1.0, 2.0}, // length 3, should truncate to 2
			inputRange:     []float64{0.0, 1.0},
			expectedDomain: []float64{0.0, 1.0}, // truncated to even length
			expectedRange:  []float64{0.0, 1.0},
		},
		{
			name:           "odd length range",
			inputDomain:    []float64{0.0, 1.0},
			inputRange:     []float64{0.0, 1.0, 2.0}, // length 3, should truncate to 2
			expectedDomain: []float64{0.0, 1.0},
			expectedRange:  []float64{0.0, 1.0}, // truncated to even length
		},
		{
			name:           "both odd lengths",
			inputDomain:    []float64{0.0, 1.0, 2.0},   // length 3
			inputRange:     []float64{5.0, 10.0, 15.0}, // length 3
			expectedDomain: []float64{0.0, 1.0},        // truncated
			expectedRange:  []float64{5.0, 10.0},       // truncated
		},
		{
			name:           "single element arrays",
			inputDomain:    []float64{0.5},      // length 1, should truncate to 0, then default
			inputRange:     []float64{10.0},     // length 1, should truncate to 0, then default
			expectedDomain: []float64{0.0, 1.0}, // becomes default after truncation
			expectedRange:  []float64{0, 1},     // becomes default after truncation
		},
		{
			name:           "even lengths remain unchanged",
			inputDomain:    []float64{0.0, 1.0, 2.0, 3.0}, // length 4, should remain
			inputRange:     []float64{5.0, 10.0},          // length 2, should remain
			expectedDomain: []float64{0.0, 1.0, 2.0, 3.0},
			expectedRange:  []float64{5.0, 10.0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fn := &Type4{
				Domain:  tt.inputDomain,
				Range:   tt.inputRange,
				Program: "0", // simple program
			}

			fn.repair()

			if len(fn.Domain) != len(tt.expectedDomain) {
				t.Errorf("Domain length mismatch: expected %d, got %d", len(tt.expectedDomain), len(fn.Domain))
			}
			for i, expected := range tt.expectedDomain {
				if i >= len(fn.Domain) || fn.Domain[i] != expected {
					t.Errorf("Domain[%d]: expected %f, got %f", i, expected, fn.Domain[i])
				}
			}

			if len(fn.Range) != len(tt.expectedRange) {
				t.Errorf("Range length mismatch: expected %d, got %d", len(tt.expectedRange), len(fn.Range))
			}
			for i, expected := range tt.expectedRange {
				if i >= len(fn.Range) || fn.Range[i] != expected {
					t.Errorf("Range[%d]: expected %f, got %f", i, expected, fn.Range[i])
				}
			}
		})
	}
}

// referenceApply evaluates a Type 4 function using the full PostScript
// interpreter.  This serves as an oracle against which the bytecode VM
// is compared.
func referenceApply(program string, inputs []float64, n int) ([]float64, error) {
	allowedOps := []string{
		"abs", "add", "atan", "ceiling", "cos", "cvi", "cvr", "div", "exp",
		"floor", "idiv", "ln", "log", "mod", "mul", "neg", "round", "sin",
		"sqrt", "sub", "truncate",
		"and", "bitshift", "eq", "ge", "gt", "le", "lt", "ne", "not", "or", "xor",
		"if", "ifelse",
		"copy", "dup", "exch", "index", "pop", "roll",
	}

	tempIntp := postscript.NewInterpreter()
	sysDict := tempIntp.SystemDict

	type4Dict := postscript.Dict{
		"true":  postscript.Boolean(true),
		"false": postscript.Boolean(false),
	}
	for _, name := range allowedOps {
		if impl, exists := sysDict[postscript.Name(name)]; exists {
			type4Dict[postscript.Name(name)] = impl
		}
	}

	intp := postscript.NewInterpreter()
	intp.DictStack = []postscript.Dict{type4Dict, {}}
	intp.SystemDict = type4Dict

	for _, input := range inputs {
		intp.Stack = append(intp.Stack, postscript.Real(input))
	}

	err := intp.ExecuteString(program)
	if err != nil {
		return nil, err
	}

	outputs := make([]float64, len(intp.Stack))
	for i, obj := range intp.Stack {
		switch v := obj.(type) {
		case postscript.Integer:
			outputs[i] = float64(v)
		case postscript.Real:
			outputs[i] = float64(v)
		case postscript.Boolean:
			if v {
				outputs[i] = 1
			}
		default:
			return nil, fmt.Errorf("invalid result type: %T", obj)
		}
	}

	if len(outputs) > n {
		outputs = outputs[len(outputs)-n:]
	} else {
		for len(outputs) < n {
			outputs = append(outputs, 0)
		}
	}

	return outputs, nil
}

func TestType4VsReference(t *testing.T) {
	programs := []struct {
		program string
		nIn     int
		nOut    int
	}{
		{"add", 2, 1},
		{"sub", 2, 1},
		{"mul", 2, 1},
		{"div", 2, 1},
		{"neg", 1, 1},
		{"abs", 1, 1},
		{"ceiling", 1, 1},
		{"floor", 1, 1},
		{"round", 1, 1},
		{"truncate", 1, 1},
		{"sqrt", 1, 1},
		{"ln", 1, 1},
		{"log", 1, 1},
		{"sin", 1, 1},
		{"cos", 1, 1},
		{"1 atan", 1, 1},
		{"0.5 exp", 1, 1},
		{"dup mul", 1, 1},
		{"dup 0.5 gt { pop 1.0 } { pop 0.0 } ifelse", 1, 1},
		{"exch", 2, 2},
		{"dup", 1, 2},
		{"360 mul sin 2 div exch 360 mul sin 2 div add", 2, 1},
	}

	rng := rand.New(rand.NewSource(42))
	for _, p := range programs {
		t.Run(p.program, func(t *testing.T) {
			for range 20 {
				inputs := make([]float64, p.nIn)
				for i := range inputs {
					inputs[i] = rng.Float64()*10 + 0.01
				}

				fn := &Type4{
					Domain:  make([]float64, p.nIn*2),
					Range:   make([]float64, p.nOut*2),
					Program: p.program,
				}
				for i := range p.nIn {
					fn.Domain[2*i] = -1000
					fn.Domain[2*i+1] = 1000
				}
				for i := range p.nOut {
					fn.Range[2*i] = -1000
					fn.Range[2*i+1] = 1000
				}

				got := make([]float64, p.nOut)
				fn.Apply(got, inputs...)

				ref, err := referenceApply(p.program, inputs, p.nOut)
				if err != nil {
					continue
				}

				// clip reference outputs
				for i := range p.nOut {
					ref[i] = clip(ref[i], fn.Range[2*i], fn.Range[2*i+1])
				}

				for i := range p.nOut {
					if math.Abs(got[i]-ref[i]) > 1e-10 {
						t.Errorf("inputs=%v: output[%d] VM=%g ref=%g",
							inputs, i, got[i], ref[i])
					}
				}
			}
		})
	}
}

func FuzzType4(f *testing.F) {
	f.Add("add", 0.3, 0.7)
	f.Add("sub", 1.0, 0.5)
	f.Add("mul", 2.0, 3.0)
	f.Add("div", 6.0, 2.0)
	f.Add("neg", 1.0, 0.0)
	f.Add("abs", -3.0, 0.0)
	f.Add("dup mul", 5.0, 0.0)
	f.Add("exch", 0.3, 0.7)
	f.Add("dup 0.5 gt { pop 1.0 } { pop 0.0 } ifelse", 0.7, 0.0)
	f.Add("360 mul sin 2 div exch 360 mul sin 2 div add", 0.25, 0.5)
	f.Add("dup dup mul exch 2 mul add sqrt", 3.0, 0.0)
	f.Add("sin", 90.0, 0.0)
	f.Add("cos", 0.0, 0.0)
	f.Add("1 atan", 1.0, 0.0)
	f.Add("0.5 exp", 9.0, 0.0)
	f.Add("ceiling", 3.2, 0.0)
	f.Add("floor", 3.7, 0.0)
	f.Add("round", 3.5, 0.0)
	f.Add("truncate", 3.7, 0.0)

	f.Fuzz(func(t *testing.T, program string, x, y float64) {
		// skip degenerate inputs
		if math.IsNaN(x) || math.IsInf(x, 0) || math.IsNaN(y) || math.IsInf(y, 0) {
			return
		}
		if math.Abs(x) > 1e6 || math.Abs(y) > 1e6 {
			return
		}

		// try compiling; skip invalid programs
		code, err := compile(program)
		if err != nil {
			return
		}

		// determine input/output counts by running in VM
		inputs := []float64{x, y}

		// try with 2 inputs first, then 1
		for _, nIn := range []int{2, 1} {
			in := inputs[:nIn]

			// run via the reference interpreter to discover nOut
			// and get expected values
			initStack := make([]value, len(in))
			for i, v := range in {
				initStack[i] = realVal(v)
			}
			stack, vmErr := execute(code, initStack)
			if vmErr != nil {
				continue
			}
			nOut := len(stack)
			if nOut == 0 || nOut > 4 {
				continue
			}

			ref, refErr := referenceApply(program, in, nOut)
			if refErr != nil {
				continue
			}

			// run via Apply
			fn := &Type4{
				Domain:  make([]float64, nIn*2),
				Range:   make([]float64, nOut*2),
				Program: program,
			}
			for i := range nIn {
				fn.Domain[2*i] = -1e7
				fn.Domain[2*i+1] = 1e7
			}
			for i := range nOut {
				fn.Range[2*i] = -1e7
				fn.Range[2*i+1] = 1e7
			}
			vmOut := make([]float64, nOut)
			fn.Apply(vmOut, in...)

			for i := range nOut {
				// clip reference to same range
				ref[i] = clip(ref[i], fn.Range[2*i], fn.Range[2*i+1])
				diff := math.Abs(vmOut[i] - ref[i])
				if diff > 1e-9 && diff > 1e-6*math.Abs(ref[i]) {
					t.Errorf("program=%q inputs=%v output[%d]: VM=%g ref=%g",
						program, in, i, vmOut[i], ref[i])
				}
			}
			return
		}
	})
}

func TestType4StackOverflow(t *testing.T) {
	// build a program: "dup 2 copy 4 copy 8 copy ... 256 copy"
	// each copy doubles the stack, so after 256 copy we'd have 512 elements
	program := "dup 2 copy 4 copy 8 copy 16 copy 32 copy 64 copy 128 copy 256 copy"
	fn := &Type4{
		Domain:  []float64{0, 1},
		Range:   []float64{0, 1},
		Program: program,
	}

	// Apply should not panic; the VM returns a stack overflow error
	// which Apply handles by returning zeros clipped to range
	result := make([]float64, 1)
	fn.Apply(result, 0.5)

	// verify the VM itself returns a stack overflow error
	code, err := compile(program)
	if err != nil {
		t.Fatal(err)
	}
	_, err = execute(code, []value{realVal(0.5)})
	if err != errStackOverflow {
		t.Errorf("expected stack overflow error, got %v", err)
	}
}
