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
				DecimalSeparator: ".",
			}},
			YAxis: []*NumberFormat{{
				Unit:             "ft",
				ConversionFactor: 5280,
				Precision:        1,
				FractionFormat:   FractionDecimal,
				SingleUse:        true,
				DecimalSeparator: ".",
			}},
			Distance: []*NumberFormat{{
				Unit:             "mi",
				ConversionFactor: 1.0,
				Precision:        100000,
				FractionFormat:   FractionDecimal,
				SingleUse:        true,
				DecimalSeparator: ".",
			}, {
				Unit:             "ft",
				ConversionFactor: 5280,
				Precision:        1,
				FractionFormat:   FractionDecimal,
				SingleUse:        true,
				DecimalSeparator: ".",
			}, {
				Unit:             "in",
				ConversionFactor: 12,
				Precision:        8,
				FractionFormat:   FractionFraction,
				SingleUse:        true,
				DecimalSeparator: ".",
			}},
			Area: []*NumberFormat{{
				Unit:             "acres",
				ConversionFactor: 640,
				Precision:        100,
				SingleUse:        true,
				DecimalSeparator: ".",
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
				DecimalSeparator: ".",
			}},
			YAxis: []*NumberFormat{{
				Unit:             "m",
				ConversionFactor: 1.0,
				Precision:        100,
				SingleUse:        true,
				DecimalSeparator: ".",
			}},
			Distance: []*NumberFormat{{
				Unit:             "m",
				ConversionFactor: 1.0,
				Precision:        100,
				SingleUse:        true,
				DecimalSeparator: ".",
			}},
			Area: []*NumberFormat{{
				Unit:             "m²",
				ConversionFactor: 1.0,
				Precision:        100,
				SingleUse:        true,
				DecimalSeparator: ".",
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
				DecimalSeparator: ".",
			}},
			YAxis: []*NumberFormat{{
				Unit:             "m",
				ConversionFactor: 100.0,
				Precision:        10,
				SingleUse:        true,
				DecimalSeparator: ".",
			}},
			Distance: []*NumberFormat{{
				Unit:             "m",
				ConversionFactor: 100.0,
				Precision:        10,
				SingleUse:        true,
				DecimalSeparator: ".",
			}},
			Area: []*NumberFormat{{
				Unit:             "m²",
				ConversionFactor: 10000.0,
				Precision:        100,
				SingleUse:        true,
				DecimalSeparator: ".",
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
				DecimalSeparator: ".",
			}},
			YAxis: []*NumberFormat{{
				Unit:             "mm",
				ConversionFactor: 1.0,
				Precision:        1,
				SingleUse:        true,
				DecimalSeparator: ".",
			}},
			Distance: []*NumberFormat{{
				Unit:             "mm",
				ConversionFactor: 1.0,
				Precision:        1,
				SingleUse:        true,
				DecimalSeparator: ".",
			}},
			Area: []*NumberFormat{{
				Unit:             "mm²",
				ConversionFactor: 1.0,
				Precision:        1,
				SingleUse:        true,
				DecimalSeparator: ".",
			}},
			Angle: []*NumberFormat{{
				Unit:             "deg",
				ConversionFactor: 1.0,
				Precision:        10,
				SingleUse:        true,
				DecimalSeparator: ".",
			}},
			Slope: []*NumberFormat{{
				Unit:             "%",
				ConversionFactor: 100.0,
				Precision:        10,
				SingleUse:        true,
				DecimalSeparator: ".",
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
		if pdf.IsWrongVersion(err) {
			t.Skip("version not supported")
		}
		t.Fatalf("embed failed: %v", err)
	}

	err = rm.Close()
	if err != nil {
		t.Fatalf("resource manager close failed: %v", err)
	}

	err = w.Close()
	if err != nil {
		t.Fatalf("w.Close failed: %v", err)
	}

	x := pdf.NewExtractor(w)
	decoded, err := pdf.ExtractorGet(x, embedded, Extract)
	if err != nil {
		t.Fatalf("extract failed: %v", err)
	}

	// Type assertion to RectilinearMeasure
	decodedRL, ok := decoded.(*RectilinearMeasure)
	if !ok {
		t.Fatalf("extracted measure is not RectilinearMeasure, got %T", decoded)
	}

	if diff := cmp.Diff(data, decodedRL); diff != "" {
		t.Errorf("round trip failed (-want +got):\n%s", diff)
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

		err := memfile.AddBlankPage(w)
		if err != nil {
			continue
		}

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

		x := pdf.NewExtractor(r)
		objGo, err := pdf.ExtractorGet(x, objPDF, Extract)
		if err != nil {
			t.Skip("malformed PDF object")
		}

		objGoRL, ok := objGo.(*RectilinearMeasure)
		if !ok {
			t.Skip("not a RectilinearMeasure")
		}

		rectilinearRoundTripTest(t, pdf.GetVersion(r), objGoRL)
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
