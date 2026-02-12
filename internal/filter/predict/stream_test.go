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
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestPartialReads(t *testing.T) {
	params := Params{
		Colors:           3,
		BitsPerComponent: 8,
		Columns:          2,
		Predictor:        11, // PNG Sub
	}

	// 2x2 RGB image
	originalData := []byte{
		100, 150, 200, 110, 160, 210, // Row 1
		105, 155, 205, 115, 165, 215, // Row 2
	}

	// First encode the data
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

	// Now test reading with various small buffer sizes
	bufferSizes := []int{1, 2, 3, 5, 7, 10}

	for _, bufSize := range bufferSizes {
		t.Run(fmt.Sprintf("buffer_size_%d", bufSize), func(t *testing.T) {
			reader, err := NewReader(io.NopCloser(bytes.NewReader(encodedData)), &params)
			if err != nil {
				t.Fatalf("failed to create reader: %v", err)
			}

			var result []byte
			for {
				buf := make([]byte, bufSize)
				n, err := reader.Read(buf)
				if n > 0 {
					result = append(result, buf[:n]...)
				}
				if err == io.EOF {
					break
				}
				if err != nil {
					t.Fatalf("read failed: %v", err)
				}
			}

			if diff := cmp.Diff(originalData, result); diff != "" {
				t.Errorf("partial read with buffer size %d failed (-original +decoded):\n%s", bufSize, diff)
			}
		})
	}
}

func TestPartialWrites(t *testing.T) {
	params := Params{
		Colors:           1,
		BitsPerComponent: 8,
		Columns:          4,
		Predictor:        12, // PNG Up
	}

	// 4x2 grayscale image
	originalData := []byte{10, 20, 30, 40, 15, 25, 35, 45}

	// Test writing with various small buffer sizes
	bufferSizes := []int{1, 2, 3, 5, 7}

	for _, bufSize := range bufferSizes {
		t.Run(fmt.Sprintf("write_buffer_size_%d", bufSize), func(t *testing.T) {
			encodedBuf := &writeCloser{Buffer: &bytes.Buffer{}}
			writer, err := NewWriter(encodedBuf, &params)
			if err != nil {
				t.Fatalf("failed to create writer: %v", err)
			}

			// Write data in chunks of bufSize
			pos := 0
			for pos < len(originalData) {
				end := min(pos+bufSize, len(originalData))

				n, err := writer.Write(originalData[pos:end])
				if err != nil {
					t.Fatalf("write failed: %v", err)
				}
				if n != end-pos {
					t.Fatalf("wrote %d bytes, expected %d", n, end-pos)
				}
				pos = end
			}

			if closer, ok := writer.(interface{ Close() error }); ok {
				if err := closer.Close(); err != nil {
					t.Fatalf("failed to close writer: %v", err)
				}
			}

			// Verify by decoding
			reader, err := NewReader(io.NopCloser(bytes.NewReader(encodedBuf.Bytes())), &params)
			if err != nil {
				t.Fatalf("failed to create reader: %v", err)
			}

			decodedData := make([]byte, len(originalData))
			n, err := reader.Read(decodedData)
			if err != nil {
				t.Fatalf("failed to read data: %v", err)
			}
			if n != len(originalData) {
				t.Fatalf("read %d bytes, expected %d", n, len(originalData))
			}

			if diff := cmp.Diff(originalData, decodedData); diff != "" {
				t.Errorf("partial write with buffer size %d failed (-original +decoded):\n%s", bufSize, diff)
			}
		})
	}
}

