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
	"io"
	"math"

	"seehuhn.de/go/pdf"
)

// Type0 represents a Type 0 sampled function that uses a table of sample
// values with interpolation to approximate functions with bounded domains
// and ranges.
type Type0 struct {
	// Domain defines the valid input ranges as [min0, max0, min1, max1, ...]
	Domain []float64

	// Range defines the valid output ranges as [min0, max0, min1, max1, ...]
	Range []float64

	// Size specifies the number of samples in each input dimension
	Size []int

	// BitsPerSample is the number of bits per sample value (1, 2, 4, 8, 12, 16, 24, 32)
	BitsPerSample int

	// Order is the interpolation order (1 for linear, 3 for cubic spline)
	Order int

	// Encode maps inputs to sample table indices as [min0, max0, min1, max1, ...]
	// Default: [0, Size[0]-1, 0, Size[1]-1, ...]
	Encode []float64

	// Decode maps samples to output range as [min0, max0, min1, max1, ...]
	// Default: same as Range
	Decode []float64

	// Samples contains the raw sample data
	Samples []byte
}

// FunctionType returns 0 for Type 0 functions.
func (f *Type0) FunctionType() int {
	return 0
}

// Shape returns the number of input and output values of the function.
func (f *Type0) Shape() (int, int) {
	m := len(f.Domain) / 2
	n := len(f.Range) / 2
	return m, n
}

// Apply applies the function to the given input values and returns the output values.
func (f *Type0) Apply(inputs ...float64) []float64 {
	m, n := f.Shape()
	if len(inputs) != m {
		panic(fmt.Sprintf("expected %d inputs, got %d", m, len(inputs)))
	}

	// Validate that we have size information for all input dimensions
	if len(f.Size) < m {
		// Return zero values for malformed functions to avoid panic
		return make([]float64, n)
	}

	// Check if we have any samples data
	if len(f.Samples) == 0 {
		// Return zero values for functions without sample data
		return make([]float64, n)
	}

	// Clip inputs to domain
	clippedInputs := make([]float64, m)
	for i := 0; i < m; i++ {
		min := f.Domain[2*i]
		max := f.Domain[2*i+1]
		clippedInputs[i] = clipValue(inputs[i], min, max)
	}

	// Apply encoding to get sample indices
	encode := f.Encode
	if encode == nil {
		encode = make([]float64, 2*m)
		for i := 0; i < m; i++ {
			encode[2*i] = 0
			encode[2*i+1] = float64(f.Size[i] - 1)
		}
	}

	indices := make([]float64, m)
	for i := 0; i < m; i++ {
		indices[i] = interpolate(clippedInputs[i], f.Domain[2*i], f.Domain[2*i+1], encode[2*i], encode[2*i+1])
	}

	// Sample the function using interpolation
	samples := f.sampleFunction(indices)

	// Apply decoding
	decode := f.Decode
	if decode == nil {
		decode = f.Range
	}

	outputs := make([]float64, n)
	maxSample := float64((uint(1) << uint(f.BitsPerSample)) - 1)
	for i := 0; i < n; i++ {
		normalized := samples[i] / maxSample
		outputs[i] = interpolate(normalized, 0, 1, decode[2*i], decode[2*i+1])
	}

	// Clip outputs to range
	for i := 0; i < n; i++ {
		min := f.Range[2*i]
		max := f.Range[2*i+1]
		outputs[i] = clipValue(outputs[i], min, max)
	}

	return outputs
}

