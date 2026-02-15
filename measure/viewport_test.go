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
	"seehuhn.de/go/geom/vec"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

// viewportTestCases holds representative samples of Viewport objects for testing
var viewportTestCases = []struct {
	name    string
	version pdf.Version
	data    *Viewport
}{
	{
		name:    "minimal viewport",
		version: pdf.V1_6,
		data: &Viewport{
			BBox:      pdf.Rectangle{LLx: 0, LLy: 0, URx: 100, URy: 100},
			SingleUse: false,
		},
	},
	{
		name:    "viewport with name",
		version: pdf.V1_6,
		data: &Viewport{
			BBox:      pdf.Rectangle{LLx: 50, LLy: 75, URx: 300, URy: 400},
			Name:      "Main Drawing Area",
			SingleUse: true,
		},
	},
	{
		name:    "viewport with rectilinear measure",
		version: pdf.V1_7,
		data: &Viewport{
			BBox: pdf.Rectangle{LLx: 10, LLy: 20, URx: 200, URy: 300},
			Name: "Engineering Drawing",
			Measure: &RectilinearMeasure{
				ScaleRatio: "1:100",
				XAxis: []*NumberFormat{{
					Unit:             "m",
					ConversionFactor: 1.0,
					Precision:        100,
					FractionFormat:   FractionDecimal,
					SingleUse:        true,
					DecimalSeparator: ".",
				}},
				Distance: []*NumberFormat{{
					Unit:             "m",
					ConversionFactor: 1.0,
					Precision:        100,
					FractionFormat:   FractionDecimal,
					SingleUse:        true,
					DecimalSeparator: ".",
				}},
				Area: []*NumberFormat{{
					Unit:             "m²",
					ConversionFactor: 1.0,
					Precision:        100,
					FractionFormat:   FractionDecimal,
					SingleUse:        true,
					DecimalSeparator: ".",
				}},
				Origin:    [2]float64{0, 0},
				SingleUse: true,
			},
			SingleUse: false,
		},
	},
	{
		name:    "viewport with PtData (PDF 2.0)",
		version: pdf.V2_0,
		data: &Viewport{
			BBox: pdf.Rectangle{LLx: 0, LLy: 0, URx: 500, URy: 500},
			Name: "Geospatial Region",
			PtData: &PtData{
				Subtype: PtDataSubtypeCloud,
				Names:   []string{PtDataNameLat, PtDataNameLon, PtDataNameAlt},
				XPTS: [][]pdf.Object{
					{pdf.Real(40.7128), pdf.Real(-74.0060), pdf.Real(10.5)},
					{pdf.Real(40.7589), pdf.Real(-73.9851), pdf.Real(15.2)},
					{pdf.Real(40.7614), pdf.Real(-73.9776), pdf.Real(12.8)},
				},
				SingleUse: false,
			},
			SingleUse: true,
		},
	},
	{
		name:    "viewport with all fields",
		version: pdf.V2_0,
		data: &Viewport{
			BBox: pdf.Rectangle{LLx: 100, LLy: 100, URx: 400, URy: 600},
			Name: "Complete Viewport",
			Measure: &RectilinearMeasure{
				ScaleRatio: "1in = 1ft",
				XAxis: []*NumberFormat{{
					Unit:             "ft",
					ConversionFactor: 1.0,
					Precision:        100,
					FractionFormat:   FractionDecimal,
					SingleUse:        true,
					DecimalSeparator: ".",
				}},
				YAxis: []*NumberFormat{{
					Unit:             "ft",
					ConversionFactor: 1.0,
					Precision:        100,
					FractionFormat:   FractionDecimal,
					SingleUse:        true,
					DecimalSeparator: ".",
				}},
				Distance: []*NumberFormat{{
					Unit:             "ft",
					ConversionFactor: 1.0,
					Precision:        100,
					FractionFormat:   FractionDecimal,
					SingleUse:        true,
					DecimalSeparator: ".",
				}},
				Area: []*NumberFormat{{
					Unit:             "ft²",
					ConversionFactor: 1.0,
					Precision:        100,
					FractionFormat:   FractionDecimal,
					SingleUse:        true,
					DecimalSeparator: ".",
				}},
				Origin:    [2]float64{100, 100},
				SingleUse: false,
			},
			PtData: &PtData{
				Subtype:   PtDataSubtypeCloud,
				Names:     []string{PtDataNameLat, PtDataNameLon},
				XPTS:      [][]pdf.Object{{pdf.Real(51.5074), pdf.Real(-0.1278)}},
				SingleUse: true,
			},
			SingleUse: false,
		},
	},
	{
		name:    "viewport with Y axis different from X",
		version: pdf.V1_7,
		data: &Viewport{
			BBox: pdf.Rectangle{LLx: 0, LLy: 0, URx: 100, URy: 200},
			Name: "Different axes",
			Measure: &RectilinearMeasure{
				ScaleRatio: "in X 1cm = 1m, in Y 1cm = 30m",
				XAxis: []*NumberFormat{{
					Unit:             "m",
					ConversionFactor: 1.0,
					Precision:        100,
					FractionFormat:   FractionDecimal,
					SingleUse:        true,
					DecimalSeparator: ".",
				}},
				YAxis: []*NumberFormat{{
					Unit:             "m",
					ConversionFactor: 30.0,
					Precision:        100,
					FractionFormat:   FractionDecimal,
					SingleUse:        true,
					DecimalSeparator: ".",
				}},
				Distance: []*NumberFormat{{
					Unit:             "m",
					ConversionFactor: 1.0,
					Precision:        100,
					FractionFormat:   FractionDecimal,
					SingleUse:        true,
					DecimalSeparator: ".",
				}},
				Area: []*NumberFormat{{
					Unit:             "m²",
					ConversionFactor: 1.0,
					Precision:        100,
					FractionFormat:   FractionDecimal,
					SingleUse:        true,
					DecimalSeparator: ".",
				}},
				Origin:    [2]float64{0, 0},
				CYX:       30.0,
				SingleUse: true,
			},
			SingleUse: false,
		},
	},
}

