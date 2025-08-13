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
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

func TestRectilinearMeasureExtractEmbed(t *testing.T) {
	// Create some number formats for testing
	milesFormat := &NumberFormat{
		Unit:             "mi",
		ConversionFactor: 1.0,
		Precision:        100000,
		FractionFormat:   FractionDecimal,
		SingleUse:        true,
	}
	feetFormat := &NumberFormat{
		Unit:             "ft",
		ConversionFactor: 5280,
		Precision:        1,
		FractionFormat:   FractionDecimal,
		SingleUse:        true,
	}
	inchFormat := &NumberFormat{
		Unit:             "in",
		ConversionFactor: 12,
		Precision:        8,
		FractionFormat:   FractionFraction,
		SingleUse:        true,
	}
	acresFormat := &NumberFormat{
		Unit:             "acres",
		ConversionFactor: 640,
		Precision:        100,
		SingleUse:        true,
	}

	tests := []struct {
		name string
		rm   *RectilinearMeasure
	}{
		{
			name: "basic measure with different X and Y",
			rm: &RectilinearMeasure{
				ScaleRatio: "1in = 0.1 mi",
				XAxis:      []*NumberFormat{milesFormat},
				YAxis:      []*NumberFormat{feetFormat},
				Distance:   []*NumberFormat{milesFormat, feetFormat, inchFormat},
				Area:       []*NumberFormat{acresFormat},
				Origin:     [2]float64{0, 0},
				CYX:        0.000189394, // feet to miles conversion
				SingleUse:  true,
			},
		},
		{
			name: "measure with same X and Y",
			rm: &RectilinearMeasure{
				ScaleRatio: "1:100",
				XAxis:      []*NumberFormat{milesFormat},
				YAxis:      []*NumberFormat{milesFormat}, // Same as X
				Distance:   []*NumberFormat{milesFormat},
				Area:       []*NumberFormat{acresFormat},
				Origin:     [2]float64{0, 0},
				CYX:        1.0,
				SingleUse:  true,
			},
		},
		{
			name: "measure with non-zero origin",
			rm: &RectilinearMeasure{
				ScaleRatio: "1cm = 1m",
				XAxis:      []*NumberFormat{milesFormat},
				YAxis:      []*NumberFormat{milesFormat},
				Distance:   []*NumberFormat{milesFormat},
				Area:       []*NumberFormat{acresFormat},
				Origin:     [2]float64{100, 200},
				CYX:        1.0,
				SingleUse:  true,
			},
		},
		{
			name: "measure with optional fields",
			rm: &RectilinearMeasure{
				ScaleRatio: "1:50",
				XAxis:      []*NumberFormat{milesFormat},
				YAxis:      []*NumberFormat{milesFormat},
				Distance:   []*NumberFormat{milesFormat},
				Area:       []*NumberFormat{acresFormat},
				Angle: []*NumberFormat{{
					Unit:             "deg",
					ConversionFactor: 1.0,
					Precision:        10,
					SingleUse:        true,
				}},
				Slope: []*NumberFormat{{
					Unit:             "%",
					ConversionFactor: 100,
					Precision:        10,
					SingleUse:        true,
				}},
				Origin:    [2]float64{0, 0},
				CYX:       1.0,
				SingleUse: false, // Test reference creation
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a test PDF writer
			w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)

			// Embed the RectilinearMeasure
			rm := pdf.NewResourceManager(w)
			embedded, _, err := tt.rm.Embed(rm)
			if err != nil {
				t.Fatalf("embed failed: %v", err)
			}

			// Extract the Measure back
			extracted, err := Extract(w, embedded)
			if err != nil {
				t.Fatalf("extract failed: %v", err)
			}

			// Check type
			extractedRL, ok := extracted.(*RectilinearMeasure)
			if !ok {
				t.Fatalf("extracted measure is not RectilinearMeasure")
			}

			// For comparison, set SingleUse to match (it's not stored in PDF)
			extractedRL.SingleUse = tt.rm.SingleUse

			// Also fix SingleUse for all NumberFormats (not stored in PDF)
			for _, nf := range extractedRL.XAxis {
				nf.SingleUse = true
			}
			for _, nf := range extractedRL.YAxis {
				nf.SingleUse = true
			}
			for _, nf := range extractedRL.Distance {
				nf.SingleUse = true
			}
			for _, nf := range extractedRL.Area {
				nf.SingleUse = true
			}
			for _, nf := range extractedRL.Angle {
				nf.SingleUse = true
			}
			for _, nf := range extractedRL.Slope {
				nf.SingleUse = true
			}

			// Compare
			if diff := cmp.Diff(extractedRL, tt.rm); diff != "" {
				t.Errorf("round trip failed (-got +want):\n%s", diff)
			}
		})
	}
}

