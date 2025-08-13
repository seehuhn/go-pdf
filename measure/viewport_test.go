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
		{"point inside", vec.Vec2{50, 100}, true},
		{"point on left edge", vec.Vec2{10, 100}, true},
		{"point on right edge", vec.Vec2{100, 100}, true},
		{"point on bottom edge", vec.Vec2{50, 20}, true},
		{"point on top edge", vec.Vec2{50, 200}, true},
		{"point at lower-left corner", vec.Vec2{10, 20}, true},
		{"point at upper-right corner", vec.Vec2{100, 200}, true},
		{"point left of rectangle", vec.Vec2{5, 100}, false},
		{"point right of rectangle", vec.Vec2{105, 100}, false},
		{"point below rectangle", vec.Vec2{50, 15}, false},
		{"point above rectangle", vec.Vec2{50, 205}, false},
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
			point:    vec.Vec2{10, 10},
			expected: viewports[0],
		},
		{
			name:     "point in overlapping area - should return last",
			point:    vec.Vec2{60, 60},
			expected: viewports[2], // Third is last in reverse order
		},
		{
			name:     "point in second and third - should return third",
			point:    vec.Vec2{70, 70},
			expected: viewports[2],
		},
		{
			name:     "point not in any viewport",
			point:    vec.Vec2{200, 200},
			expected: nil,
		},
		{
			name:     "point on boundary",
			point:    vec.Vec2{75, 75},
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
	result := viewports.Select(vec.Vec2{50, 50})
	if result != nil {
		t.Error("Select with empty array should return nil")
	}
}

func TestExtractViewportArray(t *testing.T) {
	// Create test viewports
	vp1 := &Viewport{
		BBox: pdf.Rectangle{LLx: 0, LLy: 0, URx: 100, URy: 100},
		Name: "First",
		SingleUse: true,
	}
	vp2 := &Viewport{
		BBox: pdf.Rectangle{LLx: 100, LLy: 100, URx: 200, URy: 200},
		Name: "Second",
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
			BBox: pdf.Rectangle{LLx: 0, LLy: 0, URx: 100, URy: 100},
			Name: "First",
			SingleUse: true,
		},
		{
			BBox: pdf.Rectangle{LLx: 100, LLy: 100, URx: 200, URy: 200},
			Name: "Second",
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
			BBox: pdf.Rectangle{LLx: 0, LLy: 0, URx: 100, URy: 100},
			Name: "Test",
			SingleUse: true,
		},
	}

	// Test Select method
	result := viewports.Select(vec.Vec2{50, 50})
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

func TestBackwardsCompatibility(t *testing.T) {
	// Test that the deprecated functions still work
	viewports := []*Viewport{
		{
			BBox: pdf.Rectangle{LLx: 0, LLy: 0, URx: 100, URy: 100},
			Name: "Test",
			SingleUse: true,
		},
	}

	// Test deprecated SelectViewport function
	result := SelectViewport(vec.Vec2{50, 50}, viewports)
	if result == nil || result.Name != "Test" {
		t.Errorf("SelectViewport failed: got %v, want viewport named 'Test'", result)
	}

	// Test deprecated EmbedViewportArray function
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	res := pdf.NewResourceManager(w)
	
	arr, err := EmbedViewportArray(res, viewports)
	if err != nil {
		t.Fatalf("EmbedViewportArray failed: %v", err)
	}
	
	if len(arr) != 1 {
		t.Errorf("Expected 1 element in array, got %d", len(arr))
	}
}

func TestViewportVersionCheck(t *testing.T) {
	vp := &Viewport{
		BBox: pdf.Rectangle{LLx: 0, LLy: 0, URx: 100, URy: 100},
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
		BBox: pdf.Rectangle{LLx: 0, LLy: 0, URx: 100, URy: 100},
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