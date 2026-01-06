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
				}},
				Distance: []*NumberFormat{{
					Unit:             "m",
					ConversionFactor: 1.0,
					Precision:        100,
					FractionFormat:   FractionDecimal,
					SingleUse:        true,
				}},
				Area: []*NumberFormat{{
					Unit:             "m²",
					ConversionFactor: 1.0,
					Precision:        100,
					FractionFormat:   FractionDecimal,
					SingleUse:        true,
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
				}},
				YAxis: []*NumberFormat{{
					Unit:             "ft",
					ConversionFactor: 1.0,
					Precision:        100,
					FractionFormat:   FractionDecimal,
					SingleUse:        true,
				}},
				Distance: []*NumberFormat{{
					Unit:             "ft",
					ConversionFactor: 1.0,
					Precision:        100,
					FractionFormat:   FractionDecimal,
					SingleUse:        true,
				}},
				Area: []*NumberFormat{{
					Unit:             "ft²",
					ConversionFactor: 1.0,
					Precision:        100,
					FractionFormat:   FractionDecimal,
					SingleUse:        true,
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
				}},
				YAxis: []*NumberFormat{{
					Unit:             "m",
					ConversionFactor: 30.0,
					Precision:        100,
					FractionFormat:   FractionDecimal,
					SingleUse:        true,
				}},
				Distance: []*NumberFormat{{
					Unit:             "m",
					ConversionFactor: 1.0,
					Precision:        100,
					FractionFormat:   FractionDecimal,
					SingleUse:        true,
				}},
				Area: []*NumberFormat{{
					Unit:             "m²",
					ConversionFactor: 1.0,
					Precision:        100,
					FractionFormat:   FractionDecimal,
					SingleUse:        true,
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
		t.Fatalf("embed failed: %v", err)
	}

	err = rm.Close()
	if err != nil {
		t.Fatalf("resource manager close failed: %v", err)
	}

	// Read it back
	extracted, err := ExtractViewport(w, embedded)
	if err != nil {
		t.Fatalf("extract failed: %v", err)
	}

	// Fix SingleUse fields for comparison (not stored in PDF)
	fixSingleUse(extracted, vp)

	if diff := cmp.Diff(vp, extracted); diff != "" {
		t.Errorf("round trip failed (-want +got):\n%s", diff)
	}
}

// fixSingleUse adjusts SingleUse fields that are not preserved in PDF
func fixSingleUse(extracted, original *Viewport) {
	extracted.SingleUse = original.SingleUse

	if extracted.Measure != nil && original.Measure != nil {
		if extractedRM, ok := extracted.Measure.(*RectilinearMeasure); ok {
			if originalRM, ok := original.Measure.(*RectilinearMeasure); ok {
				extractedRM.SingleUse = originalRM.SingleUse
				fixNumberFormats(extractedRM, originalRM)
			}
		}
	}

	if extracted.PtData != nil && original.PtData != nil {
		extracted.PtData.SingleUse = original.PtData.SingleUse
	}
}

func fixNumberFormats(extracted, original *RectilinearMeasure) {
	fixNumberFormatArray(extracted.XAxis, original.XAxis)
	fixNumberFormatArray(extracted.YAxis, original.YAxis)
	fixNumberFormatArray(extracted.Distance, original.Distance)
	fixNumberFormatArray(extracted.Area, original.Area)
	fixNumberFormatArray(extracted.Angle, original.Angle)
	fixNumberFormatArray(extracted.Slope, original.Slope)
}

