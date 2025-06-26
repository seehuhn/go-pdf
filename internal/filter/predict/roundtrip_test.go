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

package predict

import (
	"bytes"
	"io"
	"math/rand"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestRoundTrip(t *testing.T) {
	tests := []struct {
		name   string
		params Params
		data   []byte
	}{
		{
			name: "no predictor - single byte",
			params: Params{
				Colors:           1,
				BitsPerComponent: 8,
				Columns:          1,
				Predictor:        1,
			},
			data: []byte{42},
		},
		{
			name: "no predictor - multiple bytes",
			params: Params{
				Colors:           3,
				BitsPerComponent: 8,
				Columns:          2,
				Predictor:        1,
			},
			data: []byte{10, 20, 30, 40, 50, 60},
		},
		{
			name: "TIFF predictor - 8 bit RGB",
			params: Params{
				Colors:           3,
				BitsPerComponent: 8,
				Columns:          2,
				Predictor:        2,
			},
			data: []byte{100, 150, 200, 110, 160, 210},
		},
		{
			name: "TIFF predictor - 8 bit grayscale",
			params: Params{
				Colors:           1,
				BitsPerComponent: 8,
				Columns:          4,
				Predictor:        2,
			},
			data: []byte{50, 60, 70, 80},
		},
		{
			name: "TIFF predictor - 16 bit RGB",
			params: Params{
				Colors:           3,
				BitsPerComponent: 16,
				Columns:          1,
				Predictor:        2,
			},
			data: []byte{0x12, 0x34, 0x56, 0x78, 0x9A, 0xBC},
		},
		{
			name: "TIFF predictor - 1 bit grayscale",
			params: Params{
				Colors:           1,
				BitsPerComponent: 1,
				Columns:          8,
				Predictor:        2,
			},
			data: []byte{0b10110001},
		},
		{
			name: "TIFF predictor - 2 bit grayscale",
			params: Params{
				Colors:           1,
				BitsPerComponent: 2,
				Columns:          4,
				Predictor:        2,
			},
			data: []byte{0b11100100},
		},
		{
			name: "TIFF predictor - 4 bit grayscale",
			params: Params{
				Colors:           1,
				BitsPerComponent: 4,
				Columns:          2,
				Predictor:        2,
			},
			data: []byte{0b11110001},
		},
		{
			name: "PNG None predictor",
			params: Params{
				Colors:           3,
				BitsPerComponent: 8,
				Columns:          2,
				Predictor:        10,
			},
			data: []byte{100, 150, 200, 110, 160, 210},
		},
		{
			name: "PNG Sub predictor",
			params: Params{
				Colors:           3,
				BitsPerComponent: 8,
				Columns:          2,
				Predictor:        11,
			},
			data: []byte{100, 150, 200, 110, 160, 210},
		},
		{
			name: "PNG Up predictor - multi-row",
			params: Params{
				Colors:           1,
				BitsPerComponent: 8,
				Columns:          3,
				Predictor:        12,
			},
			data: []byte{10, 20, 30, 15, 25, 35},
		},
		{
			name: "PNG Average predictor",
			params: Params{
				Colors:           3,
				BitsPerComponent: 8,
				Columns:          2,
				Predictor:        13,
			},
			data: []byte{100, 150, 200, 110, 160, 210},
		},
		{
			name: "PNG Paeth predictor",
			params: Params{
				Colors:           3,
				BitsPerComponent: 8,
				Columns:          2,
				Predictor:        14,
			},
			data: []byte{100, 150, 200, 110, 160, 210},
		},
		{
			name: "PNG Optimum predictor",
			params: Params{
				Colors:           3,
				BitsPerComponent: 8,
				Columns:          2,
				Predictor:        15,
			},
			data: []byte{100, 150, 200, 110, 160, 210},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Encode: original data -> predictor -> compressed data
			encodedBuf := &writeCloser{Buffer: &bytes.Buffer{}}
			writer, err := NewWriter(encodedBuf, &tt.params)
			if err != nil {
				t.Fatalf("failed to create writer: %v", err)
			}

			n, err := writer.Write(tt.data)
			if err != nil {
				t.Fatalf("failed to write data: %v", err)
			}
			if n != len(tt.data) {
				t.Fatalf("wrote %d bytes, expected %d", n, len(tt.data))
			}

			// Close writer
			if err := writer.Close(); err != nil {
				t.Fatalf("failed to close writer: %v", err)
			}

			encodedData := encodedBuf.Bytes()

			// Decode: compressed data -> predictor -> original data
			reader, err := NewReader(io.NopCloser(bytes.NewReader(encodedData)), &tt.params)
			if err != nil {
				t.Fatalf("failed to create reader: %v", err)
			}

			decodedData := make([]byte, len(tt.data))
			n, err = reader.Read(decodedData)
			if err != nil {
				t.Fatalf("failed to read data: %v", err)
			}
			if n != len(tt.data) {
				t.Fatalf("read %d bytes, expected %d", n, len(tt.data))
			}

			// Verify round-trip consistency
			if diff := cmp.Diff(tt.data, decodedData); diff != "" {
				t.Errorf("round trip failed (-original +decoded):\n%s", diff)
			}
		})
	}
}