func TestRowBoundaryHandling(t *testing.T) {
	params := Params{
		Colors:           2,
		BitsPerComponent: 8,
		Columns:          3,
		Predictor:        12, // PNG Up
	}

	// 3x3 2-color image (3 rows, 3 columns, 2 colors each = 18 bytes total)
	originalData := []byte{
		// Row 1: (10,20) (30,40) (50,60)
		10, 20, 30, 40, 50, 60,
		// Row 2: (15,25) (35,45) (55,65)
		15, 25, 35, 45, 55, 65,
		// Row 3: (18,28) (38,48) (58,68)
		18, 28, 38, 48, 58, 68,
	}

	// Encode data
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

	// Test reading with buffer size that splits across row boundaries
	// Each row is 6 bytes + 1 tag byte = 7 bytes
	// Use buffer size 5 to force boundary splits
	reader, err := NewReader(io.NopCloser(bytes.NewReader(encodedData)), &params)
	if err != nil {
		t.Fatalf("failed to create reader: %v", err)
	}

	var result []byte
	bufSize := 5
	for {
		buf := make([]byte, bufSize)
		n, err := reader.Read(buf)
		if n > 0 {
			result = append(result, buf[:n]...)
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("read failed: %v", err)
		}
	}

	if diff := cmp.Diff(originalData, result); diff != "" {
		t.Errorf("row boundary handling failed (-original +decoded):\n%s", diff)
	}
}

func TestTagByteHandlingAcrossBoundaries(t *testing.T) {
	params := Params{
		Colors:           1,
		BitsPerComponent: 8,
		Columns:          2,
		Predictor:        11, // PNG Sub
	}

	// 2x3 grayscale image (3 rows, 2 columns each)
	originalData := []byte{
		10, 20, // Row 1
		30, 40, // Row 2
		50, 60, // Row 3
	}

	// Encode data
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

	// Test reading with buffer size that may split tag bytes from data
	// Each row is 2 bytes + 1 tag byte = 3 bytes
	// Use buffer size 2 to test tag byte handling
	reader, err := NewReader(io.NopCloser(bytes.NewReader(encodedData)), &params)
	if err != nil {
		t.Fatalf("failed to create reader: %v", err)
	}

	var result []byte
	bufSize := 2
	for {
		buf := make([]byte, bufSize)
		n, err := reader.Read(buf)
		if n > 0 {
			result = append(result, buf[:n]...)
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("read failed: %v", err)
		}
	}

	if diff := cmp.Diff(originalData, result); diff != "" {
		t.Errorf("tag byte boundary handling failed (-original +decoded):\n%s", diff)
	}
}

func TestEOFHandling(t *testing.T) {
	params := Params{
		Colors:           3,
		BitsPerComponent: 8,
		Columns:          2,
		Predictor:        10, // PNG None
	}

	originalData := []byte{100, 150, 200, 110, 160, 210}

	// Encode data
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

	// Test reading with exact buffer size
	reader, err := NewReader(io.NopCloser(bytes.NewReader(encodedData)), &params)
	if err != nil {
		t.Fatalf("failed to create reader: %v", err)
	}

	result := make([]byte, len(originalData))
	n, err = reader.Read(result)
	if err != nil {
		t.Fatalf("failed to read data: %v", err)
	}
	if n != len(originalData) {
		t.Fatalf("read %d bytes, expected %d", n, len(originalData))
	}

	// Try reading again - should get EOF
	buf := make([]byte, 10)
	n, err = reader.Read(buf)
	if err != io.EOF {
		t.Errorf("expected EOF, got %v", err)
	}
	if n != 0 {
		t.Errorf("expected 0 bytes after EOF, got %d", n)
	}

	if diff := cmp.Diff(originalData, result); diff != "" {
		t.Errorf("EOF handling failed (-original +decoded):\n%s", diff)
	}
}

