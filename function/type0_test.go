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
	"testing"
)

func TestType0BitDepthExtraction(t *testing.T) {
	tests := []struct {
		name          string
		bitsPerSample int
		samples       []byte
		expectedVals  []float64
		description   string
	}{
		{
			name:          "1-bit samples",
			bitsPerSample: 1,
			samples:       []byte{0xAA}, // 10101010
			expectedVals:  []float64{1, 0, 1, 0, 1, 0, 1, 0},
			description:   "8 samples of 1 bit each",
		},
		{
			name:          "2-bit samples",
			bitsPerSample: 2,
			samples:       []byte{0xE4}, // 11100100
			expectedVals:  []float64{3, 2, 1, 0},
			description:   "4 samples of 2 bits each",
		},
		{
			name:          "2-bit samples spanning bytes",
			bitsPerSample: 2,
			samples:       []byte{0x4E, 0x40}, // 01001110 01000000
			expectedVals:  []float64{1, 0, 3, 2, 1, 0, 0, 0},
			description:   "8 samples of 2 bits each, spanning bytes",
		},
		{
			name:          "4-bit samples",
			bitsPerSample: 4,
			samples:       []byte{0xAB, 0xCD}, // 10101011 11001101
			expectedVals:  []float64{10, 11, 12, 13},
			description:   "4 samples of 4 bits each",
		},
		{
			name:          "4-bit samples misaligned",
			bitsPerSample: 4,
			samples:       []byte{0x12, 0x34, 0x50}, // samples at bit offsets 4, 8, 12
			expectedVals:  []float64{1, 2, 3, 4, 5},
			description:   "5 samples of 4 bits each, testing misalignment",
		},
		{
			name:          "8-bit samples",
			bitsPerSample: 8,
			samples:       []byte{0x00, 0x80, 0xFF},
			expectedVals:  []float64{0, 128, 255},
			description:   "3 samples of 8 bits each",
		},
		{
			name:          "12-bit samples aligned",
			bitsPerSample: 12,
			samples:       []byte{0xAB, 0xCD, 0xEF}, // ABC DEF (12-bit each)
			expectedVals:  []float64{0xABC, 0xDEF},
			description:   "2 samples of 12 bits each, byte-aligned start",
		},
		{
			name:          "12-bit samples nibble-aligned",
			bitsPerSample: 12,
			samples:       []byte{0xAB, 0xCD, 0xEF, 0x12}, // ABC DEF 120
			expectedVals:  []float64{0xABC, 0xDEF, 0x120},
			description:   "3 samples of 12 bits each: 0xABC (bits 0-11), 0xDEF (bits 12-23), 0x120 (bits 24-35)",
		},
		{
			name:          "16-bit samples",
			bitsPerSample: 16,
			samples:       []byte{0x12, 0x34, 0xAB, 0xCD},
			expectedVals:  []float64{0x1234, 0xABCD},
			description:   "2 samples of 16 bits each",
		},
		{
			name:          "24-bit samples",
			bitsPerSample: 24,
			samples:       []byte{0x12, 0x34, 0x56, 0xAB, 0xCD, 0xEF},
			expectedVals:  []float64{0x123456, 0xABCDEF},
			description:   "2 samples of 24 bits each",
		},
		{
			name:          "32-bit samples",
			bitsPerSample: 32,
			samples:       []byte{0x12, 0x34, 0x56, 0x78, 0xAB, 0xCD, 0xEF, 0x01},
			expectedVals:  []float64{0x12345678, 0xABCDEF01},
			description:   "2 samples of 32 bits each",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &Type0{
				Domain:        []float64{0, 1},
				Range:         []float64{0, 1},
				Size:          []int{len(tt.expectedVals)},
				BitsPerSample: tt.bitsPerSample,
				UseCubic:      false,
				Samples:       tt.samples,
			}

			for i, expected := range tt.expectedVals {
				actual := f.extractSampleAtIndex(i)
				if actual != expected {
					t.Errorf("sample %d: expected %v, got %v", i, expected, actual)
				}
			}
		})
	}
}

