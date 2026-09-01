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

package annotation

import (
	"math"
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

func TestBorderDefaults(t *testing.T) {
	// Test that default border values are not written to PDF
	annotation := &Text{
		Common: Common{
			Rect: pdf.Rectangle{LLx: 0, LLy: 0, URx: 100, URy: 50},
			Border: &Border{
				HCornerRadius: 0,
				VCornerRadius: 0,
				Width:         1, // PDF default
				DashArray:     nil,
			},
		},
	}

	buf, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	rm := pdf.NewResourceManager(buf)

	embedded, err := annotation.Encode(rm)
	if err != nil {
		t.Fatal(err)
	}

	err = rm.Close()
	if err != nil {
		t.Fatal(err)
	}

	dict, err := pdf.NewCursor(buf).Dict(embedded)
	if err != nil {
		t.Fatal(err)
	}

	// Border should not be present since it's the default value
	if _, exists := dict["Border"]; exists {
		t.Error("default border should not be written to PDF")
	}
}

// TestBorderDashRules checks the rules of PDF 32000-2 8.4.3.6 for the dash
// array of the /Border entry: the lengths must be non-negative and must not
// all be zero.  Values which a PDF file can contain but the rules do not
// allow are repaired on read, so that they can be written back out again.
func TestBorderDashRules(t *testing.T) {
	t.Run("read", func(t *testing.T) {
		for _, tc := range []struct {
			in   pdf.Array
			want []float64
		}{
			{pdf.Array{pdf.Integer(3), pdf.Integer(2)}, []float64{3, 2}},
			{pdf.Array{pdf.Integer(-1), pdf.Integer(2)}, []float64{0, 2}},
			{pdf.Array{pdf.Integer(0), pdf.Integer(2)}, []float64{0, 2}},
			{pdf.Array{pdf.Integer(0), pdf.Integer(0)}, nil}, // all zero
		} {
			w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
			obj := pdf.Array{pdf.Integer(0), pdf.Integer(0), pdf.Integer(1), tc.in}

			b, err := ExtractBorder(pdf.NewCursor(w), obj, true)
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(tc.want, b.DashArray); diff != "" {
				t.Errorf("%v: DashArray (-want +got):\n%s", tc.in, diff)
			}

			// whatever we read must be writable again
			rm := pdf.NewResourceManager(w)
			if _, err := rm.Embed(b); err != nil {
				t.Errorf("%v: cannot write back: %v", tc.in, err)
			}
		}
	})

	t.Run("write", func(t *testing.T) {
		for _, tc := range []struct {
			dash    []float64
			wantErr bool
		}{
			{[]float64{3, 2}, false},
			{[]float64{0, 2}, false}, // a zero length is allowed
			{[]float64{-1}, true},
			{[]float64{0}, true},
			{[]float64{0, 0}, true},
			{[]float64{math.NaN()}, true},
			{[]float64{math.Inf(1)}, true},
		} {
			w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
			rm := pdf.NewResourceManager(w)

			b := &Border{Width: 1, DashArray: tc.dash, SingleUse: true}
			_, err := rm.Embed(b)
			if (err != nil) != tc.wantErr {
				t.Errorf("%v: got error %v, want error: %v", tc.dash, err, tc.wantErr)
			}
		}
	})
}

// TestBorderStyleDashRules checks the same rules for the /D entry of a border
// style dictionary.
func TestBorderStyleDashRules(t *testing.T) {
	t.Run("read", func(t *testing.T) {
		for _, tc := range []struct {
			in   pdf.Array
			want []float64
		}{
			{pdf.Array{pdf.Integer(3), pdf.Integer(2)}, []float64{3, 2}},
			{pdf.Array{pdf.Integer(-1), pdf.Integer(2)}, []float64{0, 2}},
			{pdf.Array{pdf.Integer(0), pdf.Integer(0)}, []float64{3}}, // default
			{pdf.Array{}, []float64{3}},
		} {
			w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
			dict := pdf.Dict{"S": pdf.Name("D"), "D": tc.in}

			b, err := ExtractBorderStyle(pdf.NewCursor(w), dict, true)
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(tc.want, b.DashArray); diff != "" {
				t.Errorf("%v: DashArray (-want +got):\n%s", tc.in, diff)
			}

			rm := pdf.NewResourceManager(w)
			if _, err := rm.Embed(b); err != nil {
				t.Errorf("%v: cannot write back: %v", tc.in, err)
			}
		}
	})

	t.Run("write", func(t *testing.T) {
		for _, tc := range []struct {
			dash    []float64
			wantErr bool
		}{
			{[]float64{3, 2}, false},
			{[]float64{0, 2}, false},
			{[]float64{-1}, true},
			{[]float64{0, 0}, true},
			{[]float64{math.NaN()}, true},
		} {
			w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
			rm := pdf.NewResourceManager(w)

			b := &BorderStyle{Width: 1, Style: "D", DashArray: tc.dash, SingleUse: true}
			_, err := rm.Embed(b)
			if (err != nil) != tc.wantErr {
				t.Errorf("%v: got error %v, want error: %v", tc.dash, err, tc.wantErr)
			}
		}
	})
}

// TestBorderStyleDefaultNotShared checks that two border styles which fall
// back to the default dash array do not share the same slice.
func TestBorderStyleDefaultNotShared(t *testing.T) {
	w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
	dict := pdf.Dict{"S": pdf.Name("D"), "D": pdf.Array{}}

	b1, err := ExtractBorderStyle(pdf.NewCursor(w), dict, true)
	if err != nil {
		t.Fatal(err)
	}
	b2, err := ExtractBorderStyle(pdf.NewCursor(w), dict, true)
	if err != nil {
		t.Fatal(err)
	}

	b1.DashArray[0] = 99
	if b2.DashArray[0] != 3 {
		t.Errorf("default dash array is shared: got %v", b2.DashArray)
	}
}

// TestBorderStyleWriteErrorsNotMalformed checks that errors from writing a
// border style are plain errors.  A *pdf.MalformedFileError describes a file
// being read, and a permissive caller is entitled to discard one, which would
// silently drop the invalid border instead of refusing it.
func TestBorderStyleWriteErrorsNotMalformed(t *testing.T) {
	for _, tc := range []struct {
		name  string
		style *BorderStyle
	}{
		{"negative width", &BorderStyle{Width: -1, Style: "S", SingleUse: true}},
		{"missing dash array", &BorderStyle{Width: 1, Style: "D", SingleUse: true}},
		{"unexpected dash array", &BorderStyle{
			Width: 1, Style: "S", DashArray: []float64{3}, SingleUse: true,
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
			rm := pdf.NewResourceManager(w)

			_, err := rm.Embed(tc.style)
			if err == nil {
				t.Fatal("expected an error, got none")
			}
			if pdf.IsMalformed(err) {
				t.Errorf("write error reported as a malformed-file error: %v", err)
			}
		})
	}
}
