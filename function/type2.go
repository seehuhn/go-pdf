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

// PDF 2.0 sections: 7.10.3

// Type2 represents a power interpolation function, of the form y = C0 + x^N ×
// (C1 - C0).  These functions have a single input x and can have one or more
// outputs. The PDF specification refers to this type of function as
// "exponential interpolation".
type Type2 struct {
	// XMin is the minimum value of the input range.  Input values smaller
	// than XMin are clipped to XMin.  This must be less than or equal to XMax.
	XMin float64

	// XMax is the maximum value of the input range.  Input values larger
	// than XMax are clipped to XMax.  This must be greater than or equal
	// to XMin.
	XMax float64

	// Range (optional) defines clipping ranges for the outputs, in the form
	// [min0, max0, min1, max1, ...]. It this is missing, no clipping is
	// applied.  If present, this must have the same length as C0 and C1.
	Range []float64

	// C0 defines function result when x = 0.
	// This must have the same length as C1.
	C0 []float64

	// C1 defines function result when x = 1.
	// This must have the same length as C0.
	C1 []float64

	// N is the interpolation exponent.
	N float64
}

// FunctionType returns 2.
// This implements the [pdf.Function] interface.
func (f *Type2) FunctionType() int {
	return 2
}

// Shape returns the number of input and output values of the function.
func (f *Type2) Shape() (int, int) {
	return 1, len(f.C0)
}

// GetDomain returns the function's input domain.
func (f *Type2) GetDomain() []float64 {
	return []float64{f.XMin, f.XMax}
}

// extractType2 reads a Type 2 exponential interpolation function from a PDF dictionary.
func extractType2(x *pdf.Extractor, d pdf.Dict) (*Type2, error) {
	domain, err := getFloatArray(x, d["Domain"])
	if err != nil {
		return nil, err
	}
	if len(domain) != 2 {
		domain = []float64{0, 1}
	}

	rnge, err := getFloatArray(x, d["Range"])
	if err != nil {
		return nil, err
	}

	C0, err := getFloatArray(x, d["C0"])
	if err != nil {
		return nil, err
	}

	C1, err := getFloatArray(x, d["C1"])
	if err != nil {
		return nil, err
	}

	gamma, err := x.GetNumber(d["N"])
	if err != nil {
		return nil, err
	}

	f := &Type2{
		XMin:  domain[0],
		XMax:  domain[1],
		Range: rnge,
		C0:    C0,
		C1:    C1,
		N:     float64(gamma),
	}

	f.repair()
	if err := f.validate(); err != nil {
		return nil, err
	}

	return f, nil
}

// repair sets default values and tries to fix mal-formed function dicts.
func (f *Type2) repair() {
	if len(f.C0) == 0 {
		f.C0 = []float64{0.0}
	}
	if len(f.C1) == 0 {
		f.C1 = []float64{1.0}
	}
}

// validate checks if the Type2 function is properly configured.
func (f *Type2) validate() error {
	_, n := f.Shape()

	if !isRange(f.XMin, f.XMax) {
		return newInvalidFunctionError(2, "Xmin/XMax",
			"invalid domain [%g,%g]",
			f.XMin, f.XMax)
	}

	if f.Range != nil {
		if len(f.Range) != 2*n {
			return newInvalidFunctionError(2, "Range", "invalid length %d",
				len(f.Range))
		}
		for i := range n {
			if !isRange(f.Range[2*i], f.Range[2*i+1]) {
				return newInvalidFunctionError(2, "Range",
					"invalid range for output %d: [%g, %g]",
					i, f.Range[2*i], f.Range[2*i+1])
			}
		}
	}

	if len(f.C0) < 1 || len(f.C0) != len(f.C1) {
		return newInvalidFunctionError(2, "C0/C1",
			"invalid lengths %d, %d",
			len(f.C0), len(f.C1))
	}
	for i := range n {
		if !isFinite(f.C0[i]) || !isFinite(f.C1[i]) {
			return newInvalidFunctionError(2, "C0/C1",
				"invalid value for output %d: C0=%g, C1=%g",
				i, f.C0[i], f.C1[i])
		}
	}

	if !isFinite(f.N) {
		return newInvalidFunctionError(2, "N",
			"must be a finite number, got %g", f.N)
	}

	// non-integer exponents require non-negative base
	if f.N != math.Trunc(f.N) && f.XMin < 0 {
		return newInvalidFunctionError(2, "Domain",
			"minimum must be >= 0 when N is non-integer, got %f", f.XMin)
	}
	// negative exponents would cause division by zero at x=0
	if f.N < 0 && f.XMin <= 0 && f.XMax >= 0 {
		return newInvalidFunctionError(2, "Domain",
			"must not include 0 when N is negative")
	}

	return nil
}

// Embed embeds the function into a PDF file.
func (f *Type2) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) {
	if err := pdf.CheckVersion(rm.Out(), "Type 2 functions", pdf.V1_3); err != nil {
		return nil, err
	} else if err := f.validate(); err != nil {
		return nil, err
	}

	dict := pdf.Dict{
		"FunctionType": pdf.Integer(2),
		"Domain":       pdf.Array{pdf.Number(f.XMin), pdf.Number(f.XMax)},
		"N":            pdf.Number(f.N),
	}

	if f.Range != nil {
		dict["Range"] = arrayFromFloats(f.Range)
	}

	if len(f.C0) != 1 || f.C0[0] != 0 {
		dict["C0"] = arrayFromFloats(f.C0)
	}
	if len(f.C1) != 1 || f.C1[0] != 1 {
		dict["C1"] = arrayFromFloats(f.C1)
	}

	ref := rm.Alloc()
	err := rm.Out().Put(ref, dict)
	if err != nil {
		return nil, err
	}

	return ref, nil
}

// Apply applies the function to the given input value and returns the output values.
func (f *Type2) Apply(inputs ...float64) []float64 {
	if len(inputs) != 1 {
		panic(fmt.Sprintf("Type 2 function expects 1 input, got %d", len(inputs)))
	}

	x := clip(inputs[0], f.XMin, f.XMax)
	xPowN := math.Pow(x, f.N)

	_, n := f.Shape()
	outputs := make([]float64, n)
	for i := range n {
		outputs[i] = f.C0[i] + xPowN*(f.C1[i]-f.C0[i])
	}

	if f.Range != nil {
		for i := range n {
			min := f.Range[2*i]
			max := f.Range[2*i+1]
			outputs[i] = clip(outputs[i], min, max)
		}
	}

	return outputs
}

// Equal reports whether f and other represent the same Type2 function.
func (f *Type2) Equal(other *Type2) bool {
	if f == nil || other == nil {
		return f == other
	}

	if math.Abs(f.XMin-other.XMin) > floatEpsilon {
		return false
	}
	if math.Abs(f.XMax-other.XMax) > floatEpsilon {
		return false
	}

	if !floatSlicesEqual(f.Range, other.Range, floatEpsilon) {
		return false
	}

	if !floatSlicesEqual(f.C0, other.C0, floatEpsilon) {
		return false
	}
	if !floatSlicesEqual(f.C1, other.C1, floatEpsilon) {
		return false
	}

	if math.Abs(f.N-other.N) > floatEpsilon {
		return false
	}

	return true
}

// Identity is the identity function f(x) = x for the domain [0, 1].
// This sentinel value corresponds to the name /Identity in PDF dictionaries,
// used for transfer functions (graphics state TR/TR2) and halftone
// TransferFunction entries.
var Identity = &Type2{
	XMin: 0,
	XMax: 1,
	C0:   []float64{0},
	C1:   []float64{1},
	N:    1,
}
