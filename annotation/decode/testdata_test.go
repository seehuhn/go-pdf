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

package decode

import (
	"bytes"
	"io"
	"time"

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/geom/vec"

	"golang.org/x/text/language"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/action"
	"seehuhn.de/go/pdf/action/triggers"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/annotation/appearance"
	"seehuhn.de/go/pdf/destination"
	"seehuhn.de/go/pdf/file"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/measure"
	"seehuhn.de/go/pdf/movie"
	"seehuhn.de/go/pdf/sound"
)

// testMovie is the shared movie object used by movie annotation test
// cases.  It points to an external movie file.
var testMovie = &movie.Movie{
	File: &file.Specification{
		FileName:       "video.mov",
		AFRelationship: file.RelationshipUnspecified,
	},
}

// testSound is the shared sound object used by sound annotation test
// cases.  Its Channels, BitsPerSample, and Encoding fields are
// pre-materialised so the round-trip comparison succeeds without
// special-casing the defaults.
var testSound = &sound.Sound{
	SampleRate:    22050,
	Channels:      1,
	BitsPerSample: 8,
	Encoding:      sound.EncodingRaw,
	Data: &sound.InlineSource{
		WriteData: func(w io.Writer) error {
			_, err := w.Write(bytes.Repeat([]byte{0x80}, 16))
			return err
		},
	},
}

type testCase struct {
	name       string
	annotation annotation.Annotation
}

