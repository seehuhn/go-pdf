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
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/geom/vec"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

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

func TestViewportExtractEmbed(t *testing.T) {
	// Create a test measure
	testMeasure := &RectilinearMeasure{
		ScaleRatio: "1:100",
		XAxis: []*NumberFormat{{
			Unit:             "m",
			ConversionFactor: 1.0,
			Precision:        100,
			FractionFormat:   FractionDecimal,
			SingleUse:        true,
		}},
		YAxis: []*NumberFormat{{
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
		CYX:       1.0,
		SingleUse: true,
	}

	tests := []struct {
		name string
		vp   *Viewport
	}{
		{
			name: "basic viewport with all fields",
			vp: &Viewport{
				BBox:      pdf.Rectangle{LLx: 0, LLy: 0, URx: 100, URy: 200},
				Name:      "Main Drawing Area",
				Measure:   testMeasure,
				SingleUse: true,
			},
		},
		{
			name: "viewport without name",
			vp: &Viewport{
				BBox:      pdf.Rectangle{LLx: 50, LLy: 75, URx: 300, URy: 400},
				Measure:   testMeasure,
				SingleUse: false,
			},
		},
		{
			name: "viewport without measure",
			vp: &Viewport{
				BBox:      pdf.Rectangle{LLx: 10, LLy: 10, URx: 90, URy: 90},
				Name:      "Simple Area",
				SingleUse: true,
			},
		},
		{
			name: "minimal viewport",
			vp: &Viewport{
				BBox:      pdf.Rectangle{LLx: 0, LLy: 0, URx: 50, URy: 50},
				SingleUse: false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a test PDF writer
			w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)

			// Embed the Viewport
			res := pdf.NewResourceManager(w)
			embedded, _, err := tt.vp.Embed(res)
			if err != nil {
				t.Fatalf("embed failed: %v", err)
			}

			// Extract the Viewport back
			extracted, err := ExtractViewport(w, embedded)
			if err != nil {
				t.Fatalf("extract failed: %v", err)
			}

			// For comparison, set SingleUse to match (it's not stored in PDF)
			extracted.SingleUse = tt.vp.SingleUse

			// Also fix SingleUse for embedded measure if present
			if extracted.Measure != nil && tt.vp.Measure != nil {
				if extractedRM, ok := extracted.Measure.(*RectilinearMeasure); ok {
					extractedRM.SingleUse = true
					// Fix NumberFormat SingleUse too
					for _, nf := range extractedRM.XAxis {
						nf.SingleUse = true
					}
					for _, nf := range extractedRM.YAxis {
						nf.SingleUse = true
					}
					for _, nf := range extractedRM.Distance {
						nf.SingleUse = true
					}
					for _, nf := range extractedRM.Area {
						nf.SingleUse = true
					}
				}
			}

			// Compare
			if diff := cmp.Diff(extracted, tt.vp); diff != "" {
				t.Errorf("round trip failed (-got +want):\n%s", diff)
			}
		})
	}
}

func TestExtractViewportMalformedMeasure(t *testing.T) {
	// Create a test PDF writer
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)

	// Create a viewport dictionary with malformed measure
	dict := pdf.Dict{
		"Type":    pdf.Name("Viewport"),
		"BBox":    pdf.Array{pdf.Number(0), pdf.Number(0), pdf.Number(100), pdf.Number(100)},
		"Measure": pdf.String("invalid measure"), // This should be a dictionary
	}

	// Extract should succeed but ignore malformed measure
	vp, err := ExtractViewport(w, dict)
	if err != nil {
		t.Fatalf("extract failed: %v", err)
	}

	if vp.Measure != nil {
		t.Error("expected Measure to be nil when malformed")
	}

	if vp.BBox.LLx != 0 || vp.BBox.LLy != 0 || vp.BBox.URx != 100 || vp.BBox.URy != 100 {
		t.Errorf("BBox not extracted correctly: %v", vp.BBox)
	}
}

