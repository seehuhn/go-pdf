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
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf"
)

func TestObjectCtxNext(t *testing.T) {
	tests := []struct {
		name        string
		obj         pdf.Object
		key         string
		wantErr     bool
		expectedObj pdf.Object
	}{
		{
			name: "dict valid key",
			obj: pdf.Dict{
				"Type":  pdf.Name("Page"),
				"Title": pdf.Integer(985475),
			},
			key:         "Title",
			wantErr:     false,
			expectedObj: pdf.Integer(985475),
		},
		{
			name: "dict invalid key",
			obj: pdf.Dict{
				"Type": pdf.Name("Page"),
			},
			key:     "Missing",
			wantErr: true,
		},
		{
			name: "dict with forward slash prefix",
			obj: pdf.Dict{
				"Type":  pdf.Name("Page"),
				"Title": pdf.Name("X835115"),
			},
			key:         "/Title",
			wantErr:     false,
			expectedObj: pdf.Name("X835115"),
		},
		{
			name: "array valid positive index",
			obj: pdf.Array{
				pdf.Integer(1),
				pdf.String("test"),
				pdf.Integer(3),
			},
			key:         "1",
			wantErr:     false,
			expectedObj: pdf.String("test"),
		},
		{
			name: "array valid negative index",
			obj: pdf.Array{
				pdf.Real(1.0),
				pdf.String("two"),
				pdf.Integer(3),
			},
			key:         "-1",
			wantErr:     false,
			expectedObj: pdf.Integer(3),
		},
		{
			name: "array out of bounds positive",
			obj: pdf.Array{
				pdf.Integer(1),
				pdf.String("test"),
			},
			key:     "5",
			wantErr: true,
		},
		{
			name: "array out of bounds negative",
			obj: pdf.Array{
				pdf.Integer(1),
				pdf.String("test"),
			},
			key:     "-5",
			wantErr: true,
		},
		{
			name: "array invalid index",
			obj: pdf.Array{
				pdf.Integer(1),
				pdf.String("test"),
			},
			key:     "not_a_number",
			wantErr: true,
		},
		{
			name: "stream dict key",
			obj: &pdf.Stream{
				Dict: pdf.Dict{
					"Type":   pdf.Name("XObject"),
					"Length": pdf.Integer(100),
				},
			},
			key: "dict",
			expectedObj: pdf.Dict{
				"Type":   pdf.Name("XObject"),
				"Length": pdf.Integer(100),
			},
		},
		{
			name: "stream dictionary field",
			obj: &pdf.Stream{
				Dict: pdf.Dict{
					"Type":   pdf.Name("XObject"),
					"Length": pdf.Integer(100),
				},
			},
			key:         "Length",
			wantErr:     false,
			expectedObj: pdf.Integer(100),
		},
		{
			name: "stream invalid key",
			obj: &pdf.Stream{
				Dict: pdf.Dict{
					"Type": pdf.Name("XObject"),
				},
			},
			key:     "Missing",
			wantErr: true,
		},
		{
			name:    "unsupported type",
			obj:     pdf.Integer(42),
			key:     "anything",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &objectCtx{obj: tt.obj}
			res, err := c.Next(tt.key)

			if tt.wantErr {
				switch err.(type) {
				case *KeyError:
					// pass
				case nil:
					t.Errorf("expected *KeyError but got nil")
				default:
					t.Errorf("expected *KeyError but got %T: %v", err, err)
				}
				return
			} else if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if res == nil {
				t.Errorf("expected result but got nil")
				return
			}

			objCtx, ok := res.(*objectCtx)
			if !ok {
				t.Errorf("expected *objectCtx but got %T", res)
				return
			}

			// Compare the objects
			if d := cmp.Diff(objCtx.obj, tt.expectedObj); d != "" {
				t.Errorf("object mismatch (-got +want):\n%s", d)
			}
		})
	}
}

func TestObjectCtxKeys(t *testing.T) {
	tests := []struct {
		name     string
		obj      pdf.Object
		expected []string
	}{
		{
			name: "dict keys",
			obj: pdf.Dict{
				"Type":  pdf.Name("Page"),
				"Title": pdf.String("Test"),
				"Count": pdf.Integer(1),
			},
			expected: []string{"dict keys"},
		},
		{
			name:     "empty dict",
			obj:      pdf.Dict{},
			expected: nil,
		},
		{
			name: "array indices",
			obj: pdf.Array{
				pdf.Integer(1),
				pdf.String("test"),
				pdf.Integer(3),
			},
			expected: []string{"array indices (-3 to 2)"},
		},
		{
			name:     "empty array",
			obj:      pdf.Array{},
			expected: nil,
		},
		{
			name: "stream keys",
			obj: &pdf.Stream{
				Dict: pdf.Dict{
					"Type":   pdf.Name("XObject"),
					"Length": pdf.Integer(100),
				},
			},
			expected: []string{"`@raw`", "`@stream`", "`dict`", "stream dict keys"},
		},
		{
			name: "stream with empty dict",
			obj: &pdf.Stream{
				Dict: pdf.Dict{},
			},
			expected: []string{"`@raw`", "`@stream`", "`dict`"},
		},
		{
			name:     "scalar type",
			obj:      pdf.Integer(42),
			expected: nil,
		},
		{
			name:     "nil object",
			obj:      nil,
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &objectCtx{obj: tt.obj}
			result, err := c.Keys()

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if d := cmp.Diff(result, tt.expected); d != "" {
				t.Errorf("keys mismatch (-got +want):\n%s", d)
			}
		})
	}
}

func TestObjectCtxShow(t *testing.T) {
	tests := []struct {
		name    string
		obj     pdf.Object
		wantErr bool
	}{
		{
			name: "nil object",
			obj:  nil,
		},
		{
			name: "simple dict",
			obj: pdf.Dict{
				"Type":  pdf.Name("Page"),
				"Title": pdf.String("Test"),
			},
		},
		{
			name: "simple array",
			obj: pdf.Array{
				pdf.Integer(1),
				pdf.String("test"),
				pdf.Integer(3),
			},
		},
		{
			name: "integer",
			obj:  pdf.Integer(42),
		},
		{
			name: "string",
			obj:  pdf.String("test"),
		},
		{
			name: "name",
			obj:  pdf.Name("TestName"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &objectCtx{obj: tt.obj}
			err := c.Show()

			if tt.wantErr && err == nil {
				t.Error("Expected error but got none")
			} else if !tt.wantErr && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func TestDictKeyOrdering(t *testing.T) {
	testDict := pdf.Dict{
		"Title":     pdf.String("Test"),
		"Type":      pdf.Name("Page"),
		"Subtype":   pdf.Name("Form"),
		"FirstChar": pdf.Integer(32),
		"RandomKey": pdf.String("value"),
	}

	keys := dictKeys(testDict)

	// Check that Type comes first (order 0)
	if len(keys) == 0 || keys[0] != "Type" {
		t.Errorf("Expected Type to be first key, got: %v", keys)
	}

	// Check that Subtype comes second (order 1)
	if len(keys) < 2 || keys[1] != "Subtype" {
		t.Errorf("Expected Subtype to be second key, got: %v", keys)
	}
}

func TestBinaryDetection(t *testing.T) {
	textData := []byte("This is normal text content")
	if mostlyBinary(textData) {
		t.Error("Normal text should not be detected as binary")
	}

	binaryData := make([]byte, 100)
	for i := range binaryData {
		binaryData[i] = byte(i % 256)
	}
	if !mostlyBinary(binaryData) {
		t.Error("Binary data should be detected as binary")
	}
}
