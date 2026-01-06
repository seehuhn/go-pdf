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
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

func TestStyleRoundTrip(t *testing.T) {
	tests := []struct {
		name  string
		style *Style
	}{
		{
			name:  "defaults",
			style: &Style{},
		},
		{
			name: "custom_color",
			style: &Style{
				Color: color.DeviceRGB{1, 0, 0},
			},
		},
		{
			name: "custom_width",
			style: &Style{
				LineWidth: 2.5,
			},
		},
		{
			name: "dashed",
			style: &Style{
				Style: StyleDashed,
			},
		},
		{
			name: "dashed_custom_pattern",
			style: &Style{
				Style:       StyleDashed,
				DashPattern: []float64{5, 2, 1, 2},
			},
		},
		{
			name: "full_custom",
			style: &Style{
				Color:       color.DeviceRGB{0.5, 0.5, 0.5},
				LineWidth:   3,
				Style:       StyleDashed,
				DashPattern: []float64{10, 5},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testStyleRoundTrip(t, tt.style)
		})
	}
}

func testStyleRoundTrip(t *testing.T, original *Style) {
	buf, _ := memfile.NewPDFWriter(pdf.V1_4, nil)
	rm := pdf.NewResourceManager(buf)

	obj, err := rm.Embed(original)
	if err != nil {
		t.Fatalf("embed: %v", err)
	}

	err = rm.Close()
	if err != nil {
		t.Fatalf("close writer: %v", err)
	}

	extractor := pdf.NewExtractor(buf)
	extracted, err := ExtractStyle(extractor, obj)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}

	normalizeStyle(original)
	normalizeStyle(extracted)

	if diff := cmp.Diff(original, extracted); diff != "" {
		t.Errorf("round trip failed (-want +got):\n%s", diff)
	}
}

func normalizeStyle(s *Style) {
	// apply defaults
	if s.Color == (color.DeviceRGB{}) {
		s.Color = color.DeviceRGB{0, 0, 0}
	}
	if s.LineWidth == 0 {
		s.LineWidth = 1
	}
	if s.Style == "" {
		s.Style = StyleSolid
	}
	// dash pattern defaults when dashed
	if s.Style == StyleDashed && len(s.DashPattern) == 0 {
		s.DashPattern = []float64{3}
	}
	// dash pattern not written for solid style
	if s.Style == StyleSolid {
		s.DashPattern = nil
	}
}

func TestInfoRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		info *Info
	}{
		{
			name: "empty",
			info: &Info{},
		},
		{
			name: "crop_box",
			info: &Info{
				CropBox: &Style{
					Color: color.DeviceRGB{1, 0, 0},
				},
			},
		},
		{
			name: "bleed_box",
			info: &Info{
				BleedBox: &Style{
					Color:     color.DeviceRGB{0, 1, 0},
					LineWidth: 2,
				},
			},
		},
		{
			name: "trim_box",
			info: &Info{
				TrimBox: &Style{
					Style: StyleDashed,
				},
			},
		},
		{
			name: "art_box",
			info: &Info{
				ArtBox: &Style{
					Color:       color.DeviceRGB{0, 0, 1},
					LineWidth:   1.5,
					Style:       StyleDashed,
					DashPattern: []float64{4, 2},
				},
			},
		},
		{
			name: "all_boxes",
			info: &Info{
				CropBox: &Style{
					Color: color.DeviceRGB{1, 0, 0},
				},
				BleedBox: &Style{
					Color:     color.DeviceRGB{0, 1, 0},
					LineWidth: 2,
				},
				TrimBox: &Style{
					Color: color.DeviceRGB{0, 0, 1},
					Style: StyleDashed,
				},
				ArtBox: &Style{
					Color:     color.DeviceRGB{1, 1, 0},
					LineWidth: 0.5,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testInfoRoundTrip(t, tt.info)
		})
	}
}

func testInfoRoundTrip(t *testing.T, original *Info) {
	buf, _ := memfile.NewPDFWriter(pdf.V1_4, nil)
	rm := pdf.NewResourceManager(buf)

	obj, err := rm.Embed(original)
	if err != nil {
		t.Fatalf("embed: %v", err)
	}

	err = rm.Close()
	if err != nil {
		t.Fatalf("close writer: %v", err)
	}

	extractor := pdf.NewExtractor(buf)
	extracted, err := ExtractInfo(extractor, obj)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}

	normalizeInfo(original)
	normalizeInfo(extracted)

	if diff := cmp.Diff(original, extracted); diff != "" {
		t.Errorf("round trip failed (-want +got):\n%s", diff)
	}
}

func normalizeInfo(info *Info) {
	if info.CropBox != nil {
		normalizeStyle(info.CropBox)
	}
	if info.BleedBox != nil {
		normalizeStyle(info.BleedBox)
	}
	if info.TrimBox != nil {
		normalizeStyle(info.TrimBox)
	}
	if info.ArtBox != nil {
		normalizeStyle(info.ArtBox)
	}
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
}
