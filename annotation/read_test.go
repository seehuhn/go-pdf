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

package annotation

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/text/language"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

type testCase struct {
	name       string
	annotation pdf.Annotation
}

// testCases holds test cases for all annotation types
var testCases = map[string][]testCase{
	"Text": {
		{
			name: "basic text annotation",
			annotation: &Text{
				Common: Common{
					Rect:     pdf.Rectangle{LLx: 100, LLy: 100, URx: 200, URy: 150},
					Contents: "This is a text annotation",
				},
				Open: false,
				Name: "Comment",
			},
		},
		{
			name: "text annotation with markup fields",
			annotation: &Text{
				Common: Common{
					Rect:               pdf.Rectangle{LLx: 50, LLy: 50, URx: 150, URy: 100},
					Contents:           "Text with markup",
					NM:                 "annotation-1",
					StrokingOpacity:    0.8,
					NonStrokingOpacity: 0.6,
				},
				Markup: Markup{
					T:            "Author Name",
					CreationDate: time.Date(2023, 1, 15, 10, 30, 0, 0, time.UTC),
					Subj:         "Test Subject",
				},
				Open:  true,
				Name:  "Note",
				State: "Marked",
			},
		},
		{
			name: "text annotation with all fields",
			annotation: &Text{
				Common: Common{
					Rect:     pdf.Rectangle{LLx: 0, LLy: 0, URx: 100, URy: 50},
					Contents: "Complete text annotation",
					F:        4,                        // ReadOnly flag
					C:        []float64{1.0, 0.0, 0.0}, // Red color
					Border: &Border{
						HCornerRadius: 2.0,
						VCornerRadius: 2.0,
						Width:         1.0,
						DashArray:     []float64{3.0, 2.0},
					},
					Lang: language.MustParse("en-US"),
				},
				Markup: Markup{
					T:    "Test User",
					Subj: "Complete annotation test",
					RT:   "R",
					IT:   "Note",
				},
				Open:       true,
				Name:       "Insert",
				State:      "Accepted",
				StateModel: "Review",
			},
		},
	},
	"Link": {
		{
			name: "basic link annotation",
			annotation: &Link{
				Common: Common{
					Rect: pdf.Rectangle{LLx: 10, LLy: 10, URx: 100, URy: 30},
				},
				H: "I", // Invert highlighting
			},
		},
		{
			name: "link annotation with quad points",
			annotation: &Link{
				Common: Common{
					Rect:     pdf.Rectangle{LLx: 0, LLy: 0, URx: 200, URy: 40},
					Contents: "Link description",
				},
				H:          "O",                                         // Outline highlighting
				QuadPoints: []float64{10, 10, 190, 10, 190, 30, 10, 30}, // Rectangle quad
			},
		},
		{
			name: "link annotation with border",
			annotation: &Link{
				Common: Common{
					Rect: pdf.Rectangle{LLx: 50, LLy: 50, URx: 150, URy: 70},
					Border: &Border{
						Width:     2.0,
						DashArray: []float64{5.0, 3.0},
					},
				},
				H: "P", // Push highlighting
			},
		},
	},
	"FreeText": {
		{
			name: "basic free text annotation",
			annotation: &FreeText{
				Common: Common{
					Rect:     pdf.Rectangle{LLx: 100, LLy: 100, URx: 300, URy: 150},
					Contents: "Free text content",
				},
				DA: "/Helvetica 12 Tf 0 g",
				Q:  0, // Left justified
			},
		},
		{
			name: "free text with callout",
			annotation: &FreeText{
				Common: Common{
					Rect:     pdf.Rectangle{LLx: 150, LLy: 200, URx: 350, URy: 250},
					Contents: "Callout text",
				},
				Markup: Markup{
					T:    "Reviewer",
					Subj: "Callout comment",
					IT:   "FreeTextCallout",
				},
				DA: "/Arial 10 Tf 0 0 1 rg",
				Q:  1,                                       // Centered
				CL: []float64{100, 150, 125, 175, 150, 200}, // 6-point callout line
				LE: "OpenArrow",
			},
		},
		{
			name: "free text with border and rectangle differences",
			annotation: &FreeText{
				Common: Common{
					Rect:     pdf.Rectangle{LLx: 50, LLy: 300, URx: 250, URy: 380},
					Contents: "Styled free text",
					C:        []float64{0.9, 0.9, 0.9}, // Light gray background
				},
				Markup: Markup{
					T:            "Designer",
					CreationDate: time.Date(2023, 6, 1, 14, 0, 0, 0, time.UTC),
					IT:           "FreeText",
				},
				DA: "/Times-Roman 14 Tf 0.2 0.2 0.8 rg",
				Q:  2, // Right justified
				DS: "font-size:14pt;color:#3333CC;",
				RD: []float64{5.0, 3.0, 5.0, 3.0}, // Inner rectangle margins
			},
		},
	},
	"Line": {
		{
			name: "basic line annotation",
			annotation: &Line{
				Common: Common{
					Rect: pdf.Rectangle{LLx: 10, LLy: 10, URx: 200, URy: 50},
				},
				L: []float64{20, 20, 180, 40}, // Start and end coordinates
			},
		},
		{
			name: "line with arrow endings",
			annotation: &Line{
				Common: Common{
					Rect:     pdf.Rectangle{LLx: 50, LLy: 100, URx: 250, URy: 120},
					Contents: "Arrow line",
				},
				Markup: Markup{
					T:  "Designer",
					IT: "LineArrow",
				},
				L:  []float64{60, 110, 240, 110},
				LE: []pdf.Name{"OpenArrow", "ClosedArrow"},
				IC: []float64{1.0, 0.0, 0.0}, // Red interior color
			},
		},
		{
			name: "line with leader lines and caption",
			annotation: &Line{
				Common: Common{
					Rect:     pdf.Rectangle{LLx: 0, LLy: 200, URx: 300, URy: 250},
					Contents: "Dimension line",
				},
				Markup: Markup{
					T:            "Engineer",
					CreationDate: time.Date(2023, 3, 15, 9, 0, 0, 0, time.UTC),
					IT:           "LineDimension",
				},
				L:   []float64{50, 225, 250, 225},
				LE:  []pdf.Name{"Butt", "Butt"},
				LL:  10.0,  // Leader line length
				LLE: 5.0,   // Leader line extensions
				Cap: true,  // Show caption
				CP:  "Top", // Caption on top
				CO:  []float64{0, 5}, // Caption offset
				LLO: 2.0,  // Leader line offset
			},
		},
		{
			name: "line with all fields",
			annotation: &Line{
				Common: Common{
					Rect: pdf.Rectangle{LLx: 100, LLy: 300, URx: 400, URy: 350},
					F:    2, // Print flag
					C:    []float64{0.0, 0.0, 1.0}, // Blue border color
				},
				Markup: Markup{
					T:    "Reviewer",
					Subj: "Complete line annotation",
					RT:   "R",
				},
				L:   []float64{120, 325, 380, 325},
				LE:  []pdf.Name{"Diamond", "Square"},
				IC:  []float64{0.0, 1.0, 0.0}, // Green interior
				LL:  15.0,
				LLE: 8.0,
				Cap: true,
				CP:  "Inline",
				CO:  []float64{10, -5},
				LLO: 3.0,
			},
		},
	},
	"Square": {
		{
			name: "basic square annotation",
			annotation: &Square{
				Common: Common{
					Rect:     pdf.Rectangle{LLx: 50, LLy: 50, URx: 150, URy: 100},
					Contents: "Square annotation",
				},
				Markup: Markup{
					T:    "Author",
					Subj: "Square subject",
				},
			},
		},
		{
			name: "square with interior color and border style",
			annotation: &Square{
				Common: Common{
					Rect: pdf.Rectangle{LLx: 100, LLy: 100, URx: 200, URy: 200},
					NM:   "square-001",
				},
				Markup: Markup{
					T:    "Designer",
					Subj: "Color square",
				},
				IC: []float64{1.0, 0.5, 0.0}, // RGB orange interior
			},
		},
		{
			name: "square with all fields",
			annotation: &Square{
				Common: Common{
					Rect:     pdf.Rectangle{LLx: 200, LLy: 200, URx: 350, URy: 300},
					Contents: "Complex square",
					C:        []float64{0.0, 0.0, 1.0}, // Blue border
				},
				Markup: Markup{
					T:    "Reviewer",
					Subj: "Complex annotation",
					IT:   "SquareCloud",
				},
				IC: []float64{0.9, 0.9, 0.9},    // Light gray interior
				RD: []float64{5.0, 5.0, 5.0, 5.0}, // Rectangle differences
			},
		},
	},
	"Circle": {
		{
			name: "basic circle annotation",
			annotation: &Circle{
				Common: Common{
					Rect:     pdf.Rectangle{LLx: 75, LLy: 75, URx: 175, URy: 175},
					Contents: "Circle annotation",
				},
				Markup: Markup{
					T:    "Author",
					Subj: "Circle subject",
				},
			},
		},
		{
			name: "circle with interior color",
			annotation: &Circle{
				Common: Common{
					Rect: pdf.Rectangle{LLx: 150, LLy: 150, URx: 250, URy: 200},
					NM:   "circle-001",
				},
				Markup: Markup{
					T:    "Designer",
					Subj: "Color circle",
				},
				IC: []float64{0.0, 1.0, 0.0}, // Green interior
			},
		},
		{
			name: "circle with all fields",
			annotation: &Circle{
				Common: Common{
					Rect:     pdf.Rectangle{LLx: 300, LLy: 300, URx: 450, URy: 450},
					Contents: "Complex circle",
					C:        []float64{1.0, 0.0, 0.0}, // Red border
				},
				Markup: Markup{
					T:    "Reviewer",
					Subj: "Complex circle annotation",
					IT:   "CircleCloud",
				},
				IC: []float64{1.0, 1.0, 0.0},    // Yellow interior
				RD: []float64{10.0, 10.0, 10.0, 10.0}, // Rectangle differences
			},
		},
	},
	"Polygon": {
		{
			name: "basic polygon annotation",
			annotation: &Polygon{
				Common: Common{
					Rect:     pdf.Rectangle{LLx: 50, LLy: 50, URx: 200, URy: 150},
					Contents: "Triangle polygon",
				},
				Markup: Markup{
					T:    "Author",
					Subj: "Triangle shape",
				},
				Vertices: []float64{100, 50, 50, 150, 200, 150}, // Triangle vertices
			},
		},
		{
			name: "polygon with interior color and border effect",
			annotation: &Polygon{
				Common: Common{
					Rect: pdf.Rectangle{LLx: 100, LLy: 100, URx: 300, URy: 250},
					NM:   "polygon-001",
				},
				Markup: Markup{
					T:    "Designer",
					Subj: "Colored polygon",
					IT:   "PolygonCloud",
				},
				Vertices: []float64{150, 100, 300, 150, 250, 250, 100, 200}, // Quadrilateral
				IC:       []float64{0.0, 1.0, 0.5}, // Green-cyan interior
			},
		},
		{
			name: "polygon with path and measure",
			annotation: &Polygon{
				Common: Common{
					Rect:     pdf.Rectangle{LLx: 200, LLy: 200, URx: 400, URy: 350},
					Contents: "Complex polygon with path",
				},
				Markup: Markup{
					T:    "Engineer",
					Subj: "Measured polygon",
					IT:   "PolygonDimension",
				},
				Path: [][]float64{
					{250, 200}, // moveto
					{400, 250}, // lineto
					{350, 350}, // lineto
					{200, 300}, // lineto
				},
			},
		},
	},
	"Polyline": {
		{
			name: "basic polyline annotation",
			annotation: &Polyline{
				Common: Common{
					Rect:     pdf.Rectangle{LLx: 75, LLy: 75, URx: 275, URy: 175},
					Contents: "Simple polyline",
				},
				Markup: Markup{
					T:    "Author",
					Subj: "Line path",
				},
				Vertices: []float64{75, 100, 150, 175, 225, 125, 275, 150}, // Zigzag line
				LE:       []pdf.Name{"None", "OpenArrow"},                   // Arrow at end
			},
		},
		{
			name: "polyline with line endings and colors",
			annotation: &Polyline{
				Common: Common{
					Rect: pdf.Rectangle{LLx: 150, LLy: 150, URx: 350, URy: 250},
					NM:   "polyline-001",
				},
				Markup: Markup{
					T:    "Designer",
					Subj: "Colored polyline",
					IT:   "PolyLineDimension",
				},
				Vertices: []float64{150, 200, 200, 150, 300, 250, 350, 180},
				LE:       []pdf.Name{"Circle", "Square"}, // Different endings
				IC:       []float64{1.0, 0.0, 0.0},      // Red line endings
			},
		},
		{
			name: "polyline with curved path",
			annotation: &Polyline{
				Common: Common{
					Rect:     pdf.Rectangle{LLx: 250, LLy: 250, URx: 450, URy: 350},
					Contents: "Curved polyline",
				},
				Markup: Markup{
					T:    "Artist",
					Subj: "Bezier curve",
				},
				Path: [][]float64{
					{250, 300},                         // moveto
					{300, 250, 350, 350, 400, 300},     // curveto (6 coordinates)
					{450, 320},                         // lineto
				},
				LE: []pdf.Name{"Butt", "Diamond"},
			},
		},
	},
	"Highlight": {
		{
			name: "basic highlight annotation",
			annotation: &Highlight{
				Common: Common{
					Rect:     pdf.Rectangle{LLx: 100, LLy: 200, URx: 200, URy: 220},
					Contents: "Highlighted text",
				},
				Markup: Markup{
					T:    "Reviewer",
					Subj: "Important passage",
				},
				QuadPoints: []float64{100, 200, 200, 200, 200, 220, 100, 220}, // Single word quad
			},
		},
		{
			name: "highlight with multiple quads",
			annotation: &Highlight{
				Common: Common{
					Rect: pdf.Rectangle{LLx: 50, LLy: 300, URx: 250, URy: 340},
					NM:   "highlight-001",
					C:    []float64{1.0, 1.0, 0.0}, // Yellow highlight
				},
				Markup: Markup{
					T:    "Student",
					Subj: "Study notes",
				},
				QuadPoints: []float64{
					50, 300, 100, 300, 100, 320, 50, 320,   // First word
					110, 300, 160, 300, 160, 320, 110, 320, // Second word
					170, 320, 250, 320, 250, 340, 170, 340, // Third word (different line)
				},
			},
		},
	},
	"Underline": {
		{
			name: "basic underline annotation",
			annotation: &Underline{
				Common: Common{
					Rect:     pdf.Rectangle{LLx: 150, LLy: 400, URx: 300, URy: 420},
					Contents: "Underlined text",
				},
				Markup: Markup{
					T:    "Editor",
					Subj: "Emphasis added",
				},
				QuadPoints: []float64{150, 400, 300, 400, 300, 420, 150, 420}, // Single phrase
			},
		},
		{
			name: "underline with markup fields",
			annotation: &Underline{
				Common: Common{
					Rect: pdf.Rectangle{LLx: 75, LLy: 500, URx: 225, URy: 520},
					NM:   "underline-001",
					C:    []float64{0.0, 0.0, 1.0}, // Blue underline
				},
				Markup: Markup{
					T:            "Proofreader",
					Subj:         "Grammar correction",
					CreationDate: time.Date(2023, 6, 15, 14, 30, 0, 0, time.UTC),
				},
				QuadPoints: []float64{75, 500, 225, 500, 225, 520, 75, 520},
			},
		},
	},
	"Squiggly": {
		{
			name: "basic squiggly annotation",
			annotation: &Squiggly{
				Common: Common{
					Rect:     pdf.Rectangle{LLx: 200, LLy: 600, URx: 350, URy: 620},
					Contents: "Spelling error",
				},
				Markup: Markup{
					T:    "Spellchecker",
					Subj: "Possible misspelling",
				},
				QuadPoints: []float64{200, 600, 350, 600, 350, 620, 200, 620}, // Misspelled word
			},
		},
		{
			name: "squiggly with color",
			annotation: &Squiggly{
				Common: Common{
					Rect: pdf.Rectangle{LLx: 100, LLy: 700, URx: 180, URy: 720},
					NM:   "squiggly-001",
					C:    []float64{1.0, 0.0, 0.0}, // Red squiggly
				},
				Markup: Markup{
					T:    "Grammar checker",
					Subj: "Grammar issue",
				},
				QuadPoints: []float64{100, 700, 180, 700, 180, 720, 100, 720},
			},
		},
	},
	"StrikeOut": {
		{
			name: "basic strikeout annotation",
			annotation: &StrikeOut{
				Common: Common{
					Rect:     pdf.Rectangle{LLx: 250, LLy: 800, URx: 400, URy: 820},
					Contents: "Deleted text",
				},
				Markup: Markup{
					T:    "Editor",
					Subj: "Text to be removed",
				},
				QuadPoints: []float64{250, 800, 400, 800, 400, 820, 250, 820}, // Struck-out phrase
			},
		},
		{
			name: "strikeout with multiple sections",
			annotation: &StrikeOut{
				Common: Common{
					Rect: pdf.Rectangle{LLx: 50, LLy: 900, URx: 300, URy: 940},
					NM:   "strikeout-001",
				},
				Markup: Markup{
					T:    "Reviewer",
					Subj: "Major revision",
				},
				QuadPoints: []float64{
					50, 900, 150, 900, 150, 920, 50, 920,     // First section
					160, 900, 250, 900, 250, 920, 160, 920,   // Second section
					50, 920, 300, 920, 300, 940, 50, 940,     // Third section (different line)
				},
			},
		},
	},
}

