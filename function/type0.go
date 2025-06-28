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

const maxSampleBits = 1 << 23 // 1MB

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
}

// FunctionType returns 0.
// This implements the [pdf.Function] interface.
func (f *Type0) FunctionType() int {
	return 0
}

// Shape returns the number of input and output values of the function.
func (f *Type0) Shape() (int, int) {
	m := len(f.Domain) / 2
	n := len(f.Range) / 2
	return m, n
}

// extractType0 reads a Type 0 sampled function from a PDF stream object.
func extractType0(r pdf.Getter, stream *pdf.Stream) (*Type0, error) {
	d := stream.Dict

	domain, err := readFloats(r, d["Domain"])
	if err != nil {
		return nil, err
	}

	rangeArray, err := readFloats(r, d["Range"])
	if err != nil {
		return nil, err
	}

	size, err := readInts(r, d["Size"])
	if err != nil {
		return nil, err
	}

	bitsPerSample, err := pdf.GetInteger(r, d["BitsPerSample"])
	if err != nil {
		return nil, err
	}

	order, err := pdf.GetInteger(r, d["Order"])
	if err != nil {
		return nil, err
	}
	useCubic := (int(order) == 3)

	encode, err := readFloats(r, d["Encode"])
	if err != nil {
		return nil, err
	}

	decode, err := readFloats(r, d["Decode"])
	if err != nil {
		return nil, err
	}

	f := &Type0{
		Domain:        domain,
		Range:         rangeArray,
		Size:          size,
		BitsPerSample: int(bitsPerSample),
		UseCubic:      useCubic,
		Encode:        encode,
		Decode:        decode,
	}

	stmReader, err := pdf.DecodeStream(r, stream, 0)
	if err != nil {
		return nil, err
	}
	defer stmReader.Close()

	f.Samples, err = io.ReadAll(stmReader)
	if err != nil {
		return nil, err
	}

	f.repair()
	if err := f.validate(); err != nil {
		return nil, err
	}

	return f, nil
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

	if len(f.Encode) == 0 {
		f.Encode = make([]float64, 2*m)
		for i := 0; i < min(m, len(f.Size)); i++ {
			f.Encode[2*i] = 0
			f.Encode[2*i+1] = float64(f.Size[i] - 1)
		}
	} else if len(f.Encode) > 2*m {
		f.Encode = f.Encode[:2*m]
	}

	if len(f.Decode) == 0 {
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
	if totalBytes > 0 && len(f.Samples) > totalBytes {
		f.Samples = f.Samples[:totalBytes]
	}
}

// validate checks if the Type0 struct contains valid data
func (f *Type0) validate() error {
	m, n := f.Shape()
	if m <= 0 || n <= 0 {
		return newInvalidFunctionError(0, "Shape", "invalid shape (%d, %d)",
			m, n)
	}

	if len(f.Domain) != 2*m {
		return newInvalidFunctionError(0, "Domain", "invalid length %d != %d",
			len(f.Domain), 2*m)
	}
	for i := 0; i < len(f.Domain); i += 2 {
		if !isRange(f.Domain[i], f.Domain[i+1]) {
			return newInvalidFunctionError(0, "Domain",
				"invalid domain [%g,%g] for input %d",
				f.Domain[i], f.Domain[i+1], i/2)
		}
	}

	if len(f.Range) != 2*n {
		return newInvalidFunctionError(0, "Range", "invalid length %d != %d",
			len(f.Range), 2*n)
	}
	for i := 0; i < len(f.Range); i += 2 {
		if !isRange(f.Range[i], f.Range[i+1]) {
			return newInvalidFunctionError(0, "Range",
				"invalid range [%g,%g] for output %d",
				f.Range[i], f.Range[i+1], i/2)
		}
	}

	if len(f.Size) != m {
		return newInvalidFunctionError(0, "Size", "invalid length %d != %d",
			len(f.Size), m)
	}
	for i, size := range f.Size {
		if size < 1 {
			return newInvalidFunctionError(0, "Size", "invalid size[%d] = %d < 1",
				i, size)
		}
	}

	switch f.BitsPerSample {
	case 1, 2, 4, 8, 12, 16, 24, 32:
		// pass
	default:
		return newInvalidFunctionError(0, "BitsPerSample", "invalid value %d",
			f.BitsPerSample)
	}

	if len(f.Encode) != 2*m {
		return newInvalidFunctionError(0, "Encode", "invalid length %d != %d",
			len(f.Encode), 2*m)
	}
	for i := 0; i < len(f.Encode); i += 2 {
		if !isPair(f.Encode[i], f.Encode[i+1]) {
			return newInvalidFunctionError(0, "Encode",
				"invalid encode [%g,%g] for input %d",
				f.Encode[i], f.Encode[i+1], i/2)
		}
	}

	if len(f.Decode) != 2*n {
		return newInvalidFunctionError(0, "Decode", "length must be %d, got %d",
			2*n, len(f.Decode))
	}
	for i := 0; i < len(f.Decode); i += 2 {
		if !isPair(f.Decode[i], f.Decode[i+1]) {
			return newInvalidFunctionError(0, "Decode",
				"invalid decode [%g,%g] for output %d",
				f.Decode[i], f.Decode[i+1], i/2)
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
	totalBits := totalSamples * (n * f.BitsPerSample)
	if totalSamples != totalBits/(n*f.BitsPerSample) {
		return errors.New("sample size overflow")
	}

	if totalBits > maxSampleBits {
		return errors.New("too many samples in Type 0 function")
	}

	totalBytes := (totalBits + 7) / 8
	if len(f.Samples) != totalBytes {
		return newInvalidFunctionError(0, "Samples", "length must be %d bytes, got %d",
			totalBytes, len(f.Samples))
	}

	return nil
}

// Embed embeds the function into a PDF file.
func (f *Type0) Embed(rm *pdf.ResourceManager) (pdf.Native, pdf.Unused, error) {
	var zero pdf.Unused

	if err := pdf.CheckVersion(rm.Out, "Type 0 functions", pdf.V1_2); err != nil {
		return nil, zero, err
	} else if err := f.validate(); err != nil {
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

	// TODO(voss): be more clever here
	compress := pdf.FilterFlate{
		"Predictor": pdf.Integer(15),
	}
	stm, err := rm.Out.OpenStream(ref, dict, compress)
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

	indices := make([]float64, m)
	for i, val := range inputs {
		val = clip(val, f.Domain[2*i], f.Domain[2*i+1])
		val = interpolate(val, f.Domain[2*i], f.Domain[2*i+1], f.Encode[2*i], f.Encode[2*i+1])
		indices[i] = clip(val, 0, float64(f.Size[i]-1))
	}

	var samples []float64
	if f.UseCubic {
		samples = f.sampleCubic(indices)
	} else {
		samples = f.sampleLinear(indices)
	}

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
		// optimize for common single input case
		return f.sample1D(indices[0], n)
	}

	// For multidimensional case, use separable linear interpolation
	floorIndices := make([]int, m)
	fractions := make([]float64, m)

	for i := 0; i < m; i++ {
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
				if floorIndices[dim]+1 < f.Size[dim] {
					cornerIndices[dim] = floorIndices[dim] + 1
				} else {
					cornerIndices[dim] = floorIndices[dim]
				}
				weight *= fractions[dim]
			}
		}

		// skip corners with zero weight for efficiency
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

	// avoid interpolation when fraction is exactly 0
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
	// PDF spec: "the first dimension varies fastest"
	linearIndex := 0
	stride := 1
	for i := 0; i < m; i++ {
		linearIndex += indices[i] * stride
		stride *= f.Size[i]
	}

	// Extract samples at this position
	samples := make([]float64, n)

	sampleIndex := linearIndex * n
	for i := range n {
		samples[i] = f.extractSampleAtIndex(sampleIndex + i)
	}

	return samples
}

// extractSampleAtIndex extracts a single sample value at the given sample index
// from the continuous bit stream
func (f *Type0) extractSampleAtIndex(sampleIndex int) float64 {
	if sampleIndex < 0 {
		return 0
	}

	bitOffset := sampleIndex * f.BitsPerSample
	byteOffset := bitOffset / 8
	bitInByte := bitOffset % 8

	if byteOffset >= len(f.Samples) || byteOffset < 0 {
		return 0
	}

	switch f.BitsPerSample {
	case 1:
		mask := byte(1 << (7 - bitInByte))
		if f.Samples[byteOffset]&mask != 0 {
			return 1
		}
		return 0

	case 2:
		shift := 6 - bitInByte
		return float64((f.Samples[byteOffset] >> shift) & 0b00000011)

	case 4:
		if bitInByte == 0 {
			return float64(f.Samples[byteOffset] >> 4)
		} else { // bitInByte == 4
			return float64(f.Samples[byteOffset] & 0x0F)
		}

	case 8:
		return float64(f.Samples[byteOffset])

	case 12:
		if bitInByte == 0 {
			highByte := uint16(f.Samples[byteOffset]) << 4
			lowNibble := uint16(f.Samples[byteOffset+1]) >> 4
			return float64(highByte | lowNibble)
		} else { // bitInByte == 4
			highNibble := uint16(f.Samples[byteOffset]&0x0F) << 8
			lowByte := uint16(f.Samples[byteOffset+1])
			return float64(highNibble | lowByte)
		}

	case 16:
		return float64(uint16(f.Samples[byteOffset])<<8 | uint16(f.Samples[byteOffset+1]))

	case 24:
		val := uint32(f.Samples[byteOffset])<<16 |
			uint32(f.Samples[byteOffset+1])<<8 |
			uint32(f.Samples[byteOffset+2])
		return float64(val)

	case 32:
		val := uint32(f.Samples[byteOffset])<<24 |
			uint32(f.Samples[byteOffset+1])<<16 |
			uint32(f.Samples[byteOffset+2])<<8 |
			uint32(f.Samples[byteOffset+3])
		return float64(val)

	default:
		return 0
	}
}

// cubicSplineCoeff1D computes cubic spline coefficients for 1D data using
// clamped boundary conditions (first derivative = 0 at endpoints).
// Returns coefficients [a, b, c, d] for each interval.
func (f *Type0) cubicSplineCoeff1D(x, y []float64) ([]float64, []float64, []float64, []float64) {
	n := len(x)
	if n < 2 {
		panic("need at least 2 points for cubic spline")
	}

	// fallback to linear interpolation for 2 points
	if n == 2 {
		a := []float64{y[0]}
		b := []float64{(y[1] - y[0]) / (x[1] - x[0])}
		c := []float64{0.0}
		d := []float64{0.0}
		return a, b, c, d
	}

	h := make([]float64, n-1)
	for i := 0; i < n-1; i++ {
		h[i] = x[i+1] - x[i]
	}

	// build tridiagonal system for second derivatives M
	// clamped boundary conditions: first derivative = 0 at endpoints
	A := make([][]float64, n)
	rhs := make([]float64, n)

	for i := range A {
		A[i] = make([]float64, n)
	}

	// left boundary condition: S'(x_0) = 0
	// this gives: 2*M[0] + M[1] = 6*(y[1]-y[0])/h[0]^2
	A[0][0] = 2.0
	A[0][1] = 1.0
	rhs[0] = 6.0 * (y[1] - y[0]) / (h[0] * h[0])

	// right boundary condition: S'(x_{n-1}) = 0
	// this gives: M[n-2] + 2*M[n-1] = -6*(y[n-1]-y[n-2])/h[n-2]^2
	A[n-1][n-2] = 1.0
	A[n-1][n-1] = 2.0
	rhs[n-1] = -6.0 * (y[n-1] - y[n-2]) / (h[n-2] * h[n-2])

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

	// make a copy to avoid modifying input
	matrix := make([][]float64, n)
	rhs := make([]float64, n)
	for i := range matrix {
		matrix[i] = make([]float64, n)
		copy(matrix[i], A[i])
		rhs[i] = b[i]
	}

	for i := 0; i < n-1; i++ {
		maxRow := i
		for k := i + 1; k < n; k++ {
			if math.Abs(matrix[k][i]) > math.Abs(matrix[maxRow][i]) {
				maxRow = k
			}
		}

		if maxRow != i {
			matrix[i], matrix[maxRow] = matrix[maxRow], matrix[i]
			rhs[i], rhs[maxRow] = rhs[maxRow], rhs[i]
		}

		for k := i + 1; k < n; k++ {
			if math.Abs(matrix[i][i]) < 1e-14 {
				continue
			}
			factor := matrix[k][i] / matrix[i][i]
			for j := i; j < n; j++ {
				matrix[k][j] -= factor * matrix[i][j]
			}
			rhs[k] -= factor * rhs[i]
		}
	}
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

// sampleCubic evaluates the cubic spline at given indices
func (f *Type0) sampleCubic(indices []float64) []float64 {
	m, _ := f.Shape()

	if m == 1 {
		return f.sampleCubic1DDirect(indices[0])
	} else if m == 2 {
		return f.sampleBicubicDirect(indices[0], indices[1])
	} else {
		// higher dimensions fall back to linear for now
		return f.sampleLinear(indices)
	}
}

// sampleCubic1DDirect performs direct 1D cubic spline evaluation
func (f *Type0) sampleCubic1DDirect(coord float64) []float64 {
	_, n := f.Shape()
	gridSize := f.Size[0]

	samples := make([][]float64, gridSize)
	for i := 0; i < gridSize; i++ {
		samples[i] = f.getSamplesAt([]int{i})
	}

	grid := make([]float64, gridSize)
	for i := 0; i < gridSize; i++ {
		grid[i] = float64(i)
	}

	result := make([]float64, n)
	for component := 0; component < n; component++ {
		values := make([]float64, gridSize)
		for i := 0; i < gridSize; i++ {
			values[i] = samples[i][component]
		}

		a, b, c, d := f.cubicSplineCoeff1D(grid, values)
		result[component] = f.evaluateCubicSplineAt(a, b, c, d, grid, coord)
	}

	return result
}

// sampleBicubicDirect performs direct 2D bicubic evaluation using separable cubic splines
func (f *Type0) sampleBicubicDirect(x, y float64) []float64 {
	_, n := f.Shape()
	xSize, ySize := f.Size[0], f.Size[1]

	// step 1: for each row (constant y), evaluate 1D cubic spline at x
	rowResults := make([][]float64, ySize)
	xGrid := make([]float64, xSize)
	for i := 0; i < xSize; i++ {
		xGrid[i] = float64(i)
	}

	for j := 0; j < ySize; j++ {
		rowSamples := make([][]float64, xSize)
		for i := 0; i < xSize; i++ {
			rowSamples[i] = f.getSamplesAt([]int{i, j})
		}

		rowResult := make([]float64, n)
		for component := 0; component < n; component++ {
			values := make([]float64, xSize)
			for i := 0; i < xSize; i++ {
				values[i] = rowSamples[i][component]
			}

			a, b, c, d := f.cubicSplineCoeff1D(xGrid, values)
			rowResult[component] = f.evaluateCubicSplineAt(a, b, c, d, xGrid, x)
		}
		rowResults[j] = rowResult
	}

	// step 2: for each component, fit 1D cubic spline along y using row results
	yGrid := make([]float64, ySize)
	for j := 0; j < ySize; j++ {
		yGrid[j] = float64(j)
	}

	result := make([]float64, n)
	for component := 0; component < n; component++ {
		values := make([]float64, ySize)
		for j := 0; j < ySize; j++ {
			values[j] = rowResults[j][component]
		}

		a, b, c, d := f.cubicSplineCoeff1D(yGrid, values)
		result[component] = f.evaluateCubicSplineAt(a, b, c, d, yGrid, y)
	}

	return result
}

// evaluateCubicSplineAt evaluates a 1D cubic spline at a given coordinate
func (f *Type0) evaluateCubicSplineAt(a, b, c, d []float64, grid []float64, coord float64) float64 {
	idx := int(math.Floor(coord))
	if idx < 0 {
		idx = 0
	} else if idx >= len(a) {
		idx = len(a) - 1
	}

	t := coord - grid[idx]
	if t < 0 {
		t = 0
	} else if t > 1 {
		t = 1
	}

	// evaluate cubic polynomial: S(t) = a + b*t + c*t² + d*t³
	return a[idx] + b[idx]*t + c[idx]*t*t + d[idx]*t*t*t
}
