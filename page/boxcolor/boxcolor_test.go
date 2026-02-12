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

package boxcolor

import (
	"bytes"
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

var testCases = []struct {
	name    string
	version pdf.Version
	data    *Info
}{
	{
		name:    "empty",
		version: pdf.V1_7,
		data:    &Info{},
	},
	{
		name:    "crop_box",
		version: pdf.V1_7,
		data: &Info{
			CropBox: &Style{
				Color:     color.DeviceRGB{1, 0, 0},
				LineWidth: 1,
				Style:     StyleSolid,
			},
		},
	},
	{
		name:    "bleed_box",
		version: pdf.V1_7,
		data: &Info{
			BleedBox: &Style{
				Color:     color.DeviceRGB{0, 1, 0},
				LineWidth: 2,
				Style:     StyleSolid,
			},
		},
	},
	{
		name:    "trim_box_dashed",
		version: pdf.V1_7,
		data: &Info{
			TrimBox: &Style{
				LineWidth:   1,
				Style:       StyleDashed,
				DashPattern: defaultDashPattern,
			},
		},
	},
	{
		name:    "art_box_custom_dash",
		version: pdf.V1_7,
		data: &Info{
			ArtBox: &Style{
				Color:       color.DeviceRGB{0, 0, 1},
				LineWidth:   1.5,
				Style:       StyleDashed,
				DashPattern: []float64{4, 2},
			},
		},
	},
	{
		name:    "custom_width",
		version: pdf.V2_0,
		data: &Info{
			CropBox: &Style{
				LineWidth: 2.5,
				Style:     StyleSolid,
			},
		},
	},
	{
		name:    "all_boxes",
		version: pdf.V1_7,
		data: &Info{
			CropBox: &Style{
				Color:     color.DeviceRGB{1, 0, 0},
				LineWidth: 1,
				Style:     StyleSolid,
			},
			BleedBox: &Style{
				Color:     color.DeviceRGB{0, 1, 0},
				LineWidth: 2,
				Style:     StyleSolid,
			},
			TrimBox: &Style{
				Color:       color.DeviceRGB{0, 0, 1},
				LineWidth:   1,
				Style:       StyleDashed,
				DashPattern: defaultDashPattern,
			},
			ArtBox: &Style{
				Color:     color.DeviceRGB{1, 1, 0},
				LineWidth: 0.5,
				Style:     StyleSolid,
			},
		},
	},
	{
		name:    "full_custom_style",
		version: pdf.V1_7,
		data: &Info{
			CropBox: &Style{
				Color:       color.DeviceRGB{0.5, 0.5, 0.5},
				LineWidth:   3,
				Style:       StyleDashed,
				DashPattern: []float64{10, 5},
			},
		},
	},
	{
		name:    "single_use",
		version: pdf.V1_7,
		data: &Info{
			CropBox: &Style{
				Color:       color.DeviceRGB{1, 0, 0},
				LineWidth:   1,
				Style:       StyleDashed,
				DashPattern: defaultDashPattern,
				SingleUse:   true,
			},
			BleedBox: &Style{
				Color:     color.DeviceRGB{0, 0.5, 0},
				LineWidth: 2,
				Style:     StyleSolid,
				SingleUse: true,
			},
			SingleUse: true,
		},
	},
}

var testVersions = []pdf.Version{pdf.V1_7, pdf.V2_0}

func roundTripTest(t *testing.T, version pdf.Version, data *Info) {
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
		t.Fatalf("rm.Close failed: %v", err)
	}

	err = w.Close()
	if err != nil {
		t.Fatalf("w.Close failed: %v", err)
	}

	x := pdf.NewExtractor(w)
	decoded, err := pdf.ExtractorGet(x, embedded, ExtractInfo)
	if err != nil {
		t.Fatalf("extract failed: %v", err)
	}

	if diff := cmp.Diff(data, decoded); diff != "" {
		t.Errorf("round trip failed (-want +got):\n%s", diff)
	}
}

func TestInfoRoundTrip(t *testing.T) {
	for _, v := range testVersions {
		t.Run(v.String(), func(t *testing.T) {
			for _, tc := range testCases {
				t.Run(tc.name, func(t *testing.T) {
					roundTripTest(t, v, tc.data)
				})
			}
		})
	}
}

func FuzzInfoRoundTrip(f *testing.F) {
	opt := &pdf.WriterOptions{
		HumanReadable: true,
	}

	for _, tc := range testCases {
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

		obj := r.GetMeta().Trailer["Quir:E"]
		if obj == nil {
			t.Skip("missing test object")
		}

		x := pdf.NewExtractor(r)
		data, err := pdf.ExtractorGet(x, obj, ExtractInfo)
		if err != nil {
			t.Skip("malformed object")
		}
		if data == nil {
			t.Skip("nil object")
		}

		roundTripTest(t, pdf.GetVersion(r), data)
	})
}

func TestStyleValidation(t *testing.T) {
	buf, _ := memfile.NewPDFWriter(pdf.V1_4, nil)
	rm := pdf.NewResourceManager(buf)

	// negative line width should fail
	style := &Style{
		LineWidth: -1,
	}

	_, err := rm.Embed(style)
	if err == nil {
		t.Error("expected error for negative line width, but got none")
	}

	// solid style with non-nil dash pattern should fail
	style = &Style{
		Style:       StyleSolid,
		DashPattern: []float64{3},
	}
	_, err = rm.Embed(style)
	if err == nil {
		t.Error("expected error for solid style with dash pattern, but got none")
	}
}