func TestRoundTripLargeImage(t *testing.T) {
	params := Params{
		Colors:           3,
		BitsPerComponent: 8,
		Columns:          100,
		Predictor:        14, // PNG Paeth
	}

	// Create 100x100 RGB image (30,000 bytes)
	imageSize := params.Colors * params.Columns * 100
	originalData := make([]byte, imageSize)

	// Fill with somewhat realistic image data (gradients)
	for row := 0; row < 100; row++ {
		for col := 0; col < params.Columns; col++ {
			offset := (row*params.Columns + col) * params.Colors
			originalData[offset] = byte(row + col) // R
			originalData[offset+1] = byte(row * 2) // G
			originalData[offset+2] = byte(col * 2) // B
		}
	}

	// Encode
	encodedBuf := &writeCloser{Buffer: &bytes.Buffer{}}
	writer, err := NewWriter(encodedBuf, &params)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}

	n, err := writer.Write(originalData)
	if err != nil {
		t.Fatalf("failed to write data: %v", err)
	}
	if n != len(originalData) {
		t.Fatalf("wrote %d bytes, expected %d", n, len(originalData))
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("failed to close writer: %v", err)
	}

	// Decode
	reader, err := NewReader(io.NopCloser(bytes.NewReader(encodedBuf.Bytes())), &params)
	if err != nil {
		t.Fatalf("failed to create reader: %v", err)
	}

	decodedData := make([]byte, len(originalData))
	n, err = reader.Read(decodedData)
	if err != nil {
		t.Fatalf("failed to read data: %v", err)
	}
	if n != len(originalData) {
		t.Fatalf("read %d bytes, expected %d", n, len(originalData))
	}

	// Verify
	if diff := cmp.Diff(originalData, decodedData); diff != "" {
		t.Errorf("large image round trip failed (-original +decoded):\n%s", diff)
	}
}

