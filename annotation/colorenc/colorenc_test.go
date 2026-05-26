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

package colorenc

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/internal/debug/mock"
)

func TestExtract(t *testing.T) {
	cases := []struct {
		name string
		in   pdf.Object
		want color.Color
	}{
		{"nil", nil, nil},
		{"empty", pdf.Array{}, nil},
		{"gray", pdf.Array{pdf.Number(0.5)}, color.DeviceGray(0.5)},
		{"rgb", pdf.Array{pdf.Number(0), pdf.Number(0.5), pdf.Number(1)}, color.DeviceRGB{0, 0.5, 1}},
		{"cmyk", pdf.Array{pdf.Number(0), pdf.Number(0), pdf.Number(0), pdf.Number(1)}, color.DeviceCMYK{0, 0, 0, 1}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Extract(mock.Getter, tc.in)
			if err != nil {
				t.Fatalf("Extract failed: %v", err)
			}
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("Extract mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestExtractInvalidLength(t *testing.T) {
	// a 2-element array is not a valid device colour
	_, err := Extract(mock.Getter, pdf.Array{pdf.Number(0), pdf.Number(1)})
	if err == nil {
		t.Fatal("expected error for 2-element array")
	}
	if !pdf.IsMalformed(err) {
		t.Errorf("expected malformed-file error, got %v", err)
	}
}

func TestExtractRGB(t *testing.T) {
	got, err := ExtractRGB(mock.Getter, pdf.Array{pdf.Number(1), pdf.Number(0), pdf.Number(0)})
	if err != nil {
		t.Fatalf("ExtractRGB failed: %v", err)
	}
	if diff := cmp.Diff(color.Color(color.DeviceRGB{1, 0, 0}), got); diff != "" {
		t.Errorf("ExtractRGB mismatch (-want +got):\n%s", diff)
	}

	if c, err := ExtractRGB(mock.Getter, nil); err != nil || c != nil {
		t.Errorf("ExtractRGB(nil) = %v, %v; want nil, nil", c, err)
	}
	if c, err := ExtractRGB(mock.Getter, pdf.Array{}); err != nil || c != nil {
		t.Errorf("ExtractRGB(empty) = %v, %v; want nil, nil", c, err)
	}
	if _, err := ExtractRGB(mock.Getter, pdf.Array{pdf.Number(1)}); err == nil {
		t.Error("expected error for 1-element RGB array")
	}
}

func TestEncode(t *testing.T) {
	cases := []struct {
		name string
		in   color.Color
		want pdf.Array
	}{
		{"nil", nil, nil},
		{"gray", color.DeviceGray(0.5), pdf.Array{pdf.Number(0.5)}},
		{"rgb", color.DeviceRGB{0, 0.5, 1}, pdf.Array{pdf.Number(0), pdf.Number(0.5), pdf.Number(1)}},
		{"cmyk", color.DeviceCMYK{0, 0, 0, 1}, pdf.Array{pdf.Number(0), pdf.Number(0), pdf.Number(0), pdf.Number(1)}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Encode(tc.in)
			if err != nil {
				t.Fatalf("Encode failed: %v", err)
			}
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("Encode mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestEncodeRGB(t *testing.T) {
	got, err := EncodeRGB(color.DeviceRGB{1, 0, 0})
	if err != nil {
		t.Fatalf("EncodeRGB failed: %v", err)
	}
	if diff := cmp.Diff(pdf.Array{pdf.Number(1), pdf.Number(0), pdf.Number(0)}, got); diff != "" {
		t.Errorf("EncodeRGB mismatch (-want +got):\n%s", diff)
	}

	if a, err := EncodeRGB(nil); err != nil || a != nil {
		t.Errorf("EncodeRGB(nil) = %v, %v; want nil, nil", a, err)
	}
	// a non-RGB colour must be rejected
	if _, err := EncodeRGB(color.DeviceGray(0.5)); err == nil {
		t.Error("expected error encoding DeviceGray as RGB")
	}
}

func TestEncodeWrongColorSpace(t *testing.T) {
	// CIELab is not a device colour space
	space, err := color.Lab(color.WhitePointD50, nil, nil)
	if err != nil {
		t.Fatalf("Lab space: %v", err)
	}
	lab, err := space.New(50, 0, 0)
	if err != nil {
		t.Fatalf("Lab color: %v", err)
	}
	if _, err := Encode(lab); err == nil {
		t.Error("expected error encoding a non-device colour")
	}
}
