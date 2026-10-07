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

package softclip_test

import (
	"testing"

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/extract"
	"seehuhn.de/go/pdf/graphics/form"
	"seehuhn.de/go/pdf/graphics/group"
	"seehuhn.de/go/pdf/graphics/softclip"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

func TestMaskRoundTrip(t *testing.T) {
	for _, tc := range []struct {
		name string
		mask *softclip.Mask
	}{
		{
			name: "Alpha",
			mask: &softclip.Mask{
				S: softclip.Alpha,
				G: makeTransparencyGroup(nil),
			},
		},
		{
			name: "Luminosity",
			mask: &softclip.Mask{
				S:  softclip.Luminosity,
				G:  makeTransparencyGroup(color.SpaceDeviceRGB),
				BC: []float64{0.5, 0.5, 0.5},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			writer, _ := memfile.NewPDFWriter(t, pdf.V1_4, nil)
			rm := pdf.NewResourceManager(writer)

			embedded, err := rm.Embed(tc.mask)
			if err != nil {
				t.Fatalf("Embed: %v", err)
			}

			err = rm.Close()
			if err != nil {
				t.Fatalf("rm.Close: %v", err)
			}
			err = writer.Close()
			if err != nil {
				t.Fatalf("writer.Close: %v", err)
			}

			x := pdf.NewExtractor(writer)
			got, err := extract.SoftMaskDict(pdf.CursorAt(x, nil), embedded, false)
			if err != nil {
				t.Fatalf("Extract: %v", err)
			}

			if !tc.mask.Equal(got) {
				t.Errorf("round-trip mismatch:\nwant: %+v\ngot:  %+v", tc.mask, got)
			}
		})
	}
}

// TestMaskBCInvalid checks that Embed rejects a backdrop color which does
// not match the group color space.
func TestMaskBCInvalid(t *testing.T) {
	for _, tc := range []struct {
		name string
		cs   color.Space
		bc   []float64
	}{
		{"wrong length", color.SpaceDeviceRGB, []float64{0.5}},
		{"no color space", nil, []float64{0.5}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			writer, _ := memfile.NewPDFWriter(t, pdf.V1_4, nil)
			rm := pdf.NewResourceManager(writer)
			m := &softclip.Mask{
				S:  softclip.Luminosity,
				G:  makeTransparencyGroup(tc.cs),
				BC: tc.bc,
			}
			if _, err := rm.Embed(m); err == nil {
				t.Error("invalid BC accepted")
			}
		})
	}
}

func makeTransparencyGroup(cs color.Space) *form.Form {
	return &form.Form{
		BBox:   pdf.Rectangle{URx: 100, URy: 100},
		Matrix: matrix.Identity,
		Res:    &content.Resources{},
		Group:  &group.TransparencyAttributes{CS: cs, SingleUse: true},
	}
}
