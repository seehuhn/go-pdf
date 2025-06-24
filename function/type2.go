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

	"seehuhn.de/go/pdf"
)

// Type2 represents a power interpolation functions, of the form y = C0 + x^N ×
// (C1 - C0).  These functions have a single input x and can have one or more
// outputs. The PDF specification refers to this type of function as
// "exponential interpolation".
type Type2 struct {
	// XMin is the minimum value of the input range.  Input values x smaller
	// than XMin are clipped to XMin.  This must be less than or equal to XMax.
	XMin float64

	// XMax is the maximum value of the input range.  Input values x larger
	// than XMax are clipped to XMax.  This must be greater than or equal
	// to XMin.
	XMax float64

	// Range (optional) defines clipping ranges for the outputs, in the form
	// [min0, max0, min1, max1, ...]. It this is missing, no clipping is
	// applied.  If present, this must have the same length as C0 and C1.
	Range []float64

	// C0 defines function result when x = 0.0.
	// This must contain at least one value and must have the same length as C1.
	C0 []float64

	// C1 defines function result when x = 1.0.
	// This must contain at least one value and must have the same length as C0.
	C1 []float64

	// N is the interpolation exponent.
	N float64
}

// FunctionType returns 2 for Type 2 functions.
func (f *Type2) FunctionType() int {
	return 2
}

// Shape returns the number of input and output values of the function.
func (f *Type2) Shape() (int, int) {
	return 1, len(f.C0)
}

// Apply applies the function to the given input value and returns the output values.
func (f *Type2) Apply(inputs ...float64) []float64 {
	if len(inputs) != 1 {
		panic(fmt.Sprintf("Type 2 function expects 1 input, got %d", len(inputs)))
	}

	x := clip(inputs[0], f.XMin, f.XMax)

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
			outputs[i] = clip(outputs[i], min, max)
		}
	}

	return outputs
}

// Embed embeds the function into a PDF file.
func (f *Type2) Embed(rm *pdf.ResourceManager) (pdf.Native, pdf.Unused, error) {
	var zero pdf.Unused

	if err := pdf.CheckVersion(rm.Out, "Type 2 functions", pdf.V1_3); err != nil {
		return nil, zero, err
	} else if err := f.validate(); err != nil {
		return nil, zero, err
	}

	// Build the function dictionary
	dict := pdf.Dict{
		"FunctionType": pdf.Integer(2),
		"N":            pdf.Number(f.N),
	}

	dict["Domain"] = pdf.Array{pdf.Number(f.XMin), pdf.Number(f.XMax)}

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
	if !isRange(f.XMin, f.XMax) {
		return newInvalidFunctionError(2, "Xmin/XMax", "invalid domain [%g,%g]",
			f.XMin, f.XMax)
	}

	if len(f.C0) < 1 || len(f.C0) != len(f.C1) {
		return newInvalidFunctionError(2, "C0/C1", "invalid length %d,%d",
			len(f.C0), len(f.C1))
	}

	if !isFinite(f.N) {
		return newInvalidFunctionError(2, "N", "must be a finite number, got %g", f.N)
	}
	if f.N != math.Trunc(f.N) && f.XMin < 0 {
		// If N is non-integer, x must be >= 0
		return newInvalidFunctionError(2, "Domain",
			"minimum must be >= 0 when N is non-integer, got %f", f.XMin)
	}
	if f.N < 0 && f.XMin <= 0 && f.XMax >= 0 {
		// If N is negative, x must not be 0
		return newInvalidFunctionError(2, "Domain", "must not include 0 when N is negative")
	}

	// Range validation
	_, n := f.Shape()
	if f.Range != nil {
		if len(f.Range) != 2*n {
			return newInvalidFunctionError(2, "Range", "invalid length %d",
				len(f.Range))
		}
		for i := 0; i < n; i++ {
			if !isRange(f.Range[2*i], f.Range[2*i+1]) {
				return newInvalidFunctionError(2, "Range",
					"invalid range for output %d: [%g, %g]",
					i, f.Range[2*i], f.Range[2*i+1])
			}
		}
	}

	return nil
}

// readType2 reads a Type 2 exponential interpolation function from a PDF dictionary.
func readType2(r pdf.Getter, d pdf.Dict) (*Type2, error) {
	domain, err := floatsFromPDF(r, d["Domain"])
	if err != nil {
		return nil, fmt.Errorf("failed to read Domain: %w", err)
	}
	if len(domain) < 2 || !isRange(domain[0], domain[1]) {
		domain = []float64{0, 1}
	}

	n, err := pdf.GetNumber(r, d["N"])
	if err != nil {
		return nil, fmt.Errorf("failed to read N: %w", err)
	}

	f := &Type2{
		XMin: domain[0],
		XMax: domain[1],
		N:    float64(n),
	}

	// Ensure domain has exactly 2 elements to maintain round-trip consistency

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
