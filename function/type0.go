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

const maxSamples = 1 << 20

// Type0 represents a Type 0 sampled function that uses a table of sample
// values with interpolation to approximate functions with bounded domains
// and ranges.
type Type0 struct {
	// Domain defines the valid input ranges as [min0, max0, min1, max1, ...].
	Domain []float64

	// Range gives clipping ranges for the output variables as [min0, max0, min1, max1, ...].
	Range []float64

	// Size specifies the number of samples in each input dimension.
	Size []int

	// BitsPerSample is the number of bits per sample value (1, 2, 4, 8, 12, 16, 24, 32).
	BitsPerSample int

	// UseCubic determines whether to use cubic spline interpolation (true) or linear interpolation (false).
	UseCubic bool

	// Encode maps inputs to sample table indices as [min0, max0, min1, max1, ...].
	Encode []float64

	// Decode maps samples to output range as [min0, max0, min1, max1, ...].
	Decode []float64

	// Samples contains the raw sample data.
	Samples []byte

	// cubicCoefficients stores precomputed spline coefficients for UseCubic == true.
	// Shape: [4*(Size[0]-1), 4*(Size[1]-1), ..., 4*(Size[m-1]-1)].
	// Each group of 4 contains [a, b, c, d] for one spline segment.
	cubicCoefficients []float64
	cubicShape        []int
}

