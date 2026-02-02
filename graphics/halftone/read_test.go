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

package halftone

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/function"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

// testCases holds test cases for all halftone types, indexed by type
var testCases = map[int][]testCase{
	1: {
		{
			name: "basic Type1",
			halftone: &Type1{
				Frequency:    60.0,
				Angle:        45.0,
				SpotFunction: SimpleDot,
			},
		},
		{
			name: "Type1 with all fields",
			halftone: &Type1{
				Frequency:        72.0,
				Angle:            30.0,
				SpotFunction:     Round,
				AccurateScreens:  true,
				TransferFunction: function.Identity,
			},
		},
	},
	5: {
		{
			name: "basic Type5",
			halftone: &Type5{
				Default: &Type1{
					Frequency:        60.0,
					Angle:            45.0,
					SpotFunction:     SimpleDot,
					TransferFunction: function.Identity,
				},
				Colorants: map[pdf.Name]graphics.Halftone{
					"Cyan": &Type1{
						Frequency:    72.0,
						Angle:        15.0,
						SpotFunction: Round,
					},
					"Magenta": &Type1{
						Frequency:    72.0,
						Angle:        75.0,
						SpotFunction: Ellipse,
					},
				},
			},
		},
	},
	6: {
		{
			name: "basic Type6",
			halftone: &Type6{
				Width:         4,
				Height:        4,
				ThresholdData: []byte{0, 128, 64, 192, 255, 127, 191, 63, 32, 160, 96, 224, 223, 95, 159, 31},
			},
		},
		{
			name: "Type6 with all fields",
			halftone: &Type6{
				Width:            2,
				Height:           2,
				ThresholdData:    []byte{0, 255, 127, 128},
				TransferFunction: function.Identity,
			},
		},
	},
	10: {
		{
			name: "basic Type10",
			halftone: &Type10{
				Size1:         3,
				Size2:         2,
				ThresholdData: []byte{0, 128, 64, 192, 255, 127, 191, 63, 32, 160, 96, 224, 31},
			},
		},
		{
			name: "Type10 with all fields",
			halftone: &Type10{
				Size1:            2,
				Size2:            1,
				ThresholdData:    []byte{0, 255, 127, 128, 64},
				TransferFunction: function.Identity,
			},
		},
	},
	16: {
		{
			name: "basic Type16",
			halftone: &Type16{
				Width:         2,
				Height:        2,
				ThresholdData: []uint16{0, 65535, 32767, 32768},
			},
		},
		{
			name: "Type16 with second rectangle",
			halftone: &Type16{
				Width:         2,
				Height:        1,
				Width2:        1,
				Height2:       2,
				ThresholdData: []uint16{0, 65535, 32767, 32768},
			},
		},
		{
			name: "Type16 with all fields",
			halftone: &Type16{
				Width:            1,
				Height:           1,
				ThresholdData:    []uint16{32767},
				TransferFunction: function.Identity,
			},
		},
	},
}

type testCase struct {
	name     string
	halftone graphics.Halftone
}

func TestRoundTrip(t *testing.T) {
	for halftoneType, cases := range testCases {
		for _, tc := range cases {
			t.Run(fmt.Sprintf("Type%d-%s", halftoneType, tc.name), func(t *testing.T) {
				roundTripTest(t, tc.halftone)
			})
		}
	}
}

// roundTripTest performs a round-trip test for any halftone type
func roundTripTest(t *testing.T, originalHalftone graphics.Halftone) {
	buf, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	rm := pdf.NewResourceManager(buf)

	// Embed the halftone
	embedded, err := rm.Embed(originalHalftone)
	if err != nil {
		t.Fatal(err)
	}

	ref := buf.Alloc()
	err = buf.Put(ref, embedded)
	if err != nil {
		t.Fatal(err)
	}

	err = buf.Close()
	if err != nil {
		t.Fatal(err)
	}

	// Read the halftone back
	x := pdf.NewExtractor(buf)
	readHalftone, err := Extract(x, ref)
	if err != nil {
		t.Fatal(err)
	}

	// Check that the types match
	if readHalftone.HalftoneType() != originalHalftone.HalftoneType() {
		t.Fatalf("halftone type mismatch: expected %d, got %d",
			originalHalftone.HalftoneType(), readHalftone.HalftoneType())
	}

	// Use cmp.Diff to compare the original and read halftone
	if diff := cmp.Diff(originalHalftone, readHalftone); diff != "" {
		t.Errorf("round trip failed (-want +got):\n%s", diff)
	}
}

func FuzzRoundTrip(f *testing.F) {
	opt := &pdf.WriterOptions{
		HumanReadable: true,
	}
	for _, cases := range testCases {
		for _, tc := range cases {
			w, buf := memfile.NewPDFWriter(pdf.V2_0, opt)

			// AddBlankPage creates a minimal valid PDF structure.
			// Without this, the seeds will likely be rejected by pdf.NewReader.
			err := memfile.AddBlankPage(w)
			if err != nil {
				continue
			}

			rm := pdf.NewResourceManager(w)

			embedded, err := rm.Embed(tc.halftone)
			if err != nil {
				continue
			}

			err = rm.Close()
			if err != nil {
				continue
			}

			w.GetMeta().Trailer["Quir:E"] = embedded

			err = w.Close()
			if err != nil {
				continue
			}

			f.Add(buf.Data)
		}
	}

	f.Fuzz(func(t *testing.T, fileData []byte) {
		r, err := pdf.NewReader(bytes.NewReader(fileData), nil)
		if err != nil {
			t.Skip("invalid PDF")
		}
		obj := r.GetMeta().Trailer["Quir:E"]
		if obj == nil {
			t.Skip("missing PDF object")
		}
		x := pdf.NewExtractor(r)
		halftone, err := Extract(x, obj)
		if err != nil {
			t.Skip("malformed PDF object")
		}

		roundTripTest(t, halftone)
	})
}