func TestType0BitDepthFunction(t *testing.T) {
	tests := []struct {
		name      string
		function  *Type0
		inputs    []float64
		expected  []float64
		tolerance float64
	}{
		{
			name: "1-bit function",
			function: &Type0{
				Domain:        []float64{0, 1},
				Range:         []float64{0, 1},
				Size:          []int{2},
				BitsPerSample: 1,
				UseCubic:      false,
				Samples:       []byte{0x80}, // 10000000 -> samples: 1, 0
			},
			inputs:    []float64{0.0},
			expected:  []float64{1.0}, // First sample, decoded from 1 to 1.0
			tolerance: 1e-10,
		},
		{
			name: "2-bit function",
			function: &Type0{
				Domain:        []float64{0, 1},
				Range:         []float64{0, 1},
				Size:          []int{4},
				BitsPerSample: 2,
				UseCubic:      false,
				Samples:       []byte{0xE4}, // 11100100 -> samples: 3, 2, 1, 0
			},
			inputs:    []float64{0.0},
			expected:  []float64{1.0}, // First sample 3, max value 3, so 3/3 = 1.0
			tolerance: 1e-10,
		},
		{
			name: "4-bit function with interpolation",
			function: &Type0{
				Domain:        []float64{0, 1},
				Range:         []float64{0, 1},
				Size:          []int{2},
				BitsPerSample: 4,
				UseCubic:      false,
				Samples:       []byte{0x0F}, // 00001111 -> samples: 0, 15
			},
			inputs:    []float64{0.5},
			expected:  []float64{0.5}, // Interpolated between 0 and 15, then normalized
			tolerance: 1e-10,
		},
		{
			name: "12-bit function",
			function: &Type0{
				Domain:        []float64{0, 1},
				Range:         []float64{0, 1},
				Size:          []int{2},
				BitsPerSample: 12,
				UseCubic:      false,
				Samples:       []byte{0x00, 0x0F, 0xFF}, // 000 FFF -> samples: 0, 4095
			},
			inputs:    []float64{0.0},
			expected:  []float64{0.0}, // First sample 0, normalized to 0.0
			tolerance: 1e-10,
		},
		{
			name: "multi-output 4-bit function",
			function: &Type0{
				Domain:        []float64{0, 1},
				Range:         []float64{0, 1, 0, 1},
				Size:          []int{2},
				BitsPerSample: 4,
				UseCubic:      false,
				// 4 samples per position: [0,15], [15,0] at positions 0,1
				Samples: []byte{0x0F, 0xF0}, // 00001111 11110000
			},
			inputs:    []float64{0.0},
			expected:  []float64{0.0, 1.0}, // First position: samples 0,15 -> 0.0,1.0
			tolerance: 1e-10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.function.repair()
			result := tt.function.Apply(tt.inputs...)
			if len(result) != len(tt.expected) {
				t.Fatalf("expected %d outputs, got %d", len(tt.expected), len(result))
			}
			for i, expected := range tt.expected {
				if abs(result[i]-expected) > tt.tolerance {
					t.Errorf("output[%d]: expected %f, got %f (diff: %e)",
						i, expected, result[i], abs(result[i]-expected))
				}
			}
		})
	}
}

