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

package content

import (
	"bytes"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
)

func TestWriter_WriteSimple(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(pdf.V1_7, Page, &Resources{})

	stream := Operators{
		{Name: OpSetLineWidth, Args: []pdf.Object{pdf.Number(2)}},
		{Name: OpMoveTo, Args: []pdf.Object{pdf.Number(100), pdf.Number(100)}},
		{Name: OpLineTo, Args: []pdf.Object{pdf.Number(200), pdf.Number(200)}},
		{Name: OpStroke},
	}

	if err := w.Write(&buf, stream); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	got := buf.String()
	if got == "" {
		t.Error("Write produced empty output")
	}
}

func TestWriter_VersionCheck(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(pdf.V1_0, Page, &Resources{})

	// ri operator requires PDF 1.1
	stream := Operators{
		{Name: OpSetRenderingIntent, Args: []pdf.Object{pdf.Name("Perceptual")}},
	}

	err := w.Write(&buf, stream)
	if err == nil {
		t.Error("Write should fail for PDF 1.0 with ri operator")
	}
}

func TestWriter_StateTracking(t *testing.T) {
	var buf bytes.Buffer
	res := &Resources{}
	w := NewWriter(pdf.V1_7, Page, res)

	// Write q/Q sequence
	stream := Operators{
		{Name: OpPushGraphicsState},
		{Name: OpSetLineWidth, Args: []pdf.Object{pdf.Number(5)}},
		{Name: OpPopGraphicsState},
	}

	if err := w.Write(&buf, stream); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// MaxStackDepth should be 1
	if w.v.state.MaxStackDepth != 1 {
		t.Errorf("MaxStackDepth = %d, want 1", w.v.state.MaxStackDepth)
	}
}

func TestWriter_UnbalancedQ(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(pdf.V1_7, Page, &Resources{})

	stream := Operators{
		{Name: OpPushGraphicsState},
		// Missing Q
	}

	w.Write(&buf, stream)

	if err := w.Close(); err == nil {
		t.Error("Validate should fail for unbalanced q/Q")
	}
}

func TestWriter_ResourceValidation(t *testing.T) {
	tests := []struct {
		name   string
		res    *Resources
		stream Operators
		errMsg string
	}{
		{
			name: "missing font",
			res:  &Resources{},
			stream: Operators{
				{Name: OpTextBegin},
				{Name: OpTextSetFont, Args: []pdf.Object{pdf.Name("F1"), pdf.Number(12)}},
			},
			errMsg: "font",
		},
		{
			name: "valid font",
			res: &Resources{
				Font: map[pdf.Name]font.Instance{"F1": nil},
			},
			stream: Operators{
				{Name: OpTextBegin},
				{Name: OpTextSetFont, Args: []pdf.Object{pdf.Name("F1"), pdf.Number(12)}},
			},
			errMsg: "",
		},
		{
			name: "missing XObject",
			res:  &Resources{},
			stream: Operators{
				{Name: OpXObject, Args: []pdf.Object{pdf.Name("X1")}},
			},
			errMsg: "XObject",
		},
		{
			name: "missing ExtGState",
			res:  &Resources{},
			stream: Operators{
				{Name: OpSetExtGState, Args: []pdf.Object{pdf.Name("E1")}},
			},
			errMsg: "ExtGState",
		},
		{
			name: "missing shading",
			res:  &Resources{},
			stream: Operators{
				{Name: OpShading, Args: []pdf.Object{pdf.Name("S1")}},
			},
			errMsg: "shading",
		},
		{
			name: "missing color space",
			res:  &Resources{},
			stream: Operators{
				{Name: OpSetStrokeColorSpace, Args: []pdf.Object{pdf.Name("CS1")}},
			},
			errMsg: "color space",
		},
		{
			name: "device color space allowed",
			res:  &Resources{},
			stream: Operators{
				{Name: OpSetStrokeColorSpace, Args: []pdf.Object{pdf.Name("DeviceRGB")}},
			},
			errMsg: "",
		},
		{
			name: "missing pattern",
			res:  &Resources{},
			stream: Operators{
				{Name: OpSetStrokeColorN, Args: []pdf.Object{pdf.Name("P1")}},
			},
			errMsg: "pattern",
		},
		{
			name: "SCN with color components only",
			res:  &Resources{},
			stream: Operators{
				{Name: OpSetStrokeColorN, Args: []pdf.Object{pdf.Number(0.5), pdf.Number(0.5), pdf.Number(0.5)}},
			},
			errMsg: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			w := NewWriter(pdf.V1_7, Page, tt.res)
			err := w.Write(&buf, tt.stream)

			if tt.errMsg == "" {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			} else {
				if err == nil {
					t.Errorf("expected error containing %q", tt.errMsg)
				}
			}
		})
	}
}

func TestWriter_ValidateWithScanner(t *testing.T) {
	input := "q\n1 0 0 1 100 200 cm\nQ\n"
	r := bytes.NewReader([]byte(input))
	s := NewScanner(r, pdf.V1_7, Page, &Resources{})

	w := NewWriter(pdf.V1_7, Page, &Resources{})
	if err := w.Validate(s); err != nil {
		t.Fatalf("Validate failed: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}
}
