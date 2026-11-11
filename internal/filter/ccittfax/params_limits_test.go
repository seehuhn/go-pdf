// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package ccittfax

import (
	"bytes"
	"math"
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
			columns:     math.MaxInt,
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