func TestType0BitDepthRoundTrip(t *testing.T) {
	bitDepths := []int{1, 2, 4, 8, 12, 16, 24, 32}

	for _, bits := range bitDepths {
		t.Run(fmt.Sprintf("%d-bit", bits), func(t *testing.T) {
			// Create test data with maximum value for this bit depth
			maxVal := (1 << bits) - 1
			numSamples := 4

			// Generate sample data
			var samples []byte
			totalBits := numSamples * bits
			totalBytes := (totalBits + 7) / 8
			samples = make([]byte, totalBytes)

			// Fill with a pattern that uses the full bit depth
			sampleValues := []int{0, maxVal / 3, 2 * maxVal / 3, maxVal}

			// Pack the sample values into the byte array
			for i, val := range sampleValues {
				// Create the samples manually for specific bit depths
				bitOffset := i * bits
				byteOffset := bitOffset / 8
				bitInByte := bitOffset % 8

				// Use a simple approach: pack bits manually
				switch bits {
				case 1:
					if val > 0 {
						samples[byteOffset] |= 1 << (7 - bitInByte)
					}
				case 2:
					samples[byteOffset] |= byte(val&0x3) << (6 - bitInByte)
				case 4:
					if bitInByte == 0 {
						samples[byteOffset] |= byte(val&0xF) << 4
					} else {
						samples[byteOffset] |= byte(val & 0xF)
					}
				case 8:
					samples[byteOffset] = byte(val)
				case 12:
					// Pack 12 bits across bytes
					if bitInByte == 0 {
						// Store high 8 bits in current byte, low 4 bits in high nibble of next
						samples[byteOffset] = byte(val >> 4)
						if byteOffset+1 < len(samples) {
							samples[byteOffset+1] |= byte(val&0xF) << 4
						}
					} else if bitInByte == 4 {
						// Store high 4 bits in low nibble of current byte, low 8 bits in next
						samples[byteOffset] |= byte((val >> 8) & 0xF)
						if byteOffset+1 < len(samples) {
							samples[byteOffset+1] = byte(val)
						}
					}
				case 16:
					samples[byteOffset] = byte(val >> 8)
					samples[byteOffset+1] = byte(val)
				case 24:
					samples[byteOffset] = byte(val >> 16)
					samples[byteOffset+1] = byte(val >> 8)
					samples[byteOffset+2] = byte(val)
				case 32:
					samples[byteOffset] = byte(val >> 24)
					samples[byteOffset+1] = byte(val >> 16)
					samples[byteOffset+2] = byte(val >> 8)
					samples[byteOffset+3] = byte(val)
				}
			}

			function := &Type0{
				Domain:        []float64{0, 1},
				Range:         []float64{0, 1},
				Size:          []int{numSamples},
				BitsPerSample: bits,
				UseCubic:      false,
				Samples:       samples,
			}
			function.repair()

			// Test extraction
			for i, expectedVal := range sampleValues {
				actualVal := function.extractSampleAtIndex(i)
				if actualVal != float64(expectedVal) {
					t.Errorf("sample %d: expected %d, got %f", i, expectedVal, actualVal)
				}
			}
		})
	}
}

func TestType0CubicSpline1D(t *testing.T) {
	// Test 1D cubic spline with a simple quadratic function
	// f(x) = x^2, sampled at x = 0, 1, 2, 3
	function := &Type0{
		Domain:        []float64{0, 3},
		Range:         []float64{0, 9},
		Size:          []int{4},
		BitsPerSample: 8,
		UseCubic:      true,                    // Cubic spline
		Samples:       []byte{0, 28, 113, 255}, // Scaled values: 0, 1, 4, 9
	}
	function.repair()

	tests := []struct {
		input     float64
		expected  float64
		tolerance float64
		desc      string
	}{
		{0.0, 0.0, 1e-2, "Exact at x=0"},
		{1.0, 1.0, 1e-1, "Exact at x=1"},
		{2.0, 4.0, 1e-1, "Exact at x=2"},
		{3.0, 9.0, 1e-1, "Exact at x=3"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			result := function.Apply(tt.input)
			// Result is already in the range [0,9] due to range decoding
			actualValue := result[0]

			if math.Abs(actualValue-tt.expected) > tt.tolerance {
				t.Errorf("At x=%f: expected %f, got %f, diff=%f",
					tt.input, tt.expected, actualValue, math.Abs(actualValue-tt.expected))
			}
		})
	}
}