// viewportRoundTripTest performs a read-write-read cycle and verifies the result
func viewportRoundTripTest(t *testing.T, version pdf.Version, vp *Viewport) {
	t.Helper()

	w, _ := memfile.NewPDFWriter(version, nil)
	rm := pdf.NewResourceManager(w)

	// Write the viewport
	embedded, err := rm.Embed(vp)
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

	// Read it back
	x := pdf.NewExtractor(w)
	extracted, err := pdf.ExtractorGet(x, embedded, ExtractViewport)
	if err != nil {
		t.Fatalf("extract failed: %v", err)
	}

	if diff := cmp.Diff(vp, extracted); diff != "" {
		t.Errorf("round trip failed (-want +got):\n%s", diff)
	}
}

// viewportArrayTestCases holds representative samples of ViewPortArray objects for testing
var viewportArrayTestCases = []struct {
	name    string
	version pdf.Version
	data    *ViewPortArray
}{
	{
		name:    "empty array",
		version: pdf.V1_6,
		data:    &ViewPortArray{Viewports: []*Viewport{}},
	},
	{
		name:    "single viewport",
		version: pdf.V1_7,
		data: &ViewPortArray{
			Viewports: []*Viewport{
				{
					BBox:      pdf.Rectangle{LLx: 0, LLy: 0, URx: 100, URy: 100},
					Name:      "Single",
					SingleUse: true,
				},
			},
		},
	},
	{
		name:    "multiple viewports",
		version: pdf.V2_0,
		data: &ViewPortArray{
			Viewports: []*Viewport{
				{
					BBox:      pdf.Rectangle{LLx: 0, LLy: 0, URx: 50, URy: 50},
					Name:      "First",
					SingleUse: true,
				},
				{
					BBox:      pdf.Rectangle{LLx: 50, LLy: 50, URx: 100, URy: 100},
					Name:      "Second",
					SingleUse: false,
				},
				{
					BBox:      pdf.Rectangle{LLx: 100, LLy: 100, URx: 200, URy: 200},
					Name:      "Third",
					SingleUse: true,
				},
			},
		},
	},
}

// TestViewportRoundTrip tests round-trip behavior for all test cases
func TestViewportRoundTrip(t *testing.T) {
	for _, tc := range viewportTestCases {
		t.Run(tc.name, func(t *testing.T) {
			viewportRoundTripTest(t, tc.version, tc.data)
		})
	}
}

// FuzzViewportRoundTrip implements fuzzing for viewport round-trips
func FuzzViewportRoundTrip(f *testing.F) {
	opt := &pdf.WriterOptions{
		HumanReadable: true,
	}

	// Add test cases as seed corpus
	for _, tc := range viewportTestCases {
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
			t.Skip("missing test object")
		}

		x := pdf.NewExtractor(r)
		vp, err := pdf.ExtractorGet(x, objPDF, ExtractViewport)
		if err != nil {
			t.Skip("malformed viewport")
		}

		// Use the reader's version for round-trip
		version := pdf.GetVersion(r)
		viewportRoundTripTest(t, version, vp)
	})
}

// viewportArrayRoundTripTest performs a write-read cycle for ViewPortArray
func viewportArrayRoundTripTest(t *testing.T, version pdf.Version, data *ViewPortArray) {
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
	extracted, err := pdf.ExtractorGet(x, embedded, ExtractViewportArray)
	if err != nil {
		t.Fatalf("extract failed: %v", err)
	}

	if diff := cmp.Diff(data, extracted); diff != "" {
		t.Errorf("round trip failed (-want +got):\n%s", diff)
	}
}

