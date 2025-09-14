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
	"bytes"
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
	// The length must be 2*m, where m is the number of input variables.
	Domain []float64

	// Range gives clipping ranges for the output variables as [min0, max0, min1, max1, ...].
	// The length must be 2*n, where n is the number of output variables.
	Range []float64

	// Size specifies the number of samples in each input dimension.
	// The length must be m, where m is the number of input variables.
	Size []int

	// BitsPerSample is the number of bits per sample value (1, 2, 4, 8, 12, 16, 24, 32).
	BitsPerSample int

	// UseCubic determines whether to use Catmull-Rom spline interpolation (true) or
	// linear interpolation (false).
	UseCubic bool

	// Encode maps inputs to sample table indices as [min0, max0, min1, max1, ...].
	// The length must be 2*m, where m is the number of input variables.
	Encode []float64

	// Decode maps samples to output range as [min0, max0, min1, max1, ...].
	// The length must be 2*n, where n is the number of output variables.
	Decode []float64

	// Samples contains the raw sample data.
	// This contains Size[0] * Size[1] * ... * Size[m-1] * n * BitsPerSample bits,
	// stored in a continuous bit stream (no padding), MSB first.
	Samples []byte
}

// FunctionType returns 0.
// This implements the [pdf.Function] interface.
func (f *Type0) FunctionType() int {
	return 0
}

// Shape returns the number of input and output values of the function.
func (f *Type0) Shape() (int, int) {
	return len(f.Domain) / 2, len(f.Range) / 2
}

// GetDomain returns the function's input domain.
func (f *Type0) GetDomain() []float64 {
	return f.Domain
}