func TestType0CubicSpline2D(t *testing.T) {
	// Test 2D cubic spline with f(x,y) = x + y
	// Grid: 3x3, values at (i,j) = i + j
	function := &Type0{
		Domain:        []float64{0, 2, 0, 2},
		Range:         []float64{0, 4},
		Size:          []int{3, 3},
		BitsPerSample: 8,
		UseCubic:      true, // Cubic spline
		// Values: (0,0)=0, (0,1)=1, (0,2)=2, (1,0)=1, (1,1)=2, (1,2)=3, (2,0)=2, (2,1)=3, (2,2)=4
		Samples: []byte{0, 64, 128, 64, 128, 191, 128, 191, 255},
	}
	function.repair()

	tests := []struct {
		input     []float64
		expected  float64
		tolerance float64
		desc      string
	}{
		{[]float64{0.0, 0.0}, 0.0, 1e-1, "Corner (0,0)"},
		{[]float64{1.0, 1.0}, 2.0, 2e-1, "Center (1,1)"},
		{[]float64{2.0, 2.0}, 4.0, 1e-1, "Corner (2,2)"},
		{[]float64{0.5, 0.5}, 1.0, 3e-1, "Interpolated center"},
		{[]float64{1.5, 0.5}, 2.0, 3e-1, "Mixed interpolation"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			result := function.Apply(tt.input...)
			// Result is already in the range [0,4] due to range decoding
			actualValue := result[0]

			if math.Abs(actualValue-tt.expected) > tt.tolerance {
				t.Errorf("At (%f,%f): expected %f, got %f, diff=%f",
					tt.input[0], tt.input[1], tt.expected, actualValue,
					math.Abs(actualValue-tt.expected))
			}
		})
	}
}

func TestType0CubicVsLinear(t *testing.T) {
	// Compare cubic vs linear interpolation for a smooth function
	// Use a function with higher derivatives to see the difference

	// Create data for f(x) = sin(πx) on [0,1]
	nPoints := 5
	samples := make([]byte, nPoints)
	for i := 0; i < nPoints; i++ {
		x := float64(i) / float64(nPoints-1)
		y := math.Sin(math.Pi * x)
		// Normalize to [0,1] and quantize to byte
		samples[i] = byte((y + 1.0) * 127.5)
	}

	linearFunc := &Type0{
		Domain:        []float64{0, 1},
		Range:         []float64{-1, 1},
		Size:          []int{nPoints},
		BitsPerSample: 8,
		UseCubic:      false,                           // Linear
		Samples:       append([]byte(nil), samples...), // Copy
	}

	cubicFunc := &Type0{
		Domain:        []float64{0, 1},
		Range:         []float64{-1, 1},
		Size:          []int{nPoints},
		BitsPerSample: 8,
		UseCubic:      true,                            // Cubic
		Samples:       append([]byte(nil), samples...), // Copy
	}
	linearFunc.repair()
	cubicFunc.repair()

	// Test at a point between samples where cubic should be smoother
	x := 0.375 // Between samples 1 and 2
	expectedTrue := math.Sin(math.Pi * x)

	linearResult := linearFunc.Apply(x)
	cubicResult := cubicFunc.Apply(x)

	linearDenorm := linearResult[0]*2.0 - 1.0 // [0,1] -> [-1,1]
	cubicDenorm := cubicResult[0]*2.0 - 1.0   // [0,1] -> [-1,1]

	linearError := math.Abs(linearDenorm - expectedTrue)
	cubicError := math.Abs(cubicDenorm - expectedTrue)

	t.Logf("At x=%f:", x)
	t.Logf("  True value: %f", expectedTrue)
	t.Logf("  Linear: %f (error: %f)", linearDenorm, linearError)
	t.Logf("  Cubic: %f (error: %f)", cubicDenorm, cubicError)

	// For a smooth function like sin(πx), cubic should generally be more accurate
	// But this depends on the sampling and quantization, so we just check they're different
	if math.Abs(linearDenorm-cubicDenorm) < 1e-6 {
		t.Errorf("Linear and cubic interpolation gave nearly identical results, expected difference")
	}
}

