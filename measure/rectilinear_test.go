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

package measure

import (
	"bytes"
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

var rectilinearTestCases = []struct {
	name    string
	version pdf.Version
	data    *RectilinearMeasure
}{
	{
		name:    "basic_measure_different_axes_v17",
		version: pdf.V1_7,
		data: &RectilinearMeasure{
			ScaleRatio: "1in = 0.1 mi",
			XAxis: []*NumberFormat{{
				Unit:             "mi",
				ConversionFactor: 1.0,
				Precision:        100000,
				FractionFormat:   FractionDecimal,
				SingleUse:        true,
			}},
			YAxis: []*NumberFormat{{
				Unit:             "ft",
				ConversionFactor: 5280,
				Precision:        1,
				FractionFormat:   FractionDecimal,
				SingleUse:        true,
			}},
			Distance: []*NumberFormat{{
				Unit:             "mi",
				ConversionFactor: 1.0,
				Precision:        100000,
				FractionFormat:   FractionDecimal,
				SingleUse:        true,
			}, {
				Unit:             "ft",
				ConversionFactor: 5280,
				Precision:        1,
				FractionFormat:   FractionDecimal,
				SingleUse:        true,
			}, {
				Unit:             "in",
				ConversionFactor: 12,
				Precision:        8,
				FractionFormat:   FractionFraction,
				SingleUse:        true,
			}},
			Area: []*NumberFormat{{
				Unit:             "acres",
				ConversionFactor: 640,
				Precision:        100,
				SingleUse:        true,
			}},
			Origin:    [2]float64{0, 0},
			CYX:       0.000189394,
			SingleUse: true,
		},
	},
	{
		name:    "same_xy_axes_v20",
		version: pdf.V2_0,
		data: &RectilinearMeasure{
			ScaleRatio: "1:100",
			XAxis: []*NumberFormat{{
				Unit:             "m",
				ConversionFactor: 1.0,
				Precision:        100,
				SingleUse:        true,
			}},
			YAxis: []*NumberFormat{{
				Unit:             "m",
				ConversionFactor: 1.0,
				Precision:        100,
				SingleUse:        true,
			}},
			Distance: []*NumberFormat{{
				Unit:             "m",
				ConversionFactor: 1.0,
				Precision:        100,
				SingleUse:        true,
			}},
			Area: []*NumberFormat{{
				Unit:             "m²",
				ConversionFactor: 1.0,
				Precision:        100,
				SingleUse:        true,
			}},
			Origin:    [2]float64{0, 0},
			CYX:       1.0,
			SingleUse: true,
		},
	},
	{
		name:    "non_zero_origin_v17",
		version: pdf.V1_7,
		data: &RectilinearMeasure{
			ScaleRatio: "1cm = 1m",
			XAxis: []*NumberFormat{{
				Unit:             "m",
				ConversionFactor: 100.0,
				Precision:        10,
				SingleUse:        true,
			}},
			YAxis: []*NumberFormat{{
				Unit:             "m",
				ConversionFactor: 100.0,
				Precision:        10,
				SingleUse:        true,
			}},
			Distance: []*NumberFormat{{
				Unit:             "m",
				ConversionFactor: 100.0,
				Precision:        10,
				SingleUse:        true,
			}},
			Area: []*NumberFormat{{
				Unit:             "m²",
				ConversionFactor: 10000.0,
				Precision:        100,
				SingleUse:        true,
			}},
			Origin:    [2]float64{100.5, 200.75},
			CYX:       1.0,
			SingleUse: true,
		},
	},
	{
		name:    "with_optional_fields_v20",
		version: pdf.V2_0,
		data: &RectilinearMeasure{
			ScaleRatio: "1:50",
			XAxis: []*NumberFormat{{
				Unit:             "mm",
				ConversionFactor: 1.0,
				Precision:        1,
				SingleUse:        true,
			}},
			YAxis: []*NumberFormat{{
				Unit:             "mm",
				ConversionFactor: 1.0,
				Precision:        1,
				SingleUse:        true,
			}},
			Distance: []*NumberFormat{{
				Unit:             "mm",
				ConversionFactor: 1.0,
				Precision:        1,
				SingleUse:        true,
			}},
			Area: []*NumberFormat{{
				Unit:             "mm²",
				ConversionFactor: 1.0,
				Precision:        1,
				SingleUse:        true,
			}},
			Angle: []*NumberFormat{{
				Unit:             "deg",
				ConversionFactor: 1.0,
				Precision:        10,
				SingleUse:        true,
			}},
			Slope: []*NumberFormat{{
				Unit:             "%",
				ConversionFactor: 100.0,
				Precision:        10,
				SingleUse:        true,
			}},
			Origin:    [2]float64{0, 0},
			CYX:       1.0,
			SingleUse: false,
		},
	},
}

func rectilinearRoundTripTest(t *testing.T, version pdf.Version, data *RectilinearMeasure) {
	t.Helper()

	w, _ := memfile.NewPDFWriter(version, nil)

	rm := pdf.NewResourceManager(w)
	embedded, err := rm.Embed(data)
	if err != nil {
		t.Fatalf("embed failed: %v", err)
	}

	err = rm.Close()
	if err != nil {
		t.Fatalf("resource manager close failed: %v", err)
	}

	decoded, err := Extract(w, embedded)
	if err != nil {
		t.Fatalf("extract failed: %v", err)
	}

	// Type assertion to RectilinearMeasure
	decodedRL, ok := decoded.(*RectilinearMeasure)
	if !ok {
		t.Fatalf("extracted measure is not RectilinearMeasure, got %T", decoded)
	}

	// Fix SingleUse fields for comparison (not stored in PDF)
	fixSingleUseFields(decodedRL, data)

	if diff := cmp.Diff(data, decodedRL); diff != "" {
		t.Errorf("round trip failed (-want +got):\n%s", diff)
	}
}

// fixSingleUseFields sets SingleUse fields to match original for comparison
func fixSingleUseFields(decoded, original *RectilinearMeasure) {
	decoded.SingleUse = original.SingleUse

	// Helper function to fix NumberFormat fields
	fixNumberFormat := func(decoded, original *NumberFormat) {
		decoded.SingleUse = original.SingleUse
		// DecimalSeparator: empty string becomes period after round-trip
		if original.DecimalSeparator == "" {
			decoded.DecimalSeparator = ""
		}
	}

	for i, nf := range decoded.XAxis {
		if i < len(original.XAxis) {
			fixNumberFormat(nf, original.XAxis[i])
		}
	}
	for i, nf := range decoded.YAxis {
		if i < len(original.YAxis) {
			fixNumberFormat(nf, original.YAxis[i])
		}
	}
	for i, nf := range decoded.Distance {
		if i < len(original.Distance) {
			fixNumberFormat(nf, original.Distance[i])
		}
	}
	for i, nf := range decoded.Area {
		if i < len(original.Area) {
			fixNumberFormat(nf, original.Area[i])
		}
	}
	for i, nf := range decoded.Angle {
		if i < len(original.Angle) {
			fixNumberFormat(nf, original.Angle[i])
		}
	}
	for i, nf := range decoded.Slope {
		if i < len(original.Slope) {
			fixNumberFormat(nf, original.Slope[i])
		}
	}
}

func TestRectilinearSpecificationRoundTrip(t *testing.T) {
	for _, tc := range rectilinearTestCases {
		t.Run(tc.name, func(t *testing.T) {
			rectilinearRoundTripTest(t, tc.version, tc.data)
		})
	}
}

func FuzzRectilinearRoundTrip(f *testing.F) {
	opt := &pdf.WriterOptions{
		HumanReadable: true,
	}
	for _, tc := range rectilinearTestCases {
		w, buf := memfile.NewPDFWriter(tc.version, opt)

		rm := pdf.NewResourceManager(w)
		embedded, err := rm.Embed(tc.data)
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

	f.Fuzz(func(t *testing.T, fileData []byte) {
		r, err := pdf.NewReader(bytes.NewReader(fileData), nil)
		if err != nil {
			t.Skip("invalid PDF")
		}
		objPDF := r.GetMeta().Trailer["Quir:E"]
		if objPDF == nil {
			t.Skip("missing PDF object")
		}

		objGo, err := Extract(r, objPDF)
		if err != nil {
			t.Skip("malformed PDF object")
		}

		objGoRL, ok := objGo.(*RectilinearMeasure)
		if !ok {
			t.Skip("not a RectilinearMeasure")
		}

		rectilinearRoundTripTest(t, pdf.V1_7, objGoRL)
	})
}

// TestEmbedValidation tests that Embed validates required fields
func TestEmbedValidation(t *testing.T) {
	tests := []struct {
		name string
		rm   *RectilinearMeasure
	}{
		{
			name: "missing scale ratio",
			rm: &RectilinearMeasure{
				ScaleRatio: "",
				XAxis:      []*NumberFormat{{Unit: "m", ConversionFactor: 1, Precision: 1}},
				Distance:   []*NumberFormat{{Unit: "m", ConversionFactor: 1, Precision: 1}},
				Area:       []*NumberFormat{{Unit: "m²", ConversionFactor: 1, Precision: 1}},
			},
		},
		{
			name: "missing X axis",
			rm: &RectilinearMeasure{
				ScaleRatio: "1:1",
				XAxis:      nil,
				Distance:   []*NumberFormat{{Unit: "m", ConversionFactor: 1, Precision: 1}},
				Area:       []*NumberFormat{{Unit: "m²", ConversionFactor: 1, Precision: 1}},
			},
		},
		{
			name: "empty X axis",
			rm: &RectilinearMeasure{
				ScaleRatio: "1:1",
				XAxis:      []*NumberFormat{},
				Distance:   []*NumberFormat{{Unit: "m", ConversionFactor: 1, Precision: 1}},
				Area:       []*NumberFormat{{Unit: "m²", ConversionFactor: 1, Precision: 1}},
			},
		},
		{
			name: "missing Distance",
			rm: &RectilinearMeasure{
				ScaleRatio: "1:1",
				XAxis:      []*NumberFormat{{Unit: "m", ConversionFactor: 1, Precision: 1}},
				Distance:   nil,
				Area:       []*NumberFormat{{Unit: "m²", ConversionFactor: 1, Precision: 1}},
			},
		},
		{
			name: "missing Area",
			rm: &RectilinearMeasure{
				ScaleRatio: "1:1",
				XAxis:      []*NumberFormat{{Unit: "m", ConversionFactor: 1, Precision: 1}},
				Distance:   []*NumberFormat{{Unit: "m", ConversionFactor: 1, Precision: 1}},
				Area:       nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
			rm := pdf.NewResourceManager(w)

			_, err := rm.Embed(tt.rm)
			if err == nil {
				t.Fatal("expected validation error but got none")
			}
		})
	}
}
