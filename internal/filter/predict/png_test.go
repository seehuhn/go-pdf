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
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestPNGPredictorTagBytes(t *testing.T) {
	tests := []struct {
		name           string
		predictor      int
		expectedTag    byte
		originalData   []byte
		expectedOutput []byte
	}{
		{
			name:         "PNG None predictor",
			predictor:    10,
			expectedTag:  0,
			originalData: []byte{100, 150, 200},
			// Tag byte 0 + original data unchanged
			expectedOutput: []byte{0, 100, 150, 200},
		},
		{
			name:         "PNG Sub predictor",
			predictor:    11,
			expectedTag:  1,
			originalData: []byte{100, 150, 200, 110, 160, 210},
			// Tag byte 1 + Sub algorithm applied
			expectedOutput: []byte{1, 100, 150, 200, 10, 10, 10},
		},
		{
			name:         "PNG Up predictor",
			predictor:    12,
			expectedTag:  2,
			originalData: []byte{100, 150, 200},
			// Tag byte 2 + Up algorithm (first row, so unchanged)
			expectedOutput: []byte{2, 100, 150, 200},
		},
		{
			name:         "PNG Average predictor",
			predictor:    13,
			expectedTag:  3,
			originalData: []byte{100, 150, 200},
			// Tag byte 3 + Average algorithm
			expectedOutput: []byte{3, 100, 150, 200},
		},
		{
			name:         "PNG Paeth predictor",
			predictor:    14,
			expectedTag:  4,
			originalData: []byte{100, 150, 200},
			// Tag byte 4 + Paeth algorithm
			expectedOutput: []byte{4, 100, 150, 200},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := Params{
				Colors:           3,
				BitsPerComponent: 8,
				Columns:          len(tt.originalData) / 3,
				Predictor:        tt.predictor,
			}

			encodedBuf := &writeCloser{Buffer: &bytes.Buffer{}}
			writer, err := NewWriter(encodedBuf, &params)
			if err != nil {
				t.Fatalf("failed to create writer: %v", err)
			}

			n, err := writer.Write(tt.originalData)
			if err != nil {
				t.Fatalf("failed to write data: %v", err)
			}
			if n != len(tt.originalData) {
				t.Fatalf("wrote %d bytes, expected %d", n, len(tt.originalData))
			}

			if closer, ok := writer.(interface{ Close() error }); ok {
				if err := closer.Close(); err != nil {
					t.Fatalf("failed to close writer: %v", err)
				}
			}

			encodedData := encodedBuf.Bytes()

			// Check that tag byte is correct
			if len(encodedData) == 0 {
				t.Fatalf("no encoded data produced")
			}
			if encodedData[0] != tt.expectedTag {
				t.Errorf("wrong tag byte: got %d, expected %d", encodedData[0], tt.expectedTag)
			}

			// For simple cases, check full output
			if tt.expectedOutput != nil {
				if diff := cmp.Diff(tt.expectedOutput, encodedData); diff != "" {
					t.Errorf("PNG encoding mismatch (-expected +got):\n%s", diff)
				}
			}
		})
	}
}

func TestPNGSubPredictor(t *testing.T) {
	params := Params{
		Colors:           3,
		BitsPerComponent: 8,
		Columns:          3,
		Predictor:        11,
	}

	// 3x1 RGB image: (100,150,200) (110,160,210) (105,155,205)
	originalData := []byte{100, 150, 200, 110, 160, 210, 105, 155, 205}

	// PNG Sub predictor: output[i] = input[i] - input[i - bytes_per_pixel]
	// bytes_per_pixel = 3 (RGB)
	// Tag byte: 1
	// First pixel: (100,150,200) unchanged
	// Second pixel: (110-100, 160-150, 210-200) = (10,10,10)
	// Third pixel: (105-110, 155-160, 205-210) = (-5,-5,-5) = (251,251,251)
	expectedEncoded := []byte{1, 100, 150, 200, 10, 10, 10, 251, 251, 251}

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

	if closer, ok := writer.(interface{ Close() error }); ok {
		if err := closer.Close(); err != nil {
			t.Fatalf("failed to close writer: %v", err)
		}
	}

	encodedData := encodedBuf.Bytes()
	if diff := cmp.Diff(expectedEncoded, encodedData); diff != "" {
		t.Errorf("PNG Sub encoding mismatch (-expected +got):\n%s", diff)
	}

	// Test decoding
	reader, err := NewReader(bytes.NewReader(encodedData), &params)
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

	if diff := cmp.Diff(originalData, decodedData); diff != "" {
		t.Errorf("PNG Sub decoding mismatch (-original +decoded):\n%s", diff)
	}
}