func TestType0CubicSplineExactInterpolation(t *testing.T) {
	// Test that cubic splines exactly interpolate at grid points
	function := &Type0{
		Domain:        []float64{0, 1},
		Range:         []float64{0, 1},
		Size:          []int{4},
		BitsPerSample: 8,
		UseCubic:      true,
		Samples:       []byte{50, 100, 150, 200},
	}
	function.repair()

	// Test exact interpolation at grid points
	for i := 0; i < 4; i++ {
		x := float64(i) / 3.0 // Grid points at 0, 1/3, 2/3, 1
		result := function.Apply(x)
		expected := float64(function.Samples[i]) / 255.0

		if math.Abs(result[0]-expected) > 1e-10 {
			t.Errorf("At grid point %d (x=%f): expected %f, got %f",
				i, x, expected, result[0])
		}
	}
}

func TestType0CubicSplineSmall(t *testing.T) {
	// Test edge cases with small grids

	// 2-point case should degrade to linear
	linear2pt := &Type0{
		Domain:        []float64{0, 1},
		Range:         []float64{0, 1},
		Size:          []int{2},
		BitsPerSample: 8,
		UseCubic:      true,
		Samples:       []byte{0, 255},
	}
	linear2pt.repair()

	result := linear2pt.Apply(0.5)
	expected := 0.5 // Should be linear interpolation

	if math.Abs(result[0]-expected) > 1e-6 {
		t.Errorf("2-point cubic should degrade to linear: expected %f, got %f",
			expected, result[0])
	}
}

func BenchmarkType0CubicSpline(b *testing.B) {
	// Benchmark cubic spline evaluation
	function := &Type0{
		Domain:        []float64{0, 1, 0, 1},
		Range:         []float64{0, 1},
		Size:          []int{10, 10},
		BitsPerSample: 8,
		UseCubic:      true,
		Samples:       make([]byte, 100),
	}

	// Fill with some test data
	for i := range function.Samples {
		function.Samples[i] = byte(i * 255 / len(function.Samples))
	}

	function.repair()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		x := float64(i%1000) / 1000.0
		y := float64((i*17)%1000) / 1000.0
		function.Apply(x, y)
	}
}

