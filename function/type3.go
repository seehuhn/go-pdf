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

// Type3 represents a piecewise defined function with a single input.
// The PDF specification refers to this as a "stitching function".
type Type3 struct {
	// XMin is the minimum value of the input range.  Input values smaller
	// than XMin are clipped to XMin.  This must be less than or equal to XMax.
	XMin float64

	// XMax is the maximum value of the input range.  Input values larger
	// than XMax are clipped to XMax.  This must be greater than or equal
	// to XMin.
	XMax float64

	// Range (optional) defines the valid output ranges as [min0, max0, min1,
	// max1, ...].
	Range []float64

	// Functions is the array of k functions to be combined.
	// All functions must have 1 input and the same number of outputs.
	Functions []pdf.Function

	// Bounds defines the boundaries between subdomains.
	// It must have k-1 elements, in increasing order, within the input range.
	// The first function applies to the range [XMin, Bounds[0]),
	// the second to [Bounds[0], Bounds[1]), ..., the last to
	// [Bounds[k-2], XMax].
	Bounds []float64

	// Encode maps each subdomain to corresponding function's domain as
	// [min0, max0, min1, max1, ...].
	Encode []float64
}

// FunctionType returns 3.
// This implements the [pdf.Function] interface.
func (f *Type3) FunctionType() int {
	return 3
}

// Shape returns the number of input and output values of the function.
func (f *Type3) Shape() (int, int) {
	_, n := f.Functions[0].Shape()
	return 1, n
}

// GetDomain returns the function's input domain.
func (f *Type3) GetDomain() []float64 {
	return []float64{f.XMin, f.XMax}
}

// extractType3 reads a Type 3 piecewise defined function from a PDF dictionary.
func extractType3(r pdf.Getter, d pdf.Dict, cycleChecker *pdf.CycleChecker) (*Type3, error) {
	domain, err := pdf.GetFloatArray(r, d["Domain"])
	if err != nil {
		return nil, err
	}
	if len(domain) != 2 {
		domain = []float64{0, 1}
	}

	bounds, err := pdf.GetFloatArray(r, d["Bounds"])
	if err != nil {
		return nil, err
	}

	encode, err := pdf.GetFloatArray(r, d["Encode"])
	if err != nil {
		return nil, err
	}

	functionsArray, err := pdf.GetArray(r, d["Functions"])
	if err != nil {
		return nil, err
	}

	functions := make([]pdf.Function, len(functionsArray))
	for i, funcObj := range functionsArray {
		fn, err := safeExtract(r, funcObj, cycleChecker)
		if err != nil {
			return nil, err
		}
		functions[i] = fn
	}
	if len(functions) == 0 {
		return nil, errors.New("missing child functions")
	}

	rnge, err := pdf.GetFloatArray(r, d["Range"])
	if err != nil {
		return nil, err
	}

	f := &Type3{
		XMin:      domain[0],
		XMax:      domain[1],
		Functions: functions,
		Bounds:    bounds,
		Encode:    encode,
		Range:     rnge,
	}

	if err := f.validate(); err != nil {
		return nil, err
	}

	return f, nil
}