func TestStreamStatePreservation(t *testing.T) {
	params := Params{
		Colors:           1,
		BitsPerComponent: 8,
		Columns:          3,
		Predictor:        12, // PNG Up
	}

	// Multi-row image to test state preservation
	originalData := []byte{
		10, 20, 30, // Row 1
		15, 25, 35, // Row 2
		18, 28, 38, // Row 3
		20, 30, 40, // Row 4
	}

	// Encode data
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

	// Test reading with varying buffer sizes to stress state preservation
	reader, err := NewReader(io.NopCloser(bytes.NewReader(encodedData)), &params)
	if err != nil {
		t.Fatalf("failed to create reader: %v", err)
	}

	var result []byte
	bufferSizes := []int{1, 3, 2, 5, 1, 4} // Irregular sizes to test state transitions

	for i, bufSize := range bufferSizes {
		buf := make([]byte, bufSize)
		n, err := reader.Read(buf)
		if n > 0 {
			result = append(result, buf[:n]...)
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("read %d failed: %v", i, err)
		}
	}

	// Read any remaining data
	for {
		buf := make([]byte, 10)
		n, err := reader.Read(buf)
		if n > 0 {
			result = append(result, buf[:n]...)
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("final read failed: %v", err)
		}
	}

	if diff := cmp.Diff(originalData, result); diff != "" {
		t.Errorf("state preservation failed (-original +decoded):\n%s", diff)
	}
}

// Mock reader that can simulate read errors
type errorReader struct {
	data       []byte
	pos        int
	errorAfter int
}

func (r *errorReader) Read(p []byte) (n int, err error) {
	if r.pos >= r.errorAfter {
		return 0, errors.New("simulated read error")
	}

	available := len(r.data) - r.pos
	if available == 0 {
		return 0, io.EOF
	}

	n = min(len(p), available)
	copy(p, r.data[r.pos:r.pos+n])
	r.pos += n
	return n, nil
}

// Wrapper to add Close method for io.ReadCloser
type errorReaderCloser struct {
	*errorReader
}

func (r *errorReaderCloser) Close() error {
	return nil
}

func TestErrorPropagation(t *testing.T) {
	params := Params{
		Colors:           1,
		BitsPerComponent: 8,
		Columns:          4,
		Predictor:        2,
	}

	// Create a reader that will error after reading some data
	mockData := []byte{1, 2, 3, 4, 5, 6, 7, 8}
	errorReader := &errorReader{
		data:       mockData,
		pos:        0,
		errorAfter: 3, // Error after reading 3 bytes
	}

	errorReaderCloser := &errorReaderCloser{errorReader: errorReader}
	reader, err := NewReader(errorReaderCloser, &params)
	if err != nil {
		t.Fatalf("failed to create reader: %v", err)
	}

	buf := make([]byte, 10)
	_, err = reader.Read(buf)
	if err == nil {
		t.Error("expected error to be propagated from underlying reader")
	}
	if err.Error() != "simulated read error" {
		t.Errorf("wrong error message: %v", err)
	}
}

// Mock writer that can simulate write errors
type errorWriter struct {
	data       []byte
	errorAfter int
}

func (w *errorWriter) Write(p []byte) (n int, err error) {
	if len(w.data) >= w.errorAfter {
		return 0, errors.New("simulated write error")
	}

	available := w.errorAfter - len(w.data)
	n = min(len(p), available)
	w.data = append(w.data, p[:n]...)

	if len(w.data) >= w.errorAfter {
		return n, errors.New("simulated write error")
	}
	return n, nil
}

func (w *errorWriter) Close() error {
	return nil
}

func TestWriteErrorPropagation(t *testing.T) {
	params := Params{
		Colors:           1,
		BitsPerComponent: 8,
		Columns:          4,
		Predictor:        2,
	}

	// Create a writer that will error after writing some data
	errorWriter := &errorWriter{
		data:       make([]byte, 0),
		errorAfter: 3, // Error after writing 3 bytes
	}

	writer, err := NewWriter(errorWriter, &params)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}

	data := []byte{1, 2, 3, 4, 5, 6, 7, 8}
	_, err = writer.Write(data)
	if err == nil {
		t.Error("expected error to be propagated from underlying writer")
	}
	if err.Error() != "simulated write error" {
		t.Errorf("wrong error message: %v", err)
	}
}