func TestRoundTrip(t *testing.T) {
	for annotationType, cases := range testCases {
		for _, tc := range cases {
			t.Run(fmt.Sprintf("%s-%s", annotationType, tc.name), func(t *testing.T) {
				roundTripTest(t, tc.annotation)
			})
		}
	}
}

// roundTripTest performs a round-trip test for any annotation type
func roundTripTest(t *testing.T, a1 pdf.Annotation) {
	buf, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	rm := pdf.NewResourceManager(buf)

	// Embed the annotation
	embedded, _, err := a1.Embed(rm)
	if err != nil {
		t.Fatal(err)
	}

	err = buf.Close()
	if err != nil {
		t.Fatal(err)
	}

	// Read the annotation back
	a2, err := Extract(buf, embedded)
	if err != nil {
		t.Fatal(err)
	}

	// Use EquateComparable to handle language.Tag comparison
	opts := []cmp.Option{
		cmp.AllowUnexported(language.Tag{}),
	}

	// For Unknown annotations, we don't expect perfect round-trip
	// because the embedding process may add common annotation fields
	if _, isUnknown := a1.(*Unknown); isUnknown {
		// Just verify basic properties are preserved
		if a1.AnnotationType() != a2.AnnotationType() {
			t.Errorf("annotation type mismatch: want %s, got %s", 
				a1.AnnotationType(), a2.AnnotationType())
		}
		return
	}

	if diff := cmp.Diff(a1, a2, opts...); diff != "" {
		t.Errorf("round trip failed (-want +got):\n%s", diff)
	}
}