func TestPNGUpPredictor(t *testing.T) {
	params := Params{
		Colors:           1,
		BitsPerComponent: 8,
		Columns:          3,
		Predictor:        12,
	}

	// 3x2 grayscale image (2 rows, 3 columns each)
	originalData := []byte{
		// Row 1: 10, 20, 30
		10, 20, 30,
		// Row 2: 15, 25, 35
		15, 25, 35,
	}

	// PNG Up predictor: output[i] = input[i] - previous_row[i]
	// Expected encoding:
	// Row 1: tag=2, (10,20,30) - (0,0,0) = (10,20,30)
	// Row 2: tag=2, (15,25,35) - (10,20,30) = (5,5,5)
	expectedEncoded := []byte{
		2, 10, 20, 30, // Row 1
		2, 5, 5, 5, // Row 2
	}

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

	if closer, ok := writer.(interface{ Close() error }); ok {
		if err := closer.Close(); err != nil {
			t.Fatalf("failed to close writer: %v", err)
		}
	}

	encodedData := encodedBuf.Bytes()
	if diff := cmp.Diff(expectedEncoded, encodedData); diff != "" {
		t.Errorf("PNG Up encoding mismatch (-expected +got):\n%s", diff)
	}

	// Test decoding
	reader, err := NewReader(bytes.NewReader(encodedData), &params)
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

	if diff := cmp.Diff(originalData, decodedData); diff != "" {
		t.Errorf("PNG Up decoding mismatch (-original +decoded):\n%s", diff)
	}
}

func TestPNGAveragePredictor(t *testing.T) {
	params := Params{
		Colors:           1,
		BitsPerComponent: 8,
		Columns:          2,
		Predictor:        13,
	}

	// 2x2 grayscale image
	originalData := []byte{
		// Row 1: 10, 20
		10, 20,
		// Row 2: 30, 40
		30, 40,
	}

	// PNG Average predictor: output[i] = input[i] - (left + up) / 2
	// Row 1:
	//   - First pixel: 10 - (0+0)/2 = 10
	//   - Second pixel: 20 - (10+0)/2 = 20 - 5 = 15
	// Row 2:
	//   - First pixel: 30 - (0+10)/2 = 30 - 5 = 25
	//   - Second pixel: 40 - (30+20)/2 = 40 - 25 = 15
	expectedEncoded := []byte{
		3, 10, 15, // Row 1
		3, 25, 15, // Row 2
	}

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

	if closer, ok := writer.(interface{ Close() error }); ok {
		if err := closer.Close(); err != nil {
			t.Fatalf("failed to close writer: %v", err)
		}
	}

	encodedData := encodedBuf.Bytes()
	if diff := cmp.Diff(expectedEncoded, encodedData); diff != "" {
		t.Errorf("PNG Average encoding mismatch (-expected +got):\n%s", diff)
	}

	// Test round-trip
	reader, err := NewReader(bytes.NewReader(encodedData), &params)
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

	if diff := cmp.Diff(originalData, decodedData); diff != "" {
		t.Errorf("PNG Average decoding mismatch (-original +decoded):\n%s", diff)
	}
}

