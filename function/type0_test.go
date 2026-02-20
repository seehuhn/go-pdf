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
			samples:       []byte{0xAB, 0xCD, 0xEF, 0x12, 0x00}, // ABC DEF 120 (need 5 bytes for 36 bits)
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
			result := make([]float64, len(tt.expected))
			tt.function.Apply(result, tt.inputs...)
			for i, expected := range tt.expected {
				if math.Abs(result[i]-expected) > tt.tolerance {
					t.Errorf("output[%d]: expected %f, got %f (diff: %e)",
						i, expected, result[i], math.Abs(result[i]-expected))
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

func TestType0CatmullRomSpline(t *testing.T) {
	// This test verifies the Catmull-Rom spline interpolation as implemented
	// in Ghostscript. The expected values are calculated manually based on the
	// formula in gsfunc0.c.
	function := &Type0{
		Domain:        []float64{0, 3},
		Range:         []float64{0, 100},
		Size:          []int{4},
		BitsPerSample: 8,
		UseCubic:      true,
		Samples:       []byte{0, 10, 40, 100}, // p0=0, p1=10, p2=40, p3=100
		Decode:        []float64{0, 255},      // Map sample values directly
	}
	function.repair()

	tests := []struct {
		input    float64
		expected float64
	}{
		{0.0, 0.0},    // Exact at node 0
		{0.5, 3.125},  // Interpolated (quadratic) between 0 and 1
		{1.0, 10.0},   // Exact at node 1
		{1.5, 21.875}, // Interpolated (cubic) between 1 and 2
		{2.0, 40.0},   // Exact at node 2
		{2.5, 71.875}, // Interpolated (quadratic) between 2 and 3
		{3.0, 100.0},  // Exact at node 3
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("input_%.2f", tt.input), func(t *testing.T) {
			result := make([]float64, 1)
			function.Apply(result, tt.input)
			actual := result[0]
			if math.Abs(actual-tt.expected) > 1e-6 {
				t.Errorf("expected %.6f, got %.6f", tt.expected, actual)
			}
		})
	}
}

// TestType0Empty tests that a Type0 function with no inputs and no outputs
// is handled correctly.
func TestType0Empty(t *testing.T) {
	f := &Type0{
		Domain:        []float64{},
		Range:         []float64{},
		Size:          []int{},
		BitsPerSample: 8,
		Samples:       []byte{},
	}

	m, n := f.Shape()
	if m != 0 || n != 0 {
		t.Errorf("expected shape (0, 0), got (%d, %d)", m, n)
	}
	result := make([]float64, 0)
	f.Apply(result)
	if len(result) != 0 {
		t.Errorf("expected no output, got %d", len(result))
	}
}

// TestType0Constant tests that functions with no inputs and one output
// are handled correctly.
func TestType0Constant(t *testing.T) {
	f := &Type0{
		Domain:        []float64{},
		Range:         []float64{0, 1},
		Size:          []int{},
		BitsPerSample: 8,
		Encode:        []float64{},
		Decode:        []float64{0, 1},
		Samples:       []byte{},
	}

	m, n := f.Shape()
	if m != 0 || n != 1 {
		t.Errorf("expected shape (0, 1), got (%d, %d)", m, n)
	}
	result := make([]float64, 1)
	f.Apply(result)
	if len(result) != 1 {
		t.Errorf("expected 1 output, got %d", len(result))
	}
	if result[0] != 0 {
		t.Errorf("expected output 0, got %f", result[0])
	}
}