// FuzzViewPortArrayRoundTrip implements fuzzing for viewport array round-trips
func FuzzViewPortArrayRoundTrip(f *testing.F) {
	opt := &pdf.WriterOptions{
		HumanReadable: true,
	}

	// build seed corpus from test cases
	for _, tc := range viewportArrayTestCases {
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
		data, err := pdf.ExtractorGet(x, obj, ExtractViewportArray)
		if err != nil {
			t.Skip("malformed object")
		}
		if data == nil {
			t.Skip("no data")
		}

		viewportArrayRoundTripTest(t, pdf.GetVersion(r), data)
	})
}

func TestRectangleContains(t *testing.T) {
	rect := &pdf.Rectangle{
		LLx: 10, LLy: 20,
		URx: 100, URy: 200,
	}

	tests := []struct {
		name     string
		point    vec.Vec2
		expected bool
	}{
		{"point inside", vec.Vec2{X: 50, Y: 100}, true},
		{"point on left edge", vec.Vec2{X: 10, Y: 100}, true},
		{"point on right edge", vec.Vec2{X: 100, Y: 100}, true},
		{"point on bottom edge", vec.Vec2{X: 50, Y: 20}, true},
		{"point on top edge", vec.Vec2{X: 50, Y: 200}, true},
		{"point at lower-left corner", vec.Vec2{X: 10, Y: 20}, true},
		{"point at upper-right corner", vec.Vec2{X: 100, Y: 200}, true},
		{"point left of rectangle", vec.Vec2{X: 5, Y: 100}, false},
		{"point right of rectangle", vec.Vec2{X: 105, Y: 100}, false},
		{"point below rectangle", vec.Vec2{X: 50, Y: 15}, false},
		{"point above rectangle", vec.Vec2{X: 50, Y: 205}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if result := rect.Contains(tt.point); result != tt.expected {
				t.Errorf("Contains(%v) = %v, want %v", tt.point, result, tt.expected)
			}
		})
	}
}

// TestViewportVersionRequirements tests PDF version checking
func TestViewportVersionRequirements(t *testing.T) {
	vp := &Viewport{
		BBox:      pdf.Rectangle{LLx: 0, LLy: 0, URx: 100, URy: 100},
		SingleUse: true,
	}

	// Test with PDF 1.5 (should fail - viewports require 1.6+)
	w15, _ := memfile.NewPDFWriter(pdf.V1_5, nil)
	rm15 := pdf.NewResourceManager(w15)
	_, err := rm15.Embed(vp)
	if err == nil {
		t.Error("expected version check error for PDF 1.5")
	}

	// Test with PDF 1.6 (should succeed)
	w16, _ := memfile.NewPDFWriter(pdf.V1_6, nil)
	rm16 := pdf.NewResourceManager(w16)
	_, err = rm16.Embed(vp)
	if err != nil {
		t.Errorf("unexpected error for PDF 1.6: %v", err)
	}

	// Test PtData PDF 2.0 requirement
	vpWithPtData := &Viewport{
		BBox: pdf.Rectangle{LLx: 0, LLy: 0, URx: 100, URy: 100},
		PtData: &PtData{
			Subtype:   PtDataSubtypeCloud,
			Names:     []string{PtDataNameLat, PtDataNameLon},
			XPTS:      [][]pdf.Object{{pdf.Number(0), pdf.Number(0)}},
			SingleUse: true,
		},
		SingleUse: true,
	}

	// Should fail with PDF 1.7
	w17, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	rm17 := pdf.NewResourceManager(w17)
	_, err = rm17.Embed(vpWithPtData)
	if err == nil {
		t.Error("expected version check error for PtData with PDF 1.7")
	}

	// Should succeed with PDF 2.0
	w20, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	rm20 := pdf.NewResourceManager(w20)
	_, err = rm20.Embed(vpWithPtData)
	if err != nil {
		t.Errorf("unexpected error for PtData with PDF 2.0: %v", err)
	}
}

// TestViewportSingleUse verifies SingleUse field behavior
func TestViewportSingleUse(t *testing.T) {
	vp := &Viewport{
		BBox:      pdf.Rectangle{LLx: 0, LLy: 0, URx: 100, URy: 100},
		SingleUse: true,
	}

	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	rm := pdf.NewResourceManager(w)

	// Embed with SingleUse = true
	embedded, err := rm.Embed(vp)
	if err != nil {
		t.Fatalf("embed failed: %v", err)
	}

	// Should return a dictionary directly, not a reference
	if _, ok := embedded.(pdf.Dict); !ok {
		t.Errorf("SingleUse=true should return Dict, got %T", embedded)
	}

	// Test with SingleUse = false (create new viewport to avoid caching)
	vp2 := &Viewport{
		BBox:      pdf.Rectangle{LLx: 0, LLy: 0, URx: 100, URy: 100},
		SingleUse: false,
	}
	embedded2, err := rm.Embed(vp2)
	if err != nil {
		t.Fatalf("embed failed: %v", err)
	}

	// Should return a reference
	if _, ok := embedded2.(pdf.Reference); !ok {
		t.Errorf("SingleUse=false should return Reference, got %T", embedded2)
	}
}