// validate checks if the Type3 function is properly configured.
func (f *Type3) validate() error {
	if !isRange(f.XMin, f.XMax) {
		return newInvalidFunctionError(3, "Domain", "invalid domain [%g,%g]", f.XMin, f.XMax)
	}

	k := len(f.Functions)
	if k == 0 {
		return newInvalidFunctionError(3, "Functions", "missing child functions")
	}
	_, n := f.Functions[0].Shape()
	for _, fn := range f.Functions {
		childM, childN := fn.Shape()
		if childM != 1 {
			return newInvalidFunctionError(3, "Functions", "function must have 1 input, got %d", childM)
		}
		if childN != n {
			return newInvalidFunctionError(3, "Functions", "function has %d outputs, expected %d", childN, n)
		}
	}

	if len(f.Bounds) != k-1 {
		return newInvalidFunctionError(3, "Bounds", "must have k-1 (%d) elements, got %d", k-1, len(f.Bounds))
	}

	for i, bound := range f.Bounds {
		if bound < f.XMin || bound > f.XMax {
			return newInvalidFunctionError(3, "Bounds",
				"bound[%d] = %f must be within domain [%f, %f]",
				i, bound, f.XMin, f.XMax)
		}
		if i > 0 && !(bound > f.Bounds[i-1]) {
			return newInvalidFunctionError(3, "Bounds",
				"must be in increasing order: bounds[%d] = %f <= bounds[%d] = %f",
				i-1, f.Bounds[i-1], i, bound)
		}
	}

	if len(f.Encode) != 2*k {
		return newInvalidFunctionError(3, "encode", "must have 2*k (%d) elements, got %d", 2*k, len(f.Encode))
	}
	for i := 0; i < len(f.Encode); i += 2 {
		if !isPair(f.Encode[i], f.Encode[i+1]) {
			return newInvalidFunctionError(0, "Encode",
				"invalid encode [%g,%g] for input %d", f.Encode[i], f.Encode[i+1], i/2)
		}
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

	return nil
}

// Embed embeds the function into a PDF file.
func (f *Type3) Embed(rm *pdf.ResourceManager) (pdf.Native, pdf.Unused, error) {
	var zero pdf.Unused

	if err := pdf.CheckVersion(rm.Out, "Type 3 functions", pdf.V1_3); err != nil {
		return nil, zero, err
	} else if err := f.validate(); err != nil {
		return nil, zero, err
	}

	functionRefs := make(pdf.Array, len(f.Functions))
	for i, fn := range f.Functions {
		ref, _, err := pdf.ResourceManagerEmbed(rm, fn)
		if err != nil {
			return nil, zero, err
		}
		functionRefs[i] = ref
	}

	// Build the function dictionary
	dict := pdf.Dict{
		"FunctionType": pdf.Integer(3),
		"Domain":       pdf.Array{pdf.Number(f.XMin), pdf.Number(f.XMax)},
		"Functions":    functionRefs,
		"Bounds":       arrayFromFloats(f.Bounds),
		"Encode":       arrayFromFloats(f.Encode),
	}
	if f.Range != nil {
		dict["Range"] = arrayFromFloats(f.Range)
	}

	ref := rm.Out.Alloc()
	err := rm.Out.Put(ref, dict)
	if err != nil {
		return nil, zero, err
	}

	return ref, zero, nil
}

// Apply applies the function to the given input value and returns the output values.
func (f *Type3) Apply(inputs ...float64) []float64 {
	if len(inputs) != 1 {
		panic(fmt.Sprintf("Type 3 function expects 1 input, got %d", len(inputs)))
	}
	x := inputs[0]

	x = clip(x, f.XMin, f.XMax)

	subdomainIndex, a, b := f.findSubdomain(x)
	encodeMin := f.Encode[2*subdomainIndex]
	encodeMax := f.Encode[2*subdomainIndex+1]
	encodedInput := interpolate(x, a, b, encodeMin, encodeMax)

	outputs := f.Functions[subdomainIndex].Apply(encodedInput)

	if f.Range != nil {
		for i, yi := range outputs {
			min := f.Range[2*i]
			max := f.Range[2*i+1]
			outputs[i] = clip(yi, min, max)
		}
	}

	return outputs
}

// findSubdomain determines which subdomain the input x belongs to and returns
// the subdomain index and the corresponding domain boundaries.
// This implements the PDF specification rules for Type 3 function intervals:
//   - Normal intervals are half-open [a, b), closed on left, open on right
//   - Last interval is always closed on right [a, b]
//   - Special case: when XMin = Bounds[0], first interval is [XMin, Bounds[0]]
//     (closed on both sides) and second interval is (Bounds[0], ...] (open on left)
func (f *Type3) findSubdomain(x float64) (int, float64, float64) {
	domain0, domain1 := f.XMin, f.XMax

	if len(f.Bounds) == 0 {
		return 0, domain0, domain1
	}

	// For the special case domain0 = Bounds[0], the first interval consists of
	// a single point.  The next interval is open on the left.
	if domain0 == f.Bounds[0] && x <= domain0 {
		return 0, domain0, f.Bounds[0]
	}

	left := domain0
	for i, right := range f.Bounds {
		if x < right {
			return i, left, right
		}
		left = right
	}

	return len(f.Functions) - 1, left, domain1
}

// Equal reports whether f and other represent the same Type3 function.
func (f *Type3) Equal(other *Type3) bool {
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

	// compare child functions recursively
	if len(f.Functions) != len(other.Functions) {
		return false
	}
	for i := range f.Functions {
		if !Equal(f.Functions[i], other.Functions[i]) {
			return false
		}
	}

	if !floatSlicesEqual(f.Bounds, other.Bounds, floatEpsilon) {
		return false
	}

	if !floatSlicesEqual(f.Encode, other.Encode, floatEpsilon) {
		return false
	}

	return true
}