func TestPNGPaethPredictor(t *testing.T) {
	params := Params{
		Colors:           1,
		BitsPerComponent: 8,
		Columns:          2,
		Predictor:        14,
	}

	// 2x2 grayscale image
	originalData := []byte{
		// Row 1: 10, 30
		10, 30,
		// Row 2: 20, 40
		20, 40,
	}

	// PNG Paeth predictor uses complex algorithm
	// For this test, we'll verify round-trip rather than exact encoding
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

	if closer, ok := writer.(interface{ Close() error }); ok {
		if err := closer.Close(); err != nil {
			t.Fatalf("failed to close writer: %v", err)
		}
	}

	encodedData := encodedBuf.Bytes()

	// Verify tag bytes are correct (should be 4 for Paeth)
	expectedTags := []byte{4, 4} // Two rows
	tagPositions := []int{0, 3}  // Positions of tag bytes

	for i, pos := range tagPositions {
		if pos >= len(encodedData) {
			t.Fatalf("encoded data too short, missing tag byte at position %d", pos)
		}
		if encodedData[pos] != expectedTags[i] {
			t.Errorf("wrong tag byte at position %d: got %d, expected %d", pos, encodedData[pos], expectedTags[i])
		}
	}

	// Test round-trip
	reader, err := NewReader(bytes.NewReader(encodedData), &params)
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

	if diff := cmp.Diff(originalData, decodedData); diff != "" {
		t.Errorf("PNG Paeth decoding mismatch (-original +decoded):\n%s", diff)
	}
}

func TestPNGPredictorPreviousRowBufferManagement(t *testing.T) {
	params := Params{
		Colors:           3,
		BitsPerComponent: 8,
		Columns:          2,
		Predictor:        12, // Up predictor
	}

	// 2x3 RGB image (3 rows, 2 columns, 3 colors each)
	originalData := []byte{
		// Row 1: (100,150,200) (110,160,210)
		100, 150, 200, 110, 160, 210,
		// Row 2: (105,155,205) (115,165,215)
		105, 155, 205, 115, 165, 215,
		// Row 3: (108,158,208) (118,168,218)
		108, 158, 208, 118, 168, 218,
	}

	// Test that each row correctly uses the previous decoded row
	// This is primarily a round-trip test to ensure buffer management works
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

	if closer, ok := writer.(interface{ Close() error }); ok {
		if err := closer.Close(); err != nil {
			t.Fatalf("failed to close writer: %v", err)
		}
	}

	encodedData := encodedBuf.Bytes()

	// Verify we have correct number of tag bytes (one per row)
	expectedTags := 3
	tagCount := 0
	rowBytes := params.Colors * params.Columns
	for i := 0; i < len(encodedData); i += rowBytes + 1 {
		if i < len(encodedData) && encodedData[i] == 2 { // Tag 2 = Up
			tagCount++
		}
	}
	if tagCount != expectedTags {
		t.Errorf("expected %d tag bytes, found %d", expectedTags, tagCount)
	}

	// Test decoding with buffer management
	reader, err := NewReader(bytes.NewReader(encodedData), &params)
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

	if diff := cmp.Diff(originalData, decodedData); diff != "" {
		t.Errorf("previous row buffer management failed (-original +decoded):\n%s", diff)
	}
}