func TestUnknownAnnotation(t *testing.T) {
	// Create an unknown annotation type
	unknownDict := pdf.Dict{
		"Type":        pdf.Name("Annot"),
		"Subtype":     pdf.Name("CustomType"),
		"Rect":        pdf.Array{pdf.Number(0), pdf.Number(0), pdf.Number(100), pdf.Number(50)},
		"CustomField": pdf.TextString("custom value"),
	}

	buf, _ := memfile.NewPDFWriter(pdf.V2_0, nil)

	// Add the dictionary directly to the PDF
	ref := buf.Alloc()
	buf.Put(ref, unknownDict)

	err := buf.Close()
	if err != nil {
		t.Fatal(err)
	}

	// Extract should return an Unknown annotation
	annotation, err := Extract(buf, ref)
	if err != nil {
		t.Fatal(err)
	}

	unknown, ok := annotation.(*Unknown)
	if !ok {
		t.Fatalf("expected *Unknown, got %T", annotation)
	}

	// Check that the annotation type is correct
	if unknown.AnnotationType() != "CustomType" {
		t.Errorf("expected annotation type 'CustomType', got '%s'", unknown.AnnotationType())
	}

	// Check that the custom field is preserved
	if customField := unknown.Data["CustomField"]; customField == nil {
		t.Error("custom field not preserved")
	}

	// For unknown annotations, we don't expect perfect round-trip
	// because the embedding process may add common annotation fields
	// Just verify it doesn't crash and basic fields are preserved
	buf2, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	rm2 := pdf.NewResourceManager(buf2)
	_, _, err = unknown.Embed(rm2)
	if err != nil {
		t.Errorf("failed to embed unknown annotation: %v", err)
	}
}