func TestDataSizeCalculations(t *testing.T) {
	// Test that our test data sizes match the predict.md formulas
	tests := []struct {
		name             string
		colors           int
		bitsPerComponent int
		columns          int
		dataSize         int
		expectedRows     int
	}{
		{
			name:             "8-bit RGB",
			colors:           3,
			bitsPerComponent: 8,
			columns:          10,
			dataSize:         30, // 10 pixels × 3 colors × 1 byte = 30 bytes
			expectedRows:     1,
		},
		{
			name:             "4-bit grayscale with padding",
			colors:           1,
			bitsPerComponent: 4,
			columns:          3,
			dataSize:         4, // 12 bits = 2 bytes per row, 2 rows = 4 bytes
			expectedRows:     2,
		},
		{
			name:             "1-bit grayscale cross-byte",
			colors:           1,
			bitsPerComponent: 1,
			columns:          9,
			dataSize:         6, // 9 bits = 2 bytes per row, 3 rows = 6 bytes
			expectedRows:     3,
		},
		{
			name:             "2-bit RGB cross-byte",
			colors:           3,
			bitsPerComponent: 2,
			columns:          5,
			dataSize:         8, // 30 bits = 4 bytes per row, 2 rows = 8 bytes
			expectedRows:     2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use predict.md formulas
			bitsPerPixel := tt.colors * tt.bitsPerComponent
			bitsPerRow := bitsPerPixel * tt.columns
			bytesPerRow := (bitsPerRow + 7) / 8 // Round up to bytes

			// Verify our test data matches the expected dimensions
			calculatedRows := tt.dataSize / bytesPerRow
			if calculatedRows != tt.expectedRows {
				t.Errorf("data size mismatch: %d bytes ÷ %d bytes/row = %d rows, expected %d rows",
					tt.dataSize, bytesPerRow, calculatedRows, tt.expectedRows)
			}

			// Verify the calculation is exact (no partial rows)
			if tt.dataSize%bytesPerRow != 0 {
				t.Errorf("data size %d is not a multiple of row size %d bytes", tt.dataSize, bytesPerRow)
			}

			// Log the calculations for verification
			t.Logf("Calculations: %d colors × %d bits × %d columns = %d bits/row = %d bytes/row",
				tt.colors, tt.bitsPerComponent, tt.columns, bitsPerRow, bytesPerRow)
			t.Logf("Total: %d bytes = %d rows × %d bytes/row", tt.dataSize, tt.expectedRows, bytesPerRow)
		})
	}
}

func TestRoundTripRandomData(t *testing.T) {
	testCases := []Params{
		{Colors: 1, BitsPerComponent: 8, Columns: 10, Predictor: 2},
		{Colors: 3, BitsPerComponent: 8, Columns: 10, Predictor: 11},
		{Colors: 4, BitsPerComponent: 8, Columns: 10, Predictor: 12},
		{Colors: 1, BitsPerComponent: 16, Columns: 5, Predictor: 2},
	}

	for _, params := range testCases {
		t.Run(params.String(), func(t *testing.T) {
			// Generate random data
			dataSize := (params.Colors*params.BitsPerComponent*params.Columns*10 + 7) / 8
			originalData := make([]byte, dataSize)
			rand.Read(originalData)

			// Encode
			encodedBuf := &writeCloser{Buffer: &bytes.Buffer{}}
			writer, err := NewWriter(encodedBuf, &params)
			if err != nil {
				t.Fatalf("failed to create writer: %v", err)
			}

			n, err := writer.Write(originalData)
			if err != nil {
				t.Fatalf("failed to write data: %v", err)
			}
			if n != len(originalData) {
				t.Fatalf("wrote %d bytes, expected %d", n, len(originalData))
			}

			if err := writer.Close(); err != nil {
				t.Fatalf("failed to close writer: %v", err)
			}

			// Decode
			reader, err := NewReader(io.NopCloser(bytes.NewReader(encodedBuf.Bytes())), &params)
			if err != nil {
				t.Fatalf("failed to create reader: %v", err)
			}

			decodedData := make([]byte, len(originalData))
			n, err = reader.Read(decodedData)
			if err != nil {
				t.Fatalf("failed to read data: %v", err)
			}
			if n != len(originalData) {
				t.Fatalf("read %d bytes, expected %d", n, len(originalData))
			}

			// Verify
			if diff := cmp.Diff(originalData, decodedData); diff != "" {
				t.Errorf("random data round trip failed (-original +decoded):\n%s", diff)
			}
		})
	}
}

