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

package traverse

import (
	"fmt"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf"
)

// Helper function to navigate using the new Context interface
func navigateStreamContext(ctx Context, key string) (Context, error) {
	steps := ctx.Next()
	for _, step := range steps {
		if step.Match.MatchString(key) {
			return step.Next(key)
		}
	}
	return nil, &KeyError{Key: key, Ctx: "navigation"}
}

// Helper function to get available step descriptions (replaces Keys())
func getStreamStepDescriptions(ctx Context) []string {
	steps := ctx.Next()
	if len(steps) == 0 {
		return nil
	}
	descs := make([]string, len(steps))
	for i, step := range steps {
		descs[i] = step.Desc
	}
	return descs
}

func TestStreamCtxShow(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		isBinary bool
	}{
		{
			name:     "text content",
			content:  "This is normal text content\nwith multiple lines\nand some formatting.",
			isBinary: false,
		},
		{
			name:     "empty stream",
			content:  "",
			isBinary: false,
		},
		{
			name:     "binary content",
			content:  string([]byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00, 0x00, 0x0D}),
			isBinary: true,
		},
		{
			name:     "line endings conversion",
			content:  "Line 1\r\nLine 2\rLine 3\nLine 4",
			isBinary: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &streamCtx{
				r:    strings.NewReader(tt.content),
				name: "test stream",
			}

			err := ctx.Show()
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestStreamCtxNext(t *testing.T) {
	ctx := &streamCtx{
		r:    strings.NewReader("test content"),
		name: "test stream",
	}

	result, err := navigateStreamContext(ctx, "any_key")
	if result != nil {
		t.Errorf("expected nil result but got %v", result)
	}
	if err == nil {
		t.Error("expected error but got nil")
	}
	if _, ok := err.(*KeyError); !ok {
		t.Errorf("expected *KeyError but got %T: %v", err, err)
	}
}

func TestStreamCtxKeys(t *testing.T) {
	ctx := &streamCtx{
		r:    strings.NewReader("test content"),
		name: "test stream",
	}

	keys := getStreamStepDescriptions(ctx)
	if len(keys) != 0 {
		t.Errorf("expected empty keys but got %v", keys)
	}
}

func TestStreamNavigation(t *testing.T) {
	tests := []struct {
		name        string
		key         string
		expectType  string
		expectError bool
	}{
		{
			name:        "encoded stream",
			key:         "@encoded",
			expectType:  "*traverse.rawStreamCtx",
			expectError: false,
		},
		{
			name:        "decoded stream (error - no reader)",
			key:         "@raw",
			expectType:  "*traverse.streamCtx",
			expectError: true,
		},
		{
			name:        "dict access",
			key:         "dict",
			expectType:  "*traverse.objectCtx",
			expectError: false,
		},
		{
			name:        "stream dict key",
			key:         "Length",
			expectType:  "*traverse.objectCtx",
			expectError: false,
		},
		{
			name:        "invalid key",
			key:         "NonExistent",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stream := &pdf.Stream{
				Dict: pdf.Dict{
					"Type":   pdf.Name("XObject"),
					"Length": pdf.Integer(100),
				},
				R: strings.NewReader("test stream content"),
			}

			ctx := &objectCtx{obj: stream, r: nil} // Note: r is nil for this simple test
			result, err := navigateStreamContext(ctx, tt.key)

			if tt.expectError {
				if err == nil {
					t.Error("expected error but got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if result == nil {
				t.Error("expected result but got nil")
				return
			}

			resultType := fmt.Sprintf("%T", result)
			if resultType != tt.expectType {
				t.Errorf("expected type %s but got %s", tt.expectType, resultType)
			}
		})
	}
}

func TestPageContents(t *testing.T) {
	tests := []struct {
		name        string
		pageDict    pdf.Dict
		expectError bool
	}{
		{
			name: "valid page with single stream",
			pageDict: pdf.Dict{
				"Type":     pdf.Name("Page"),
				"Parent":   pdf.NewReference(1, 0),
				"Contents": &pdf.Stream{R: strings.NewReader("BT /F1 12 Tf (Hello) Tj ET")},
			},
			expectError: false,
		},
		{
			name: "valid page with content array",
			pageDict: pdf.Dict{
				"Type":     pdf.Name("Page"),
				"Parent":   pdf.NewReference(1, 0),
				"Contents": pdf.Array{&pdf.Stream{R: strings.NewReader("BT")}, &pdf.Stream{R: strings.NewReader("ET")}},
			},
			expectError: false,
		},
		{
			name: "invalid - missing Type",
			pageDict: pdf.Dict{
				"Parent":   pdf.NewReference(1, 0),
				"Contents": &pdf.Stream{R: strings.NewReader("test")},
			},
			expectError: true,
		},
		{
			name: "invalid - wrong Type",
			pageDict: pdf.Dict{
				"Type":     pdf.Name("Catalog"),
				"Parent":   pdf.NewReference(1, 0),
				"Contents": &pdf.Stream{R: strings.NewReader("test")},
			},
			expectError: true,
		},
		{
			name: "invalid - missing Parent",
			pageDict: pdf.Dict{
				"Type":     pdf.Name("Page"),
				"Contents": &pdf.Stream{R: strings.NewReader("test")},
			},
			expectError: true,
		},
		{
			name: "invalid - missing Contents",
			pageDict: pdf.Dict{
				"Type":   pdf.Name("Page"),
				"Parent": pdf.NewReference(1, 0),
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &objectCtx{obj: tt.pageDict, r: nil} // Note: r is nil for this simple test
			result, err := navigateStreamContext(ctx, "@contents")

			if tt.expectError {
				if err == nil {
					t.Error("expected error but got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if result == nil {
				t.Error("expected result but got nil")
				return
			}

			streamCtx, ok := result.(*streamCtx)
			if !ok {
				t.Errorf("expected *streamCtx but got %T", result)
				return
			}

			if streamCtx.r == nil {
				t.Error("expected reader but got nil")
			}
		})
	}
}

func TestLineEndings(t *testing.T) {
	testContent := "Line 1\r\nLine 2\rLine 3\nLine 4"

	ctx := &streamCtx{
		r:    strings.NewReader(testContent),
		name: "line endings test",
	}

	// We can't easily test the output directly, but we can ensure
	// the Show() method doesn't error
	err := ctx.Show()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestObjectCtxKeysWithSpecialActions(t *testing.T) {
	tests := []struct {
		name     string
		obj      pdf.Object
		expected []string
	}{
		{
			name: "stream with special actions",
			obj: &pdf.Stream{
				Dict: pdf.Dict{
					"Type":   pdf.Name("XObject"),
					"Length": pdf.Integer(100),
				},
			},
			expected: []string{"`@encoded`", "`@raw`", "`dict`", "stream dict keys"},
		},
		{
			name: "page dict with @contents",
			obj: pdf.Dict{
				"Type":     pdf.Name("Page"),
				"Parent":   pdf.NewReference(1, 0),
				"Contents": pdf.NewReference(2, 0),
				"MediaBox": pdf.Array{pdf.Integer(0), pdf.Integer(0), pdf.Integer(612), pdf.Integer(792)},
			},
			expected: []string{"`@contents`", "dict keys (with optional /)"},
		},
		{
			name: "pages dict with page numbers",
			obj: pdf.Dict{
				"Type":  pdf.Name("Pages"),
				"Kids":  pdf.Array{pdf.NewReference(1, 0), pdf.NewReference(2, 0)},
				"Count": pdf.Integer(2),
			},
			expected: []string{"page numbers", "dict keys (with optional /)"},
		},
		{
			name: "regular dict",
			obj: pdf.Dict{
				"Type":  pdf.Name("Catalog"),
				"Pages": pdf.NewReference(1, 0),
			},
			expected: []string{"dict keys (with optional /)"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &objectCtx{obj: tt.obj}
			result := getStreamStepDescriptions(ctx)

			if d := cmp.Diff(result, tt.expected); d != "" {
				t.Errorf("keys mismatch (-got +want):\n%s", d)
			}
		})
	}
}
