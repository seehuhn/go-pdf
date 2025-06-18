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
	"errors"
	"fmt"
	"math"

	"seehuhn.de/go/pdf"
)

// Type2 represents a power interpolation function of the form
// y = C0 + x^N × (C1 - C0).
type Type2 struct {
	// Domain defines the valid input range as [min, max]
	Domain []float64

	// Range defines the valid output ranges as [min0, max0, min1, max1, ...]
	// This is optional for Type 2 functions
	Range []float64

	// C0 defines function result when x = 0.0
	// Default: [0.0]
	C0 []float64

	// C1 defines function result when x = 1.0
	// Default: [1.0]
	C1 []float64

	// N is the interpolation exponent
	N float64
}

// FunctionType returns 2 for Type 2 functions.
func (f *Type2) FunctionType() int {
	return 2
}

// Shape returns the number of input and output values of the function.
func (f *Type2) Shape() (int, int) {
	n := len(f.C0)
	if len(f.C1) > n {
		n = len(f.C1)
	}
	if n == 0 {
		n = 1 // Default case
	}
	return 1, n
}

// Apply applies the function to the given input value and returns the output values.
func (f *Type2) Apply(inputs ...float64) []float64 {
	if len(inputs) != 1 {
		panic(fmt.Sprintf("Type 2 function expects 1 input, got %d", len(inputs)))
	}

	x := inputs[0]

	// Clip input to domain
	if len(f.Domain) >= 2 {
		x = clipValue(x, f.Domain[0], f.Domain[1])
	}

	// Get C0 and C1 arrays, using defaults if not specified
	c0 := f.C0
	if c0 == nil {
		c0 = []float64{0.0}
	}

	c1 := f.C1
	if c1 == nil {
		c1 = []float64{1.0}
	}

	// Determine output size
	_, n := f.Shape()
	outputs := make([]float64, n)

	// Calculate x^N
	var xPowN float64
	switch f.N {
	case 0:
		xPowN = 1.0
	case 1:
		xPowN = x
	default:
		xPowN = math.Pow(x, f.N)
	}

	// Apply formula: y_j = C0_j + x^N × (C1_j - C0_j)
	for i := 0; i < n; i++ {
		var c0Val, c1Val float64

		if i < len(c0) {
			c0Val = c0[i]
		} else if len(c0) > 0 {
			c0Val = c0[len(c0)-1] // Use last value if index out of bounds
		}

		if i < len(c1) {
			c1Val = c1[i]
		} else if len(c1) > 0 {
			c1Val = c1[len(c1)-1] // Use last value if index out of bounds
		} else {
			c1Val = 1.0 // Default
		}

		outputs[i] = c0Val + xPowN*(c1Val-c0Val)
	}

	// Clip outputs to range if specified
	if len(f.Range) >= 2*n {
		for i := 0; i < n; i++ {
			min := f.Range[2*i]
			max := f.Range[2*i+1]
			outputs[i] = clipValue(outputs[i], min, max)
		}
	}

	return outputs
}

// Embed embeds the function into a PDF file.
func (f *Type2) Embed(rm *pdf.ResourceManager) (pdf.Native, pdf.Unused, error) {
	var zero pdf.Unused

	if err := pdf.CheckVersion(rm.Out, "Type 2 functions", pdf.V1_3); err != nil {
		return nil, zero, err
	}

	// Build the function dictionary
	dict := pdf.Dict{
		"FunctionType": pdf.Integer(2),
		"N":            pdf.Number(f.N),
	}

	// Add domain (required) - ensure we always have a valid domain
	domain := f.Domain
	if len(domain) < 2 {
		domain = []float64{0, 1} // Default domain
	}
	dict["Domain"] = arrayFromFloats(domain[:2])

	// Add range (optional)
	if len(f.Range) > 0 {
		dict["Range"] = arrayFromFloats(f.Range)
	}

	// Add C0 (optional, default [0.0])
	if f.C0 != nil {
		dict["C0"] = arrayFromFloats(f.C0)
	}

	// Add C1 (optional, default [1.0])
	if f.C1 != nil {
		dict["C1"] = arrayFromFloats(f.C1)
	}

	ref := rm.Out.Alloc()
	err := rm.Out.Put(ref, dict)
	if err != nil {
		return nil, zero, err
	}

	return ref, zero, nil
}

// validate checks if the Type2 function is properly configured.
func (f *Type2) validate() error {
	// Domain validation
	if len(f.Domain) != 0 && len(f.Domain) != 2 {
		return errors.New("domain must have 2 elements or be empty")
	}

	if len(f.Domain) == 2 {
		// Check domain constraints for special exponents
		min, max := f.Domain[0], f.Domain[1]

		// If N is non-integer, x must be >= 0
		if f.N != math.Floor(f.N) && min < 0 {
			return fmt.Errorf("domain minimum must be >= 0 when N is non-integer, got %f", min)
		}

		// If N is negative, x must not be 0
		if f.N < 0 && min <= 0 && max >= 0 {
			return errors.New("domain must not include 0 when N is negative")
		}
	}

	// Range validation
	_, n := f.Shape()
	if len(f.Range) != 0 && len(f.Range) != 2*n {
		return fmt.Errorf("range must have 2*n (%d) elements or be empty", 2*n)
	}

	// C0 and C1 validation
	if f.C0 != nil && f.C1 != nil && len(f.C0) != len(f.C1) {
		return errors.New("C0 and C1 must have the same length")
	}

	return nil
}

func readType2(r pdf.Getter, d pdf.Dict) (*Type2, error) {
	domain, err := floatsFromPDF(r, d["Domain"])
	if err != nil {
		return nil, fmt.Errorf("failed to read Domain: %w", err)
	}

	n, err := pdf.GetNumber(r, d["N"])
	if err != nil {
		return nil, fmt.Errorf("failed to read N: %w", err)
	}

	f := &Type2{
		Domain: domain,
		N:      float64(n),
	}

	// Ensure domain has exactly 2 elements to maintain round-trip consistency
	if len(f.Domain) < 2 {
		f.Domain = []float64{0, 1}
	} else if len(f.Domain) > 2 {
		f.Domain = f.Domain[:2]
	}

	if rangeObj, ok := d["Range"]; ok {
		f.Range, err = floatsFromPDF(r, rangeObj)
		if err != nil {
			return nil, fmt.Errorf("failed to read Range: %w", err)
		}
	}

	if c0Obj, ok := d["C0"]; ok {
		f.C0, err = floatsFromPDF(r, c0Obj)
		if err != nil {
			return nil, fmt.Errorf("failed to read C0: %w", err)
		}
	}

	if c1Obj, ok := d["C1"]; ok {
		f.C1, err = floatsFromPDF(r, c1Obj)
		if err != nil {
			return nil, fmt.Errorf("failed to read C1: %w", err)
		}
	}

	return f, nil
}