func TestRoundTripEdgeCases(t *testing.T) {
	tests := []struct {
		name   string
		params Params
		data   []byte
	}{
		{
			name: "1x1 image",
			params: Params{
				Colors:           3,
				BitsPerComponent: 8,
				Columns:          1,
				Predictor:        11,
			},
			data: []byte{255, 128, 64},
		},
		{
			name: "single column image",
			params: Params{
				Colors:           1,
				BitsPerComponent: 8,
				Columns:          1,
				Predictor:        12,
			},
			data: []byte{10, 20, 30, 40},
		},
		{
			name: "all zeros",
			params: Params{
				Colors:           3,
				BitsPerComponent: 8,
				Columns:          5,
				Predictor:        13,
			},
			data: make([]byte, 15), // All zeros
		},
		{
			name: "all 255s",
			params: Params{
				Colors:           1,
				BitsPerComponent: 8,
				Columns:          8,
				Predictor:        2,
			},
			data: bytes.Repeat([]byte{255}, 8),
		},
		{
			name: "cross-byte pixel boundaries - 1 bit",
			params: Params{
				Colors:           1,
				BitsPerComponent: 1,
				Columns:          9, // 9 bits per row
				Predictor:        2,
			},
			// Calculate: 9 bits = (9+7)/8 = 2 bytes per row, 1 row
			data: []byte{0b10110001, 0b10000000}, // 2 bytes (9 bits used, 7 bits padding)
		},
		{
			name: "cross-byte pixel boundaries - 2 bit",
			params: Params{
				Colors:           1,
				BitsPerComponent: 2,
				Columns:          5, // 10 bits per row
				Predictor:        2,
			},
			// Calculate: 10 bits = (10+7)/8 = 2 bytes per row, 1 row
			data: []byte{0b11100100, 0b11000000}, // 2 bytes (10 bits used, 6 bits padding)
		},
		{
			name: "cross-byte pixel boundaries - 4 bit",
			params: Params{
				Colors:           1,
				BitsPerComponent: 4,
				Columns:          3, // 12 bits per row
				Predictor:        2,
			},
			// Calculate: 12 bits = (12+7)/8 = 2 bytes per row, 1 row
			data: []byte{0b11110001, 0b10100000}, // 2 bytes (12 bits used, 4 bits padding)
		},
		{
			name: "multi-component cross-byte - RGB 2 bit",
			params: Params{
				Colors:           3,
				BitsPerComponent: 2,
				Columns:          3, // 3*2*3 = 18 bits per row
				Predictor:        2,
			},
			// Calculate: 18 bits = (18+7)/8 = 3 bytes per row, 1 row
			data: []byte{0b11100100, 0b10011001, 0b11000000}, // 3 bytes (18 bits used, 6 bits padding)
		},
		{
			name: "PNG with cross-byte boundaries",
			params: Params{
				Colors:           1,
				BitsPerComponent: 4,
				Columns:          3,  // 12 bits per row
				Predictor:        11, // PNG Sub
			},
			// Calculate: 12 bits = (12+7)/8 = 2 bytes per row, 2 rows
			data: []byte{0b11110001, 0b10100000, 0b00010010, 0b11000000}, // 4 bytes total (2 rows × 2 bytes)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Encode
			encodedBuf := &writeCloser{Buffer: &bytes.Buffer{}}
			writer, err := NewWriter(encodedBuf, &tt.params)
			if err != nil {
				t.Fatalf("failed to create writer: %v", err)
			}

			n, err := writer.Write(tt.data)
			if err != nil {
				t.Fatalf("failed to write data: %v", err)
			}
			if n != len(tt.data) {
				t.Fatalf("wrote %d bytes, expected %d", n, len(tt.data))
			}

			if err := writer.Close(); err != nil {
				t.Fatalf("failed to close writer: %v", err)
			}

			// Decode
			reader, err := NewReader(io.NopCloser(bytes.NewReader(encodedBuf.Bytes())), &tt.params)
			if err != nil {
				t.Fatalf("failed to create reader: %v", err)
			}

			decodedData := make([]byte, len(tt.data))
			n, err = reader.Read(decodedData)
			if err != nil {
				t.Fatalf("failed to read data: %v", err)
			}
			if n != len(tt.data) {
				t.Fatalf("read %d bytes, expected %d", n, len(tt.data))
			}

			// Verify
			if diff := cmp.Diff(tt.data, decodedData); diff != "" {
				t.Errorf("edge case round trip failed (-original +decoded):\n%s", diff)
			}
		})
	}
}