// readType0 reads a Type 0 sampled function from a PDF stream object.
func readType0(r pdf.Getter, stream *pdf.Stream) (*Type0, error) {
	d := stream.Dict

	domain, err := floatsFromPDF(r, d["Domain"])
	if err != nil {
		return nil, err
	}

	rangeArray, err := floatsFromPDF(r, d["Range"])
	if err != nil {
		return nil, err
	}

	size, err := intsFromPDF(r, d["Size"])
	if err != nil {
		return nil, err
	}

	bitsPerSample, err := pdf.GetInteger(r, d["BitsPerSample"])
	if err != nil {
		return nil, err
	}

	f := &Type0{
		Domain:        domain,
		Range:         rangeArray,
		Size:          size,
		BitsPerSample: int(bitsPerSample),
		UseCubic:      false, // Default to linear interpolation
	}

	if orderObj, ok := d["Order"]; ok {
		order, err := pdf.GetInteger(r, orderObj)
		if err != nil {
			return nil, err
		}
		f.UseCubic = (int(order) == 3)
	}

	if encodeObj, ok := d["Encode"]; ok {
		f.Encode, err = floatsFromPDF(r, encodeObj)
		if err != nil {
			return nil, err
		}
	}

	if decodeObj, ok := d["Decode"]; ok {
		f.Decode, err = floatsFromPDF(r, decodeObj)
		if err != nil {
			return nil, err
		}
	}

	// Read stream data
	stmReader, err := pdf.DecodeStream(r, stream, 0)
	if err != nil {
		return nil, err
	}
	defer stmReader.Close()
	f.Samples, err = io.ReadAll(stmReader)
	if err != nil {
		return nil, err
	}

	// Apply repair to set defaults and fix up the function.
	f.repair()
	if err := f.validate(); err != nil {
		return nil, err
	}

	return f, nil
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

// Embed embeds the function into a PDF file.
func (f *Type0) Embed(rm *pdf.ResourceManager) (pdf.Native, pdf.Unused, error) {
	var zero pdf.Unused

	if err := pdf.CheckVersion(rm.Out, "Type 0 functions", pdf.V1_2); err != nil {
		return nil, zero, err
	}
	if err := f.validate(); err != nil {
		return nil, zero, err
	}

	dict := pdf.Dict{
		"FunctionType":  pdf.Integer(0),
		"Domain":        arrayFromFloats(f.Domain),
		"Range":         arrayFromFloats(f.Range),
		"Size":          arrayFromInts(f.Size),
		"BitsPerSample": pdf.Integer(f.BitsPerSample),
	}
	if f.UseCubic {
		dict["Order"] = pdf.Integer(3)
	}
	if !f.isDefaultEncode() {
		dict["Encode"] = arrayFromFloats(f.Encode)
	}
	if !f.isDefaultDecode() {
		dict["Decode"] = arrayFromFloats(f.Decode)
	}

	ref := rm.Out.Alloc()

	stm, err := rm.Out.OpenStream(ref, dict, pdf.FilterCompress{})
	if err != nil {
		return nil, zero, err
	}
	_, err = stm.Write(f.Samples)
	if err != nil {
		return nil, zero, err
	}
	err = stm.Close()
	if err != nil {
		return nil, zero, err
	}

	return ref, zero, nil
}

// validate checks if the Type0 struct contains valid data
func (f *Type0) validate() error {
	m, n := f.Shape()
	if m <= 0 || n <= 0 {
		return newInvalidFunctionError(0, "Shape", "invalid shape (%d, %d)",
			m, n)
	}

	if len(f.Domain) != 2*m {
		return newInvalidFunctionError(0, "Domain", "length must be %d, got %d",
			2*m, len(f.Domain))
	}
	for i := 0; i < len(f.Domain); i += 2 {
		if f.Domain[i] >= f.Domain[i+1] {
			return newInvalidFunctionError(0, "Domain", "need Domain[%d] < Domain[%d]",
				i, i+1)
		}
	}

	if len(f.Range) != 2*n {
		return newInvalidFunctionError(0, "Range", "length must be %d, got %d", 2*n, len(f.Range))
	}
	for i := 0; i < len(f.Range); i += 2 {
		if f.Range[i] >= f.Range[i+1] {
			return newInvalidFunctionError(0, "Range", "need Range[%d] < Range[%d]",
				i, i+1)
		}
	}

	if len(f.Size) != m {
		return newInvalidFunctionError(0, "Size", "invalid length %d != %d",
			len(f.Size), m)
	}
	minSize := 1
	if f.UseCubic {
		minSize = 4
	}
	for i, size := range f.Size {
		if size < minSize {
			return newInvalidFunctionError(0, "Size", "invalid size[%d] = %d < %d",
				i, size, minSize)
		}
	}

	switch f.BitsPerSample {
	case 1, 2, 4, 8, 12, 16, 24, 32:
		// pass
	default:
		return newInvalidFunctionError(0, "BitsPerSample", "invalid value %d",
			f.BitsPerSample)
	}

	if f.Encode != nil && len(f.Encode) != 2*m {
		return newInvalidFunctionError(0, "Encode", "length must be %d, got %d",
			2*m, len(f.Encode))
	}
	for i := 0; i < len(f.Encode); i += 2 {
		if f.Encode[i] >= f.Encode[i+1] {
			return newInvalidFunctionError(0, "Encode", "need Encode[%d] < Encode[%d]",
				i, i+1)
		}
	}

	if f.Decode != nil && len(f.Decode) != 2*n {
		return newInvalidFunctionError(0, "Decode", "length must be %d, got %d",
			2*n, len(f.Decode))
	}
	for i := 0; i < len(f.Decode); i += 2 {
		if f.Decode[i] >= f.Decode[i+1] {
			return newInvalidFunctionError(0, "Decode", "need Decode[%d] < Decode[%d]",
				i, i+1)
		}
	}

	totalSamples := 1
	for _, size := range f.Size {
		next := totalSamples * size
		if totalSamples != next/size {
			return errors.New("sample size overflow")
		}
		totalSamples = next
	}

	if totalSamples > maxSamples {
		return errors.New("too many samples in Type 0 function")
	}

	totalBits := totalSamples * n * f.BitsPerSample
	totalBytes := (totalBits + 7) / 8
	if len(f.Samples) != totalBytes {
		return newInvalidFunctionError(0, "Samples", "length must be %d bytes, got %d",
			totalBytes, len(f.Samples))
	}

	return nil
}

// repair sets default values and tries to fix up mal-formed function dicts.
func (f *Type0) repair() {
	m, n := f.Shape()

	if len(f.Domain) > 2*m {
		f.Domain = f.Domain[:2*m]
	}
	if len(f.Range) > 2*n {
		f.Range = f.Range[:2*n]
	}
	if len(f.Size) > m {
		f.Size = f.Size[:m]
	}
	for _, size := range f.Size {
		if size < 4 {
			f.UseCubic = false
			break
		}
	}

	if len(f.Encode) == 0 { // set default Encode
		f.Encode = make([]float64, 2*m)
		for i := 0; i < min(m, len(f.Size)); i++ {
			f.Encode[2*i] = 0
			f.Encode[2*i+1] = float64(f.Size[i] - 1)
		}
	} else if len(f.Encode) > 2*m {
		f.Encode = f.Encode[:2*m]
	}

	if len(f.Decode) == 0 { // set default Decode
		f.Decode = make([]float64, len(f.Range))
		copy(f.Decode, f.Range)
	} else if len(f.Decode) > len(f.Range) {
		f.Decode = f.Decode[:len(f.Range)]
	}

	// We don't need to worry about integer overflows here, because
	// these will be checked in validate(), and decreasing the size
	// of f.Samples in case of overflow will not cause any additional damage.
	totalSamples := 1
	for _, size := range f.Size {
		totalSamples *= size
	}
	totalBits := totalSamples * n * f.BitsPerSample
	totalBytes := (totalBits + 7) / 8
	if len(f.Samples) > totalBytes && totalBytes > 0 {
		f.Samples = f.Samples[:totalBytes]
	}
}

// isDefaultEncode checks if the Encode array equals the default value
func (f *Type0) isDefaultEncode() bool {
	for i := range f.Size {
		if f.Encode[2*i] != 0 || f.Encode[2*i+1] != float64(f.Size[i]-1) {
			return false
		}
	}
	return true
}

// isDefaultDecode checks if the Decode array equals the default value (same as Range)
func (f *Type0) isDefaultDecode() bool {
	for i := range f.Decode {
		if f.Decode[i] != f.Range[i] {
			return false
		}
	}
	return true
}

// Apply applies the function to the given input values.
func (f *Type0) Apply(inputs ...float64) []float64 {
	m, n := f.Shape()
	if len(inputs) != m {
		panic(fmt.Sprintf("expected %d inputs, got %d", m, len(inputs)))
	}

	// encode the inputs
	indices := make([]float64, m)
	for i, val := range inputs {
		// clip to domain
		val = clip(val, f.Domain[2*i], f.Domain[2*i+1])

		// apply encoding
		val = interpolate(val, f.Domain[2*i], f.Domain[2*i+1], f.Encode[2*i], f.Encode[2*i+1])

		// clip to number of samples
		indices[i] = clip(val, 0, float64(f.Size[i]-1))
	}

	// sample the function using interpolation
	var samples []float64
	if f.UseCubic {
		samples = f.sampleCubic(indices)
	} else {
		samples = f.sampleLinear(indices)
	}

	// decode the output
	outputs := make([]float64, n)
	maxSample := float64((uint(1) << f.BitsPerSample) - 1)
	for j, xj := range samples {
		val := interpolate(xj, 0, maxSample, f.Decode[2*j], f.Decode[2*j+1])
		outputs[j] = clip(val, f.Range[2*j], f.Range[2*j+1])
	}

	return outputs
}

// sampleLinear performs multidimensional linear interpolation on the sample table
func (f *Type0) sampleLinear(indices []float64) []float64 {
	m, n := f.Shape()

	if m == 1 {
		// faster code for common single input case
		return f.sample1D(indices[0], n)
	}

	// For multidimensional case, use separable linear interpolation
	floorIndices := make([]int, m)
	fractions := make([]float64, m)

	for i := 0; i < m; i++ {
		// Handle single sample case in this dimension
		if f.Size[i] <= 1 {
			floorIndices[i] = 0
			fractions[i] = 0
			continue
		}

		floorIndices[i] = int(math.Floor(indices[i]))
		fractions[i] = indices[i] - float64(floorIndices[i])

		// Clamp to valid range with consistent boundary handling
		if floorIndices[i] < 0 {
			floorIndices[i] = 0
			fractions[i] = 0
		} else if floorIndices[i] >= f.Size[i]-1 {
			// Consistent with 1D case: clamp to last sample
			floorIndices[i] = f.Size[i] - 1
			fractions[i] = 0
		}
	}

	// Check if we can avoid interpolation (all fractions are 0)
	allExact := true
	for i := 0; i < m; i++ {
		if fractions[i] != 0.0 {
			allExact = false
			break
		}
	}
	if allExact {
		return f.getSamplesAt(floorIndices)
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
				// Ensure we don't go out of bounds
				if floorIndices[dim]+1 < f.Size[dim] {
					cornerIndices[dim] = floorIndices[dim] + 1
				} else {
					cornerIndices[dim] = floorIndices[dim]
				}
				weight *= fractions[dim]
			}
		}

		// Skip corners with zero weight for efficiency
		if weight > 0 {
			cornerSamples := f.getSamplesAt(cornerIndices)
			for i := 0; i < n; i++ {
				result[i] += weight * cornerSamples[i]
			}
		}
	}

	return result
}