// sampleFunction performs multidimensional interpolation on the sample table
func (f *Type0) sampleFunction(indices []float64) []float64 {
	m, n := f.Shape()

	// For simplicity, implement linear interpolation
	// In a full implementation, cubic spline would be supported when Order == 3

	if m == 1 {
		return f.sample1D(indices[0], n)
	}

	// For multidimensional case, use separable linear interpolation
	// This is a simplified implementation
	floorIndices := make([]int, m)
	fractions := make([]float64, m)

	for i := 0; i < m; i++ {
		floorIndices[i] = int(math.Floor(indices[i]))
		fractions[i] = indices[i] - float64(floorIndices[i])

		// Clamp to valid range
		if floorIndices[i] < 0 {
			floorIndices[i] = 0
			fractions[i] = 0
		}
		if floorIndices[i] >= f.Size[i]-1 {
			floorIndices[i] = f.Size[i] - 2
			fractions[i] = 1
		}
	}

	// Perform multilinear interpolation
	numCorners := 1 << m
	result := make([]float64, n)

	for corner := 0; corner < numCorners; corner++ {
		weight := 1.0
		cornerIndices := make([]int, m)

		for dim := 0; dim < m; dim++ {
			if (corner>>dim)&1 == 0 {
				cornerIndices[dim] = floorIndices[dim]
				weight *= 1 - fractions[dim]
			} else {
				cornerIndices[dim] = floorIndices[dim] + 1
				weight *= fractions[dim]
			}
		}

		cornerSamples := f.getSamplesAt(cornerIndices)
		for i := 0; i < n; i++ {
			result[i] += weight * cornerSamples[i]
		}
	}

	return result
}

// sample1D performs 1D linear interpolation
func (f *Type0) sample1D(index float64, n int) []float64 {
	i0 := int(math.Floor(index))
	i1 := i0 + 1
	frac := index - float64(i0)

	if i0 < 0 {
		i0, i1, frac = 0, 0, 0
	}
	if i1 >= f.Size[0] {
		i0, i1, frac = f.Size[0]-1, f.Size[0]-1, 0
	}

	samples0 := f.getSamplesAt([]int{i0})
	samples1 := f.getSamplesAt([]int{i1})

	result := make([]float64, n)
	for i := 0; i < n; i++ {
		result[i] = samples0[i]*(1-frac) + samples1[i]*frac
	}

	return result
}

// getSamplesAt extracts sample values at the given multidimensional index
func (f *Type0) getSamplesAt(indices []int) []float64 {
	m, n := f.Shape()

	// Calculate linear index in the sample array
	linearIndex := 0
	stride := 1
	for i := m - 1; i >= 0; i-- {
		linearIndex += indices[i] * stride
		stride *= f.Size[i]
	}

	// Extract samples at this position
	samples := make([]float64, n)
	bytesPerSample := (f.BitsPerSample + 7) / 8
	startByte := linearIndex * n * bytesPerSample

	for i := 0; i < n; i++ {
		byteOffset := startByte + i*bytesPerSample
		samples[i] = f.extractSample(byteOffset)
	}

	return samples
}

// extractSample extracts a single sample value from the byte array
func (f *Type0) extractSample(byteOffset int) float64 {
	if byteOffset < 0 || byteOffset >= len(f.Samples) {
		return 0
	}

	switch f.BitsPerSample {
	case 8:
		return float64(f.Samples[byteOffset])
	case 16:
		if byteOffset+1 >= len(f.Samples) {
			return 0
		}
		return float64(uint16(f.Samples[byteOffset])<<8 | uint16(f.Samples[byteOffset+1]))
	case 32:
		if byteOffset+3 >= len(f.Samples) {
			return 0
		}
		val := uint32(f.Samples[byteOffset])<<24 |
			uint32(f.Samples[byteOffset+1])<<16 |
			uint32(f.Samples[byteOffset+2])<<8 |
			uint32(f.Samples[byteOffset+3])
		return float64(val)
	default:
		// For other bit depths, would need bit-level extraction
		// This is a simplified implementation
		return float64(f.Samples[byteOffset])
	}
}

// Embed embeds the function into a PDF file.
func (f *Type0) Embed(rm *pdf.ResourceManager) (pdf.Native, pdf.Unused, error) {
	var zero pdf.Unused

	if err := pdf.CheckVersion(rm.Out, "Type 0 functions", pdf.V1_2); err != nil {
		return nil, zero, err
	}

	// Build the function dictionary
	dict := pdf.Dict{
		"FunctionType":  pdf.Integer(0),
		"Domain":        arrayFromFloats(f.Domain),
		"Range":         arrayFromFloats(f.Range),
		"Size":          arrayFromInts(f.Size),
		"BitsPerSample": pdf.Integer(f.BitsPerSample),
	}

	if f.Order != 1 {
		dict["Order"] = pdf.Integer(f.Order)
	}

	if f.Encode != nil {
		dict["Encode"] = arrayFromFloats(f.Encode)
	}

	if f.Decode != nil {
		dict["Decode"] = arrayFromFloats(f.Decode)
	}

	// Create stream with sample data
	ref := rm.Out.Alloc()
	stm, err := rm.Out.OpenStream(ref, dict, pdf.FilterCompress{})
	if err != nil {
		return nil, zero, err
	}

	if len(f.Samples) > 0 {
		_, err = stm.Write(f.Samples)
		if err != nil {
			return nil, zero, err
		}
	}

	err = stm.Close()
	if err != nil {
		return nil, zero, err
	}

	return ref, zero, nil
}