func TestSelectViewport(t *testing.T) {
	viewports := ViewPortArray{
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
	}

	tests := []struct {
		name     string
		point    vec.Vec2
		expected *Viewport
	}{
		{
			name:     "point in first viewport only",
			point:    vec.Vec2{X: 10, Y: 10},
			expected: viewports[0],
		},
		{
			name:     "point in overlapping area - should return last",
			point:    vec.Vec2{X: 60, Y: 60},
			expected: viewports[2], // Third is last in reverse order
		},
		{
			name:     "point in second and third - should return third",
			point:    vec.Vec2{X: 70, Y: 70},
			expected: viewports[2],
		},
		{
			name:     "point not in any viewport",
			point:    vec.Vec2{X: 200, Y: 200},
			expected: nil,
		},
		{
			name:     "point on boundary",
			point:    vec.Vec2{X: 75, Y: 75},
			expected: viewports[2],
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

func TestSelectViewportEmptyArray(t *testing.T) {
	viewports := ViewPortArray{}
	result := viewports.Select(vec.Vec2{X: 50, Y: 50})
	if result != nil {
		t.Error("Select with empty array should return nil")
	}
}

func TestExtractViewportArray(t *testing.T) {
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
	embedded1, _, err := vp1.Embed(res)
	if err != nil {
		t.Fatalf("embed vp1 failed: %v", err)
	}
	embedded2, _, err := vp2.Embed(res)
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

	if len(extracted) != 2 {
		t.Fatalf("expected 2 viewports, got %d", len(extracted))
	}

	// Fix SingleUse for comparison
	extracted[0].SingleUse = true
	extracted[1].SingleUse = true

	if extracted[0].Name != "First" {
		t.Errorf("first viewport name = %q, want %q", extracted[0].Name, "First")
	}
	if extracted[1].Name != "Second" {
		t.Errorf("second viewport name = %q, want %q", extracted[1].Name, "Second")
	}
}

func TestEmbedViewportArray(t *testing.T) {
	viewports := ViewPortArray{
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
	}

	// Create a test PDF writer
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	res := pdf.NewResourceManager(w)

	// Embed array
	embedded, _, err := viewports.Embed(res)
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

	// Fix SingleUse for comparison
	extracted[0].SingleUse = true
	extracted[1].SingleUse = false

	if diff := cmp.Diff(extracted, viewports); diff != "" {
		t.Errorf("array round trip failed (-got +want):\n%s", diff)
	}
}

func TestViewPortArrayType(t *testing.T) {
	// Test that ViewPortArray implements the expected methods
	viewports := ViewPortArray{
		{
			BBox:      pdf.Rectangle{LLx: 0, LLy: 0, URx: 100, URy: 100},
			Name:      "Test",
			SingleUse: true,
		},
	}

	// Test Select method
	result := viewports.Select(vec.Vec2{X: 50, Y: 50})
	if result == nil || result.Name != "Test" {
		t.Errorf("Select failed: got %v, want viewport named 'Test'", result)
	}

	// Test Embed method
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	res := pdf.NewResourceManager(w)

	embedded, _, err := viewports.Embed(res)
	if err != nil {
		t.Fatalf("Embed failed: %v", err)
	}

	if _, ok := embedded.(pdf.Array); !ok {
		t.Errorf("Expected pdf.Array, got %T", embedded)
	}

	// Test round-trip
	arr := embedded.(pdf.Array)
	extracted, err := ExtractViewportArray(w, arr)
	if err != nil {
		t.Fatalf("ExtractViewportArray failed: %v", err)
	}

	if len(extracted) != 1 {
		t.Errorf("Expected 1 viewport, got %d", len(extracted))
	}
}

func TestViewportVersionCheck(t *testing.T) {
	vp := &Viewport{
		BBox:      pdf.Rectangle{LLx: 0, LLy: 0, URx: 100, URy: 100},
		SingleUse: true,
	}

	// Test with PDF 1.5 (should fail)
	w15, _ := memfile.NewPDFWriter(pdf.V1_5, nil)
	res15 := pdf.NewResourceManager(w15)

	_, _, err := vp.Embed(res15)
	if err == nil {
		t.Error("expected version check error for PDF 1.5")
	}

	// Test with PDF 1.6 (should succeed)
	w16, _ := memfile.NewPDFWriter(pdf.V1_6, nil)
	res16 := pdf.NewResourceManager(w16)

	_, _, err = vp.Embed(res16)
	if err != nil {
		t.Errorf("unexpected error for PDF 1.6: %v", err)
	}
}

func TestExtractViewportErrors(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)

	tests := []struct {
		name string
		obj  pdf.Object
	}{
		{
			name: "not a dictionary",
			obj:  pdf.String("invalid"),
		},
		{
			name: "missing BBox",
			obj: pdf.Dict{
				"Type": pdf.Name("Viewport"),
			},
		},
		{
			name: "invalid BBox",
			obj: pdf.Dict{
				"Type": pdf.Name("Viewport"),
				"BBox": pdf.String("invalid bbox"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ExtractViewport(w, tt.obj)
			if err == nil {
				t.Error("expected error but got none")
			}
		})
	}
}

func TestViewportSingleUse(t *testing.T) {
	vp := &Viewport{
		BBox:      pdf.Rectangle{LLx: 0, LLy: 0, URx: 100, URy: 100},
		SingleUse: true,
	}

	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	res := pdf.NewResourceManager(w)

	// Embed with SingleUse = true
	embedded, _, err := vp.Embed(res)
	if err != nil {
		t.Fatalf("embed failed: %v", err)
	}

	// Should return a dictionary directly, not a reference
	if _, ok := embedded.(pdf.Dict); !ok {
		t.Error("SingleUse=true should return Dict, not Reference")
	}

	// Test with SingleUse = false
	vp.SingleUse = false
	embedded2, _, err := vp.Embed(res)
	if err != nil {
		t.Fatalf("embed failed: %v", err)
	}

	// Should return a reference
	if _, ok := embedded2.(pdf.Reference); !ok {
		t.Error("SingleUse=false should return Reference, not Dict")
	}
}

// TestViewportWithPtData verifies that PtData is properly handled during
// viewport read/write cycles.
func TestViewportWithPtData(t *testing.T) {
	writer1, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	rm1 := pdf.NewResourceManager(writer1)

	// create test PtData with some geospatial point data
	testPtData := &PtData{
		Subtype: PtDataSubtypeCloud,
		Names:   []string{PtDataNameLat, PtDataNameLon, PtDataNameAlt},
		XPTS: [][]pdf.Object{
			{pdf.Number(40.7128), pdf.Number(-74.0060), pdf.Number(10.5)}, // NYC coordinates
			{pdf.Number(40.7589), pdf.Number(-73.9851), pdf.Number(15.2)}, // Central Park
		},
		SingleUse: false, // use as indirect object
	}

	vp0 := &Viewport{
		BBox:      pdf.Rectangle{LLx: 0, LLy: 0, URx: 100, URy: 100},
		Name:      "Geospatial Viewport",
		PtData:    testPtData,
		SingleUse: true,
	}

	embedded, _, err := vp0.Embed(rm1)
	if err != nil {
		t.Fatal(err)
	}
	err = rm1.Close()
	if err != nil {
		t.Fatal(err)
	}
	err = writer1.Close()
	if err != nil {
		t.Fatal(err)
	}

	vp1, err := ExtractViewport(writer1, embedded)
	if err != nil {
		t.Fatal(err)
	}

	// verify PtData was preserved
	if vp1.PtData == nil {
		t.Error("PtData was not preserved during extraction")
		return
	}

	// check PtData content
	if vp1.PtData.Subtype != PtDataSubtypeCloud {
		t.Errorf("PtData subtype mismatch: got %s, want %s", vp1.PtData.Subtype, PtDataSubtypeCloud)
	}
	if len(vp1.PtData.Names) != 3 {
		t.Errorf("PtData names length mismatch: got %d, want 3", len(vp1.PtData.Names))
	}
	if len(vp1.PtData.XPTS) != 2 {
		t.Errorf("PtData XPTS length mismatch: got %d, want 2", len(vp1.PtData.XPTS))
	}

	// test round-trip
	writer2, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	rm2 := pdf.NewResourceManager(writer2)
	embedded2, _, err := vp1.Embed(rm2)
	if err != nil {
		t.Fatal(err)
	}
	err = rm2.Close()
	if err != nil {
		t.Fatal(err)
	}
	err = writer2.Close()
	if err != nil {
		t.Fatal(err)
	}

	vp2, err := ExtractViewport(writer2, embedded2)
	if err != nil {
		t.Fatal(err)
	}

	// check that PtData round-tripped correctly
	if vp2.PtData == nil {
		t.Error("PtData was lost during round-trip")
		return
	}

	// Fix SingleUse for comparison (not stored in PDF)
	vp2.SingleUse = vp1.SingleUse
	vp2.PtData.SingleUse = vp1.PtData.SingleUse

	// use cmp to compare the PtData structures
	if diff := cmp.Diff(vp1.PtData, vp2.PtData, cmp.AllowUnexported(PtData{})); diff != "" {
		t.Errorf("PtData round trip failed (-got +want):\n%s", diff)
	}

	// verify basic viewport properties were preserved
	if vp1.BBox != vp2.BBox {
		t.Errorf("BBox changed: %v -> %v", vp1.BBox, vp2.BBox)
	}
	if vp1.Name != vp2.Name {
		t.Errorf("Name changed: %s -> %s", vp1.Name, vp2.Name)
	}
}