func TestRectilinearMeasureYAxisOptimization(t *testing.T) {
	// Create identical number formats
	format := &NumberFormat{
		Unit:             "m",
		ConversionFactor: 1.0,
		Precision:        100,
		SingleUse:        true,
	}

	rm := &RectilinearMeasure{
		ScaleRatio: "1:1",
		XAxis:      []*NumberFormat{format},
		YAxis:      []*NumberFormat{format}, // Same pointer as X
		Distance:   []*NumberFormat{format},
		Area:       []*NumberFormat{format},
		Origin:     [2]float64{0, 0},
		CYX:        1.0,
		SingleUse:  true,
	}

	// Create a test PDF writer
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)

	// Embed the RectilinearMeasure
	res := pdf.NewResourceManager(w)
	embedded, _, err := rm.Embed(res)
	if err != nil {
		t.Fatalf("embed failed: %v", err)
	}

	// Check that Y is not in the dictionary
	dict, ok := embedded.(pdf.Dict)
	if !ok {
		t.Fatalf("embedded measure is not a dictionary")
	}

	if _, hasY := dict["Y"]; hasY {
		t.Error("Y axis should be omitted when pointer-equal to X")
	}

	// Extract and verify it's reconstructed correctly
	extracted, err := Extract(w, dict)
	if err != nil {
		t.Fatalf("extract failed: %v", err)
	}

	extractedRL := extracted.(*RectilinearMeasure)

	// Should have same values (though not same pointers after extraction)
	if len(extractedRL.YAxis) != len(rm.YAxis) {
		t.Errorf("Y axis length mismatch: got %d, want %d", len(extractedRL.YAxis), len(rm.YAxis))
	}

	// CYX should be 1.0 when Y is copied from X
	if extractedRL.CYX != 1.0 {
		t.Errorf("CYX should be 1.0 when Y copied from X, got %f", extractedRL.CYX)
	}
}

func TestRectilinearMeasureCYXHandling(t *testing.T) {
	tests := []struct {
		name        string
		hasY        bool
		hasCYX      bool
		cyx         float64
		expectedCYX float64
	}{
		{
			name:        "Y missing, CYX missing",
			hasY:        false,
			hasCYX:      false,
			cyx:         0,
			expectedCYX: 1.0, // Should be set to 1.0
		},
		{
			name:        "Y present, CYX missing",
			hasY:        true,
			hasCYX:      false,
			cyx:         0,
			expectedCYX: 0, // Should remain 0
		},
		{
			name:        "Y present, CYX present",
			hasY:        true,
			hasCYX:      true,
			cyx:         2.5,
			expectedCYX: 2.5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a test PDF writer
			w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)

			// Create a measure dictionary manually
			xFormat := pdf.Dict{
				"U": pdf.String("m"),
				"C": pdf.Number(1.0),
				"D": pdf.Integer(100),
			}

			dict := pdf.Dict{
				"Subtype": pdf.Name("RL"),
				"R":       pdf.String("1:1"),
				"X":       pdf.Array{xFormat},
				"D":       pdf.Array{xFormat},
				"A":       pdf.Array{xFormat},
			}

			if tt.hasY {
				dict["Y"] = pdf.Array{xFormat}
			}
			if tt.hasCYX {
				dict["CYX"] = pdf.Number(tt.cyx)
			}

			// Extract the measure
			extracted, err := Extract(w, dict)
			if err != nil {
				t.Fatalf("extract failed: %v", err)
			}

			extractedRL := extracted.(*RectilinearMeasure)
			if extractedRL.CYX != tt.expectedCYX {
				t.Errorf("CYX = %f, want %f", extractedRL.CYX, tt.expectedCYX)
			}
		})
	}
}

func TestMeasureInterface(t *testing.T) {
	rm := &RectilinearMeasure{
		ScaleRatio: "1:1",
		XAxis:      []*NumberFormat{},
		YAxis:      []*NumberFormat{},
		Distance:   []*NumberFormat{},
		Area:       []*NumberFormat{},
		Origin:     [2]float64{0, 0},
	}

	// Check that RectilinearMeasure implements Measure
	var _ Measure = rm

	// Check MeasureType
	if rm.MeasureType() != "RL" {
		t.Errorf("MeasureType() = %q, want %q", rm.MeasureType(), "RL")
	}
}

func TestExtractMeasureSubtypeHandling(t *testing.T) {
	tests := []struct {
		name         string
		subtype      pdf.Name
		shouldPanic  bool
		expectedType pdf.Name
	}{
		{
			name:         "explicit RL subtype",
			subtype:      "RL",
			shouldPanic:  false,
			expectedType: "RL",
		},
		{
			name:         "missing subtype defaults to RL",
			subtype:      "",
			shouldPanic:  false,
			expectedType: "RL",
		},
		{
			name:         "unknown subtype defaults to RL",
			subtype:      "Unknown",
			shouldPanic:  false,
			expectedType: "RL",
		},
		{
			name:        "GEO subtype panics",
			subtype:     "GEO",
			shouldPanic: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a test PDF writer
			w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)

			// Create a minimal measure dictionary
			xFormat := pdf.Dict{
				"U": pdf.String("m"),
				"C": pdf.Number(1.0),
				"D": pdf.Integer(100),
			}

			dict := pdf.Dict{
				"R": pdf.String("1:1"),
				"X": pdf.Array{xFormat},
				"D": pdf.Array{xFormat},
				"A": pdf.Array{xFormat},
			}

			if tt.subtype != "" {
				dict["Subtype"] = tt.subtype
			}

			if tt.shouldPanic {
				defer func() {
					if r := recover(); r == nil {
						t.Error("expected panic but didn't get one")
					}
				}()
			}

			// Extract the measure
			extracted, err := Extract(w, dict)
			if err != nil && !tt.shouldPanic {
				t.Fatalf("extract failed: %v", err)
			}

			if !tt.shouldPanic {
				if extracted.MeasureType() != tt.expectedType {
					t.Errorf("MeasureType() = %q, want %q", extracted.MeasureType(), tt.expectedType)
				}
			}
		})
	}
}
