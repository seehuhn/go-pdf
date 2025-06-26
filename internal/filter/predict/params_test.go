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
	"fmt"
	"io"
	"testing"
)

// writeCloser wraps a bytes.Buffer to implement io.WriteCloser
type writeCloser struct {
	*bytes.Buffer
}

func (w *writeCloser) Close() error {
	return nil
}

func TestParamsValidate(t *testing.T) {
	tests := []struct {
		name        string
		params      Params
		expectError bool
	}{
		{
			name: "valid TIFF predictor",
			params: Params{
				Colors:           3,
				BitsPerComponent: 8,
				Columns:          100,
				Predictor:        2,
			},
			expectError: false,
		},
		{
			name: "valid PNG predictor",
			params: Params{
				Colors:           4,
				BitsPerComponent: 8,
				Columns:          50,
				Predictor:        12,
			},
			expectError: false,
		},
		{
			name: "valid no predictor",
			params: Params{
				Colors:           1,
				BitsPerComponent: 1,
				Columns:          1,
				Predictor:        1,
			},
			expectError: false,
		},
		{
			name: "invalid Colors - zero",
			params: Params{
				Colors:           0,
				BitsPerComponent: 8,
				Columns:          10,
				Predictor:        2,
			},
			expectError: true,
		},
		{
			name: "invalid Colors - too high for TIFF",
			params: Params{
				Colors:           61,
				BitsPerComponent: 8,
				Columns:          10,
				Predictor:        2,
			},
			expectError: true,
		},
		{
			name: "invalid Colors - too high for PNG",
			params: Params{
				Colors:           257,
				BitsPerComponent: 8,
				Columns:          10,
				Predictor:        12,
			},
			expectError: true,
		},
		{
			name: "invalid BitsPerComponent - zero",
			params: Params{
				Colors:           3,
				BitsPerComponent: 0,
				Columns:          10,
				Predictor:        2,
			},
			expectError: true,
		},
		{
			name: "invalid BitsPerComponent - 3",
			params: Params{
				Colors:           3,
				BitsPerComponent: 3,
				Columns:          10,
				Predictor:        2,
			},
			expectError: true,
		},
		{
			name: "invalid BitsPerComponent - 32",
			params: Params{
				Colors:           3,
				BitsPerComponent: 32,
				Columns:          10,
				Predictor:        2,
			},
			expectError: true,
		},
		{
			name: "valid BitsPerComponent - 1",
			params: Params{
				Colors:           1,
				BitsPerComponent: 1,
				Columns:          10,
				Predictor:        2,
			},
			expectError: false,
		},
		{
			name: "valid BitsPerComponent - 2",
			params: Params{
				Colors:           2,
				BitsPerComponent: 2,
				Columns:          10,
				Predictor:        2,
			},
			expectError: false,
		},
		{
			name: "valid BitsPerComponent - 4",
			params: Params{
				Colors:           1,
				BitsPerComponent: 4,
				Columns:          10,
				Predictor:        2,
			},
			expectError: false,
		},
		{
			name: "valid BitsPerComponent - 16",
			params: Params{
				Colors:           3,
				BitsPerComponent: 16,
				Columns:          10,
				Predictor:        2,
			},
			expectError: false,
		},
		{
			name: "invalid Columns - zero",
			params: Params{
				Colors:           3,
				BitsPerComponent: 8,
				Columns:          0,
				Predictor:        2,
			},
			expectError: true,
		},
		{
			name: "invalid Columns - negative",
			params: Params{
				Colors:           3,
				BitsPerComponent: 8,
				Columns:          -1,
				Predictor:        2,
			},
			expectError: true,
		},
		{
			name: "invalid Predictor - zero",
			params: Params{
				Colors:           3,
				BitsPerComponent: 8,
				Columns:          10,
				Predictor:        0,
			},
			expectError: true,
		},
		{
			name: "invalid Predictor - 3 to 9 range",
			params: Params{
				Colors:           3,
				BitsPerComponent: 8,
				Columns:          10,
				Predictor:        5,
			},
			expectError: true,
		},
		{
			name: "invalid Predictor - too high",
			params: Params{
				Colors:           3,
				BitsPerComponent: 8,
				Columns:          10,
				Predictor:        16,
			},
			expectError: true,
		},
		{
			name: "all PNG predictors valid",
			params: Params{
				Colors:           3,
				BitsPerComponent: 8,
				Columns:          10,
				Predictor:        15,
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.params.Validate()
			if tt.expectError && err == nil {
				t.Errorf("expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("expected no error but got: %v", err)
			}
		})
	}
}

func TestNewReaderWithInvalidParams(t *testing.T) {
	tests := []struct {
		name   string
		params Params
	}{
		{
			name: "invalid Colors",
			params: Params{
				Colors:           0,
				BitsPerComponent: 8,
				Columns:          10,
				Predictor:        2,
			},
		},
		{
			name: "invalid BitsPerComponent",
			params: Params{
				Colors:           3,
				BitsPerComponent: 3,
				Columns:          10,
				Predictor:        2,
			},
		},
		{
			name: "invalid Predictor",
			params: Params{
				Colors:           3,
				BitsPerComponent: 8,
				Columns:          10,
				Predictor:        5,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := io.NopCloser(bytes.NewReader([]byte{1, 2, 3}))
			_, err := NewReader(r, &tt.params)
			if err == nil {
				t.Error("expected error from NewReader with invalid params")
			}
		})
	}
}

func TestNewWriterWithInvalidParams(t *testing.T) {
	tests := []struct {
		name   string
		params Params
	}{
		{
			name: "invalid Colors",
			params: Params{
				Colors:           -1,
				BitsPerComponent: 8,
				Columns:          10,
				Predictor:        2,
			},
		},
		{
			name: "invalid BitsPerComponent",
			params: Params{
				Colors:           3,
				BitsPerComponent: 7,
				Columns:          10,
				Predictor:        2,
			},
		},
		{
			name: "invalid Predictor",
			params: Params{
				Colors:           3,
				BitsPerComponent: 8,
				Columns:          10,
				Predictor:        20,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := &writeCloser{Buffer: &bytes.Buffer{}}
			_, err := NewWriter(buf, &tt.params)
			if err == nil {
				t.Error("expected error from NewWriter with invalid params")
			}
		})
	}
}

func TestNewReaderWriterWithValidParams(t *testing.T) {
	validParams := []Params{
		{Colors: 1, BitsPerComponent: 1, Columns: 8, Predictor: 1},
		{Colors: 1, BitsPerComponent: 8, Columns: 10, Predictor: 2},
		{Colors: 3, BitsPerComponent: 8, Columns: 10, Predictor: 10},
		{Colors: 3, BitsPerComponent: 8, Columns: 10, Predictor: 11},
		{Colors: 3, BitsPerComponent: 8, Columns: 10, Predictor: 12},
		{Colors: 3, BitsPerComponent: 8, Columns: 10, Predictor: 13},
		{Colors: 3, BitsPerComponent: 8, Columns: 10, Predictor: 14},
		{Colors: 3, BitsPerComponent: 8, Columns: 10, Predictor: 15},
		{Colors: 4, BitsPerComponent: 16, Columns: 5, Predictor: 2},
		{Colors: 2, BitsPerComponent: 2, Columns: 4, Predictor: 2},
		{Colors: 1, BitsPerComponent: 4, Columns: 2, Predictor: 2},
	}

	for i, params := range validParams {
		t.Run(params.String(), func(t *testing.T) {
			// Test NewReader
			r := io.NopCloser(bytes.NewReader([]byte{1, 2, 3, 4, 5}))
			reader, err := NewReader(r, &params)
			if err != nil {
				t.Errorf("test %d: unexpected error from NewReader: %v", i, err)
			}
			if reader == nil {
				t.Errorf("test %d: NewReader returned nil reader", i)
			}

			// Test NewWriter
			buf := &writeCloser{Buffer: &bytes.Buffer{}}
			writer, err := NewWriter(buf, &params)
			if err != nil {
				t.Errorf("test %d: unexpected error from NewWriter: %v", i, err)
			}
			if writer == nil {
				t.Errorf("test %d: NewWriter returned nil writer", i)
			}
		})
	}
}

// String method for Params to help with test output
func (p Params) String() string {
	return fmt.Sprintf("Colors=%d,BPC=%d,Cols=%d,Pred=%d",
		p.Colors, p.BitsPerComponent, p.Columns, p.Predictor)
}