func TestAnnotationTypes(t *testing.T) {
	tests := []struct {
		annotation   pdf.Annotation
		expectedType string
	}{
		{&Text{}, "Text"},
		{&Link{}, "Link"},
		{&FreeText{}, "FreeText"},
		{&Line{}, "Line"},
		{&Unknown{Data: pdf.Dict{"Subtype": pdf.Name("Custom")}}, "Custom"},
		{&Unknown{Data: pdf.Dict{}}, "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expectedType, func(t *testing.T) {
			if got := tt.annotation.AnnotationType(); got != tt.expectedType {
				t.Errorf("AnnotationType() = %v, want %v", got, tt.expectedType)
			}
		})
	}
}

func TestBorderDefaults(t *testing.T) {
	// Test that default border values are not written to PDF
	annotation := &Text{
		Common: Common{
			Rect: pdf.Rectangle{LLx: 0, LLy: 0, URx: 100, URy: 50},
			Border: &Border{
				HCornerRadius: 0,
				VCornerRadius: 0,
				Width:         1, // PDF default
				DashArray:     nil,
			},
		},
	}

	buf, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	rm := pdf.NewResourceManager(buf)

	embedded, _, err := annotation.Embed(rm)
	if err != nil {
		t.Fatal(err)
	}

	dict, ok := embedded.(pdf.Dict)
	if !ok {
		t.Fatal("embedded annotation is not a dictionary")
	}

	// Border should not be present since it's the default value
	if _, exists := dict["Border"]; exists {
		t.Error("default border should not be written to PDF")
	}
}

