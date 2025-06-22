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

	"seehuhn.de/go/pdf"
)

// Type3 represents a piecewise defined function with a single input.
// The PDF specification refers to this as a "stitching function".
type Type3 struct {
	// Domain defines the overall input range as [min, max].
	Domain []float64

	// Range (optional) defines the valid output ranges as [min0, max0, min1,
	// max1, ...].
	Range []float64

	// Functions is the array of k functions to be combined.
	// All functions must have 1 input and the same number of outputs.
	Functions []pdf.Function

	// Bounds defines the boundaries between subdomains.
	// It must have k-1 elements, in increasing order, within the domain.
	// The first function applies to the range [Domain[0], Bounds[0]),
	// the second to [Bounds[0], Bounds[1]), ..., the last to
	// [Bounds[k-2], Domain[1]].
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

// Apply applies the function to the given input value and returns the output values.
func (f *Type3) Apply(inputs ...float64) []float64 {
	if len(inputs) != 1 {
		panic(fmt.Sprintf("Type 3 function expects 1 input, got %d", len(inputs)))
	}
	x := inputs[0]

	// Clip input to domain
	if len(f.Domain) >= 2 {
		x = clip(x, f.Domain[0], f.Domain[1])
	}

	if len(f.Functions) == 0 {
		return []float64{0} // No functions defined
	}

	// Find which subdomain the input belongs to
	k := len(f.Functions)
	subdomainIndex, subdomain := f.findSubdomain(x, k)

	// Encode the input for the selected function
	encodeMin := f.Encode[2*subdomainIndex]
	encodeMax := f.Encode[2*subdomainIndex+1]
	encodedInput := interpolate(x, subdomain[0], subdomain[1], encodeMin, encodeMax)

	// Apply the selected function
	outputs := f.Functions[subdomainIndex].Apply(encodedInput)

	// Clip outputs to range if specified
	_, n := f.Shape()
	if len(f.Range) >= 2*n {
		for i := range n {
			min := f.Range[2*i]
			max := f.Range[2*i+1]
			outputs[i] = clip(outputs[i], min, max)
		}
	}

	return outputs
}

// findSubdomain determines which subdomain the input x belongs to and returns
// the subdomain index and the corresponding domain boundaries.
func (f *Type3) findSubdomain(x float64, k int) (int, [2]float64) {
	if len(f.Domain) < 2 {
		return 0, [2]float64{0, 1} // Default domain
	}

	domain0, domain1 := f.Domain[0], f.Domain[1]

	// Handle case with no bounds (single function)
	if len(f.Bounds) == 0 {
		return 0, [2]float64{domain0, domain1}
	}

	// Check first subdomain [Domain[0], Bounds[0])
	if x < f.Bounds[0] {
		return 0, [2]float64{domain0, f.Bounds[0]}
	}

	// Check intermediate subdomains [Bounds[i], Bounds[i+1])
	for i := 0; i < len(f.Bounds)-1; i++ {
		if x < f.Bounds[i+1] {
			return i + 1, [2]float64{f.Bounds[i], f.Bounds[i+1]}
		}
	}

	// Last subdomain [Bounds[k-2], Domain[1]]
	lastIndex := len(f.Bounds) - 1
	return k - 1, [2]float64{f.Bounds[lastIndex], domain1}
}

// Embed embeds the function into a PDF file.
func (f *Type3) Embed(rm *pdf.ResourceManager) (pdf.Native, pdf.Unused, error) {
	var zero pdf.Unused

	if err := f.validate(); err != nil {
		return nil, zero, err
	}

	if err := pdf.CheckVersion(rm.Out, "Type 3 functions", pdf.V1_3); err != nil {
		return nil, zero, err
	}

	// Embed all sub-functions first
	functionRefs := make(pdf.Array, len(f.Functions))
	for i, fn := range f.Functions {
		ref, _, err := fn.Embed(rm)
		if err != nil {
			return nil, zero, fmt.Errorf("failed to embed function %d: %w", i, err)
		}
		functionRefs[i] = ref
	}

	// Build the function dictionary
	dict := pdf.Dict{
		"FunctionType": pdf.Integer(3),
		"Functions":    functionRefs,
		"Bounds":       arrayFromFloats(f.Bounds),
		"Encode":       arrayFromFloats(f.Encode),
	}

	// Add domain (required)
	if len(f.Domain) >= 2 {
		dict["Domain"] = arrayFromFloats(f.Domain[:2])
	} else {
		dict["Domain"] = pdf.Array{pdf.Number(0), pdf.Number(1)} // Default
	}

	// Add range (optional)
	if len(f.Range) > 0 {
		dict["Range"] = arrayFromFloats(f.Range)
	}

	ref := rm.Out.Alloc()
	err := rm.Out.Put(ref, dict)
	if err != nil {
		return nil, zero, err
	}

	return ref, zero, nil
}

// validate checks if the Type3 function is properly configured.
func (f *Type3) validate() error {
	// Domain validation
	if len(f.Domain) != 2 {
		return newInvalidFunctionError(3, "domain", "must have exactly 2 elements, got %d", len(f.Domain))
	}

	k := len(f.Functions)
	if k == 0 {
		return newInvalidFunctionError(3, "functions", "at least one function must be specified")
	}

	// Bounds validation
	if len(f.Bounds) != k-1 {
		return newInvalidFunctionError(3, "bounds", "must have k-1 (%d) elements, got %d", k-1, len(f.Bounds))
	}

	// Check bounds are in increasing order within domain
	domain0, domain1 := f.Domain[0], f.Domain[1]
	for i, bound := range f.Bounds {
		if bound <= domain0 || bound >= domain1 {
			return newInvalidFunctionError(3, "bounds", "bound[%d] = %f must be within domain [%f, %f]", i, bound, domain0, domain1)
		}
		if i > 0 && bound <= f.Bounds[i-1] {
			return newInvalidFunctionError(3, "bounds", "must be in increasing order: bounds[%d] = %f <= bounds[%d] = %f", i-1, f.Bounds[i-1], i, bound)
		}
	}

	// Encode validation
	if len(f.Encode) != 2*k {
		return newInvalidFunctionError(3, "encode", "must have 2*k (%d) elements, got %d", 2*k, len(f.Encode))
	}

	// Function validation - all must be 1-input and have same output dimensionality
	_, expectedN := f.Functions[0].Shape()
	for i, fn := range f.Functions {
		m, n := fn.Shape()
		if m != 1 {
			return newInvalidFunctionError(3, "functions", "function[%d] must have 1 input, got %d", i, m)
		}
		if n != expectedN {
			return newInvalidFunctionError(3, "functions", "function[%d] has %d outputs, expected %d", i, n, expectedN)
		}
	}

	// Range validation
	if len(f.Range) != 0 && len(f.Range) != 2*expectedN {
		return fmt.Errorf("range must have 2*n (%d) elements or be empty", 2*expectedN)
	}

	return nil
}

// readType3 reads a Type 3 piecewise defined function from a PDF dictionary.
func readType3(r pdf.Getter, d pdf.Dict, cycleChecker *pdf.CycleChecker) (*Type3, error) {
	domain, err := floatsFromPDF(r, d["Domain"])
	if err != nil {
		return nil, fmt.Errorf("failed to read Domain: %w", err)
	}

	bounds, err := floatsFromPDF(r, d["Bounds"])
	if err != nil {
		return nil, fmt.Errorf("failed to read Bounds: %w", err)
	}

	encode, err := floatsFromPDF(r, d["Encode"])
	if err != nil {
		return nil, fmt.Errorf("failed to read Encode: %w", err)
	}

	functionsArray, err := pdf.GetArray(r, d["Functions"])
	if err != nil {
		return nil, fmt.Errorf("failed to read Functions: %w", err)
	}

	functions := make([]pdf.Function, len(functionsArray))
	for i, funcObj := range functionsArray {
		fn, err := readWithCycleChecker(r, funcObj, cycleChecker)
		if err != nil {
			return nil, fmt.Errorf("failed to read function %d: %w", i, err)
		}
		functions[i] = fn
	}

	f := &Type3{
		Domain:    domain,
		Functions: functions,
		Bounds:    bounds,
		Encode:    encode,
	}

	// Ensure domain is always set to maintain round-trip consistency
	if len(f.Domain) == 0 {
		f.Domain = []float64{0, 1}
	}

	if rangeObj, ok := d["Range"]; ok {
		f.Range, err = floatsFromPDF(r, rangeObj)
		if err != nil {
			return nil, fmt.Errorf("failed to read Range: %w", err)
		}
	}

	return f, nil
}