// testCases holds test cases for all annotation types
var testCases = map[pdf.Name][]testCase{
	"Text": {
		{
			name: "basic text annotation",
			annotation: &annotation.Text{
				Rect:     pdf.Rectangle{LLx: 100, LLy: 100, URx: 200, URy: 150},
				Contents: pdf.Text("This is a text annotation"),
				Open:     false,
				Icon:     "Comment",
			},
		},
		{
			name: "text annotation with markup fields",
			annotation: &annotation.Text{
				Rect:                    pdf.Rectangle{LLx: 50, LLy: 50, URx: 150, URy: 100},
				Contents:                pdf.Text("Text with markup"),
				Name:                    "annotation-1",
				StrokingTransparency:    0.8,
				NonStrokingTransparency: 0.6,
				User:                    pdf.Text("Author Name"),
				CreationDate:            time.Date(2023, 1, 15, 10, 30, 0, 0, time.UTC),
				Subject:                 pdf.Text("Test Subject"),
				InReplyTo:               pdf.NewReference(1, 0),
				Open:                    true,
				Icon:                    "Note",
				State:                   annotation.TextStateMarked,
			},
		},
		{
			name: "text annotation with all fields",
			annotation: &annotation.Text{
				Rect:     pdf.Rectangle{LLx: 0, LLy: 0, URx: 100, URy: 50},
				Contents: pdf.Text("Complete text annotation"),
				Flags:    4,                        // ReadOnly flag
				Color:    color.DeviceRGB{1, 0, 0}, // red
				Border: &annotation.Border{
					HCornerRadius: 2.0,
					VCornerRadius: 2.0,
					Width:         1.0,
					DashArray:     []float64{3.0, 2.0},
				},
				Lang:      language.MustParse("en-US"),
				User:      pdf.Text("Test User"),
				Subject:   pdf.Text("Complete annotation test"),
				RT:        "R",
				Intent:    "Note",
				InReplyTo: pdf.NewReference(2, 0),
				Open:      true,
				Icon:      "Insert",
				State:     annotation.TextStateAccepted,
			},
		},
	},
	"Link": {
		{
			name: "basic link annotation",
			annotation: &annotation.Link{
				Rect:      pdf.Rectangle{LLx: 10, LLy: 10, URx: 100, URy: 30},
				Highlight: annotation.HighlightInvert,
			},
		},
		{
			name: "link annotation with quad points",
			annotation: &annotation.Link{
				Rect:       pdf.Rectangle{LLx: 0, LLy: 0, URx: 200, URy: 40},
				Contents:   pdf.Text("Link description"),
				Highlight:  annotation.HighlightOutline,
				QuadPoints: []vec.Vec2{{X: 10, Y: 10}, {X: 190, Y: 10}, {X: 190, Y: 30}, {X: 10, Y: 30}}, // Rectangle quad
			},
		},
		{
			name: "link annotation with border",
			annotation: &annotation.Link{
				Rect: pdf.Rectangle{LLx: 50, LLy: 50, URx: 150, URy: 70},
				Border: &annotation.Border{
					Width:     2.0,
					DashArray: []float64{5.0, 3.0},
				},
				Highlight: annotation.HighlightPush,
			},
		},
		{
			name: "link with URI action",
			annotation: &annotation.Link{
				Rect:      pdf.Rectangle{LLx: 10, LLy: 10, URx: 100, URy: 30},
				Action:    &action.URI{URI: "https://example.com"},
				Highlight: annotation.HighlightInvert,
			},
		},
		{
			name: "link with named destination",
			annotation: &annotation.Link{
				Rect:        pdf.Rectangle{LLx: 10, LLy: 10, URx: 100, URy: 30},
				Destination: &destination.Named{Name: pdf.String("chapter1")},
				Highlight:   annotation.HighlightInvert,
			},
		},
	},
	"FreeText": {
		{
			name: "basic free text annotation",
			annotation: &annotation.FreeText{
				Rect:              pdf.Rectangle{LLx: 100, LLy: 100, URx: 300, URy: 150},
				Contents:          pdf.Text("Free text content"),
				DefaultAppearance: "/Helvetica 12 Tf 0 g",
				Align:             pdf.TextAlignLeft, // Left justified
			},
		},
		{
			name: "free text with callout",
			annotation: &annotation.FreeText{
				Rect:              pdf.Rectangle{LLx: 150, LLy: 200, URx: 350, URy: 250},
				Contents:          pdf.Text("Callout text"),
				User:              pdf.Text("Reviewer"),
				Subject:           pdf.Text("Callout comment"),
				Intent:            "FreeTextCallout",
				DefaultAppearance: "/Arial 10 Tf 0 0 1 rg",
				Align:             pdf.TextAlignCenter,                                              // Centered
				CalloutLine:       []vec.Vec2{{X: 100, Y: 150}, {X: 125, Y: 175}, {X: 150, Y: 200}}, // 3-point callout line
				LineEndingStyle:   "OpenArrow",
			},
		},
		{
			name: "free text with border and rectangle differences",
			annotation: &annotation.FreeText{
				Common: annotation.Common{
					Rect:     pdf.Rectangle{LLx: 50, LLy: 300, URx: 250, URy: 380},
					Contents: pdf.Text("Styled free text"),
					Color:    color.DeviceGray(0.9), // Light gray background
				},
				User:              pdf.Text("Designer"),
				CreationDate:      time.Date(2023, 6, 1, 14, 0, 0, 0, time.UTC),
				DefaultAppearance: "/Times-Roman 14 Tf 0.2 0.2 0.8 rg",
				Align:             pdf.TextAlignRight, // Right justified
				DefaultStyle:      "font-size:14pt;color:#3333CC;",
				Margin:            []float64{5.0, 3.0, 5.0, 3.0}, // Inner rectangle margins
			},
		},
		{
			name: "free text with border style",
			annotation: &annotation.FreeText{
				Rect:              pdf.Rectangle{LLx: 10, LLy: 10, URx: 200, URy: 60},
				DefaultAppearance: "/Helvetica 10 Tf 0 g",
				BorderStyle: &annotation.BorderStyle{
					Width:     2,
					Style:     "D",
					DashArray: []float64{3, 1},
				},
			},
		},
		{
			name: "free text with 2-point callout line",
			annotation: &annotation.FreeText{
				Rect:              pdf.Rectangle{LLx: 100, LLy: 100, URx: 300, URy: 150},
				Intent:            "FreeTextCallout",
				DefaultAppearance: "/Helvetica 10 Tf 0 g",
				CalloutLine:       []vec.Vec2{{X: 50, Y: 80}, {X: 100, Y: 100}},
				LineEndingStyle:   annotation.LineEndingStyleNone,
			},
		},
		{
			name: "free text with border effect",
			annotation: &annotation.FreeText{
				Rect:              pdf.Rectangle{LLx: 20, LLy: 20, URx: 220, URy: 80},
				DefaultAppearance: "/Courier 11 Tf 0 g",
				BorderEffect: &annotation.BorderEffect{
					Style:     "C",
					Intensity: 1.5,
				},
			},
		},
		{
			name: "free text callout with line ending",
			annotation: &annotation.FreeText{
				Rect:              pdf.Rectangle{LLx: 100, LLy: 200, URx: 300, URy: 260},
				Intent:            "FreeTextCallout",
				DefaultAppearance: "/Helvetica 9 Tf 0 0 0 rg",
				CalloutLine:       []vec.Vec2{{X: 50, Y: 180}, {X: 75, Y: 190}, {X: 100, Y: 200}},
				LineEndingStyle:   annotation.LineEndingStyleClosedArrow,
			},
		},
	},
	"Line": {
		{
			name: "basic line annotation",
			annotation: &annotation.Line{
				Rect:            pdf.Rectangle{LLx: 10, LLy: 10, URx: 200, URy: 50},
				Coords:          [4]float64{20, 20, 180, 40}, // Start and end coordinates
				LineEndingStyle: [2]annotation.LineEndingStyle{annotation.LineEndingStyleNone, annotation.LineEndingStyleNone},
			},
		},
		{
			name: "line with arrow endings",
			annotation: &annotation.Line{
				Rect:            pdf.Rectangle{LLx: 50, LLy: 100, URx: 250, URy: 120},
				Contents:        pdf.Text("Arrow line"),
				User:            pdf.Text("Designer"),
				Intent:          "LineArrow",
				Coords:          [4]float64{60, 110, 240, 110},
				LineEndingStyle: [2]annotation.LineEndingStyle{annotation.LineEndingStyleOpenArrow, annotation.LineEndingStyleClosedArrow},
				FillColor:       color.DeviceRGB{1.0, 0.0, 0.0}, // Red interior color
			},
		},
		{
			name: "line with leader lines and caption",
			annotation: &annotation.Line{
				Rect:            pdf.Rectangle{LLx: 0, LLy: 200, URx: 300, URy: 250},
				Contents:        pdf.Text("Dimension line"),
				User:            pdf.Text("Engineer"),
				CreationDate:    time.Date(2023, 3, 15, 9, 0, 0, 0, time.UTC),
				Intent:          "LineDimension",
				Coords:          [4]float64{50, 225, 250, 225},
				LineEndingStyle: [2]annotation.LineEndingStyle{annotation.LineEndingStyleButt, annotation.LineEndingStyleButt},
				LL:              10.0,            // Leader line length
				LLE:             5.0,             // Leader line extensions
				Caption:         true,            // Show caption
				CaptionAbove:    true,            // Caption on top
				CaptionOffset:   []float64{0, 5}, // Caption offset
				LLO:             2.0,             // Leader line offset
			},
		},
		{
			name: "line with all fields",
			annotation: &annotation.Line{
				Common: annotation.Common{
					Rect:  pdf.Rectangle{LLx: 100, LLy: 300, URx: 400, URy: 350},
					Flags: 2,                            // Print flag
					Color: color.DeviceCMYK{1, 1, 0, 0}, // blue
				},
				User:            pdf.Text("Reviewer"),
				Subject:         pdf.Text("Complete line annotation"),
				RT:              "R",
				InReplyTo:       pdf.NewReference(1, 0),
				Coords:          [4]float64{120, 325, 380, 325},
				LineEndingStyle: [2]annotation.LineEndingStyle{annotation.LineEndingStyleDiamond, annotation.LineEndingStyleSquare},
				FillColor:       color.DeviceRGB{0.0, 1.0, 0.0}, // Green interior
				LL:              15.0,
				LLE:             8.0,
				Caption:         true,
				CaptionAbove:    false,
				CaptionOffset:   []float64{10, -5},
				LLO:             3.0,
			},
		},
	},
	"Square": {
		{
			name: "basic square annotation",
			annotation: &annotation.Square{
				Rect:     pdf.Rectangle{LLx: 50, LLy: 50, URx: 150, URy: 100},
				Contents: pdf.Text("Square annotation"),
				User:     pdf.Text("Author"),
				Subject:  pdf.Text("Square subject"),
			},
		},
		{
			name: "square with interior color and border style",
			annotation: &annotation.Square{
				Rect:      pdf.Rectangle{LLx: 100, LLy: 100, URx: 200, URy: 200},
				Name:      "square-001",
				User:      pdf.Text("Designer"),
				Subject:   pdf.Text("Color square"),
				FillColor: color.DeviceRGB{1.0, 0.5, 0.0}, // RGB orange interior
			},
		},
		{
			name: "square with all fields",
			annotation: &annotation.Square{
				Rect:      pdf.Rectangle{LLx: 200, LLy: 200, URx: 350, URy: 300},
				Contents:  pdf.Text("Complex square"),
				Color:     color.DeviceGray(0),
				User:      pdf.Text("Reviewer"),
				Subject:   pdf.Text("Complex annotation"),
				Intent:    "SquareCloud",
				FillColor: color.DeviceRGB{0.9, 0.9, 0.9}, // Light gray interior
				Margin:    []float64{5.0, 5.0, 5.0, 5.0},  // Rectangle differences
			},
		},
		{
			name: "square with border style",
			annotation: &annotation.Square{
				Rect: pdf.Rectangle{LLx: 50, LLy: 50, URx: 200, URy: 150},
				BorderStyle: &annotation.BorderStyle{
					Width:     2,
					Style:     "D",
					DashArray: []float64{3, 1},
				},
			},
		},
		{
			name: "square with border style and effect",
			annotation: &annotation.Square{
				Rect:     pdf.Rectangle{LLx: 100, LLy: 100, URx: 300, URy: 250},
				Contents: pdf.Text("Cloudy square"),
				BorderStyle: &annotation.BorderStyle{
					Width: 1.5,
					Style: "S",
				},
				BorderEffect: &annotation.BorderEffect{
					Style:     "C",
					Intensity: 1.0,
				},
				FillColor: color.DeviceGray(0.95),
				Margin:    []float64{3.0, 3.0, 3.0, 3.0},
			},
		},
	},
	"Circle": {
		{
			name: "basic circle annotation",
			annotation: &annotation.Circle{
				Rect:     pdf.Rectangle{LLx: 75, LLy: 75, URx: 175, URy: 175},
				Contents: pdf.Text("Circle annotation"),
				User:     pdf.Text("Author"),
				Subject:  pdf.Text("Circle subject"),
			},
		},
		{
			name: "circle with interior color",
			annotation: &annotation.Circle{
				Rect:      pdf.Rectangle{LLx: 150, LLy: 150, URx: 250, URy: 200},
				Name:      "circle-001",
				User:      pdf.Text("Designer"),
				Subject:   pdf.Text("Color circle"),
				FillColor: color.DeviceRGB{0.0, 1.0, 0.0}, // Green interior
			},
		},
		{
			name: "circle with all fields",
			annotation: &annotation.Circle{
				Common: annotation.Common{
					Rect:     pdf.Rectangle{LLx: 300, LLy: 300, URx: 450, URy: 450},
					Contents: pdf.Text("Complex circle"),
					Color:    color.DeviceGray(0.5), // gray
				},
				User:      pdf.Text("Reviewer"),
				Subject:   pdf.Text("Complex circle annotation"),
				Intent:    "CircleCloud",
				FillColor: color.DeviceRGB{1.0, 1.0, 0.0},    // Yellow interior
				Margin:    []float64{10.0, 10.0, 10.0, 10.0}, // Rectangle differences
			},
		},
	},
	"Polygon": {
		{
			name: "basic polygon annotation",
			annotation: &annotation.Polygon{
				Rect:     pdf.Rectangle{LLx: 50, LLy: 50, URx: 200, URy: 150},
				Contents: pdf.Text("Triangle polygon"),
				User:     pdf.Text("Author"),
				Subject:  pdf.Text("Triangle shape"),
				Vertices: []float64{100, 50, 50, 150, 200, 150}, // Triangle vertices
			},
		},
		{
			name: "polygon with interior color and border effect",
			annotation: &annotation.Polygon{
				Rect:      pdf.Rectangle{LLx: 100, LLy: 100, URx: 300, URy: 250},
				Name:      "polygon-001",
				User:      pdf.Text("Designer"),
				Subject:   pdf.Text("Colored polygon"),
				Intent:    "PolygonCloud",
				Vertices:  []float64{150, 100, 300, 150, 250, 250, 100, 200}, // Quadrilateral
				FillColor: color.DeviceRGB{0.0, 1.0, 0.5},                    // Green-cyan interior
				BorderStyle: &annotation.BorderStyle{
					Width: 2.0,
					Style: "S",
				},
				BorderEffect: &annotation.BorderEffect{
					Style:     "C",
					Intensity: 1.5,
				},
			},
		},
		{
			name: "polygon with path and measure",
			annotation: &annotation.Polygon{
				Rect:     pdf.Rectangle{LLx: 200, LLy: 200, URx: 400, URy: 350},
				Contents: pdf.Text("Complex polygon with path"),
				User:     pdf.Text("Engineer"),
				Subject:  pdf.Text("Measured polygon"),
				Intent:   "PolygonDimension",
				Vertices: []float64{250, 200, 400, 250, 350, 350, 200, 300},
				Path: [][]vec.Vec2{
					{{X: 250, Y: 200}}, // moveto
					{{X: 400, Y: 250}}, // lineto
					{{X: 350, Y: 350}}, // lineto
					{{X: 200, Y: 300}}, // lineto
				},
				Measure: &measure.RectilinearMeasure{
					ScaleRatio: pdf.Text("1 cm = 1 m"),
					XAxis: []*measure.NumberFormat{{
						Unit:             pdf.Text("m"),
						ConversionFactor: 0.01,
						Precision:        100,
						DecimalSeparator: pdf.Text("."),
					}},
					Distance: []*measure.NumberFormat{{
						Unit:             pdf.Text("m"),
						ConversionFactor: 0.01,
						Precision:        100,
						DecimalSeparator: pdf.Text("."),
					}},
					Area: []*measure.NumberFormat{{
						Unit:             pdf.Text("m²"),
						ConversionFactor: 0.0001,
						Precision:        100,
						DecimalSeparator: pdf.Text("."),
					}},
					SingleUse: true,
				},
			},
		},
	},
	"PolyLine": {
		{
			name: "basic polyline annotation",
			annotation: &annotation.PolyLine{
				Rect:            pdf.Rectangle{LLx: 75, LLy: 75, URx: 275, URy: 175},
				Contents:        pdf.Text("Simple polyline"),
				User:            pdf.Text("Author"),
				Subject:         pdf.Text("Line path"),
				Vertices:        []float64{75, 100, 150, 175, 225, 125, 275, 150},                                                   // Zigzag line
				LineEndingStyle: [2]annotation.LineEndingStyle{annotation.LineEndingStyleNone, annotation.LineEndingStyleOpenArrow}, // Arrow at end
			},
		},
		{
			name: "polyline with line endings and colors",
			annotation: &annotation.PolyLine{
				Rect:            pdf.Rectangle{LLx: 150, LLy: 150, URx: 350, URy: 250},
				Name:            "polyline-001",
				User:            pdf.Text("Designer"),
				Subject:         pdf.Text("Colored polyline"),
				Intent:          "PolyLineDimension",
				Vertices:        []float64{150, 200, 200, 150, 300, 250, 350, 180},
				LineEndingStyle: [2]annotation.LineEndingStyle{annotation.LineEndingStyleCircle, annotation.LineEndingStyleSquare}, // Different endings
				FillColor:       color.DeviceRGB{1.0, 0.0, 0.0},                                                                    // Red line endings
			},
		},
		{
			name: "polyline with curved path",
			annotation: &annotation.PolyLine{
				Rect:     pdf.Rectangle{LLx: 250, LLy: 250, URx: 450, URy: 350},
				Contents: pdf.Text("Curved polyline"),
				User:     pdf.Text("Artist"),
				Subject:  pdf.Text("Bezier curve"),
				Vertices: []float64{250, 300, 400, 300, 450, 320},
				Path: [][]vec.Vec2{
					{{X: 250, Y: 300}}, // moveto
					{{X: 300, Y: 250}, {X: 350, Y: 350}, {X: 400, Y: 300}}, // curveto
					{{X: 450, Y: 320}}, // lineto
				},
				LineEndingStyle: [2]annotation.LineEndingStyle{annotation.LineEndingStyleButt, annotation.LineEndingStyleDiamond},
			},
		},
	},
	"Highlight": {
		{
			name: "basic highlight annotation",
			annotation: &annotation.TextMarkup{
				Type:       annotation.TextMarkupTypeHighlight,
				Rect:       pdf.Rectangle{LLx: 100, LLy: 200, URx: 200, URy: 220},
				Contents:   pdf.Text("Highlighted text"),
				User:       pdf.Text("Reviewer"),
				Subject:    pdf.Text("Important passage"),
				QuadPoints: []vec.Vec2{{X: 100, Y: 200}, {X: 200, Y: 200}, {X: 200, Y: 220}, {X: 100, Y: 220}}, // Single word quad
			},
		},
		{
			name: "highlight with multiple quads",
			annotation: &annotation.TextMarkup{
				Type: annotation.TextMarkupTypeHighlight,
				Common: annotation.Common{
					Rect:  pdf.Rectangle{LLx: 50, LLy: 300, URx: 250, URy: 340},
					Name:  "highlight-001",
					Color: color.DeviceRGB{1.0, 1.0, 0.0}, // yellow
				},
				User:    pdf.Text("Student"),
				Subject: pdf.Text("Study notes"),
				QuadPoints: []vec.Vec2{
					{X: 50, Y: 300}, {X: 100, Y: 300}, {X: 100, Y: 320}, {X: 50, Y: 320}, // First word
					{X: 110, Y: 300}, {X: 160, Y: 300}, {X: 160, Y: 320}, {X: 110, Y: 320}, // Second word
					{X: 170, Y: 320}, {X: 250, Y: 320}, {X: 250, Y: 340}, {X: 170, Y: 340}, // Third word (different line)
				},
			},
		},
	},
	"Underline": {
		{
			name: "basic underline annotation",
			annotation: &annotation.TextMarkup{
				Type:       annotation.TextMarkupTypeUnderline,
				Rect:       pdf.Rectangle{LLx: 150, LLy: 400, URx: 300, URy: 420},
				Contents:   pdf.Text("Underlined text"),
				User:       pdf.Text("Editor"),
				Subject:    pdf.Text("Emphasis added"),
				QuadPoints: []vec.Vec2{{X: 150, Y: 400}, {X: 300, Y: 400}, {X: 300, Y: 420}, {X: 150, Y: 420}}, // Single phrase
			},
		},
		{
			name: "underline with markup fields",
			annotation: &annotation.TextMarkup{
				Type: annotation.TextMarkupTypeUnderline,
				Common: annotation.Common{
					Rect:  pdf.Rectangle{LLx: 75, LLy: 500, URx: 225, URy: 520},
					Name:  "underline-001",
					Color: color.DeviceRGB{0, 0, 1}, // blue
				},
				User:         pdf.Text("Proofreader"),
				Subject:      pdf.Text("Grammar correction"),
				CreationDate: time.Date(2023, 6, 15, 14, 30, 0, 0, time.UTC),
				QuadPoints:   []vec.Vec2{{X: 75, Y: 500}, {X: 225, Y: 500}, {X: 225, Y: 520}, {X: 75, Y: 520}},
			},
		},
	},
	"Squiggly": {
		{
			name: "basic squiggly annotation",
			annotation: &annotation.TextMarkup{
				Type:       annotation.TextMarkupTypeSquiggly,
				Rect:       pdf.Rectangle{LLx: 200, LLy: 600, URx: 350, URy: 620},
				Contents:   pdf.Text("Spelling error"),
				User:       pdf.Text("Spellchecker"),
				Subject:    pdf.Text("Possible misspelling"),
				QuadPoints: []vec.Vec2{{X: 200, Y: 600}, {X: 350, Y: 600}, {X: 350, Y: 620}, {X: 200, Y: 620}}, // Misspelled word
			},
		},
		{
			name: "squiggly with color",
			annotation: &annotation.TextMarkup{
				Type: annotation.TextMarkupTypeSquiggly,
				Common: annotation.Common{
					Rect:  pdf.Rectangle{LLx: 100, LLy: 700, URx: 180, URy: 720},
					Name:  "squiggly-001",
					Color: color.DeviceRGB{1, 0.5, 0}, // orange
				},
				User:       pdf.Text("Grammar checker"),
				Subject:    pdf.Text("Grammar issue"),
				QuadPoints: []vec.Vec2{{X: 100, Y: 700}, {X: 180, Y: 700}, {X: 180, Y: 720}, {X: 100, Y: 720}},
			},
		},
	},
	"StrikeOut": {
		{
			name: "basic strikeout annotation",
			annotation: &annotation.TextMarkup{
				Type:       annotation.TextMarkupTypeStrikeOut,
				Rect:       pdf.Rectangle{LLx: 250, LLy: 800, URx: 400, URy: 820},
				Contents:   pdf.Text("Deleted text"),
				User:       pdf.Text("Editor"),
				Subject:    pdf.Text("Text to be removed"),
				QuadPoints: []vec.Vec2{{X: 250, Y: 800}, {X: 400, Y: 800}, {X: 400, Y: 820}, {X: 250, Y: 820}}, // Struck-out phrase
			},
		},
		{
			name: "strikeout with multiple sections",
			annotation: &annotation.TextMarkup{
				Type:    annotation.TextMarkupTypeStrikeOut,
				Rect:    pdf.Rectangle{LLx: 50, LLy: 900, URx: 300, URy: 940},
				Name:    "strikeout-001",
				User:    pdf.Text("Reviewer"),
				Subject: pdf.Text("Major revision"),
				QuadPoints: []vec.Vec2{
					{X: 50, Y: 900}, {X: 150, Y: 900}, {X: 150, Y: 920}, {X: 50, Y: 920}, // First section
					{X: 160, Y: 900}, {X: 250, Y: 900}, {X: 250, Y: 920}, {X: 160, Y: 920}, // Second section
					{X: 50, Y: 920}, {X: 300, Y: 920}, {X: 300, Y: 940}, {X: 50, Y: 940}, // Third section (different line)
				},
			},
		},
	},
	"Caret": {
		{
			name: "basic caret annotation",
			annotation: &annotation.Caret{
				Rect:     pdf.Rectangle{LLx: 100, LLy: 100, URx: 110, URy: 120},
				Contents: pdf.Text("Text insertion point"),
				User:     pdf.Text("Editor"),
				Subject:  pdf.Text("Insert text here"),
				Symbol:   "None",
			},
		},
		{
			name: "caret with paragraph symbol",
			annotation: &annotation.Caret{
				Rect:         pdf.Rectangle{LLx: 200, LLy: 200, URx: 220, URy: 230},
				Name:         "caret-001",
				User:         pdf.Text("Proofreader"),
				Subject:      pdf.Text("New paragraph needed"),
				CreationDate: time.Date(2023, 8, 20, 10, 15, 0, 0, time.UTC),
				Symbol:       "P", // Paragraph symbol
			},
		},
		{
			name: "caret with rectangle differences",
			annotation: &annotation.Caret{
				Rect:     pdf.Rectangle{LLx: 300, LLy: 300, URx: 330, URy: 335},
				Contents: pdf.Text("Complex caret with spacing"),
				User:     pdf.Text("Reviewer"),
				Subject:  pdf.Text("Spacing adjustment"),
				Margin:   []float64{2.0, 3.0, 2.0, 5.0}, // Rectangle differences
				Symbol:   "None",
			},
		},
	},
	"Stamp": {
		{
			name: "basic stamp annotation",
			annotation: &annotation.Stamp{
				Rect:     pdf.Rectangle{LLx: 50, LLy: 50, URx: 150, URy: 100},
				Contents: pdf.Text("Approved document"),
				User:     pdf.Text("Manager"),
				Subject:  pdf.Text("Document approval"),
				Intent:   "Stamp",
				Icon:     "Approved",
			},
		},
		{
			name: "stamp with default name",
			annotation: &annotation.Stamp{
				Rect:         pdf.Rectangle{LLx: 200, LLy: 200, URx: 300, URy: 250},
				Name:         "stamp-001",
				User:         pdf.Text("Editor"),
				Subject:      pdf.Text("Draft version"),
				CreationDate: time.Date(2023, 9, 10, 16, 30, 0, 0, time.UTC),
				Intent:       "Stamp",
				Icon:         "Draft", // Default value
			},
		},
		{
			name: "confidential stamp",
			annotation: &annotation.Stamp{
				Rect:     pdf.Rectangle{LLx: 100, LLy: 300, URx: 250, URy: 350},
				Contents: pdf.Text("Confidential material"),
				User:     pdf.Text("Security Officer"),
				Subject:  pdf.Text("Classification stamp"),
				Intent:   "Stamp",
				Icon:     "Confidential",
			},
		},
		{
			name: "stamp with intent",
			annotation: &annotation.Stamp{
				Rect:     pdf.Rectangle{LLx: 300, LLy: 400, URx: 450, URy: 450},
				Contents: pdf.Text("Image stamp"),
				Markup: annotation.Markup{
					User:   pdf.Text("Designer"),
					Intent: "StampImage", // Intent: image stamp
				},
				// Icon omitted when Intent != "Stamp" per PDF specification
			},
		},
		{
			name: "stamp with all standard values",
			annotation: &annotation.Stamp{
				Rect:     pdf.Rectangle{LLx: 150, LLy: 500, URx: 300, URy: 550},
				Contents: pdf.Text("Final document"),
				Markup: annotation.Markup{
					User:   pdf.Text("Publisher"),
					Intent: "Stamp", // Explicit rubber stamp intent
				},
				Icon: "Final",
			},
		},
	},
	"Ink": {
		{
			name: "simple ink annotation",
			annotation: &annotation.Ink{
				Rect:     pdf.Rectangle{LLx: 50, LLy: 50, URx: 200, URy: 150},
				Contents: pdf.Text("Handwritten note"),
				User:     pdf.Text("Annotator"),
				Subject:  pdf.Text("Hand drawing"),
				InkList: [][]vec.Vec2{
					{{X: 50, Y: 100}, {X: 75, Y: 120}, {X: 100, Y: 110}, {X: 125, Y: 130}, {X: 150, Y: 125}}, // Single stroke
				},
			},
		},
		{
			name: "multi-stroke ink annotation",
			annotation: &annotation.Ink{
				Common: annotation.Common{
					Rect:  pdf.Rectangle{LLx: 100, LLy: 200, URx: 300, URy: 350},
					Name:  "ink-001",
					Color: color.DeviceRGB{0.0, 0.0, 1.0}, // blue
				},
				User:         pdf.Text("Artist"),
				Subject:      pdf.Text("Sketch"),
				CreationDate: time.Date(2023, 10, 5, 14, 20, 0, 0, time.UTC),
				InkList: [][]vec.Vec2{
					{{X: 100, Y: 250}, {X: 150, Y: 280}, {X: 200, Y: 260}}, // First stroke
					{{X: 180, Y: 300}, {X: 220, Y: 320}, {X: 250, Y: 310}}, // Second stroke
					{{X: 120, Y: 320}, {X: 160, Y: 340}, {X: 190, Y: 330}}, // Third stroke
				},
			},
		},
		{
			name: "ink annotation with border style",
			annotation: &annotation.Ink{
				Rect:     pdf.Rectangle{LLx: 200, LLy: 400, URx: 400, URy: 500},
				Contents: pdf.Text("Stylized ink drawing"),
				User:     pdf.Text("Designer"),
				Subject:  pdf.Text("Stylized annotation"),
				InkList: [][]vec.Vec2{
					{{X: 200, Y: 450}, {X: 250, Y: 470}, {X: 300, Y: 460}, {X: 350, Y: 480}, {X: 400, Y: 475}},
				},
			},
		},
		{
			name: "ink annotation with path (PDF 2.0)",
			annotation: &annotation.Ink{
				Rect:     pdf.Rectangle{LLx: 150, LLy: 600, URx: 350, URy: 700},
				Contents: pdf.Text("Curved ink path"),
				User:     pdf.Text("Artist"),
				Subject:  pdf.Text("Bezier curve drawing"),
				InkList: [][]vec.Vec2{
					{{X: 150, Y: 650}, {X: 300, Y: 650}, {X: 350, Y: 670}},
				},
				Path: [][]vec.Vec2{
					{{X: 150, Y: 650}}, // moveto
					{{X: 200, Y: 620}, {X: 250, Y: 680}, {X: 300, Y: 650}}, // curveto
					{{X: 350, Y: 670}}, // lineto
				},
			},
		},
		{
			name: "complex ink annotation",
			annotation: &annotation.Ink{
				Rect:     pdf.Rectangle{LLx: 50, LLy: 750, URx: 400, URy: 850},
				Contents: pdf.Text("Complex freehand drawing"),
				User:     pdf.Text("User"),
				Subject:  pdf.Text("Signature and notes"),
				InkList: [][]vec.Vec2{
					{{X: 50, Y: 800}, {X: 80, Y: 810}, {X: 120, Y: 790}, {X: 160, Y: 805}, {X: 200, Y: 795}}, // Signature stroke 1
					{{X: 180, Y: 820}, {X: 220, Y: 830}, {X: 260, Y: 815}, {X: 300, Y: 825}},                 // Signature stroke 2
					{{X: 320, Y: 780}, {X: 350, Y: 790}, {X: 380, Y: 785}, {X: 400, Y: 795}},                 // Additional mark
				},
			},
		},
	},
	"Popup": {
		{
			name: "basic popup annotation",
			annotation: &annotation.Popup{
				Rect:     pdf.Rectangle{LLx: 100, LLy: 100, URx: 200, URy: 150},
				Contents: pdf.Text("Popup window content"),
			},
		},
		{
			name: "popup with parent reference",
			annotation: &annotation.Popup{
				Rect:   pdf.Rectangle{LLx: 150, LLy: 200, URx: 250, URy: 280},
				Name:   "popup-001",
				Parent: pdf.NewReference(123, 0), // Reference to parent annotation
			},
		},
		{
			name: "popup initially open",
			annotation: &annotation.Popup{
				Rect:     pdf.Rectangle{LLx: 200, LLy: 300, URx: 350, URy: 400},
				Contents: pdf.Text("Initially visible popup"),
				Open:     true, // Initially displayed open
			},
		},
		{
			name: "popup with parent and open state",
			annotation: &annotation.Popup{
				Rect:   pdf.Rectangle{LLx: 50, LLy: 400, URx: 180, URy: 480},
				Name:   "popup-with-parent",
				Parent: pdf.NewReference(456, 0), // Parent markup annotation
				Open:   true,                     // Initially open
			},
		},
		{
			name: "minimal popup annotation",
			annotation: &annotation.Popup{
				Rect: pdf.Rectangle{LLx: 300, LLy: 500, URx: 400, URy: 550},
				// No parent, not initially open (all defaults)
			},
		},
	},
	"FileAttachment": {
		{
			name: "basic file attachment",
			annotation: &annotation.FileAttachment{
				Rect:     pdf.Rectangle{LLx: 50, LLy: 50, URx: 80, URy: 80},
				Contents: pdf.Text("Attached spreadsheet file"),
				User:     pdf.Text("Data Analyst"),
				Subject:  pdf.Text("Supporting data"),
				FS: &file.Specification{
					FileName:       "spreadsheet.xlsx",
					AFRelationship: file.RelationshipUnspecified,
				},
				Icon: annotation.FileAttachmentIconGraph, // Icon for data files
			},
		},
		{
			name: "file attachment with default icon",
			annotation: &annotation.FileAttachment{
				Rect:         pdf.Rectangle{LLx: 150, LLy: 150, URx: 180, URy: 180},
				Name:         "attachment-001",
				User:         pdf.Text("Author"),
				Subject:      pdf.Text("Supporting document"),
				CreationDate: time.Date(2023, 11, 15, 9, 30, 0, 0, time.UTC),
				FS: &file.Specification{
					FileName:       "document.pdf",
					AFRelationship: file.RelationshipUnspecified,
				},
				Icon: annotation.FileAttachmentIconPushPin, // Default value
			},
		},
		{
			name: "paperclip attachment",
			annotation: &annotation.FileAttachment{
				Rect:     pdf.Rectangle{LLx: 250, LLy: 250, URx: 280, URy: 280},
				Contents: pdf.Text("Attached document"),
				User:     pdf.Text("Secretary"),
				Subject:  pdf.Text("Office document"),
				FS: &file.Specification{
					FileName:       "attachment.docx",
					AFRelationship: file.RelationshipUnspecified,
				},
				Icon: annotation.FileAttachmentIconPaperclip, // Paperclip icon
			},
		},
		{
			name: "tag attachment with metadata",
			annotation: &annotation.FileAttachment{
				Common: annotation.Common{
					Rect:     pdf.Rectangle{LLx: 100, LLy: 350, URx: 130, URy: 380},
					Contents: pdf.Text("Tagged reference file"),
					Color:    color.DeviceGray(0.8), // light gray
				},
				User:    pdf.Text("Librarian"),
				Subject: pdf.Text("Reference material"),
				FS: &file.Specification{
					FileName:       "reference.pdf",
					AFRelationship: file.RelationshipUnspecified,
				},
				Icon: annotation.FileAttachmentIconTag, // Tag icon
			},
		},
		{
			name: "minimal file attachment",
			annotation: &annotation.FileAttachment{
				Rect: pdf.Rectangle{LLx: 300, LLy: 450, URx: 330, URy: 480},
				User: pdf.Text("User"),
				FS: &file.Specification{
					FileName:       "file.dat",
					AFRelationship: file.RelationshipUnspecified,
				},
				Icon: annotation.FileAttachmentIconPushPin, // Default value
			},
		},
	},
	"Sound": {
		{
			name: "basic sound annotation",
			annotation: &annotation.Sound{
				Rect:     pdf.Rectangle{LLx: 100, LLy: 100, URx: 130, URy: 130},
				Contents: pdf.Text("Recorded audio note"),
				User:     pdf.Text("Narrator"),
				Subject:  pdf.Text("Audio explanation"),
				Sound:    testSound,
				Icon:     "Speaker", // Default speaker icon
			},
		},
		{
			name: "microphone sound annotation",
			annotation: &annotation.Sound{
				Rect:         pdf.Rectangle{LLx: 200, LLy: 200, URx: 230, URy: 230},
				Name:         "sound-001",
				User:         pdf.Text("Reporter"),
				Subject:      pdf.Text("Field recording"),
				CreationDate: time.Date(2023, 12, 1, 15, 45, 0, 0, time.UTC),
				Sound:        testSound,
				Icon:         "Mic", // Microphone icon
			},
		},
		{
			name: "sound annotation with default icon",
			annotation: &annotation.Sound{
				Rect:     pdf.Rectangle{LLx: 300, LLy: 300, URx: 330, URy: 330},
				Contents: pdf.Text("Audio clip"),
				User:     pdf.Text("Audio Engineer"),
				Subject:  pdf.Text("Sound sample"),
				Sound:    testSound,
				Icon:     "Speaker", // Default value
			},
		},
		{
			name: "sound annotation with metadata",
			annotation: &annotation.Sound{
				Common: annotation.Common{
					Rect:     pdf.Rectangle{LLx: 150, LLy: 400, URx: 180, URy: 430},
					Contents: pdf.Text("Interview recording"),
					Color:    color.DeviceRGB{0.5, 0.5, 0.5}, // gray
				},
				User:    pdf.Text("Journalist"),
				Subject: pdf.Text("Interview audio"),
				Sound:   testSound,
				Icon:    "Speaker", // Explicit speaker icon
			},
		},
		{
			name: "minimal sound annotation",
			annotation: &annotation.Sound{
				Rect:  pdf.Rectangle{LLx: 50, LLy: 500, URx: 80, URy: 530},
				User:  pdf.Text("User"),
				Sound: testSound,
				Icon:  "Speaker", // Default value
			},
		},
	},
	"Movie": {
		{
			name: "basic movie annotation",
			annotation: &annotation.Movie{
				Rect:       pdf.Rectangle{LLx: 100, LLy: 100, URx: 300, URy: 200},
				Contents:   pdf.Text("Educational video"),
				Title:      pdf.Text("Introduction Video"),
				Movie:      testMovie,
				Activation: movie.DefaultActivation,
			},
		},
		{
			name: "movie annotation, do not play",
			annotation: &annotation.Movie{
				Rect:  pdf.Rectangle{LLx: 200, LLy: 200, URx: 400, URy: 300},
				Name:  "movie-001",
				Title: pdf.Text("Training Module 1"),
				Movie: testMovie,
				// Activation nil: do not play on activation (A=false)
			},
		},
		{
			name: "movie annotation with custom activation",
			annotation: &annotation.Movie{
				Common: annotation.Common{
					Rect:     pdf.Rectangle{LLx: 50, LLy: 50, URx: 250, URy: 150},
					Contents: pdf.Text("Interactive presentation"),
					Color:    color.DeviceRGB{0.7, 0.6, 0.5}, // brown
				},
				Title: pdf.Text("Interactive Demo"),
				Movie: testMovie,
				Activation: &movie.Activation{
					Rate:         1.0,
					Volume:       0.75,
					Mode:         movie.ModeRepeat,
					ShowControls: true,
				},
			},
		},
		{
			name: "movie annotation with default activation",
			annotation: &annotation.Movie{
				Rect:       pdf.Rectangle{LLx: 300, LLy: 300, URx: 500, URy: 400},
				Movie:      testMovie,
				Activation: movie.DefaultActivation,
			},
		},
		{
			name: "minimal movie annotation",
			annotation: &annotation.Movie{
				Rect:       pdf.Rectangle{LLx: 0, LLy: 0, URx: 100, URy: 100},
				Movie:      testMovie,
				Activation: movie.DefaultActivation,
			},
		},
	},
	"Screen": {
		{
			name: "basic screen annotation",
			annotation: &annotation.Screen{
				Rect:     pdf.Rectangle{LLx: 100, LLy: 100, URx: 400, URy: 300},
				Contents: pdf.Text("Media playback region"),
				Title:    pdf.Text("Video Player"),
			},
		},
		{
			name: "screen annotation with appearance characteristics",
			annotation: &annotation.Screen{
				Rect:  pdf.Rectangle{LLx: 200, LLy: 200, URx: 500, URy: 400},
				Name:  "screen-001",
				Title: pdf.Text("Interactive Media"),
				Style: &appearance.Characteristics{
					BorderColor:     color.DeviceRGB{0, 0, 1},
					BackgroundColor: color.DeviceGray(0.5),
					Rotation:        90,
				},
			},
		},
		{
			name: "screen annotation with action",
			annotation: &annotation.Screen{
				Rect:     pdf.Rectangle{LLx: 50, LLy: 50, URx: 350, URy: 250},
				Contents: pdf.Text("Click to play video"),
				Color:    color.DeviceGray(0),
				Title:    pdf.Text("Action Trigger"),
				Action:   &action.URI{URI: "https://example.com/play"},
			},
		},
		{
			name: "screen annotation with additional actions",
			annotation: &annotation.Screen{
				Rect:  pdf.Rectangle{LLx: 300, LLy: 300, URx: 600, URy: 500},
				Title: pdf.Text("Advanced Media Control"),
				AA:    &triggers.Annotation{Enter: &action.URI{URI: "https://example.com/enter"}},
			},
		},
		{
			name: "screen annotation with all features",
			annotation: &annotation.Screen{
				Rect:     pdf.Rectangle{LLx: 150, LLy: 150, URx: 450, URy: 350},
				Contents: pdf.Text("Full-featured media player"),
				Color:    color.DeviceRGB{0.9, 0.9, 0.9},
				Title:    pdf.Text("Complete Media Player"),
				Style: &appearance.Characteristics{
					BorderColor:     color.DeviceCMYK{0, 0, 0, 1},
					Caption:         pdf.Text("Play"),
					RolloverCaption: pdf.Text("Hover"),
					DownCaption:     pdf.Text("Pressed"),
					TextPosition:    appearance.TextPositionCaptionBelowIcon,
					IconFit: &appearance.IconFit{
						ScaleWhen:     appearance.IconScaleWhenBigger,
						Scaling:       appearance.IconScalingProportional,
						LeftoverSpace: &[2]float64{0.5, 0.5},
						FitToBounds:   true,
					},
				},
				Action: &action.URI{URI: "https://example.com/media"},
				AA:     &triggers.Annotation{Exit: &action.URI{URI: "https://example.com/exit"}},
			},
		},
		{
			name: "minimal screen annotation",
			annotation: &annotation.Screen{
				Rect: pdf.Rectangle{LLx: 0, LLy: 0, URx: 200, URy: 100},
				// Only required fields from Common
			},
		},
	},
	"Widget": {
		{
			name: "basic widget annotation",
			annotation: &annotation.Widget{
				Rect:      pdf.Rectangle{LLx: 100, LLy: 100, URx: 300, URy: 130},
				Contents:  pdf.Text("Text input field"),
				Highlight: annotation.HighlightInvert,
			},
		},
		{
			name: "widget with push highlighting",
			annotation: &annotation.Widget{
				Rect:      pdf.Rectangle{LLx: 200, LLy: 200, URx: 350, URy: 240},
				Name:      "button-001",
				Highlight: annotation.HighlightPush,
			},
		},
		{
			name: "widget with appearance characteristics",
			annotation: &annotation.Widget{
				Rect:      pdf.Rectangle{LLx: 50, LLy: 50, URx: 250, URy: 80},
				Contents:  pdf.Text("Button with custom appearance"),
				Highlight: annotation.HighlightOutline,
				Style: &appearance.Characteristics{
					BorderColor:     color.DeviceGray(0),
					BackgroundColor: color.DeviceRGB{1, 1, 0},
					Caption:         pdf.Text("OK"),
					SingleUse:       true,
				},
			},
		},
		{
			name: "widget with action and additional actions",
			annotation: &annotation.Widget{
				Rect:      pdf.Rectangle{LLx: 300, LLy: 300, URx: 500, URy: 340},
				Color:     color.DeviceCMYK{0.1, 0.2, 0.3, 0.4},
				Highlight: annotation.HighlightInvert,
				Action:    &action.URI{URI: "https://example.com/widget-activate"},
				AA:        &triggers.Annotation{Up: &action.URI{URI: "https://example.com/widget-up"}},
			},
		},
		{
			name: "widget with border style",
			annotation: &annotation.Widget{
				Rect:      pdf.Rectangle{LLx: 150, LLy: 150, URx: 400, URy: 190},
				Contents:  pdf.Text("Field with custom border"),
				Highlight: annotation.HighlightNone,
				BorderStyle: &annotation.BorderStyle{
					Width: 2,
					Style: "S",
				},
			},
		},
		{
			// a widget's /Parent is owned by the field tree, not round-tripped on a
			// standalone widget, so it is not set here (see the form tests).
			name: "widget without parent",
			annotation: &annotation.Widget{
				Rect:      pdf.Rectangle{LLx: 250, LLy: 250, URx: 450, URy: 290},
				Name:      "child-widget-001",
				Highlight: annotation.HighlightOutline,
			},
		},
		{
			name: "widget with all features",
			annotation: &annotation.Widget{
				Rect:      pdf.Rectangle{LLx: 100, LLy: 400, URx: 350, URy: 450},
				Contents:  pdf.Text("Complete widget annotation"),
				Color:     color.DeviceRGB{0.8, 0.9, 1.0},
				Highlight: annotation.HighlightPush,
				Style: &appearance.Characteristics{
					Rotation:     180,
					BorderColor:  color.DeviceRGB{0.25, 0.5, 0.75},
					TextPosition: appearance.TextPositionIconOnly,
					IconFit: &appearance.IconFit{
						ScaleWhen: appearance.IconScaleNever,
						SingleUse: true,
					},
				},
				Action: &action.Named{N: "NextPage"},
				AA:     &triggers.Annotation{Focus: &action.URI{URI: "https://example.com/focus"}},
				BorderStyle: &annotation.BorderStyle{
					Width:     1.5,
					Style:     "D",
					DashArray: []float64{5, 2},
				},
			},
		},
		{
			name: "minimal widget annotation",
			annotation: &annotation.Widget{
				Rect:      pdf.Rectangle{LLx: 0, LLy: 0, URx: 100, URy: 30},
				Highlight: annotation.HighlightInvert,
			},
		},
	},
	"PrinterMark": {
		{
			name: "basic printer's mark annotation",
			annotation: &annotation.PrinterMark{
				Rect:     pdf.Rectangle{LLx: 50, LLy: 50, URx: 100, URy: 100},
				Contents: pdf.Text("Registration target"),
				Flags:    annotation.FlagPrint | annotation.FlagReadOnly,
				MarkName: "RegistrationTarget",
			},
		},
		{
			name: "color bar printer's mark",
			annotation: &annotation.PrinterMark{
				Rect:     pdf.Rectangle{LLx: 200, LLy: 780, URx: 400, URy: 800},
				Name:     "colorbar-001",
				Flags:    annotation.FlagPrint | annotation.FlagReadOnly,
				MarkName: "ColorBar",
			},
		},
		{
			name: "cut mark printer's mark",
			annotation: &annotation.PrinterMark{
				Rect:     pdf.Rectangle{LLx: 0, LLy: 0, URx: 20, URy: 20},
				Contents: pdf.Text("Corner cut mark"),
				Color:    color.DeviceRGB{0, 0, 0},
				Flags:    annotation.FlagPrint | annotation.FlagReadOnly,
				MarkName: "CutMark",
			},
		},
		{
			name: "printer's mark with border",
			annotation: &annotation.PrinterMark{
				Rect:     pdf.Rectangle{LLx: 100, LLy: 100, URx: 150, URy: 150},
				Contents: pdf.Text("Registration mark with border"),
				Border: &annotation.Border{
					HCornerRadius: 2.0,
					VCornerRadius: 2.0,
					Width:         2.0,
				},
				Flags:    annotation.FlagPrint | annotation.FlagReadOnly,
				MarkName: "RegistrationTarget",
			},
		},
		{
			name: "printer's mark with print and readonly flags",
			annotation: &annotation.PrinterMark{
				Rect:     pdf.Rectangle{LLx: 150, LLy: 150, URx: 200, URy: 200},
				Flags:    annotation.FlagPrint | annotation.FlagReadOnly,
				MarkName: "GrayRamp",
			},
		},
		{
			name: "minimal printer's mark annotation",
			annotation: &annotation.PrinterMark{
				Rect:  pdf.Rectangle{LLx: 300, LLy: 300, URx: 320, URy: 320},
				Flags: annotation.FlagPrint | annotation.FlagReadOnly,
				// MN field is optional, not specified here
			},
		},
	},
	"TrapNet": {
		{
			name: "trap network with LastModified",
			annotation: &annotation.TrapNet{
				Rect:         pdf.Rectangle{LLx: 0, LLy: 0, URx: 612, URy: 792}, // Full page
				Flags:        annotation.FlagPrint | annotation.FlagReadOnly,
				LastModified: time.Date(2023, 12, 15, 10, 30, 0, 0, time.UTC),
			},
		},
		{
			name: "trap network with Version and AnnotStates",
			annotation: &annotation.TrapNet{
				Rect:  pdf.Rectangle{LLx: 0, LLy: 0, URx: 612, URy: 792}, // Full page
				Flags: annotation.FlagPrint | annotation.FlagReadOnly,
				Name:  "trapnet-001",
				Version: []pdf.Reference{
					pdf.NewReference(100, 0), // Content stream
					pdf.NewReference(101, 0), // Resource object
					pdf.NewReference(102, 0), // Form XObject
				},
				AnnotStates: []pdf.Name{"N", "Off", ""}, // Mixed states including null
			},
		},
		{
			name: "trap network with FontFauxing",
			annotation: &annotation.TrapNet{
				Rect:         pdf.Rectangle{LLx: 0, LLy: 0, URx: 612, URy: 792}, // Full page
				Flags:        annotation.FlagPrint | annotation.FlagReadOnly,
				Contents:     pdf.Text("Trap network with font substitutions"),
				LastModified: time.Date(2023, 12, 1, 12, 0, 0, 0, time.UTC),
				FontFauxing: []pdf.Reference{
					pdf.NewReference(200, 0), // Substitute font 1
					pdf.NewReference(201, 0), // Substitute font 2
				},
			},
		},
		{
			name: "complex trap network",
			annotation: &annotation.TrapNet{
				Rect:  pdf.Rectangle{LLx: 0, LLy: 0, URx: 612, URy: 792}, // Full page
				Flags: annotation.FlagPrint | annotation.FlagReadOnly,
				Name:  "complex-trapnet",
				Version: []pdf.Reference{
					pdf.NewReference(300, 0), // Page content stream
					pdf.NewReference(301, 0), // Font resource
					pdf.NewReference(302, 0), // Image resource
					pdf.NewReference(303, 0), // Graphics state
				},
				AnnotStates: []pdf.Name{"N", "Off", "", "Hover"}, // Various annotation states
				FontFauxing: []pdf.Reference{
					pdf.NewReference(400, 0), // Helvetica substitute
				},
			},
		},
		{
			name: "minimal trap network",
			annotation: &annotation.TrapNet{
				Rect:         pdf.Rectangle{LLx: 0, LLy: 0, URx: 612, URy: 792}, // Full page
				Flags:        annotation.FlagPrint | annotation.FlagReadOnly,
				LastModified: time.Date(2023, 12, 1, 0, 0, 0, 0, time.UTC), // Minimal required field
			},
		},
	},
	"Watermark": {
		{
			name: "basic watermark annotation",
			annotation: &annotation.Watermark{
				Rect:     pdf.Rectangle{LLx: 0, LLy: 0, URx: 200, URy: 100},
				Contents: pdf.Text("Confidential watermark"),
			},
		},
		{
			name: "watermark with fixed print dictionary",
			annotation: &annotation.Watermark{
				Rect: pdf.Rectangle{LLx: 100, LLy: 100, URx: 300, URy: 150},
				Name: "watermark-001",
				FixedPrint: &annotation.FixedPrint{
					Matrix: matrix.Matrix{1, 0, 0, 1, 72, -72}, // Translate 1 inch right and down
					H:      0.0,                                // Left edge
					V:      1.0,                                // Top edge
				},
			},
		},
		{
			name: "watermark with percentage positioning",
			annotation: &annotation.Watermark{
				Rect:     pdf.Rectangle{LLx: 0, LLy: 0, URx: 150, URy: 50},
				Contents: pdf.Text("Draft - Do Not Distribute"),
				FixedPrint: &annotation.FixedPrint{
					Matrix: matrix.Identity,
					H:      0.5, // Center horizontally (50%)
					V:      0.1, // Near top (10% from top)
				},
			},
		},
		{
			name: "watermark with rotation matrix",
			annotation: &annotation.Watermark{
				Rect:     pdf.Rectangle{LLx: 0, LLy: 0, URx: 200, URy: 30},
				Contents: pdf.Text("SAMPLE"),
				FixedPrint: &annotation.FixedPrint{
					Matrix: matrix.Matrix{0.707, 0.707, -0.707, 0.707, 0, 0}, // 45-degree rotation
					H:      0.25,                                             // 25% from left
					V:      0.75,                                             // 75% from bottom
				},
			},
		},
		{
			name: "watermark with default values",
			annotation: &annotation.Watermark{
				Rect: pdf.Rectangle{LLx: 50, LLy: 50, URx: 250, URy: 100},
				FixedPrint: &annotation.FixedPrint{
					Matrix: matrix.Identity,
					H:      0.0, // Default horizontal position
					V:      0.0, // Default vertical position
				},
			},
		},
		{
			name: "watermark bottom-right corner",
			annotation: &annotation.Watermark{
				Rect:     pdf.Rectangle{LLx: 0, LLy: 0, URx: 100, URy: 20},
				Contents: pdf.Text("© 2023 Company Name"),
				FixedPrint: &annotation.FixedPrint{
					Matrix: matrix.Matrix{1, 0, 0, 1, -20, 20}, // Offset from edge for margins
					H:      0.95,                               // 95% from left (near right edge)
					V:      0.05,                               // 5% from bottom (near bottom edge)
				},
			},
		},
		{
			name: "minimal watermark annotation",
			annotation: &annotation.Watermark{
				Rect: pdf.Rectangle{LLx: 0, LLy: 0, URx: 100, URy: 50},
				// No FixedPrint dictionary - should be drawn without special media consideration
			},
		},
		{
			name: "watermark with inline fixed print",
			annotation: &annotation.Watermark{
				Rect: pdf.Rectangle{LLx: 0, LLy: 0, URx: 100, URy: 50},
				FixedPrint: &annotation.FixedPrint{
					Matrix:    matrix.Matrix{1, 0, 0, 1, 10, 10},
					H:         0.2,
					SingleUse: true, // written as a direct dictionary
				},
			},
		},
		{
			name: "watermark with default fixed print",
			annotation: &annotation.Watermark{
				Rect: pdf.Rectangle{LLx: 0, LLy: 0, URx: 100, URy: 50},
				FixedPrint: &annotation.FixedPrint{
					Matrix: matrix.Identity, // absent Matrix decodes to identity
				},
			},
		},
	},
	"3D": {
		{
			name: "basic 3D annotation",
			annotation: &annotation.Annot3D{
				Rect: pdf.Rectangle{LLx: 100, LLy: 100, URx: 400, URy: 300},
				D:    pdf.NewReference(100, 0), // 3D stream reference
				I:    true,                     // Default value for interactive flag
			},
		},
		{
			name: "3D annotation with view specification",
			annotation: &annotation.Annot3D{
				Rect:     pdf.Rectangle{LLx: 50, LLy: 50, URx: 350, URy: 250},
				Contents: pdf.Text("Interactive 3D Model"),
				Name:     "3d-model-001",
				D:        pdf.NewReference(200, 0), // 3D stream reference
				V:        pdf.Integer(0),           // View index
				I:        true,                     // Default value for interactive flag
			},
		},
		{
			name: "3D annotation with activation dictionary",
			annotation: &annotation.Annot3D{
				Rect: pdf.Rectangle{LLx: 0, LLy: 0, URx: 300, URy: 200},
				D:    pdf.NewReference(300, 0), // 3D stream reference
				A: &annotation.Annot3DActivation{
					A:   "PO",  // Activate on page open
					AIS: "L",   // Live state
					D:   "PC",  // Deactivate on page close
					DIS: "U",   // Uninstantiated state
					TB:  false, // No toolbar
					NP:  true,  // Show navigation panel
				},
			},
		},
		{
			name: "3D annotation with view box",
			annotation: &annotation.Annot3D{
				Rect: pdf.Rectangle{LLx: 0, LLy: 0, URx: 400, URy: 300},
				D:    pdf.NewReference(400, 0),                                 // 3D stream reference
				B:    &pdf.Rectangle{LLx: -150, LLy: -100, URx: 150, URy: 100}, // View box in target coordinates
				I:    true,                                                     // Default value for interactive flag
			},
		},
		{
			name: "3D annotation with PDF 2.0 features",
			annotation: &annotation.Annot3D{
				Rect: pdf.Rectangle{LLx: 100, LLy: 100, URx: 500, URy: 400},
				Name: "advanced-3d-model",
				D:    pdf.NewReference(500, 0), // 3D stream reference
				A: &annotation.Annot3DActivation{
					A:           "XA",                     // Explicit activation
					AIS:         "I",                      // Instantiated state
					Style:       "Windowed",               // Windowed display
					Window:      pdf.NewReference(501, 0), // Window dictionary
					Transparent: true,                     // Transparent background
				},
				U:   pdf.NewReference(502, 0), // Units dictionary
				GEO: pdf.NewReference(503, 0), // Geospatial information
			},
		},
		{
			name: "3D annotation with string view reference",
			annotation: &annotation.Annot3D{
				Rect: pdf.Rectangle{LLx: 200, LLy: 200, URx: 600, URy: 500},
				D:    pdf.NewReference(600, 0),  // 3D stream reference
				V:    pdf.String("DefaultView"), // View name
				I:    false,                     // Not interactive (non-default)
			},
		},
		{
			name: "3D annotation with name view reference",
			annotation: &annotation.Annot3D{
				Rect: pdf.Rectangle{LLx: 0, LLy: 0, URx: 400, URy: 300},
				D:    pdf.NewReference(700, 0), // 3D stream reference
				V:    pdf.Name("F"),            // First view in VA array
				I:    true,                     // Default value for interactive flag
			},
		},
		{
			name: "comprehensive 3D annotation",
			annotation: &annotation.Annot3D{
				Common: annotation.Common{
					Rect:     pdf.Rectangle{LLx: 50, LLy: 50, URx: 450, URy: 350},
					Contents: pdf.Text("Complex 3D Scene with Multiple Features"),
					Name:     "complex-3d-scene",
					Flags:    1, // Invisible flag
				},
				D: pdf.NewReference(800, 0), // 3D stream reference
				V: pdf.Integer(2),           // Third view (index 2)
				A: &annotation.Annot3DActivation{
					A:   "PV", // Activate on page visible
					AIS: "L",  // Live state
					D:   "PI", // Deactivate on page invisible
					DIS: "I",  // Instantiated state
					// TB: true, NP: false, Style: "Embedded", Transparent: false are defaults and won't be written
				},
				I: true,                                                     // Interactive
				B: &pdf.Rectangle{LLx: -200, LLy: -150, URx: 200, URy: 150}, // Custom view box
			},
		},
		{
			name: "minimal 3D annotation",
			annotation: &annotation.Annot3D{
				Rect: pdf.Rectangle{LLx: 0, LLy: 0, URx: 200, URy: 150},
				D:    pdf.NewReference(900, 0), // 3D stream reference (required)
				I:    true,                     // Default interactive flag
			},
		},
	},
	"Redact": {
		{
			name: "basic redaction annotation",
			annotation: &annotation.Redact{
				Rect: pdf.Rectangle{LLx: 100, LLy: 100, URx: 300, URy: 120},
			},
		},
		{
			name: "redaction with QuadPoints",
			annotation: &annotation.Redact{
				Rect:     pdf.Rectangle{LLx: 50, LLy: 200, URx: 400, URy: 250},
				Contents: pdf.Text("Redacted content area"),
				QuadPoints: []float64{
					100, 210, 200, 210, 200, 240, 100, 240, // First quadrilateral
					250, 210, 350, 210, 350, 240, 250, 240, // Second quadrilateral
				},
			},
		},
		{
			name: "redaction with interior color",
			annotation: &annotation.Redact{
				Rect:      pdf.Rectangle{LLx: 0, LLy: 0, URx: 200, URy: 50},
				Name:      "redact-001",
				FillColor: color.DeviceRGB{0.0, 0.0, 0.0}, // Black interior color
			},
		},
		{
			name: "redaction with overlay text",
			annotation: &annotation.Redact{
				Rect:              pdf.Rectangle{LLx: 100, LLy: 300, URx: 500, URy: 350},
				OverlayText:       pdf.Text("CONFIDENTIAL"),
				DefaultAppearance: "/Helvetica 12 Tf 1 0 0 rg", // Red text appearance
				Align:             pdf.TextAlignCenter,         // Centered
			},
		},
		{
			name: "redaction with repeating overlay text",
			annotation: &annotation.Redact{
				Rect:              pdf.Rectangle{LLx: 50, LLy: 400, URx: 550, URy: 450},
				Contents:          pdf.Text("Large redacted area"),
				OverlayText:       pdf.Text("REDACTED"),
				DefaultAppearance: "/Arial 10 Tf 0.5 0.5 0.5 rg", // Gray text
				Repeat:            true,
				Align:             pdf.TextAlignRight, // Right-justified
			},
		},
		{
			name: "redaction with form XObject overlay",
			annotation: &annotation.Redact{
				Rect: pdf.Rectangle{LLx: 200, LLy: 500, URx: 400, URy: 550},
				Name: "redact-with-xobject",
				RO:   pdf.NewReference(100, 0), // Form XObject reference
			},
		},
		{
			name: "comprehensive redaction annotation",
			annotation: &annotation.Redact{
				Common: annotation.Common{
					Rect:     pdf.Rectangle{LLx: 50, LLy: 600, URx: 450, URy: 700},
					Contents: pdf.Text("Sensitive information requiring redaction"),
					Name:     "comprehensive-redact",
					Flags:    4, // Print flag
				},
				Markup: annotation.Markup{
					User:      pdf.Text("Redaction Tool"),
					RC:        &pdf.StringOrStream{Value: pdf.Text("This content contains sensitive information")},
					InReplyTo: pdf.NewReference(200, 0), // In reply to reference
				},
				QuadPoints: []float64{
					75, 620, 175, 620, 175, 640, 75, 640, // First quad
					200, 620, 300, 620, 300, 640, 200, 640, // Second quad
					325, 620, 425, 620, 425, 640, 325, 640, // Third quad
				},
				FillColor:         color.DeviceRGB{0.8, 0.8, 0.8}, // Light gray background
				OverlayText:       pdf.Text("CLASSIFIED"),
				DefaultAppearance: "/Times-Bold 14 Tf 1 0 0 rg", // Bold red text
				Repeat:            false,
				Align:             pdf.TextAlignCenter, // Centered
			},
		},
		{
			name: "redaction with markup properties",
			annotation: &annotation.Redact{
				Common: annotation.Common{
					Rect:  pdf.Rectangle{LLx: 100, LLy: 750, URx: 300, URy: 780},
					Flags: 1, // Invisible flag
				},
				User:      pdf.Text("Security Officer"),
				Subject:   pdf.Text("Personal Information Redaction"),
				FillColor: color.DeviceRGB{1.0, 1.0, 0.0}, // Yellow background
			},
		},
		{
			name: "minimal redaction annotation",
			annotation: &annotation.Redact{
				Rect: pdf.Rectangle{LLx: 0, LLy: 0, URx: 100, URy: 20},
				// Only required fields (Subtype is automatically set)
			},
		},
	},
	"Projection": {
		{
			name: "basic projection annotation",
			annotation: &annotation.Projection{
				Rect:     pdf.Rectangle{LLx: 100, LLy: 100, URx: 200, URy: 150},
				Contents: pdf.Text("3D measurement comment"),
				User:     pdf.Text("3D Analyst"),
				Subject:  pdf.Text("Measurement annotation"),
			},
		},
		{
			name: "projection with external data dictionary",
			annotation: &annotation.Projection{
				Rect: pdf.Rectangle{LLx: 50, LLy: 200, URx: 300, URy: 250},
				Name: "proj-001",
				Markup: annotation.Markup{
					User:         pdf.Text("Measurement Tool"),
					CreationDate: time.Date(2023, 5, 10, 9, 15, 0, 0, time.UTC),
					Subject:      pdf.Text("3D measurement association"),
					ExData:       pdf.NewReference(500, 0), // Reference to external data dictionary
				},
			},
		},
		{
			name: "projection with zero rect (no AP)",
			annotation: &annotation.Projection{
				Rect:     pdf.Rectangle{LLx: 100, LLy: 300, URx: 100, URy: 350}, // Zero width
				Contents: pdf.Text("Measurement with zero width rect"),
				User:     pdf.Text("Specialist"),
				Subject:  pdf.Text("Zero-dimension measurement"),
				ExData:   pdf.NewReference(600, 0),
			},
		},
		{
			name: "comprehensive projection annotation",
			annotation: &annotation.Projection{
				Rect:                    pdf.Rectangle{LLx: 200, LLy: 400, URx: 500, URy: 500},
				Contents:                pdf.Text("Comprehensive 3D measurement annotation"),
				Name:                    "comprehensive-projection",
				Flags:                   2, // Print flag
				Color:                   color.DeviceGray(0.8),
				StrokingTransparency:    0.9,
				NonStrokingTransparency: 0.7,
				Markup: annotation.Markup{
					User:         pdf.Text("3D Measurement System"),
					Subject:      pdf.Text("Complex geospatial measurement"),
					RC:           &pdf.StringOrStream{Value: pdf.Text("Associated with 3D measurement data")},
					CreationDate: time.Date(2023, 8, 20, 14, 30, 0, 0, time.UTC),
					InReplyTo:    pdf.NewReference(300, 0), // In reply to reference
					Intent:       "Group",
					ExData:       pdf.NewReference(700, 0), // External data dictionary with 3DM subtype
				},
			},
		},
		{
			name: "minimal projection annotation",
			annotation: &annotation.Projection{
				Rect: pdf.Rectangle{LLx: 0, LLy: 0, URx: 50, URy: 25},
				User: pdf.Text("User"),
				// No ExData - optional for projection annotations
			},
		},
	},
	"RichMedia": {
		{
			name: "basic rich media annotation",
			annotation: &annotation.RichMedia{
				Rect:             pdf.Rectangle{LLx: 100, LLy: 100, URx: 400, URy: 300},
				Contents:         pdf.Text("3D model annotation"),
				RichMediaContent: pdf.NewReference(500, 0), // Required reference
			},
		},
		{
			name: "rich media with settings",
			annotation: &annotation.RichMedia{
				Common: annotation.Common{
					Rect:  pdf.Rectangle{LLx: 50, LLy: 200, URx: 350, URy: 400},
					Name:  "richmedia-001",
					Flags: 2, // Print flag
				},
				RichMediaContent:  pdf.NewReference(600, 0), // Required content dictionary
				RichMediaSettings: pdf.NewReference(700, 0), // Optional settings dictionary
			},
		},
		{
			name: "comprehensive rich media annotation",
			annotation: &annotation.RichMedia{
				Rect:                    pdf.Rectangle{LLx: 200, LLy: 500, URx: 600, URy: 700},
				Contents:                pdf.Text("Interactive 3D content with video and sound"),
				Name:                    "comprehensive-richmedia",
				Flags:                   0, // No flags
				Color:                   color.DeviceGray(0.5),
				StrokingTransparency:    1.0,
				NonStrokingTransparency: 0.8,
				Border: &annotation.Border{
					HCornerRadius: 5.0,
					VCornerRadius: 5.0,
					Width:         2.0,
				},
				RichMediaContent:  pdf.NewReference(800, 0), // Content with 3D, Sound, Video instances
				RichMediaSettings: pdf.NewReference(900, 0), // Activation/deactivation settings
			},
		},
		{
			name: "minimal rich media annotation",
			annotation: &annotation.RichMedia{
				Rect:             pdf.Rectangle{LLx: 0, LLy: 0, URx: 200, URy: 150},
				RichMediaContent: pdf.NewReference(400, 0), // Only required field
				// No RichMediaSettings - uses defaults
			},
		},
		{
			name: "rich media with other fields",
			annotation: &annotation.RichMedia{
				Rect:                    pdf.Rectangle{LLx: 300, LLy: 800, URx: 500, URy: 900},
				Contents:                pdf.Text("Rich media with additional fields"),
				Name:                    "richmedia-advanced",
				StrokingTransparency:    0.9,
				NonStrokingTransparency: 0.7,
				RichMediaContent:        pdf.NewReference(1100, 0),
				RichMediaSettings:       pdf.NewReference(1200, 0),
			},
		},
	},
}

// testDicts contains annotation dictionaries for round-trip tests. Some of the
// dictionaries are mal-formed. These are used to make sure that:
//  1. The reader does not crash on malformed dictionaries.
//  2. Every dict which can be read can then be written back
//     (possibly leading to a different representation).
var testDicts = []pdf.Dict{
	{
		"Type":       pdf.Name("Annot"),
		"Subtype":    pdf.Name("Text"),
		"Rect":       &pdf.Rectangle{LLx: 100, LLy: 100, URx: 200, URy: 150},
		"T":          pdf.String("Grumpy Boss"),
		"State":      pdf.Name("Rejected"),
		"StateModel": pdf.Name("Review"),
		"IRT":        pdf.NewReference(1, 0),
	},
	{
		"Type":       pdf.Name("Annot"),
		"Subtype":    pdf.Name("Text"),
		"Rect":       &pdf.Rectangle{LLx: 100, LLy: 100, URx: 200, URy: 150},
		"T":          pdf.String("Grumpy Boss"),
		"State":      pdf.Name("Rejected"),
		"StateModel": pdf.Name("Review"),
		// invalid due to missing "IRT" field
	},
}
