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

package appearance

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/form"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

var (
	appA = &form.Form{
		Draw: func(page *graphics.Writer) error {
			page.SetFillColor(color.DeviceGray(0.25))
			page.Rectangle(0, 0, 24, 24)
			page.Fill()
			return nil
		},
		BBox:   pdf.Rectangle{LLx: 0, LLy: 0, URx: 24, URy: 24},
		Matrix: matrix.Identity,
	}
	appB = &form.Form{
		Draw: func(page *graphics.Writer) error {
			page.SetFillColor(color.DeviceGray(0.5))
			page.Rectangle(0, 0, 24, 24)
			page.Fill()
			return nil
		},
		BBox:   pdf.Rectangle{LLx: 0, LLy: 0, URx: 24, URy: 24},
		Matrix: matrix.Identity,
	}
	appC = &form.Form{
		Draw: func(page *graphics.Writer) error {
			page.SetFillColor(color.DeviceGray(0.75))
			page.Rectangle(0, 0, 24, 24)
			page.Fill()
			return nil
		},
		BBox:   pdf.Rectangle{LLx: 0, LLy: 0, URx: 24, URy: 24},
		Matrix: matrix.Identity,
	}
)

type testCase struct {
	name string
	data *Dict
}

var testCases = []testCase{
	{
		name: "streams",
		data: &Dict{
			Normal:   appA,
			RollOver: appB,
			Down:     appC,
		},
	},
	{
		name: "single",
		data: &Dict{
			Normal:    appA,
			RollOver:  appB,
			Down:      appC,
			SingleUse: true,
		},
	},
	{
		name: "maps",
		data: &Dict{
			NormalMap: map[pdf.Name]*form.Form{
				"N": appA,
				"D": appB,
			},
			RollOverMap: map[pdf.Name]*form.Form{
				"N": appB,
				"D": appC,
			},
			DownMap: map[pdf.Name]*form.Form{
				"N": appC,
				"D": appA,
			},
		},
	},
}

// TestRoundTrip tests the round-trip of an annotation appearance dictionary.
func TestRoundTrip(t *testing.T) {
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// embed the Dict into a PDF
			w1, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
			rm1 := pdf.NewResourceManager(w1)
			ref, _, err := pdf.ResourceManagerEmbed(rm1, tc.data)
			if err != nil {
				t.Fatal(err)
			}
			err = rm1.Close()
			if err != nil {
				t.Fatal(err)
			}
			err = w1.Close()
			if err != nil {
				t.Fatal(err)
			}

			// extract the Dict from the PDF
			extracted1, err := Extract(w1, ref)
			if err != nil {
				t.Fatal(err)
			}

			// embed the extracted Dict into a new PDF
			w2, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
			rm2 := pdf.NewResourceManager(w2)
			ref2, _, err := pdf.ResourceManagerEmbed(rm2, extracted1)
			if err != nil {
				t.Fatal(err)
			}
			err = rm2.Close()
			if err != nil {
				t.Fatal(err)
			}
			err = w2.Close()
			if err != nil {
				t.Fatal(err)
			}

			// extract the Dict again
			extracted2, err := Extract(w2, ref2)
			if err != nil {
				t.Fatal(err)
			}

			if diff := cmp.Diff(extracted1, extracted2); diff != "" {
				t.Errorf("round trip failed (-got +want):\n%s", diff)
			}
		})
	}
}