// validate checks if the Type0 function is properly configured.
func (f *Type0) validate() error {
	m, n := f.Shape()
	_ = m // Used for validation below
	_ = n // Used for validation below

	if len(f.Domain) != 2*m {
		return errors.New("domain length must be 2*m")
	}
	if len(f.Range) != 2*n {
		return errors.New("range length must be 2*n")
	}
	if len(f.Size) != m {
		return errors.New("size length must be m")
	}

	for i := 0; i < m; i++ {
		if f.Size[i] <= 0 {
			return fmt.Errorf("size[%d] must be positive", i)
		}
	}

	validBits := []int{1, 2, 4, 8, 12, 16, 24, 32}
	validBitsPerSample := false
	for _, bits := range validBits {
		if f.BitsPerSample == bits {
			validBitsPerSample = true
			break
		}
	}
	if !validBitsPerSample {
		return fmt.Errorf("invalid BitsPerSample: %d", f.BitsPerSample)
	}

	if f.Order != 1 && f.Order != 3 {
		return fmt.Errorf("order must be 1 or 3, got %d", f.Order)
	}

	if f.Encode != nil && len(f.Encode) != 2*m {
		return errors.New("encode length must be 2*m")
	}

	if f.Decode != nil && len(f.Decode) != 2*n {
		return errors.New("decode length must be 2*n")
	}

	return nil
}

func readType0(r pdf.Getter, stream *pdf.Stream) (*Type0, error) {
	d := stream.Dict
	domain, err := floatsFromPDF(r, d["Domain"])
	if err != nil {
		return nil, fmt.Errorf("failed to read Domain: %w", err)
	}

	rangeArray, err := floatsFromPDF(r, d["Range"])
	if err != nil {
		return nil, fmt.Errorf("failed to read Range: %w", err)
	}

	size, err := intsFromPDF(r, d["Size"])
	if err != nil {
		return nil, fmt.Errorf("failed to read Size: %w", err)
	}

	bitsPerSample, err := pdf.GetInteger(r, d["BitsPerSample"])
	if err != nil {
		return nil, fmt.Errorf("failed to read BitsPerSample: %w", err)
	}

	f := &Type0{
		Domain:        domain,
		Range:         rangeArray,
		Size:          size,
		BitsPerSample: int(bitsPerSample),
		Order:         1, // Default
	}

	if orderObj, ok := d["Order"]; ok {
		order, err := pdf.GetInteger(r, orderObj)
		if err != nil {
			return nil, fmt.Errorf("failed to read Order: %w", err)
		}
		f.Order = int(order)
	}

	if encodeObj, ok := d["Encode"]; ok {
		f.Encode, err = floatsFromPDF(r, encodeObj)
		if err != nil {
			return nil, fmt.Errorf("failed to read Encode: %w", err)
		}
	}

	if decodeObj, ok := d["Decode"]; ok {
		f.Decode, err = floatsFromPDF(r, decodeObj)
		if err != nil {
			return nil, fmt.Errorf("failed to read Decode: %w", err)
		}
	}

	// Read stream data
	stmReader, err := pdf.DecodeStream(r, stream, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to decode stream: %w", err)
	}
	defer stmReader.Close()

	// Calculate expected data size
	totalSamples := 1
	for _, size := range f.Size {
		totalSamples *= size
	}
	_, n := f.Shape()
	bytesPerSample := (f.BitsPerSample + 7) / 8
	expectedSize := totalSamples * n * bytesPerSample

	f.Samples = make([]byte, expectedSize)
	_, err = io.ReadFull(stmReader, f.Samples)
	if err != nil {
		return nil, fmt.Errorf("failed to read sample data: %w", err)
	}

	return f, nil
}
