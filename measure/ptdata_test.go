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

var testCases = []struct {
	name    string
	version pdf.Version
	data    *PtData
}{
	{
		name:    "complete_cloud_data",
		version: pdf.V2_0,
		data: &PtData{
			Subtype: PtDataSubtypeCloud,
			Names:   []string{PtDataNameLat, PtDataNameLon, PtDataNameAlt, "temperature", "sensor_id"},
			XPTS: [][]pdf.Object{
				{pdf.Number(40.7128), pdf.Number(-74.0060), pdf.Number(10.5), pdf.Number(22.3), pdf.String("NYC001")},
				{pdf.Number(40.7589), pdf.Number(-73.9851), pdf.Number(15.2), pdf.Number(21.8), pdf.String("NYC002")},
				{pdf.Number(40.7831), pdf.Number(-73.9712), pdf.Number(12.1), pdf.Number(23.1), pdf.String("NYC003")},
			},
			SingleUse: true,
		},
	},
	{
		name:    "minimal_cloud_data",
		version: pdf.V2_0,
		data: &PtData{
			Subtype:   PtDataSubtypeCloud,
			Names:     []string{PtDataNameLat, PtDataNameLon},
			XPTS:      [][]pdf.Object{},
			SingleUse: true,
		},
	},
	{
		name:    "indirect_object",
		version: pdf.V2_0,
		data: &PtData{
			Subtype: PtDataSubtypeCloud,
			Names:   []string{PtDataNameLat, PtDataNameLon},
			XPTS: [][]pdf.Object{
				{pdf.Real(40.7128), pdf.Real(-74.0060)},
			},
			SingleUse: false,
		},
	},
	{
		name:    "custom_names",
		version: pdf.V2_0,
		data: &PtData{
			Subtype: PtDataSubtypeCloud,
			Names:   []string{"pressure", "velocity", "direction"},
			XPTS: [][]pdf.Object{
				{pdf.Number(1013.25), pdf.Number(15.2), pdf.Number(180)},
				{pdf.Number(1012.8), pdf.Number(12.1), pdf.Number(175)},
			},
			SingleUse: true,
		},
	},
	{
		name:    "mixed_object_types",
		version: pdf.V2_0,
		data: &PtData{
			Subtype: PtDataSubtypeCloud,
			Names:   []string{PtDataNameLat, "status", "count"},
			XPTS: [][]pdf.Object{
				{pdf.Real(37.7749), pdf.Name("active"), pdf.Integer(42)},
				{pdf.Real(40.7589), pdf.Name("inactive"), pdf.Integer(15)},
			},
			SingleUse: false,
		},
	},
}

func roundTripTest(t *testing.T, version pdf.Version, data *PtData) {
	t.Helper()

	w, _ := memfile.NewPDFWriter(version, nil)
	rm := pdf.NewResourceManager(w)

	// Embed the object
	embedded, _, err := pdf.ResourceManagerEmbed[pdf.Unused](rm, data)
	if err != nil {
		t.Fatalf("embed failed: %v", err)
	}

	// Extract the object
	extracted, err := ExtractPtData(w, embedded)
	if err != nil {
		t.Fatalf("extract failed: %v", err)
	}

	// SingleUse is not stored in PDF, so reset it for comparison
	extracted.SingleUse = data.SingleUse

	if diff := cmp.Diff(data, extracted, cmp.AllowUnexported(PtData{})); diff != "" {
		t.Errorf("round trip failed (-want +got):\n%s", diff)
	}
}

func TestSpecificationRoundTrip(t *testing.T) {
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			roundTripTest(t, tc.version, tc.data)
		})
	}
}

func FuzzRoundTrip(f *testing.F) {
	opt := &pdf.WriterOptions{
		HumanReadable: true,
	}
	for _, tc := range testCases {
		w, buf := memfile.NewPDFWriter(tc.version, opt)

		rm := pdf.NewResourceManager(w)
		embedded, _, err := pdf.ResourceManagerEmbed[pdf.Unused](rm, tc.data)
		if err != nil {
			continue
		}
		err = rm.Close()
		if err != nil {
			continue
		}

		w.GetMeta().Trailer["PtData"] = embedded
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
		objPDF := r.GetMeta().Trailer["PtData"]
		if objPDF == nil {
			t.Skip("missing PDF object")
		}

		objGo, err := ExtractPtData(r, objPDF)
		if err != nil {
			t.Skip("malformed PDF object")
		}

		roundTripTest(t, r.GetMeta().Version, objGo)
	})
}
