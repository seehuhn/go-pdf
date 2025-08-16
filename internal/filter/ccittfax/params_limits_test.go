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

package ccittfax

import (
	"bytes"
	"testing"
)

func TestColumnsLimit(t *testing.T) {
	tests := []struct {
		name        string
		columns     int
		expectError bool
	}{
		{
			name:        "default columns (0 -> 1728)",
			columns:     0,
			expectError: false,
		},
		{
			name:        "reasonable columns",
			columns:     1728,
			expectError: false,
		},
		{
			name:        "maximum allowed columns",
			columns:     maxColumns,
			expectError: false,
		},
		{
			name:        "excessive columns - prevent memory exhaustion",
			columns:     9000000000002,
			expectError: true,
		},
		{
			name:        "negative columns",
			columns:     -1,
			expectError: true,
		},
		{
			name:        "just above maximum",
			columns:     maxColumns + 1,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := &Params{
				Columns: tt.columns,
				K:       0,
			}

			// Test NewReader
			_, err := NewReader(bytes.NewReader(nil), params)
			if tt.expectError {
				if err == nil {
					t.Errorf("NewReader: expected error for columns=%d, got nil", tt.columns)
				}
			} else {
				if err != nil {
					t.Errorf("NewReader: unexpected error for columns=%d: %v", tt.columns, err)
				}
			}

			// Test NewWriter
			buf := &bytes.Buffer{}
			_, err = NewWriter(buf, params)
			if tt.expectError {
				if err == nil {
					t.Errorf("NewWriter: expected error for columns=%d, got nil", tt.columns)
				}
			} else {
				if err != nil {
					t.Errorf("NewWriter: unexpected error for columns=%d: %v", tt.columns, err)
				}
			}
		})
	}
}
