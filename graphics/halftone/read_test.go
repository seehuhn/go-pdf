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
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/transfer"
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
				TransferFunction: transfer.Identity,
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
					TransferFunction: transfer.Identity,
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
				TransferFunction: transfer.Identity,
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
				TransferFunction: transfer.Identity,
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
				TransferFunction: transfer.Identity,
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
	embedded, _, err := originalHalftone.Embed(rm)
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

func FuzzRead(f *testing.F) {
	// Seed the fuzzer with valid test cases from all halftone types
	for _, cases := range testCases {
		for _, tc := range cases {
			out := memfile.New()
			opt := &pdf.WriterOptions{
				HumanReadable: true,
			}
			w, err := pdf.NewWriter(out, pdf.V2_0, opt)
			if err != nil {
				f.Fatal(err)
			}
			rm := pdf.NewResourceManager(w)

			ref := w.Alloc()

			embedded, _, err := tc.halftone.Embed(rm)
			if err != nil {
				continue
			}

			err = w.Put(ref, embedded)
			if err != nil {
				continue
			}

			err = rm.Close()
			if err != nil {
				continue
			}

			w.GetMeta().Trailer["Quir:E"] = ref

			err = w.Close()
			if err != nil {
				continue
			}

			f.Add(out.Data)
		}
	}

	f.Fuzz(func(t *testing.T, fileData []byte) {
		// Get a "random" halftone from the PDF file.

		// Make sure we don't panic on random input.
		opt := &pdf.ReaderOptions{
			ErrorHandling: pdf.ErrorHandlingReport,
		}
		r, err := pdf.NewReader(bytes.NewReader(fileData), opt)
		if err != nil {
			t.Skip("broken PDF: " + err.Error())
		}
		obj := r.GetMeta().Trailer["Quir:E"]
		if obj == nil {
			t.Skip("broken reference")
		}
		x := pdf.NewExtractor(r)
		halftone, err := Extract(x, obj)
		if err != nil {
			t.Skip("broken halftone")
		}

		// Make sure we can write the halftone, and read it back.
		roundTripTest(t, halftone)
	})
}