func TestPNGOptimumPredictor(t *testing.T) {
	// PNG Optimum predictor (15) selects the best predictor for each row
	// Per predict.md: "tag varies" - different algorithms can be chosen per row
	params := Params{
		Colors:           1,
		BitsPerComponent: 8,
		Columns:          4,
		Predictor:        15,
	}

	originalData := []byte{10, 20, 30, 40, 15, 25, 35, 45}

	// Test round-trip (implementation chooses optimal algorithm per row)
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

	encodedData := encodedBuf.Bytes()

	// Verify that tag bytes are valid PNG predictor tags (0-4)
	// Note: Predictor 15 may choose different algorithms per row
	rowBytes := params.Colors * params.Columns
	rowCount := len(originalData) / rowBytes
	tagPositions := make([]int, 0, rowCount)

	for i := 0; i < len(encodedData); i += rowBytes + 1 {
		if i < len(encodedData) {
			tag := encodedData[i]
			if tag > 4 {
				t.Errorf("invalid PNG tag byte %d at position %d", tag, i)
			}
			tagPositions = append(tagPositions, i)
		}
	}

	// Verify we have the expected number of tag bytes
	if len(tagPositions) != rowCount {
		t.Errorf("expected %d tag bytes, found %d", rowCount, len(tagPositions))
	}

	// Test decoding - should work regardless of which algorithms were chosen
	reader, err := NewReader(bytes.NewReader(encodedData), &params)
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

	if diff := cmp.Diff(originalData, decodedData); diff != "" {
		t.Errorf("PNG Optimum round trip failed (-original +decoded):\n%s", diff)
	}
}

func TestPNGDataSizeWithTagBytes(t *testing.T) {
	// Test that PNG predictors correctly account for tag bytes in encoded data size
	// Per predict.md: Each row gets a tag byte + row data
	tests := []struct {
		name   string
		params Params
		rows   int // Number of rows in the test image
	}{
		{
			name: "single row",
			params: Params{
				Colors:           3,
				BitsPerComponent: 8,
				Columns:          2,
				Predictor:        11,
			},
			rows: 1,
		},
		{
			name: "multiple rows",
			params: Params{
				Colors:           1,
				BitsPerComponent: 8,
				Columns:          3,
				Predictor:        12,
			},
			rows: 3,
		},
		{
			name: "sub-byte with padding",
			params: Params{
				Colors:           1,
				BitsPerComponent: 4,
				Columns:          3, // 12 bits per row
				Predictor:        11,
			},
			rows: 2,
		},
		{
			name: "cross-byte boundaries",
			params: Params{
				Colors:           1,
				BitsPerComponent: 2,
				Columns:          5, // 10 bits per row
				Predictor:        11,
			},
			rows: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Calculate data size using predict.md formulas
			bitsPerPixel := tt.params.Colors * tt.params.BitsPerComponent
			bitsPerRow := bitsPerPixel * tt.params.Columns
			bytesPerRow := (bitsPerRow + 7) / 8 // Round up to bytes
			originalDataSize := bytesPerRow * tt.rows

			originalData := make([]byte, originalDataSize)
			for i := range originalData {
				originalData[i] = byte(i + 1) // Non-zero test data
			}

			// Encode
			encodedBuf := &writeCloser{Buffer: &bytes.Buffer{}}
			writer, err := NewWriter(encodedBuf, &tt.params)
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

			encodedData := encodedBuf.Bytes()

			// Calculate expected encoded size: original data + tag bytes
			// Per predict.md: each row gets a tag byte
			expectedEncodedSize := originalDataSize + tt.rows
			if len(encodedData) != expectedEncodedSize {
				t.Errorf("encoded size mismatch: got %d bytes, expected %d (original %d + %d tag bytes)",
					len(encodedData), expectedEncodedSize, originalDataSize, tt.rows)
			}

			// Verify tag byte positions and values
			for i := 0; i < tt.rows; i++ {
				tagPos := i * (bytesPerRow + 1)
				if tagPos >= len(encodedData) {
					t.Errorf("missing tag byte for row %d at position %d", i, tagPos)
					continue
				}
				tag := encodedData[tagPos]
				if tag > 4 {
					t.Errorf("invalid tag byte %d at row %d", tag, i)
				}
			}

			// Test round-trip
			reader, err := NewReader(bytes.NewReader(encodedData), &tt.params)
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

			if diff := cmp.Diff(originalData, decodedData); diff != "" {
				t.Errorf("PNG data size round trip failed (-original +decoded):\n%s", diff)
			}
		})
	}
}