// TestExtractViewportMalformed tests extraction with malformed data
func TestExtractViewportMalformed(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)

	tests := []struct {
		name      string
		obj       pdf.Object
		shouldErr bool
	}{
		{
			name:      "not a dictionary",
			obj:       pdf.String("invalid"),
			shouldErr: true,
		},
		{
			name: "missing BBox",
			obj: pdf.Dict{
				"Type": pdf.Name("Viewport"),
				"Name": pdf.String("Test"),
			},
			shouldErr: true,
		},
		{
			name: "invalid BBox",
			obj: pdf.Dict{
				"Type": pdf.Name("Viewport"),
				"BBox": pdf.String("invalid bbox"),
			},
			shouldErr: true,
		},
		{
			name: "malformed measure (ignored)",
			obj: pdf.Dict{
				"Type":    pdf.Name("Viewport"),
				"BBox":    pdf.Array{pdf.Number(0), pdf.Number(0), pdf.Number(100), pdf.Number(100)},
				"Measure": pdf.String("invalid measure"),
			},
			shouldErr: false, // Malformed optional fields are ignored
		},
		{
			name: "malformed name (ignored)",
			obj: pdf.Dict{
				"Type": pdf.Name("Viewport"),
				"BBox": pdf.Array{pdf.Number(10), pdf.Number(20), pdf.Number(30), pdf.Number(40)},
				"Name": pdf.Integer(123), // Should be string
			},
			shouldErr: false, // Malformed optional fields are ignored
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x := pdf.NewExtractor(w)
			vp, err := pdf.ExtractorGet(x, tt.obj, ExtractViewport)
			if tt.shouldErr {
				if err == nil {
					t.Error("expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if vp == nil {
					t.Error("expected viewport but got nil")
				}
			}
		})
	}
}

// TestViewPortArraySelect tests the Select method for finding viewports
func TestViewPortArraySelect(t *testing.T) {
	viewports := &ViewPortArray{
		Viewports: []*Viewport{
			{
				BBox: pdf.Rectangle{LLx: 0, LLy: 0, URx: 100, URy: 100},
				Name: "First",
			},
			{
				BBox: pdf.Rectangle{LLx: 50, LLy: 50, URx: 150, URy: 150},
				Name: "Second",
			},
			{
				BBox: pdf.Rectangle{LLx: 25, LLy: 25, URx: 75, URy: 75},
				Name: "Third",
			},
		},
	}

	tests := []struct {
		name     string
		point    vec.Vec2
		expected *Viewport
	}{
		{
			name:     "point in first viewport only",
			point:    vec.Vec2{X: 10, Y: 10},
			expected: viewports.Viewports[0],
		},
		{
			name:     "point in overlapping area - should return last",
			point:    vec.Vec2{X: 60, Y: 60},
			expected: viewports.Viewports[2], // Third is last in reverse order
		},
		{
			name:     "point in second and third - should return third",
			point:    vec.Vec2{X: 70, Y: 70},
			expected: viewports.Viewports[2],
		},
		{
			name:     "point not in any viewport",
			point:    vec.Vec2{X: 200, Y: 200},
			expected: nil,
		},
		{
			name:     "point on boundary",
			point:    vec.Vec2{X: 75, Y: 75},
			expected: viewports.Viewports[2],
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := viewports.Select(tt.point)
			if result != tt.expected {
				var resultName, expectedName string
				if result != nil {
					resultName = result.Name
				}
				if tt.expected != nil {
					expectedName = tt.expected.Name
				}
				t.Errorf("Select(%v) = %q, want %q", tt.point, resultName, expectedName)
			}
		})
	}
}

// TestViewPortArrayRoundTrip tests array-specific round-trip behavior
func TestViewPortArrayRoundTrip(t *testing.T) {
	for _, tc := range viewportArrayTestCases {
		t.Run(tc.name, func(t *testing.T) {
			viewportArrayRoundTripTest(t, tc.version, tc.data)
		})
	}
}

// TestViewPortArraySelectEmptyArray tests Select with empty array
func TestViewPortArraySelectEmptyArray(t *testing.T) {
	viewports := &ViewPortArray{}
	result := viewports.Select(vec.Vec2{X: 50, Y: 50})
	if result != nil {
		t.Error("Select with empty array should return nil")
	}
}