// sample1D performs linear interpolation with a single input dimension.
func (f *Type0) sample1D(index float64, n int) []float64 {
	// Handle single sample case
	if f.Size[0] <= 1 {
		return f.getSamplesAt([]int{0})
	}

	i0 := int(math.Floor(index))
	i1 := i0 + 1
	frac := index - float64(i0)

	// Clamp to valid range with consistent boundary handling
	if i0 < 0 {
		i0, i1, frac = 0, 0, 0
	} else if i0 >= f.Size[0]-1 {
		i0, i1, frac = f.Size[0]-1, f.Size[0]-1, 0
	}

	// Optimization: avoid interpolation when fraction is exactly 0
	if frac == 0.0 {
		return f.getSamplesAt([]int{i0})
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

	// Calculate bit offset in the continuous bit stream
	sampleIndex := linearIndex * n
	for i := range n {
		samples[i] = f.extractSampleAtIndex(sampleIndex + i)
	}

	return samples
}

// extractSampleAtIndex extracts a single sample value at the given sample index
// from the continuous bit stream
func (f *Type0) extractSampleAtIndex(sampleIndex int) float64 {
	// Bounds check
	if sampleIndex < 0 {
		return 0
	}

	// Calculate bit position in the continuous bit stream
	bitOffset := sampleIndex * f.BitsPerSample
	byteOffset := bitOffset / 8
	bitInByte := bitOffset % 8

	// Bounds check
	if byteOffset >= len(f.Samples) || byteOffset < 0 {
		return 0
	}

	switch f.BitsPerSample {
	case 1:
		// Extract 1 bit
		mask := byte(1 << (7 - bitInByte))
		if f.Samples[byteOffset]&mask != 0 {
			return 1
		}
		return 0

	case 2:
		// Extract 2 bits
		shift := 6 - bitInByte
		if shift >= 0 {
			// Fits in current byte
			mask := byte(0x03 << shift)
			return float64((f.Samples[byteOffset] & mask) >> shift)
		} else {
			// Spans two bytes
			if byteOffset+1 >= len(f.Samples) {
				return 0
			}
			highBits := (f.Samples[byteOffset] & 0x01) << 1
			lowBits := (f.Samples[byteOffset+1] & 0x80) >> 7
			return float64(highBits | lowBits)
		}

	case 4:
		// Extract 4 bits
		if bitInByte == 0 {
			// High nibble
			return float64((f.Samples[byteOffset] & 0xF0) >> 4)
		} else if bitInByte == 4 {
			// Low nibble
			return float64(f.Samples[byteOffset] & 0x0F)
		} else {
			// Spans two bytes
			if byteOffset+1 >= len(f.Samples) {
				return 0
			}
			shift := 4 - bitInByte
			highBits := (f.Samples[byteOffset] & ((1 << (8 - bitInByte)) - 1)) << shift
			lowBits := (f.Samples[byteOffset+1] & (0xFF << (8 - shift))) >> (8 - shift)
			return float64(highBits | lowBits)
		}

	case 8:
		// 8 bits = 1 byte, must be byte-aligned
		if byteOffset >= len(f.Samples) {
			return 0
		}
		return float64(f.Samples[byteOffset])

	case 12:
		// Extract 12 bits
		if bitInByte == 0 {
			// Starts at byte boundary: 8 bits from current + 4 bits from next
			if byteOffset >= len(f.Samples) {
				return 0
			}
			if byteOffset+1 >= len(f.Samples) {
				// Only have partial data, use what we have
				return float64(uint16(f.Samples[byteOffset]) << 4)
			}
			highByte := uint16(f.Samples[byteOffset]) << 4
			lowNibble := uint16(f.Samples[byteOffset+1]) >> 4
			return float64(highByte | lowNibble)
		} else if bitInByte == 4 {
			// Starts at nibble boundary: 4 bits from current + 8 bits from next
			if byteOffset >= len(f.Samples) {
				return 0
			}
			if byteOffset+1 >= len(f.Samples) {
				// Only have partial data, use what we have
				return float64(uint16(f.Samples[byteOffset]&0x0F) << 8)
			}
			highNibble := uint16(f.Samples[byteOffset]&0x0F) << 8
			lowByte := uint16(f.Samples[byteOffset+1])
			return float64(highNibble | lowByte)
		} else {
			// General case - spans multiple bytes
			return f.extractBitsGeneral(bitOffset, 12)
		}

	case 16:
		// Extract 16 bits, must be byte-aligned
		if byteOffset >= len(f.Samples) || byteOffset+1 >= len(f.Samples) {
			return 0
		}
		return float64(uint16(f.Samples[byteOffset])<<8 | uint16(f.Samples[byteOffset+1]))

	case 24:
		// Extract 24 bits, must be byte-aligned
		if byteOffset >= len(f.Samples) || byteOffset+2 >= len(f.Samples) {
			return 0
		}
		val := uint32(f.Samples[byteOffset])<<16 |
			uint32(f.Samples[byteOffset+1])<<8 |
			uint32(f.Samples[byteOffset+2])
		return float64(val)

	case 32:
		// Extract 32 bits, must be byte-aligned
		if byteOffset >= len(f.Samples) || byteOffset+3 >= len(f.Samples) {
			return 0
		}
		val := uint32(f.Samples[byteOffset])<<24 |
			uint32(f.Samples[byteOffset+1])<<16 |
			uint32(f.Samples[byteOffset+2])<<8 |
			uint32(f.Samples[byteOffset+3])
		return float64(val)

	default:
		return 0
	}
}

// extractBitsGeneral extracts a specified number of bits starting at bitOffset.
// This handles the general case where bits span multiple bytes.
func (f *Type0) extractBitsGeneral(bitOffset, numBits int) float64 {
	if numBits <= 0 || numBits > 32 {
		return 0
	}

	var result uint32 = 0
	bitsRemaining := numBits
	currentBitOffset := bitOffset

	for bitsRemaining > 0 {
		byteOffset := currentBitOffset / 8
		bitInByte := currentBitOffset % 8

		if byteOffset >= len(f.Samples) {
			break
		}

		// How many bits can we read from this byte?
		bitsInCurrentByte := 8 - bitInByte
		bitsToRead := bitsRemaining
		if bitsToRead > bitsInCurrentByte {
			bitsToRead = bitsInCurrentByte
		}

		// Create mask and extract bits
		mask := byte((1 << bitsToRead) - 1)
		shift := bitsInCurrentByte - bitsToRead
		bits := (f.Samples[byteOffset] >> shift) & mask

		// Add to result
		result = (result << bitsToRead) | uint32(bits)

		bitsRemaining -= bitsToRead
		currentBitOffset += bitsToRead
	}

	return float64(result)
}

// cubicSplineCoeff1D computes natural cubic spline coefficients for 1D data.
// Returns coefficients [a, b, c, d] for each interval.
func (f *Type0) cubicSplineCoeff1D(x, y []float64) ([]float64, []float64, []float64, []float64) {
	n := len(x)
	if n < 2 {
		panic("need at least 2 points for cubic spline")
	}

	// for 2 points, use linear interpolation
	if n == 2 {
		a := []float64{y[0]}
		b := []float64{(y[1] - y[0]) / (x[1] - x[0])}
		c := []float64{0.0}
		d := []float64{0.0}
		return a, b, c, d
	}

	// Compute intervals
	h := make([]float64, n-1)
	for i := 0; i < n-1; i++ {
		h[i] = x[i+1] - x[i]
	}

	// build tridiagonal system for second derivatives M
	// natural boundary conditions: M[0] = M[n-1] = 0
	A := make([][]float64, n)
	rhs := make([]float64, n)

	for i := range A {
		A[i] = make([]float64, n)
	}

	// boundary conditions
	A[0][0] = 1.0
	A[n-1][n-1] = 1.0
	rhs[0] = 0.0
	rhs[n-1] = 0.0

	// interior points
	for i := 1; i < n-1; i++ {
		A[i][i-1] = h[i-1]
		A[i][i] = 2.0 * (h[i-1] + h[i])
		A[i][i+1] = h[i]
		rhs[i] = 6.0 * ((y[i+1]-y[i])/h[i] - (y[i]-y[i-1])/h[i-1])
	}

	// solve tridiagonal system for M (second derivatives)
	M := f.solveTridiagonal(A, rhs)

	// compute spline coefficients
	a := make([]float64, n-1)
	b := make([]float64, n-1)
	c := make([]float64, n-1)
	d := make([]float64, n-1)

	for i := 0; i < n-1; i++ {
		a[i] = y[i]
		b[i] = (y[i+1]-y[i])/h[i] - h[i]*(2*M[i]+M[i+1])/6
		c[i] = M[i] / 2
		d[i] = (M[i+1] - M[i]) / (6 * h[i])
	}

	return a, b, c, d
}

// solveTridiagonal solves a tridiagonal system using Gaussian elimination.
// This is suitable for the small systems typically found in PDF functions.
func (f *Type0) solveTridiagonal(A [][]float64, b []float64) []float64 {
	n := len(b)
	x := make([]float64, n)

	// Make a copy to avoid modifying input
	matrix := make([][]float64, n)
	rhs := make([]float64, n)
	for i := range matrix {
		matrix[i] = make([]float64, n)
		copy(matrix[i], A[i])
		rhs[i] = b[i]
	}

	// forward elimination
	for i := 0; i < n-1; i++ {
		// find pivot
		maxRow := i
		for k := i + 1; k < n; k++ {
			if math.Abs(matrix[k][i]) > math.Abs(matrix[maxRow][i]) {
				maxRow = k
			}
		}

		// swap rows if needed
		if maxRow != i {
			matrix[i], matrix[maxRow] = matrix[maxRow], matrix[i]
			rhs[i], rhs[maxRow] = rhs[maxRow], rhs[i]
		}

		// eliminate column
		for k := i + 1; k < n; k++ {
			if math.Abs(matrix[i][i]) < 1e-14 {
				continue // Skip if pivot is too small
			}
			factor := matrix[k][i] / matrix[i][i]
			for j := i; j < n; j++ {
				matrix[k][j] -= factor * matrix[i][j]
			}
			rhs[k] -= factor * rhs[i]
		}
	}

	// back substitution
	for i := n - 1; i >= 0; i-- {
		x[i] = rhs[i]
		for j := i + 1; j < n; j++ {
			x[i] -= matrix[i][j] * x[j]
		}
		if math.Abs(matrix[i][i]) > 1e-14 {
			x[i] /= matrix[i][i]
		}
	}

	return x
}

// computeCubicCoefficients computes tensor product cubic spline coefficients
func (f *Type0) computeCubicCoefficients() {
	if !f.UseCubic {
		return // Only compute for cubic splines
	}

	m, n := f.Shape()
	if m == 0 || n == 0 {
		return
	}

	// Convert samples to float64 array organized by grid points
	sampleData := f.convertSamplesToFloat64()

	// Compute tensor product spline coefficients
	f.cubicCoefficients = f.recursiveSplineFit(sampleData, 0)

	// Store coefficient shape for indexing
	f.cubicShape = make([]int, m)
	for i := 0; i < m; i++ {
		f.cubicShape[i] = 4 * (f.Size[i] - 1)
	}
}

// convertSamplesToFloat64 extracts all sample values as float64 arrays
func (f *Type0) convertSamplesToFloat64() []float64 {
	m, n := f.Shape()
	totalPoints := 1
	for i := 0; i < m; i++ {
		totalPoints *= f.Size[i]
	}

	result := make([]float64, totalPoints*n)

	// Extract samples for each grid point
	idx := 0
	f.iterateGridPoints(f.Size, func(gridIdx []int) {
		samples := f.getSamplesAt(gridIdx)
		for i := 0; i < n; i++ {
			result[idx*n+i] = samples[i]
		}
		idx++
	})

	return result
}

// iterateGridPoints calls fn for each grid point in lexicographic order
func (f *Type0) iterateGridPoints(sizes []int, fn func([]int)) {
	if len(sizes) == 0 {
		return
	}

	indices := make([]int, len(sizes))
	f.iterateGridPointsRecursive(sizes, indices, 0, fn)
}

// iterateGridPointsRecursive is the recursive helper for iterateGridPoints
func (f *Type0) iterateGridPointsRecursive(sizes, indices []int, dim int, fn func([]int)) {
	if dim == len(sizes) {
		// Make a copy since indices is reused
		copy := make([]int, len(indices))
		copy = append(copy[:0], indices...)
		fn(copy)
		return
	}

	for i := 0; i < sizes[dim]; i++ {
		indices[dim] = i
		f.iterateGridPointsRecursive(sizes, indices, dim+1, fn)
	}
}

// recursiveSplineFit applies 1D splines recursively along each dimension
func (f *Type0) recursiveSplineFit(data []float64, dim int) []float64 {
	m, n := f.Shape()

	if dim == m {
		// Base case: processed all dimensions
		return data
	}

	// Current data shape
	inputShape := f.getDataShape(dim)
	nPts := inputShape[dim]

	// Output shape: 4*(nPts-1) in dimension 'dim'
	outputShape := make([]int, len(inputShape))
	copy(outputShape, inputShape)
	outputShape[dim] = 4 * (nPts - 1)

	// Calculate strides for indexing
	inputStride := f.calculateStride(inputShape)
	outputStride := f.calculateStride(outputShape)

	// total size
	outputSize := 1
	for _, s := range outputShape {
		outputSize *= s
	}
	output := make([]float64, outputSize)

	// create grid coordinates for current dimension
	// for PDF functions, use normalized coordinates [0, 1, 2, ..., nPts-1]
	grid := make([]float64, nPts)
	for i := 0; i < nPts; i++ {
		grid[i] = float64(i)
	}

	// process each 1D "fiber" along dimension 'dim'
	f.processFibers(data, output, grid, dim, inputShape, outputShape, inputStride, outputStride, n)

	// Recursively process next dimension
	return f.recursiveSplineFit(output, dim+1)
}

// getDataShape returns the shape of data array at processing stage 'dim'
func (f *Type0) getDataShape(dim int) []int {
	m, n := f.Shape()
	shape := make([]int, m+1) // +1 for output dimension

	for i := 0; i < m; i++ {
		if i < dim {
			// Already processed: 4*(Size[i]-1) coefficients per interval
			shape[i] = 4 * (f.Size[i] - 1)
		} else {
			// Not yet processed: original grid size
			shape[i] = f.Size[i]
		}
	}
	shape[m] = n // Output dimension

	return shape
}

// calculateStride computes stride array for multi-dimensional indexing
func (f *Type0) calculateStride(shape []int) []int {
	stride := make([]int, len(shape))
	stride[len(shape)-1] = 1
	for i := len(shape) - 2; i >= 0; i-- {
		stride[i] = stride[i+1] * shape[i+1]
	}
	return stride
}

// processFibers applies 1D splines to all fibers along a given dimension
func (f *Type0) processFibers(input, output []float64, grid []float64, dim int, inputShape, outputShape, inputStride, outputStride []int, n int) {
	nPts := inputShape[dim]

	// Iterate over all possible fiber positions
	f.iterateFiberPositions(inputShape, dim, func(pos []int) {
		// Extract 1D fiber data
		fiberData := make([]float64, nPts*n)
		for i := 0; i < nPts; i++ {
			pos[dim] = i
			inputIdx := f.computeLinearIndex(pos, inputStride)
			for j := 0; j < n; j++ {
				fiberData[i*n+j] = input[inputIdx+j]
			}
		}

		// Process each output component separately
		for component := 0; component < n; component++ {
			// Extract y values for this component
			y := make([]float64, nPts)
			for i := 0; i < nPts; i++ {
				y[i] = fiberData[i*n+component]
			}

			// Compute 1D spline coefficients
			a, b, c, d := f.cubicSplineCoeff1D(grid, y)

			// Store coefficients in output array
			for i := 0; i < len(a); i++ {
				pos[dim] = i * 4
				outputIdx := f.computeLinearIndex(pos, outputStride)
				output[outputIdx+component] = a[i]

				pos[dim] = i*4 + 1
				outputIdx = f.computeLinearIndex(pos, outputStride)
				output[outputIdx+component] = b[i]

				pos[dim] = i*4 + 2
				outputIdx = f.computeLinearIndex(pos, outputStride)
				output[outputIdx+component] = c[i]

				pos[dim] = i*4 + 3
				outputIdx = f.computeLinearIndex(pos, outputStride)
				output[outputIdx+component] = d[i]
			}
		}
	})
}

// iterateFiberPositions calls fn for each position that defines a fiber along 'dim'
func (f *Type0) iterateFiberPositions(shape []int, dim int, fn func([]int)) {
	pos := make([]int, len(shape))
	f.iterateFiberPositionsRecursive(shape, pos, 0, dim, fn)
}

// iterateFiberPositionsRecursive is the recursive helper
func (f *Type0) iterateFiberPositionsRecursive(shape, pos []int, currentDim, skipDim int, fn func([]int)) {
	if currentDim == len(shape)-1 { // -1 because last dimension is output components
		// We've filled all spatial dimensions except the skip dimension
		fn(pos)
		return
	}

	if currentDim == skipDim {
		// Skip the fiber dimension - will be iterated in processFibers
		f.iterateFiberPositionsRecursive(shape, pos, currentDim+1, skipDim, fn)
		return
	}

	for i := 0; i < shape[currentDim]; i++ {
		pos[currentDim] = i
		f.iterateFiberPositionsRecursive(shape, pos, currentDim+1, skipDim, fn)
	}
}

// computeLinearIndex converts multi-dimensional index to linear index
func (f *Type0) computeLinearIndex(indices, stride []int) int {
	idx := 0
	for i := 0; i < len(indices)-1; i++ { // -1 to exclude output dimension from indexing
		idx += indices[i] * stride[i]
	}
	return idx
}

// sampleCubic evaluates the tensor product cubic spline at given indices
func (f *Type0) sampleCubic(indices []float64) []float64 {
	if len(f.cubicCoefficients) == 0 {
		f.computeCubicCoefficients()
	}

	m, n := f.Shape()

	// Find cell containing the point and compute local coordinates
	cellInfo := make([]struct {
		interval int
		t        float64
	}, m)

	for d := 0; d < m; d++ {
		// Find interval containing indices[d]
		idx := int(math.Floor(indices[d]))

		// Handle boundary cases
		if idx < 0 {
			idx = 0
		} else if idx >= f.Size[d]-1 {
			idx = f.Size[d] - 2
		}

		// Local coordinate within interval [0, 1]
		t := indices[d] - float64(idx)
		if t < 0 {
			t = 0
		} else if t > 1 {
			t = 1
		}

		cellInfo[d].interval = idx
		cellInfo[d].t = t
	}

	// Evaluate tensor product
	result := make([]float64, n)

	// Iterate over all 4^m combinations of basis functions
	f.iteratePowerCombinations(m, func(powers []int) {
		// Build coefficient array index
		coeffIdx := make([]int, m+1) // +1 for output dimension
		basisProd := 1.0

		for d := 0; d < m; d++ {
			interval := cellInfo[d].interval
			t := cellInfo[d].t

			// Coefficient index for this dimension: interval*4 + power
			coeffIdx[d] = interval*4 + powers[d]

			// Basis function value: t^power
			basisProd *= math.Pow(t, float64(powers[d]))
		}

		// Add contribution from this basis function to each output component
		for component := 0; component < n; component++ {
			coeffIdx[m] = component
			linearIdx := f.computeCubicLinearIndex(coeffIdx)
			if linearIdx < len(f.cubicCoefficients) {
				result[component] += f.cubicCoefficients[linearIdx] * basisProd
			}
		}
	})

	return result
}

// iteratePowerCombinations calls fn for all combinations of powers [0,1,2,3]^m
func (f *Type0) iteratePowerCombinations(m int, fn func([]int)) {
	powers := make([]int, m)
	f.iteratePowerCombinationsRecursive(powers, 0, fn)
}

// iteratePowerCombinationsRecursive is the recursive helper
func (f *Type0) iteratePowerCombinationsRecursive(powers []int, dim int, fn func([]int)) {
	if dim == len(powers) {
		// Make a copy since powers is reused
		powersCopy := make([]int, len(powers))
		copy(powersCopy, powers)
		fn(powersCopy)
		return
	}

	for p := 0; p < 4; p++ {
		powers[dim] = p
		f.iteratePowerCombinationsRecursive(powers, dim+1, fn)
	}
}

// computeCubicLinearIndex converts multi-dimensional coefficient index to linear index
func (f *Type0) computeCubicLinearIndex(indices []int) int {
	m, n := f.Shape()

	// Use row-major ordering: [dim0][dim1]...[dimM-1][component]
	// Total shape: [cubicShape[0], cubicShape[1], ..., cubicShape[m-1], n]

	idx := 0
	multiplier := 1

	// Start from the rightmost dimension (component)
	idx += indices[m] * multiplier
	multiplier *= n

	// Process spatial dimensions from right to left
	for i := m - 1; i >= 0; i-- {
		idx += indices[i] * multiplier
		multiplier *= f.cubicShape[i]
	}

	return idx
}