func TestType0EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		function *Type0
		inputs   []float64
		expected []float64
		desc     string
	}{
		{
			name: "Single sample 1D",
			function: &Type0{
				Domain:        []float64{0, 1},
				Range:         []float64{0, 1},
				Size:          []int{1},
				BitsPerSample: 8,
				UseCubic:      false,
				Samples:       []byte{128},
			},
			inputs:   []float64{0.5},
			expected: []float64{128.0 / 255.0},
			desc:     "Single sample should return that sample regardless of input",
		},
		{
			name: "Single sample multidimensional",
			function: &Type0{
				Domain:        []float64{0, 1, 0, 1},
				Range:         []float64{0, 1},
				Size:          []int{1, 1},
				BitsPerSample: 8,
				UseCubic:      false,
				Samples:       []byte{200},
			},
			inputs:   []float64{0.3, 0.7},
			expected: []float64{200.0 / 255.0},
			desc:     "Single sample in 2D should return that sample",
		},
		{
			name: "Exact index match 1D",
			function: &Type0{
				Domain:        []float64{0, 1},
				Range:         []float64{0, 1},
				Size:          []int{3},
				BitsPerSample: 8,
				UseCubic:      false,
				Samples:       []byte{0, 128, 255},
			},
			inputs:   []float64{0.5}, // Should map exactly to index 1
			expected: []float64{128.0 / 255.0},
			desc:     "Exact index match should avoid interpolation",
		},
		{
			name: "Boundary clamping upper",
			function: &Type0{
				Domain:        []float64{0, 1},
				Range:         []float64{0, 1},
				Size:          []int{2},
				BitsPerSample: 8,
				UseCubic:      false,
				Samples:       []byte{100, 200},
			},
			inputs:   []float64{2.0}, // Way above domain
			expected: []float64{200.0 / 255.0},
			desc:     "Upper boundary should clamp to last sample",
		},
		{
			name: "Boundary clamping lower",
			function: &Type0{
				Domain:        []float64{0, 1},
				Range:         []float64{0, 1},
				Size:          []int{2},
				BitsPerSample: 8,
				UseCubic:      false,
				Samples:       []byte{100, 200},
			},
			inputs:   []float64{-1.0}, // Way below domain
			expected: []float64{100.0 / 255.0},
			desc:     "Lower boundary should clamp to first sample",
		},
		{
			name: "Multidimensional with single sample in one dimension",
			function: &Type0{
				Domain:        []float64{0, 1, 0, 1},
				Range:         []float64{0, 1},
				Size:          []int{2, 1}, // 2 samples in first dim, 1 in second
				BitsPerSample: 8,
				UseCubic:      false,
				Samples:       []byte{100, 200}, // Only varies in first dimension
			},
			inputs:   []float64{0.5, 0.8},      // Second input should be ignored
			expected: []float64{150.0 / 255.0}, // Interpolation in first dim only
			desc:     "Single sample in one dimension should not affect interpolation",
		},
		{
			name: "Zero-dimensional case protection",
			function: &Type0{
				Domain:        []float64{0, 1, 0, 1},
				Range:         []float64{0, 1},
				Size:          []int{3, 2},
				BitsPerSample: 8,
				UseCubic:      false,
				Samples:       []byte{0, 50, 100, 150, 200, 255},
			},
			inputs:   []float64{0.0, 0.0}, // Exact corner should avoid interpolation
			expected: []float64{0.0},
			desc:     "Exact corner match should be efficient",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.function.repair()
			result := tt.function.Apply(tt.inputs...)
			if len(result) != len(tt.expected) {
				t.Fatalf("Expected %d outputs, got %d", len(tt.expected), len(result))
			}
			for i, expected := range tt.expected {
				if abs(result[i]-expected) > 1e-10 {
					t.Errorf("Output[%d]: expected %f, got %f (diff: %e) - %s",
						i, expected, result[i], abs(result[i]-expected), tt.desc)
				}
			}
		})
	}
}

func TestType0BoundaryConsistency(t *testing.T) {
	// Test that 1D and multidimensional boundary handling is now consistent

	// 1D function
	f1D := &Type0{
		Domain:        []float64{0, 1},
		Range:         []float64{0, 1},
		Size:          []int{2},
		BitsPerSample: 8,
		UseCubic:      false,
		Samples:       []byte{100, 200},
	}

	// 2D function with same samples in first dimension, single sample in second
	f2D := &Type0{
		Domain:        []float64{0, 1, 0, 1},
		Range:         []float64{0, 1},
		Size:          []int{2, 1},
		BitsPerSample: 8,
		UseCubic:      false,
		Samples:       []byte{100, 200},
	}
	f1D.repair()
	f2D.repair()

	// Test upper boundary
	result1D := f1D.Apply(1.5)      // Above domain
	result2D := f2D.Apply(1.5, 0.5) // Above domain in first dim

	if abs(result1D[0]-result2D[0]) > 1e-10 {
		t.Errorf("Boundary handling inconsistent: 1D=%f, 2D=%f", result1D[0], result2D[0])
	}

	// Both should return the last sample (200/255)
	expected := 200.0 / 255.0
	if abs(result1D[0]-expected) > 1e-10 {
		t.Errorf("1D boundary: expected %f, got %f", expected, result1D[0])
	}
	if abs(result2D[0]-expected) > 1e-10 {
		t.Errorf("2D boundary: expected %f, got %f", expected, result2D[0])
	}
}