func TestOpacityHandling(t *testing.T) {
	tests := []struct {
		name             string
		strokeOpacity    float64
		nonStrokeOpacity float64
		expectCA         bool
		expectCa         bool
	}{
		{
			name:             "default opacities",
			strokeOpacity:    1.0,
			nonStrokeOpacity: 1.0,
			expectCA:         false,
			expectCa:         false,
		},
		{
			name:             "custom stroke opacity",
			strokeOpacity:    0.5,
			nonStrokeOpacity: 0.5,
			expectCA:         true,
			expectCa:         false,
		},
		{
			name:             "different opacities",
			strokeOpacity:    0.8,
			nonStrokeOpacity: 0.6,
			expectCA:         true,
			expectCa:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			annotation := &Text{
				Common: Common{
					Rect:               pdf.Rectangle{LLx: 0, LLy: 0, URx: 100, URy: 50},
					StrokingOpacity:    tt.strokeOpacity,
					NonStrokingOpacity: tt.nonStrokeOpacity,
				},
			}

			buf, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
			rm := pdf.NewResourceManager(buf)

			embedded, _, err := annotation.Embed(rm)
			if err != nil {
				t.Fatal(err)
			}

			dict, ok := embedded.(pdf.Dict)
			if !ok {
				t.Fatal("embedded annotation is not a dictionary")
			}

			_, hasCA := dict["CA"]
			_, hasCa := dict["ca"]

			if hasCA != tt.expectCA {
				t.Errorf("CA entry presence: got %v, want %v", hasCA, tt.expectCA)
			}
			if hasCa != tt.expectCa {
				t.Errorf("ca entry presence: got %v, want %v", hasCa, tt.expectCa)
			}
		})
	}
}

