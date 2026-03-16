// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2026  Jochen Voss <voss@seehuhn.de>
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

var geospatialTestCases = []struct {
	name string
	data *GeospatialMeasure
}{
	{
		name: "basic_geographic",
		data: &GeospatialMeasure{
			GCS: &CoordinateSystem{
				CSType:    CoordSysGeographic,
				EPSG:      4326,
				SingleUse: true,
			},
			GPTS:      []float64{48.8566, 2.3522, 48.8606, 2.3370},
			SingleUse: true,
		},
	},
	{
		name: "projected_with_wkt",
		data: &GeospatialMeasure{
			GCS: &CoordinateSystem{
				CSType:    CoordSysProjected,
				WKT:       `PROJCS["WGS 84 / UTM zone 32N"]`,
				SingleUse: true,
			},
			GPTS:      []float64{500000, 5400000, 500100, 5400100},
			Bounds:    []float64{0, 0, 1, 0, 1, 1, 0, 1},
			LPTS:      []float64{500000, 5400000, 500100, 5400100},
			SingleUse: true,
		},
	},
	{
		name: "with_dcs_and_pdu",
		data: &GeospatialMeasure{
			GCS: &CoordinateSystem{
				CSType:    CoordSysGeographic,
				EPSG:      4326,
				SingleUse: true,
			},
			DCS: &CoordinateSystem{
				CSType:    CoordSysProjected,
				EPSG:      32632,
				SingleUse: true,
			},
			GPTS:      []float64{48.0, 2.0, 49.0, 3.0},
			PDU:       [3]pdf.Name{"M", "SQM", "DEG"},
			SingleUse: true,
		},
	},
	{
		name: "with_pcsm",
		data: &GeospatialMeasure{
			GCS: &CoordinateSystem{
				CSType:    CoordSysGeographic,
				EPSG:      4326,
				SingleUse: true,
			},
			GPTS:      []float64{0, 0, 1, 1},
			PCSM:      []float64{1, 0, 0, 0, 1, 0, 0, 0, 1, 0, 0, 0},
			SingleUse: true,
		},
	},
	{
		name: "minimal",
		data: &GeospatialMeasure{
			GCS: &CoordinateSystem{
				CSType:    CoordSysGeographic,
				EPSG:      4326,
				SingleUse: true,
			},
			GPTS:      []float64{51.5074, -0.1278},
			SingleUse: true,
		},
	},
	{
		name: "indirect",
		data: &GeospatialMeasure{
			GCS: &CoordinateSystem{
				CSType:    CoordSysGeographic,
				EPSG:      4326,
				SingleUse: false,
			},
			GPTS:      []float64{40.7128, -74.0060, 34.0522, -118.2437},
			SingleUse: false,
		},
	},
}

func geospatialRoundTripTest(t *testing.T, data *GeospatialMeasure) {
	t.Helper()

	w, _ := memfile.NewPDFWriter(pdf.V2_0, nil)

	rm := pdf.NewResourceManager(w)
	embedded, err := rm.Embed(data)
	if err != nil {
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
	decoded, err := pdf.ExtractorGet(x, nil, embedded, Extract)
	if err != nil {
		t.Fatalf("extract failed: %v", err)
	}

	decodedGEO, ok := decoded.(*GeospatialMeasure)
	if !ok {
		t.Fatalf("extracted measure is not GeospatialMeasure, got %T", decoded)
	}

	if diff := cmp.Diff(data, decodedGEO); diff != "" {
		t.Errorf("round trip failed (-want +got):\n%s", diff)
	}
}

func TestGeospatialSpecificationRoundTrip(t *testing.T) {
	for _, tc := range geospatialTestCases {
		t.Run(tc.name, func(t *testing.T) {
			geospatialRoundTripTest(t, tc.data)
		})
	}
}

func FuzzGeospatialRoundTrip(f *testing.F) {
	opt := &pdf.WriterOptions{
		HumanReadable: true,
	}
	for _, tc := range geospatialTestCases {
		w, buf := memfile.NewPDFWriter(pdf.V2_0, opt)

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
		r, err := pdf.NewReader(bytes.NewReader(fileData), int64(len(fileData)), nil)
		if err != nil {
			t.Skip("invalid PDF")
		}
		objPDF := r.GetMeta().Trailer["Quir:E"]
		if objPDF == nil {
			t.Skip("missing PDF object")
		}

		x := pdf.NewExtractor(r)
		objGo, err := pdf.ExtractorGet(x, nil, objPDF, Extract)
		if err != nil {
			t.Skip("malformed PDF object")
		}

		objGoGEO, ok := objGo.(*GeospatialMeasure)
		if !ok {
			t.Skip("not a GeospatialMeasure")
		}

		geospatialRoundTripTest(t, objGoGEO)
	})
}

func TestGeospatialEmbedValidation(t *testing.T) {
	tests := []struct {
		name string
		gm   *GeospatialMeasure
	}{
		{
			name: "missing GCS",
			gm: &GeospatialMeasure{
				GCS:  nil,
				GPTS: []float64{1, 2},
			},
		},
		{
			name: "empty GPTS",
			gm: &GeospatialMeasure{
				GCS: &CoordinateSystem{
					CSType: CoordSysGeographic,
					EPSG:   4326,
				},
				GPTS: nil,
			},
		},
		{
			name: "LPTS length mismatch",
			gm: &GeospatialMeasure{
				GCS: &CoordinateSystem{
					CSType: CoordSysGeographic,
					EPSG:   4326,
				},
				GPTS: []float64{1, 2, 3, 4},
				LPTS: []float64{0, 0},
			},
		},
		{
			name: "PCSM wrong length",
			gm: &GeospatialMeasure{
				GCS: &CoordinateSystem{
					CSType: CoordSysGeographic,
					EPSG:   4326,
				},
				GPTS: []float64{1, 2},
				PCSM: []float64{1, 2, 3},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
			rm := pdf.NewResourceManager(w)

			_, err := rm.Embed(tt.gm)
			if err == nil {
				t.Fatal("expected validation error but got none")
			}
		})
	}
}