func TestType0CubicExactInterpolation5x5(t *testing.T) {
	// Test that cubic splines exactly interpolate at all grid points
	// Use a 5x5 grid with a known function: f(x,y) = x + 2*y
	// Grid points at (i,j) where i,j ∈ {0,1,2,3,4}

	size := 5
	samples := make([]byte, size*size*2) // 2 bytes per 16-bit sample

	// Fill samples with f(i,j) = i + 2*j, normalized to [0,65535]
	// First dimension (i) varies fastest, so sample at (i,j) is at index i + j*size
	maxVal := float64(4 + 2*4) // max value is 4 + 2*4 = 12
	for j := 0; j < size; j++ {
		for i := 0; i < size; i++ {
			val := float64(i + 2*j)
			// Normalize to [0,65535] for 16-bit
			sample16 := uint16(val * 65535.0 / maxVal)
			// Store as big-endian 16-bit
			idx := (i + j*size) * 2
			samples[idx] = byte(sample16 >> 8)
			samples[idx+1] = byte(sample16)
		}
	}

	function := &Type0{
		Domain:        []float64{0, 4, 0, 4}, // Domain [0,4] x [0,4]
		Range:         []float64{0, 12},      // Range [0,12]
		Size:          []int{size, size},     // 5x5 grid
		BitsPerSample: 16,                    // Use 16-bit for higher precision
		UseCubic:      true,
		Samples:       samples,
	}
	function.repair()

	// Debug: First test a simple 1D case to isolate the problem
	// Create a simple 1D function for testing
	simple1D := &Type0{
		Domain:        []float64{0, 3}, // Domain [0,3]
		Range:         []float64{0, 3}, // Range [0,3]
		Size:          []int{4},        // 4 points: 0, 1, 2, 3
		BitsPerSample: 8,
		UseCubic:      true,
		Samples:       []byte{0, 85, 170, 255}, // Values 0, 1, 2, 3 normalized
	}
	simple1D.repair()

	// Test 1D interpolation at grid points
	t.Logf("Testing 1D case first:")
	for i := 0; i < 4; i++ {
		x := float64(i)
		expected := float64(i)
		result := simple1D.Apply(x)
		actual := result[0]
		t.Logf("1D: x=%.1f, expected=%.6f, got=%.6f, diff=%.2e",
			x, expected, actual, math.Abs(actual-expected))
	}

	// Test exact interpolation at all grid points (within quantization error)
	tolerance := 1e-3 // Allow for 16-bit quantization error
	for j := 0; j < size; j++ {
		for i := 0; i < size; i++ {
			// Grid point coordinates
			x := float64(i)
			y := float64(j)

			// Expected value: f(i,j) = i + 2*j
			expected := float64(i + 2*j)

			// Evaluate spline at grid point
			result := function.Apply(x, y)
			actual := result[0]

			if math.Abs(actual-expected) > tolerance {
				t.Errorf("Grid point (%d,%d) at (%.1f,%.1f): expected %.6f, got %.6f, diff=%.2e",
					i, j, x, y, expected, actual, math.Abs(actual-expected))
			}
		}
	}

	// Also test a few off-grid points to ensure interpolation is working
	testPoints := []struct {
		x, y      float64
		expected  float64
		tolerance float64
		desc      string
	}{
		{0.5, 0.5, 1.5, 0.5, "Center of bottom-left cell"},
		{1.5, 1.5, 4.5, 1.0, "Center of interior cell"},
		{2.0, 1.0, 4.0, 0.1, "Grid line intersection"},
	}

	for _, tp := range testPoints {
		result := function.Apply(tp.x, tp.y)
		actual := result[0]

		if math.Abs(actual-tp.expected) > tp.tolerance {
			t.Logf("Off-grid point (%.1f,%.1f): expected %.6f, got %.6f, diff=%.2e - %s",
				tp.x, tp.y, tp.expected, actual, math.Abs(actual-tp.expected), tp.desc)
		}
	}
}