func FuzzRead(f *testing.F) {
	// Seed the fuzzer with valid test cases from all annotation types
	for _, cases := range testCases {
		for _, tc := range cases {
			opt := &pdf.WriterOptions{
				HumanReadable: true,
			}
			w, out := memfile.NewPDFWriter(pdf.V2_0, opt)
			rm := pdf.NewResourceManager(w)

			embedded, _, err := tc.annotation.Embed(rm)
			if err != nil {
				continue
			}

			err = rm.Close()
			if err != nil {
				continue
			}

			w.GetMeta().Trailer["Quir:X"] = embedded

			err = w.Close()
			if err != nil {
				continue
			}

			f.Add(out.Data)
		}
	}

	f.Fuzz(func(t *testing.T, fileData []byte) {
		// Make sure we don't panic on random input.
		opt := &pdf.ReaderOptions{
			ErrorHandling: pdf.ErrorHandlingReport,
		}
		r, err := pdf.NewReader(bytes.NewReader(fileData), opt)
		if err != nil {
			t.Skip("invalid PDF")
		}
		obj := r.GetMeta().Trailer["Quir:X"]
		if obj == nil {
			t.Skip("broken reference")
		}
		annotation, err := Extract(r, obj)
		if err != nil {
			t.Skip("broken annotation")
		}

		// Make sure we can write the annotation, and read it back.
		roundTripTest(t, annotation)

		// Test that basic operations don't panic
		annotationType := annotation.AnnotationType()
		// For Unknown annotations, empty type is acceptable if no subtype is present
		if _, isUnknown := annotation.(*Unknown); !isUnknown && annotationType == "" {
			t.Error("annotation type should not be empty for known types")
		}

		// Test embedding doesn't panic
		buf, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
		rm := pdf.NewResourceManager(buf)
		_, _, err = annotation.Embed(rm)
		if err != nil {
			t.Errorf("failed to embed annotation: %v", err)
		}
	})
}