// extractType0 reads a Type 0 sampled function from a PDF stream object.
func extractType0(r pdf.Getter, stream *pdf.Stream) (*Type0, error) {
	d := stream.Dict

	domain, err := pdf.GetFloatArray(r, d["Domain"])
	if err != nil {
		return nil, err
	}

	rangeArray, err := pdf.GetFloatArray(r, d["Range"])
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

	encode, err := pdf.GetFloatArray(r, d["Encode"])
	if err != nil {
		return nil, err
	}

	decode, err := pdf.GetFloatArray(r, d["Decode"])
	if err != nil {
		return nil, err
	}

	f := &Type0{
		Domain:        domain,
		Range:         rangeArray,
		Size:          size,
		BitsPerSample: int(bitsPerSample),
		UseCubic:      int(order) == 3,
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

// repair sets default values and tries to fix mal-formed function dicts.
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
	// We need to be careful here, because integer overflow may lead
	// to negative totalBytes:
	if totalBytes > 0 && len(f.Samples) > totalBytes {
		f.Samples = f.Samples[:totalBytes]
	}
}

// validate checks if the Type0 struct contains valid data.
func (f *Type0) validate() error {
	m, n := f.Shape()
	if m <= 0 || n <= 0 {
		return newInvalidFunctionError(0, "Shape", "invalid shape (%d, %d)", m, n)
	}

	if len(f.Domain) != 2*m {
		return newInvalidFunctionError(0, "Domain", "invalid length %d != %d", len(f.Domain), 2*m)
	}
	for i := 0; i < len(f.Domain); i += 2 {
		if !isRange(f.Domain[i], f.Domain[i+1]) {
			return newInvalidFunctionError(0, "Domain",
				"invalid domain [%g,%g] for input %d", f.Domain[i], f.Domain[i+1], i/2)
		}
	}

	if len(f.Range) != 2*n {
		return newInvalidFunctionError(0, "Range", "invalid length %d != %d", len(f.Range), 2*n)
	}
	for i := 0; i < len(f.Range); i += 2 {
		if !isRange(f.Range[i], f.Range[i+1]) {
			return newInvalidFunctionError(0, "Range",
				"invalid range [%g,%g] for output %d", f.Range[i], f.Range[i+1], i/2)
		}
	}

	if len(f.Size) != m {
		return newInvalidFunctionError(0, "Size", "invalid length %d != %d", len(f.Size), m)
	}
	for i, size := range f.Size {
		if size < 1 {
			return newInvalidFunctionError(0, "Size", "invalid size[%d] = %d < 1", i, size)
		}
	}

	switch f.BitsPerSample {
	case 1, 2, 4, 8, 12, 16, 24, 32:
	default:
		return newInvalidFunctionError(0, "BitsPerSample", "invalid value %d", f.BitsPerSample)
	}

	if len(f.Encode) != 2*m {
		return newInvalidFunctionError(0, "Encode", "invalid length %d != %d", len(f.Encode), 2*m)
	}
	for i := 0; i < len(f.Encode); i += 2 {
		if !isPair(f.Encode[i], f.Encode[i+1]) {
			return newInvalidFunctionError(0, "Encode",
				"invalid encode [%g,%g] for input %d", f.Encode[i], f.Encode[i+1], i/2)
		}
	}

	if len(f.Decode) != 2*n {
		return newInvalidFunctionError(0, "Decode", "invalid length %d != %d", 2*n, len(f.Decode))
	}
	for i := 0; i < len(f.Decode); i += 2 {
		if !isPair(f.Decode[i], f.Decode[i+1]) {
			return newInvalidFunctionError(0, "Decode",
				"invalid decode [%g,%g] for output %d", f.Decode[i], f.Decode[i+1], i/2)
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
		return newInvalidFunctionError(0, "Samples", "invalid length %d != %d", len(f.Samples), totalBytes)
	}

	return nil
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
	if _, err := stm.Write(f.Samples); err != nil {
		return nil, zero, err
	}
	if err := stm.Close(); err != nil {
		return nil, zero, err
	}

	return ref, zero, nil
}

// isDefaultEncode checks if the Encode array equals the default value.
func (f *Type0) isDefaultEncode() bool {
	for i := range f.Size {
		if f.Encode[2*i] != 0 || f.Encode[2*i+1] != float64(f.Size[i]-1) {
			return false
		}
	}
	return true
}

// isDefaultDecode checks if the Decode array equals the default value (same as Range).
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

// sampleLinear performs multidimensional linear interpolation on the sample table.
func (f *Type0) sampleLinear(indices []float64) []float64 {
	m, n := f.Shape()

	if m == 1 {
		return f.sample1D(indices[0], n)
	}

	floorIndices := make([]int, m)
	fractions := make([]float64, m)

	for i := range m {
		if f.Size[i] <= 1 {
			floorIndices[i] = 0
			fractions[i] = 0
			continue
		}

		floorIndices[i] = int(math.Floor(indices[i]))
		fractions[i] = indices[i] - float64(floorIndices[i])

		if floorIndices[i] < 0 {
			floorIndices[i] = 0
			fractions[i] = 0
		} else if floorIndices[i] >= f.Size[i]-1 {
			floorIndices[i] = f.Size[i] - 1
			fractions[i] = 0
		}
	}

	// If all fractions are zero, we can avoid interpolation.
	allExact := true
	for _, frac := range fractions {
		if frac != 0 {
			allExact = false
			break
		}
	}
	if allExact {
		return f.getSamplesAt(floorIndices)
	}

	numCorners := 1 << m
	result := make([]float64, n)
	for corner := range numCorners {
		weight := 1.0
		cornerIndices := make([]int, m)

		for dim := range m {
			if (corner>>dim)&1 == 0 {
				// corner is at the lower edge in this dimension
				cornerIndices[dim] = floorIndices[dim]
				weight *= 1 - fractions[dim]
			} else {
				// corner is at the upper edge in this dimension
				if floorIndices[dim]+1 < f.Size[dim] {
					cornerIndices[dim] = floorIndices[dim] + 1
				} else {
					cornerIndices[dim] = floorIndices[dim]
				}
				weight *= fractions[dim]
			}
		}

		if weight > 0 {
			cornerSamples := f.getSamplesAt(cornerIndices)
			for i := range n {
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

	if i0 < 0 {
		i0, i1, frac = 0, 0, 0
	} else if i0 >= f.Size[0]-1 {
		i0, i1, frac = f.Size[0]-1, f.Size[0]-1, 0
	}

	if frac == 0 {
		return f.getSamplesAt([]int{i0})
	}

	result := make([]float64, n)
	samples0 := f.getSamplesAt([]int{i0})
	samples1 := f.getSamplesAt([]int{i1})
	for i := range n {
		result[i] = samples0[i]*(1-frac) + samples1[i]*frac
	}

	return result
}

// getSamplesAt extracts sample values at the given multidimensional index.
// Indices must have length m, the result has length n.
func (f *Type0) getSamplesAt(indices []int) []float64 {
	m, n := f.Shape()

	// the first dimension varies fastest
	linearIndex := 0
	stride := 1
	for i := range m {
		linearIndex += indices[i] * stride
		stride *= f.Size[i]
	}

	samples := make([]float64, n)
	sampleIndex := linearIndex * n
	for i := range n {
		samples[i] = f.extractSampleAtIndex(sampleIndex + i)
	}

	return samples
}

// extractSampleAtIndex extracts a single sample value from the continuous bit stream.
func (f *Type0) extractSampleAtIndex(sampleIndex int) float64 {
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
		return float64((f.Samples[byteOffset] >> shift) & 0x03)

	case 4:
		if bitInByte == 0 {
			return float64(f.Samples[byteOffset] >> 4)
		}
		return float64(f.Samples[byteOffset] & 0x0F)

	case 8:
		return float64(f.Samples[byteOffset])

	case 12:
		if bitInByte == 0 {
			highByte := uint16(f.Samples[byteOffset]) << 4
			lowNibble := uint16(f.Samples[byteOffset+1]) >> 4
			return float64(highByte | lowNibble)
		}
		highNibble := uint16(f.Samples[byteOffset]&0x0F) << 8
		lowByte := uint16(f.Samples[byteOffset+1])
		return float64(highNibble | lowByte)

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

// sampleCubic performs multidimensional Catmull-Rom spline interpolation.
func (f *Type0) sampleCubic(indices []float64) []float64 {
	m, n := f.Shape()

	fparts := make([]float64, m)
	iparts := make([]int, m)
	for i, idx := range indices {
		ipart := int(math.Floor(idx))
		fpart := idx - float64(ipart)
		iparts[i] = ipart
		fparts[i] = fpart
	}

	// Calculate bit offset for the base position.
	factors := make([]int, m)
	offset := 0
	stride := 1
	for i := range m {
		factors[i] = stride * n * f.BitsPerSample
		offset += iparts[i] * factors[i]
		stride *= f.Size[i]
	}

	return f.interpolateCubicRecursive(fparts, iparts, factors, offset, m)
}

// interpolateCubicRecursive performs recursive multi-dimensional Catmull-Rom interpolation.
func (f *Type0) interpolateCubicRecursive(fparts []float64, iparts []int, factors []int, offset int, mCurrent int) []float64 {
	if mCurrent == 0 {
		return f.getSamplesAtBitOffset(offset)
	}

	mTotal, _ := f.Shape()
	mDim := mTotal - mCurrent

	samples := f.interpolateCubicRecursive(fparts, iparts, factors, offset, mCurrent-1)

	fpart := fparts[mDim]
	if fpart == 0 {
		return samples
	}

	ipart := iparts[mDim]
	delta := factors[mDim]
	size := f.Size[mDim]

	samples1 := f.interpolateCubicRecursive(fparts, iparts, factors, offset+delta, mCurrent-1)

	if size <= 2 {
		// Linear interpolation for domains with only two sample points.
		for j := range samples {
			samples[j] += (samples1[j] - samples[j]) * fpart
		}
		return samples
	}

	if ipart == 0 {
		// quadratic interpolation at the start of the domain (between first two points)
		samples2 := f.interpolateCubicRecursive(fparts, iparts, factors, offset+2*delta, mCurrent-1)
		for j := range samples {
			samples[j] = interpolateQuadratic(fpart, samples[j], samples1[j], samples2[j])
		}
		return samples
	}

	if ipart == size-2 {
		// quadratic interpolation at the end of the domain (between last two points)
		samplesm1 := f.interpolateCubicRecursive(fparts, iparts, factors, offset-delta, mCurrent-1)
		for j := range samples {
			samples[j] = interpolateQuadratic(1-fpart, samples1[j], samples[j], samplesm1[j])
		}
		return samples
	}

	// full Catmull-Rom cubic interpolation for interior points
	samplesm1 := f.interpolateCubicRecursive(fparts, iparts, factors, offset-delta, mCurrent-1)
	samples2 := f.interpolateCubicRecursive(fparts, iparts, factors, offset+2*delta, mCurrent-1)
	for j := range samples {
		samples[j] = interpolateCatmullRom(fpart, samplesm1[j], samples[j], samples1[j], samples2[j])
	}
	return samples
}

// getSamplesAtBitOffset extracts n samples starting at a given bit offset.
func (f *Type0) getSamplesAtBitOffset(offset int) []float64 {
	_, n := f.Shape()
	samples := make([]float64, n)
	firstSampleIndex := offset / f.BitsPerSample
	for i := range n {
		samples[i] = f.extractSampleAtIndex(firstSampleIndex + i)
	}
	return samples
}

// interpolateCatmullRom performs 1D Catmull-Rom spline interpolation.
// It computes an interpolated value between p1 and p2, using p0 and p3 as
// control points to define the tangents.
// The parameter t is expected to be in [0, 1].
func interpolateCatmullRom(t, p0, p1, p2, p3 float64) float64 {
	x := t + 1
	xm1 := t
	m2x := 1 - t
	m3x := 2 - t

	const a = -0.5

	c0 := a*x*x*x - 5*a*x*x + 8*a*x - 4*a
	c1 := (a+2)*xm1*xm1*xm1 - (a+3)*xm1*xm1 + 1
	c2 := (a+2)*m2x*m2x*m2x - (a+3)*m2x*m2x + 1
	c3 := a*m3x*m3x*m3x - 5*a*m3x*m3x + 8*a*m3x - 4*a

	return c0*p0 + c1*p1 + c2*p2 + c3*p3
}

// interpolateQuadratic performs quadratic interpolation at the boundaries
// using the Catmull-Rom formula with a duplicated point.
// The parameter t is in [0, 1], interpolating between p1 and p2.
func interpolateQuadratic(t, p1, p2, p3 float64) float64 {
	return interpolateCatmullRom(t, p1, p1, p2, p3)
}

// Equal reports whether f and other represent the same Type0 function.
func (f *Type0) Equal(other *Type0) bool {
	if f == nil || other == nil {
		return f == other
	}

	if !floatSlicesEqual(f.Domain, other.Domain, floatEpsilon) {
		return false
	}
	if !floatSlicesEqual(f.Range, other.Range, floatEpsilon) {
		return false
	}

	if !intSlicesEqual(f.Size, other.Size) {
		return false
	}

	if f.BitsPerSample != other.BitsPerSample {
		return false
	}
	if f.UseCubic != other.UseCubic {
		return false
	}

	if !floatSlicesEqual(f.Encode, other.Encode, floatEpsilon) {
		return false
	}
	if !floatSlicesEqual(f.Decode, other.Decode, floatEpsilon) {
		return false
	}

	if !bytes.Equal(f.Samples, other.Samples) {
		return false
	}

	return true
}