func fixNumberFormatArray(extracted, original []*NumberFormat) {
	for i, nf := range extracted {
		if i < len(original) {
			nf.SingleUse = original[i].SingleUse
			// Empty DecimalSeparator becomes default after round-trip
			if original[i].DecimalSeparator == "" {
				nf.DecimalSeparator = ""
			}
		}
	}
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
		rm := pdf.NewResourceManager(w)

		err := memfile.AddBlankPage(w)
		if err != nil {
			continue
		}

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

		vp, err := ExtractViewport(r, objPDF)
		if err != nil {
			t.Skip("malformed viewport")
		}

		// Use the reader's version for round-trip
		version := pdf.GetVersion(r)
		viewportRoundTripTest(t, version, vp)
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
			vp, err := ExtractViewport(w, tt.obj)
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

// TestViewPortArrayEmbed tests embedding of viewport arrays
func TestViewPortArrayEmbed(t *testing.T) {
	viewports := &ViewPortArray{
		Viewports: []*Viewport{
			{
				BBox:      pdf.Rectangle{LLx: 0, LLy: 0, URx: 100, URy: 100},
				Name:      "First",
				SingleUse: true,
			},
			{
				BBox:      pdf.Rectangle{LLx: 100, LLy: 100, URx: 200, URy: 200},
				Name:      "Second",
				SingleUse: false,
			},
		},
		SingleUse: true,
	}

	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	rm := pdf.NewResourceManager(w)

	// Embed array
	embedded, err := rm.Embed(viewports)
	if err != nil {
		t.Fatalf("Embed failed: %v", err)
	}

	arr, ok := embedded.(pdf.Array)
	if !ok {
		t.Fatalf("Expected pdf.Array, got %T", embedded)
	}

	if len(arr) != 2 {
		t.Fatalf("expected 2 elements in array, got %d", len(arr))
	}

	// Extract back to verify
	extracted, err := ExtractViewportArray(w, arr)
	if err != nil {
		t.Fatalf("ExtractViewportArray failed: %v", err)
	}

	// Fix SingleUse for comparison (SingleUse is write-only, not stored in PDF)
	extracted.Viewports[0].SingleUse = true
	extracted.Viewports[1].SingleUse = false
	extracted.SingleUse = true

	if diff := cmp.Diff(viewports, extracted); diff != "" {
		t.Errorf("array round trip failed (-want +got):\n%s", diff)
	}
}

// TestViewPortArrayExtract tests extraction of viewport arrays
func TestViewPortArrayExtract(t *testing.T) {
	// Create test viewports
	vp1 := &Viewport{
		BBox:      pdf.Rectangle{LLx: 0, LLy: 0, URx: 100, URy: 100},
		Name:      "First",
		SingleUse: true,
	}
	vp2 := &Viewport{
		BBox:      pdf.Rectangle{LLx: 100, LLy: 100, URx: 200, URy: 200},
		Name:      "Second",
		SingleUse: true,
	}

	// Create a test PDF writer
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	res := pdf.NewResourceManager(w)

	// Embed viewports
	embedded1, err := res.Embed(vp1)
	if err != nil {
		t.Fatalf("embed vp1 failed: %v", err)
	}
	embedded2, err := res.Embed(vp2)
	if err != nil {
		t.Fatalf("embed vp2 failed: %v", err)
	}

	// Create array
	arr := pdf.Array{embedded1, embedded2}

	// Extract array
	extracted, err := ExtractViewportArray(w, arr)
	if err != nil {
		t.Fatalf("ExtractViewportArray failed: %v", err)
	}

	if len(extracted.Viewports) != 2 {
		t.Fatalf("expected 2 viewports, got %d", len(extracted.Viewports))
	}

	// Fix SingleUse for comparison
	extracted.Viewports[0].SingleUse = true
	extracted.Viewports[1].SingleUse = true

	if extracted.Viewports[0].Name != "First" {
		t.Errorf("first viewport name = %q, want %q", extracted.Viewports[0].Name, "First")
	}
	if extracted.Viewports[1].Name != "Second" {
		t.Errorf("second viewport name = %q, want %q", extracted.Viewports[1].Name, "Second")
	}
}

// TestViewPortArrayRoundTrip tests array-specific round-trip behavior
func TestViewPortArrayRoundTrip(t *testing.T) {
	testArrays := []struct {
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

	for _, tc := range testArrays {
		t.Run(tc.name, func(t *testing.T) {
			w, _ := memfile.NewPDFWriter(tc.version, nil)
			rm := pdf.NewResourceManager(w)

			// Embed the array
			embedded, err := rm.Embed(tc.data)
			if err != nil {
				t.Fatalf("embed failed: %v", err)
			}

			err = rm.Close()
			if err != nil {
				t.Fatalf("resource manager close failed: %v", err)
			}

			extracted, err := ExtractViewportArray(w, embedded)
			if err != nil {
				t.Fatalf("extract failed: %v", err)
			}

			// Fix SingleUse for comparison
			for i := range extracted.Viewports {
				if i < len(tc.data.Viewports) {
					extracted.Viewports[i].SingleUse = tc.data.Viewports[i].SingleUse
				}
			}

			if diff := cmp.Diff(tc.data, extracted); diff != "" {
				t.Errorf("round trip failed (-want +got):\n%s", diff)
			}
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
