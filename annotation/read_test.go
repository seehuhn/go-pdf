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
	"reflect"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"golang.org/x/text/language"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation/appearance"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/form"
	"seehuhn.de/go/pdf/internal/debug/memfile"
	"seehuhn.de/go/pdf/internal/debug/mock"
)

type testCase struct {
	name       string
	annotation Annotation
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
				Icon: "Comment",
			},
		},
		{
			name: "text annotation with markup fields",
			annotation: &Text{
				Common: Common{
					Rect:                    pdf.Rectangle{LLx: 50, LLy: 50, URx: 150, URy: 100},
					Contents:                "Text with markup",
					Name:                    "annotation-1",
					StrokingTransparency:    0.8,
					NonStrokingTransparency: 0.6,
				},
				Markup: Markup{
					User:         "Author Name",
					CreationDate: time.Date(2023, 1, 15, 10, 30, 0, 0, time.UTC),
					Subject:      "Test Subject",
					InReplyTo:    pdf.NewReference(1, 0),
				},
				Open:  true,
				Icon:  "Note",
				State: TextStateMarked,
			},
		},
		{
			name: "text annotation with all fields",
			annotation: &Text{
				Common: Common{
					Rect:     pdf.Rectangle{LLx: 0, LLy: 0, URx: 100, URy: 50},
					Contents: "Complete text annotation",
					Flags:    4,                        // ReadOnly flag
					Color:    color.DeviceRGB(1, 0, 0), // red
					Border: &Border{
						HCornerRadius: 2.0,
						VCornerRadius: 2.0,
						Width:         1.0,
						DashArray:     []float64{3.0, 2.0},
					},
					Lang: language.MustParse("en-US"),
				},
				Markup: Markup{
					User:      "Test User",
					Subject:   "Complete annotation test",
					RT:        "R",
					Intent:    "Note",
					InReplyTo: pdf.NewReference(2, 0),
				},
				Open:  true,
				Icon:  "Insert",
				State: TextStateAccepted,
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
				Highlight: LinkHighlightInvert,
			},
		},
		{
			name: "link annotation with quad points",
			annotation: &Link{
				Common: Common{
					Rect:     pdf.Rectangle{LLx: 0, LLy: 0, URx: 200, URy: 40},
					Contents: "Link description",
				},
				Highlight:  LinkHighlightOutline,
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
				Highlight: LinkHighlightPush,
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
					User:    "Reviewer",
					Subject: "Callout comment",
					Intent:  "FreeTextCallout",
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
					Color:    color.DeviceGray(0.9), // Light gray background
				},
				Markup: Markup{
					User:         "Designer",
					CreationDate: time.Date(2023, 6, 1, 14, 0, 0, 0, time.UTC),
					Intent:       "FreeText",
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
					User:   "Designer",
					Intent: "LineArrow",
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
					User:         "Engineer",
					CreationDate: time.Date(2023, 3, 15, 9, 0, 0, 0, time.UTC),
					Intent:       "LineDimension",
				},
				L:   []float64{50, 225, 250, 225},
				LE:  []pdf.Name{"Butt", "Butt"},
				LL:  10.0,            // Leader line length
				LLE: 5.0,             // Leader line extensions
				Cap: true,            // Show caption
				CP:  "Top",           // Caption on top
				CO:  []float64{0, 5}, // Caption offset
				LLO: 2.0,             // Leader line offset
			},
		},
		{
			name: "line with all fields",
			annotation: &Line{
				Common: Common{
					Rect:  pdf.Rectangle{LLx: 100, LLy: 300, URx: 400, URy: 350},
					Flags: 2,                            // Print flag
					Color: color.DeviceCMYK(1, 1, 0, 0), // blue
				},
				Markup: Markup{
					User:    "Reviewer",
					Subject: "Complete line annotation",
					RT:      "R",
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
					User:    "Author",
					Subject: "Square subject",
				},
			},
		},
		{
			name: "square with interior color and border style",
			annotation: &Square{
				Common: Common{
					Rect: pdf.Rectangle{LLx: 100, LLy: 100, URx: 200, URy: 200},
					Name: "square-001",
				},
				Markup: Markup{
					User:    "Designer",
					Subject: "Color square",
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
					Color:    Transparent, // transparent
				},
				Markup: Markup{
					User:    "Reviewer",
					Subject: "Complex annotation",
					Intent:  "SquareCloud",
				},
				IC: []float64{0.9, 0.9, 0.9},      // Light gray interior
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
					User:    "Author",
					Subject: "Circle subject",
				},
			},
		},
		{
			name: "circle with interior color",
			annotation: &Circle{
				Common: Common{
					Rect: pdf.Rectangle{LLx: 150, LLy: 150, URx: 250, URy: 200},
					Name: "circle-001",
				},
				Markup: Markup{
					User:    "Designer",
					Subject: "Color circle",
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
					Color:    color.DeviceGray(0.5), // gray
				},
				Markup: Markup{
					User:    "Reviewer",
					Subject: "Complex circle annotation",
					Intent:  "CircleCloud",
				},
				IC: []float64{1.0, 1.0, 0.0},          // Yellow interior
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
					User:    "Author",
					Subject: "Triangle shape",
				},
				Vertices: []float64{100, 50, 50, 150, 200, 150}, // Triangle vertices
			},
		},
		{
			name: "polygon with interior color and border effect",
			annotation: &Polygon{
				Common: Common{
					Rect: pdf.Rectangle{LLx: 100, LLy: 100, URx: 300, URy: 250},
					Name: "polygon-001",
				},
				Markup: Markup{
					User:    "Designer",
					Subject: "Colored polygon",
					Intent:  "PolygonCloud",
				},
				Vertices: []float64{150, 100, 300, 150, 250, 250, 100, 200}, // Quadrilateral
				IC:       []float64{0.0, 1.0, 0.5},                          // Green-cyan interior
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
					User:    "Engineer",
					Subject: "Measured polygon",
					Intent:  "PolygonDimension",
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
					User:    "Author",
					Subject: "Line path",
				},
				Vertices: []float64{75, 100, 150, 175, 225, 125, 275, 150}, // Zigzag line
				LE:       []pdf.Name{"None", "OpenArrow"},                  // Arrow at end
			},
		},
		{
			name: "polyline with line endings and colors",
			annotation: &Polyline{
				Common: Common{
					Rect: pdf.Rectangle{LLx: 150, LLy: 150, URx: 350, URy: 250},
					Name: "polyline-001",
				},
				Markup: Markup{
					User:    "Designer",
					Subject: "Colored polyline",
					Intent:  "PolyLineDimension",
				},
				Vertices: []float64{150, 200, 200, 150, 300, 250, 350, 180},
				LE:       []pdf.Name{"Circle", "Square"}, // Different endings
				IC:       []float64{1.0, 0.0, 0.0},       // Red line endings
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
					User:    "Artist",
					Subject: "Bezier curve",
				},
				Path: [][]float64{
					{250, 300},                     // moveto
					{300, 250, 350, 350, 400, 300}, // curveto (6 coordinates)
					{450, 320},                     // lineto
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
					User:    "Reviewer",
					Subject: "Important passage",
				},
				QuadPoints: []float64{100, 200, 200, 200, 200, 220, 100, 220}, // Single word quad
			},
		},
		{
			name: "highlight with multiple quads",
			annotation: &Highlight{
				Common: Common{
					Rect:  pdf.Rectangle{LLx: 50, LLy: 300, URx: 250, URy: 340},
					Name:  "highlight-001",
					Color: color.DeviceRGB(1.0, 1.0, 0.0), // yellow
				},
				Markup: Markup{
					User:    "Student",
					Subject: "Study notes",
				},
				QuadPoints: []float64{
					50, 300, 100, 300, 100, 320, 50, 320, // First word
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
					User:    "Editor",
					Subject: "Emphasis added",
				},
				QuadPoints: []float64{150, 400, 300, 400, 300, 420, 150, 420}, // Single phrase
			},
		},
		{
			name: "underline with markup fields",
			annotation: &Underline{
				Common: Common{
					Rect:  pdf.Rectangle{LLx: 75, LLy: 500, URx: 225, URy: 520},
					Name:  "underline-001",
					Color: color.DeviceRGB(0, 0, 1), // blue
				},
				Markup: Markup{
					User:         "Proofreader",
					Subject:      "Grammar correction",
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
					User:    "Spellchecker",
					Subject: "Possible misspelling",
				},
				QuadPoints: []float64{200, 600, 350, 600, 350, 620, 200, 620}, // Misspelled word
			},
		},
		{
			name: "squiggly with color",
			annotation: &Squiggly{
				Common: Common{
					Rect:  pdf.Rectangle{LLx: 100, LLy: 700, URx: 180, URy: 720},
					Name:  "squiggly-001",
					Color: color.DeviceRGB(1, 0.5, 0), // orange
				},
				Markup: Markup{
					User:    "Grammar checker",
					Subject: "Grammar issue",
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
					User:    "Editor",
					Subject: "Text to be removed",
				},
				QuadPoints: []float64{250, 800, 400, 800, 400, 820, 250, 820}, // Struck-out phrase
			},
		},
		{
			name: "strikeout with multiple sections",
			annotation: &StrikeOut{
				Common: Common{
					Rect: pdf.Rectangle{LLx: 50, LLy: 900, URx: 300, URy: 940},
					Name: "strikeout-001",
				},
				Markup: Markup{
					User:    "Reviewer",
					Subject: "Major revision",
				},
				QuadPoints: []float64{
					50, 900, 150, 900, 150, 920, 50, 920, // First section
					160, 900, 250, 900, 250, 920, 160, 920, // Second section
					50, 920, 300, 920, 300, 940, 50, 940, // Third section (different line)
				},
			},
		},
	},
	"Caret": {
		{
			name: "basic caret annotation",
			annotation: &Caret{
				Common: Common{
					Rect:     pdf.Rectangle{LLx: 100, LLy: 100, URx: 110, URy: 120},
					Contents: "Text insertion point",
				},
				Markup: Markup{
					User:    "Editor",
					Subject: "Insert text here",
				},
			},
		},
		{
			name: "caret with paragraph symbol",
			annotation: &Caret{
				Common: Common{
					Rect: pdf.Rectangle{LLx: 200, LLy: 200, URx: 220, URy: 230},
					Name: "caret-001",
				},
				Markup: Markup{
					User:         "Proofreader",
					Subject:      "New paragraph needed",
					CreationDate: time.Date(2023, 8, 20, 10, 15, 0, 0, time.UTC),
				},
				Sy: "P", // Paragraph symbol
			},
		},
		{
			name: "caret with rectangle differences",
			annotation: &Caret{
				Common: Common{
					Rect:     pdf.Rectangle{LLx: 300, LLy: 300, URx: 330, URy: 335},
					Contents: "Complex caret with spacing",
				},
				Markup: Markup{
					User:    "Reviewer",
					Subject: "Spacing adjustment",
				},
				RD: []float64{2.0, 3.0, 2.0, 5.0}, // Rectangle differences
			},
		},
	},
	"Stamp": {
		{
			name: "basic stamp annotation",
			annotation: &Stamp{
				Common: Common{
					Rect:     pdf.Rectangle{LLx: 50, LLy: 50, URx: 150, URy: 100},
					Contents: "Approved document",
				},
				Markup: Markup{
					User:    "Manager",
					Subject: "Document approval",
				},
				Name: "Approved",
			},
		},
		{
			name: "stamp with default name",
			annotation: &Stamp{
				Common: Common{
					Rect: pdf.Rectangle{LLx: 200, LLy: 200, URx: 300, URy: 250},
					Name: "stamp-001",
				},
				Markup: Markup{
					User:         "Editor",
					Subject:      "Draft version",
					CreationDate: time.Date(2023, 9, 10, 16, 30, 0, 0, time.UTC),
				},
				Name: "Draft", // Default value
			},
		},
		{
			name: "confidential stamp",
			annotation: &Stamp{
				Common: Common{
					Rect:     pdf.Rectangle{LLx: 100, LLy: 300, URx: 250, URy: 350},
					Contents: "Confidential material",
				},
				Markup: Markup{
					User:    "Security Officer",
					Subject: "Classification stamp",
				},
				Name: "Confidential",
			},
		},
		{
			name: "stamp with intent",
			annotation: &Stamp{
				Common: Common{
					Rect:     pdf.Rectangle{LLx: 300, LLy: 400, URx: 450, URy: 450},
					Contents: "Image stamp",
				},
				Markup: Markup{
					User:   "Designer",
					Intent: "StampImage", // Intent: image stamp
				},
				Name: "Draft", // Default value (Name not written to PDF when IT != "Stamp")
			},
		},
		{
			name: "stamp with all standard values",
			annotation: &Stamp{
				Common: Common{
					Rect:     pdf.Rectangle{LLx: 150, LLy: 500, URx: 300, URy: 550},
					Contents: "Final document",
				},
				Markup: Markup{
					User:   "Publisher",
					Intent: "Stamp", // Explicit rubber stamp intent
				},
				Name: "Final",
			},
		},
	},
	"Ink": {
		{
			name: "simple ink annotation",
			annotation: &Ink{
				Common: Common{
					Rect:     pdf.Rectangle{LLx: 50, LLy: 50, URx: 200, URy: 150},
					Contents: "Handwritten note",
				},
				Markup: Markup{
					User:    "Annotator",
					Subject: "Hand drawing",
				},
				InkList: [][]float64{
					{50, 100, 75, 120, 100, 110, 125, 130, 150, 125}, // Single stroke
				},
			},
		},
		{
			name: "multi-stroke ink annotation",
			annotation: &Ink{
				Common: Common{
					Rect:  pdf.Rectangle{LLx: 100, LLy: 200, URx: 300, URy: 350},
					Name:  "ink-001",
					Color: color.DeviceRGB(0.0, 0.0, 1.0), // blue
				},
				Markup: Markup{
					User:         "Artist",
					Subject:      "Sketch",
					CreationDate: time.Date(2023, 10, 5, 14, 20, 0, 0, time.UTC),
				},
				InkList: [][]float64{
					{100, 250, 150, 280, 200, 260}, // First stroke
					{180, 300, 220, 320, 250, 310}, // Second stroke
					{120, 320, 160, 340, 190, 330}, // Third stroke
				},
			},
		},
		{
			name: "ink annotation with border style",
			annotation: &Ink{
				Common: Common{
					Rect:     pdf.Rectangle{LLx: 200, LLy: 400, URx: 400, URy: 500},
					Contents: "Stylized ink drawing",
				},
				Markup: Markup{
					User:    "Designer",
					Subject: "Stylized annotation",
				},
				InkList: [][]float64{
					{200, 450, 250, 470, 300, 460, 350, 480, 400, 475},
				},
			},
		},
		{
			name: "ink annotation with path (PDF 2.0)",
			annotation: &Ink{
				Common: Common{
					Rect:     pdf.Rectangle{LLx: 150, LLy: 600, URx: 350, URy: 700},
					Contents: "Curved ink path",
				},
				Markup: Markup{
					User:    "Artist",
					Subject: "Bezier curve drawing",
				},
				Path: [][]float64{
					{150, 650},                     // moveto
					{200, 620, 250, 680, 300, 650}, // curveto (6 coordinates)
					{350, 670},                     // lineto
				},
			},
		},
		{
			name: "complex ink annotation",
			annotation: &Ink{
				Common: Common{
					Rect:     pdf.Rectangle{LLx: 50, LLy: 750, URx: 400, URy: 850},
					Contents: "Complex freehand drawing",
				},
				Markup: Markup{
					User:    "User",
					Subject: "Signature and notes",
				},
				InkList: [][]float64{
					{50, 800, 80, 810, 120, 790, 160, 805, 200, 795}, // Signature stroke 1
					{180, 820, 220, 830, 260, 815, 300, 825},         // Signature stroke 2
					{320, 780, 350, 790, 380, 785, 400, 795},         // Additional mark
				},
			},
		},
	},
	"Popup": {
		{
			name: "basic popup annotation",
			annotation: &Popup{
				Common: Common{
					Rect:     pdf.Rectangle{LLx: 100, LLy: 100, URx: 200, URy: 150},
					Contents: "Popup window content",
				},
			},
		},
		{
			name: "popup with parent reference",
			annotation: &Popup{
				Common: Common{
					Rect: pdf.Rectangle{LLx: 150, LLy: 200, URx: 250, URy: 280},
					Name: "popup-001",
				},
				Parent: pdf.NewReference(123, 0), // Reference to parent annotation
			},
		},
		{
			name: "popup initially open",
			annotation: &Popup{
				Common: Common{
					Rect:     pdf.Rectangle{LLx: 200, LLy: 300, URx: 350, URy: 400},
					Contents: "Initially visible popup",
				},
				Open: true, // Initially displayed open
			},
		},
		{
			name: "popup with parent and open state",
			annotation: &Popup{
				Common: Common{
					Rect: pdf.Rectangle{LLx: 50, LLy: 400, URx: 180, URy: 480},
					Name: "popup-with-parent",
				},
				Parent: pdf.NewReference(456, 0), // Parent markup annotation
				Open:   true,                     // Initially open
			},
		},
		{
			name: "minimal popup annotation",
			annotation: &Popup{
				Common: Common{
					Rect: pdf.Rectangle{LLx: 300, LLy: 500, URx: 400, URy: 550},
				},
				// No parent, not initially open (all defaults)
			},
		},
	},
	"FileAttachment": {
		{
			name: "basic file attachment",
			annotation: &FileAttachment{
				Common: Common{
					Rect:     pdf.Rectangle{LLx: 50, LLy: 50, URx: 80, URy: 80},
					Contents: "Attached spreadsheet file",
				},
				Markup: Markup{
					User:    "Data Analyst",
					Subject: "Supporting data",
				},
				FS:   pdf.NewReference(100, 0), // File specification reference
				Name: "Graph",                  // Icon for data files
			},
		},
		{
			name: "file attachment with default icon",
			annotation: &FileAttachment{
				Common: Common{
					Rect: pdf.Rectangle{LLx: 150, LLy: 150, URx: 180, URy: 180},
					Name: "attachment-001",
				},
				Markup: Markup{
					User:         "Author",
					Subject:      "Supporting document",
					CreationDate: time.Date(2023, 11, 15, 9, 30, 0, 0, time.UTC),
				},
				FS: pdf.NewReference(200, 0), // File specification reference
				// Name not set, should default to "PushPin"
			},
		},
		{
			name: "paperclip attachment",
			annotation: &FileAttachment{
				Common: Common{
					Rect:     pdf.Rectangle{LLx: 250, LLy: 250, URx: 280, URy: 280},
					Contents: "Attached document",
				},
				Markup: Markup{
					User:    "Secretary",
					Subject: "Office document",
				},
				FS:   pdf.NewReference(300, 0), // File specification reference
				Name: "Paperclip",              // Paperclip icon
			},
		},
		{
			name: "tag attachment with metadata",
			annotation: &FileAttachment{
				Common: Common{
					Rect:     pdf.Rectangle{LLx: 100, LLy: 350, URx: 130, URy: 380},
					Contents: "Tagged reference file",
					Color:    color.DeviceGray(0.8), // light gray
				},
				Markup: Markup{
					User:    "Librarian",
					Subject: "Reference material",
				},
				FS:   pdf.NewReference(400, 0), // File specification reference
				Name: "Tag",                    // Tag icon
			},
		},
		{
			name: "minimal file attachment",
			annotation: &FileAttachment{
				Common: Common{
					Rect: pdf.Rectangle{LLx: 300, LLy: 450, URx: 330, URy: 480},
				},
				Markup: Markup{
					User: "User",
				},
				FS: pdf.NewReference(500, 0), // Required file specification
				// Name will default to "PushPin"
			},
		},
	},
	"Sound": {
		{
			name: "basic sound annotation",
			annotation: &Sound{
				Common: Common{
					Rect:     pdf.Rectangle{LLx: 100, LLy: 100, URx: 130, URy: 130},
					Contents: "Recorded audio note",
				},
				Markup: Markup{
					User:    "Narrator",
					Subject: "Audio explanation",
				},
				Sound: pdf.NewReference(600, 0), // Sound object reference
				Name:  "Speaker",                // Default speaker icon
			},
		},
		{
			name: "microphone sound annotation",
			annotation: &Sound{
				Common: Common{
					Rect: pdf.Rectangle{LLx: 200, LLy: 200, URx: 230, URy: 230},
					Name: "sound-001",
				},
				Markup: Markup{
					User:         "Reporter",
					Subject:      "Field recording",
					CreationDate: time.Date(2023, 12, 1, 15, 45, 0, 0, time.UTC),
				},
				Sound: pdf.NewReference(700, 0), // Sound object reference
				Name:  "Mic",                    // Microphone icon
			},
		},
		{
			name: "sound annotation with default icon",
			annotation: &Sound{
				Common: Common{
					Rect:     pdf.Rectangle{LLx: 300, LLy: 300, URx: 330, URy: 330},
					Contents: "Audio clip",
				},
				Markup: Markup{
					User:    "Audio Engineer",
					Subject: "Sound sample",
				},
				Sound: pdf.NewReference(800, 0), // Sound object reference
				Name:  "Speaker",                // Default value
			},
		},
		{
			name: "sound annotation with metadata",
			annotation: &Sound{
				Common: Common{
					Rect:     pdf.Rectangle{LLx: 150, LLy: 400, URx: 180, URy: 430},
					Contents: "Interview recording",
					Color:    color.DeviceRGB(0.5, 0.5, 0.5), // gray
				},
				Markup: Markup{
					User:    "Journalist",
					Subject: "Interview audio",
				},
				Sound: pdf.NewReference(900, 0), // Sound object reference
				Name:  "Speaker",                // Explicit speaker icon
			},
		},
		{
			name: "minimal sound annotation",
			annotation: &Sound{
				Common: Common{
					Rect: pdf.Rectangle{LLx: 50, LLy: 500, URx: 80, URy: 530},
				},
				Markup: Markup{
					User: "User",
				},
				Sound: pdf.NewReference(1000, 0), // Required sound object
				Name:  "Speaker",                 // Default value
			},
		},
	},
	"Movie": {
		{
			name: "basic movie annotation",
			annotation: &Movie{
				Common: Common{
					Rect:     pdf.Rectangle{LLx: 100, LLy: 100, URx: 300, URy: 200},
					Contents: "Educational video",
				},
				T:     "Introduction Video",
				Movie: pdf.NewReference(500, 0), // Movie dictionary reference
				A:     pdf.Boolean(true),        // Default activation
			},
		},
		{
			name: "movie annotation with title",
			annotation: &Movie{
				Common: Common{
					Rect: pdf.Rectangle{LLx: 200, LLy: 200, URx: 400, URy: 300},
					Name: "movie-001",
				},
				T:     "Training Module 1",
				Movie: pdf.NewReference(600, 0), // Movie dictionary reference
				A:     pdf.Boolean(false),       // Do not play automatically
			},
		},
		{
			name: "movie annotation with activation dictionary",
			annotation: &Movie{
				Common: Common{
					Rect:     pdf.Rectangle{LLx: 50, LLy: 50, URx: 250, URy: 150},
					Contents: "Interactive presentation",
					Color:    color.DeviceRGB(0.7, 0.6, 0.5), // brown
				},
				T:     "Interactive Demo",
				Movie: pdf.NewReference(700, 0), // Movie dictionary reference
				A:     pdf.NewReference(800, 0), // Movie activation dictionary
			},
		},
		{
			name: "movie annotation with default activation",
			annotation: &Movie{
				Common: Common{
					Rect: pdf.Rectangle{LLx: 300, LLy: 300, URx: 500, URy: 400},
				},
				Movie: pdf.NewReference(900, 0), // Movie dictionary reference
				A:     pdf.Boolean(true),        // Explicit default value
			},
		},
		{
			name: "minimal movie annotation",
			annotation: &Movie{
				Common: Common{
					Rect: pdf.Rectangle{LLx: 0, LLy: 0, URx: 100, URy: 100},
				},
				Movie: pdf.NewReference(1000, 0), // Required movie dictionary
				A:     pdf.Boolean(true),         // Default value
			},
		},
	},
	"Screen": {
		{
			name: "basic screen annotation",
			annotation: &Screen{
				Common: Common{
					Rect:     pdf.Rectangle{LLx: 100, LLy: 100, URx: 400, URy: 300},
					Contents: "Media playback region",
				},
				T: "Video Player",
			},
		},
		{
			name: "screen annotation with appearance characteristics",
			annotation: &Screen{
				Common: Common{
					Rect: pdf.Rectangle{LLx: 200, LLy: 200, URx: 500, URy: 400},
					Name: "screen-001",
				},
				T:  "Interactive Media",
				MK: pdf.NewReference(1100, 0), // Appearance characteristics dictionary
			},
		},
		{
			name: "screen annotation with action",
			annotation: &Screen{
				Common: Common{
					Rect:     pdf.Rectangle{LLx: 50, LLy: 50, URx: 350, URy: 250},
					Contents: "Click to play video",
					Color:    Transparent,
				},
				T: "Action Trigger",
				A: pdf.NewReference(1200, 0), // Action dictionary
			},
		},
		{
			name: "screen annotation with additional actions",
			annotation: &Screen{
				Common: Common{
					Rect: pdf.Rectangle{LLx: 300, LLy: 300, URx: 600, URy: 500},
				},
				T:  "Advanced Media Control",
				AA: pdf.NewReference(1300, 0), // Additional-actions dictionary
			},
		},
		{
			name: "screen annotation with all features",
			annotation: &Screen{
				Common: Common{
					Rect:     pdf.Rectangle{LLx: 150, LLy: 150, URx: 450, URy: 350},
					Contents: "Full-featured media player",
					Color:    color.DeviceRGB(0.9, 0.9, 0.9),
				},
				T:  "Complete Media Player",
				MK: pdf.NewReference(1400, 0), // Appearance characteristics
				A:  pdf.NewReference(1500, 0), // Action dictionary
				AA: pdf.NewReference(1600, 0), // Additional-actions dictionary
			},
		},
		{
			name: "minimal screen annotation",
			annotation: &Screen{
				Common: Common{
					Rect: pdf.Rectangle{LLx: 0, LLy: 0, URx: 200, URy: 100},
				},
				// Only required fields from Common
			},
		},
	},
	"Widget": {
		{
			name: "basic widget annotation",
			annotation: &Widget{
				Common: Common{
					Rect:     pdf.Rectangle{LLx: 100, LLy: 100, URx: 300, URy: 130},
					Contents: "Text input field",
				},
				H: "I", // Default highlighting mode
			},
		},
		{
			name: "widget with push highlighting",
			annotation: &Widget{
				Common: Common{
					Rect: pdf.Rectangle{LLx: 200, LLy: 200, URx: 350, URy: 240},
					Name: "button-001",
				},
				H: "P", // Push highlighting
			},
		},
		{
			name: "widget with appearance characteristics",
			annotation: &Widget{
				Common: Common{
					Rect:     pdf.Rectangle{LLx: 50, LLy: 50, URx: 250, URy: 80},
					Contents: "Button with custom appearance",
				},
				H:  "O",                       // Outline highlighting
				MK: pdf.NewReference(1700, 0), // Appearance characteristics dictionary
			},
		},
		{
			name: "widget with action and additional actions",
			annotation: &Widget{
				Common: Common{
					Rect:  pdf.Rectangle{LLx: 300, LLy: 300, URx: 500, URy: 340},
					Color: color.DeviceCMYK(0.1, 0.2, 0.3, 0.4),
				},
				H:  "I",                       // Default highlighting
				A:  pdf.NewReference(1800, 0), // Action dictionary
				AA: pdf.NewReference(1900, 0), // Additional-actions dictionary
			},
		},
		{
			name: "widget with border style",
			annotation: &Widget{
				Common: Common{
					Rect:     pdf.Rectangle{LLx: 150, LLy: 150, URx: 400, URy: 190},
					Contents: "Field with custom border",
				},
				H:  "N",                       // No highlighting
				BS: pdf.NewReference(2000, 0), // Border style dictionary
			},
		},
		{
			name: "widget with parent field",
			annotation: &Widget{
				Common: Common{
					Rect: pdf.Rectangle{LLx: 250, LLy: 250, URx: 450, URy: 290},
					Name: "child-widget-001",
				},
				H:      "T",                       // Toggle highlighting (same as Push)
				Parent: pdf.NewReference(2100, 0), // Parent field reference
			},
		},
		{
			name: "widget with all features",
			annotation: &Widget{
				Common: Common{
					Rect:     pdf.Rectangle{LLx: 100, LLy: 400, URx: 350, URy: 450},
					Contents: "Complete widget annotation",
					Color:    color.DeviceRGB(0.8, 0.9, 1.0),
				},
				H:      "P",                       // Push highlighting
				MK:     pdf.NewReference(2200, 0), // Appearance characteristics
				A:      pdf.NewReference(2300, 0), // Action dictionary
				AA:     pdf.NewReference(2400, 0), // Additional-actions dictionary
				BS:     pdf.NewReference(2500, 0), // Border style dictionary
				Parent: pdf.NewReference(2600, 0), // Parent field reference
			},
		},
		{
			name: "minimal widget annotation",
			annotation: &Widget{
				Common: Common{
					Rect: pdf.Rectangle{LLx: 0, LLy: 0, URx: 100, URy: 30},
				},
				H: "I", // Default value
			},
		},
	},
	"PrinterMark": {
		{
			name: "basic printer's mark annotation",
			annotation: &PrinterMark{
				Common: Common{
					Rect:     pdf.Rectangle{LLx: 50, LLy: 50, URx: 100, URy: 100},
					Contents: "Registration target",
				},
				MN: "RegistrationTarget",
			},
		},
		{
			name: "color bar printer's mark",
			annotation: &PrinterMark{
				Common: Common{
					Rect: pdf.Rectangle{LLx: 200, LLy: 780, URx: 400, URy: 800},
					Name: "colorbar-001",
				},
				MN: "ColorBar",
			},
		},
		{
			name: "cut mark printer's mark",
			annotation: &PrinterMark{
				Common: Common{
					Rect:     pdf.Rectangle{LLx: 0, LLy: 0, URx: 20, URy: 20},
					Contents: "Corner cut mark",
					Color:    color.DeviceRGB(0, 0, 0),
				},
				MN: "CutMark",
			},
		},
		{
			name: "printer's mark with border",
			annotation: &PrinterMark{
				Common: Common{
					Rect:     pdf.Rectangle{LLx: 100, LLy: 100, URx: 150, URy: 150},
					Contents: "Registration mark with border",
					Border: &Border{
						HCornerRadius: 2.0,
						VCornerRadius: 2.0,
						Width:         2.0,
					},
				},
				MN: "RegistrationTarget",
			},
		},
		{
			name: "printer's mark with print and readonly flags",
			annotation: &PrinterMark{
				Common: Common{
					Rect:  pdf.Rectangle{LLx: 150, LLy: 150, URx: 200, URy: 200},
					Flags: 6, // Print (2) + ReadOnly (4) flags as per spec
				},
				MN: "GrayRamp",
			},
		},
		{
			name: "minimal printer's mark annotation",
			annotation: &PrinterMark{
				Common: Common{
					Rect: pdf.Rectangle{LLx: 300, LLy: 300, URx: 320, URy: 320},
				},
				// MN field is optional, not specified here
			},
		},
	},
	"TrapNet": {
		{
			name: "trap network with LastModified",
			annotation: &TrapNet{
				Common: Common{
					Rect:  pdf.Rectangle{LLx: 0, LLy: 0, URx: 612, URy: 792}, // Full page
					Flags: 6,                                                 // Print (2) + ReadOnly (4) flags as per spec
				},
				LastModified: "D:20231215103000Z",
			},
		},
		{
			name: "trap network with Version and AnnotStates",
			annotation: &TrapNet{
				Common: Common{
					Rect:  pdf.Rectangle{LLx: 0, LLy: 0, URx: 612, URy: 792}, // Full page
					Flags: 6,                                                 // Print (2) + ReadOnly (4) flags as per spec
					Name:  "trapnet-001",
				},
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
			annotation: &TrapNet{
				Common: Common{
					Rect:     pdf.Rectangle{LLx: 0, LLy: 0, URx: 612, URy: 792}, // Full page
					Flags:    6,                                                 // Print (2) + ReadOnly (4) flags
					Contents: "Trap network with font substitutions",
				},
				LastModified: "D:20231201120000Z",
				FontFauxing: []pdf.Reference{
					pdf.NewReference(200, 0), // Substitute font 1
					pdf.NewReference(201, 0), // Substitute font 2
				},
			},
		},
		{
			name: "complex trap network",
			annotation: &TrapNet{
				Common: Common{
					Rect:  pdf.Rectangle{LLx: 0, LLy: 0, URx: 612, URy: 792}, // Full page
					Flags: 6,                                                 // Print (2) + ReadOnly (4) flags
					Name:  "complex-trapnet",
				},
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
			annotation: &TrapNet{
				Common: Common{
					Rect:  pdf.Rectangle{LLx: 0, LLy: 0, URx: 612, URy: 792}, // Full page
					Flags: 6,                                                 // Required Print + ReadOnly flags
				},
				LastModified: "D:20231201000000Z", // Minimal required field
			},
		},
	},
	"Watermark": {
		{
			name: "basic watermark annotation",
			annotation: &Watermark{
				Common: Common{
					Rect:     pdf.Rectangle{LLx: 0, LLy: 0, URx: 200, URy: 100},
					Contents: "Confidential watermark",
				},
			},
		},
		{
			name: "watermark with fixed print dictionary",
			annotation: &Watermark{
				Common: Common{
					Rect: pdf.Rectangle{LLx: 100, LLy: 100, URx: 300, URy: 150},
					Name: "watermark-001",
				},
				FixedPrint: &FixedPrint{
					Matrix: []float64{1, 0, 0, 1, 72, -72}, // Translate 1 inch right and down
					H:      0.0,                            // Left edge
					V:      1.0,                            // Top edge
				},
			},
		},
		{
			name: "watermark with percentage positioning",
			annotation: &Watermark{
				Common: Common{
					Rect:     pdf.Rectangle{LLx: 0, LLy: 0, URx: 150, URy: 50},
					Contents: "Draft - Do Not Distribute",
				},
				FixedPrint: &FixedPrint{
					Matrix: []float64{1, 0, 0, 1, 0, 0}, // Identity matrix
					H:      0.5,                         // Center horizontally (50%)
					V:      0.1,                         // Near top (10% from top)
				},
			},
		},
		{
			name: "watermark with rotation matrix",
			annotation: &Watermark{
				Common: Common{
					Rect:     pdf.Rectangle{LLx: 0, LLy: 0, URx: 200, URy: 30},
					Contents: "SAMPLE",
				},
				FixedPrint: &FixedPrint{
					Matrix: []float64{0.707, 0.707, -0.707, 0.707, 0, 0}, // 45-degree rotation
					H:      0.25,                                         // 25% from left
					V:      0.75,                                         // 75% from bottom
				},
			},
		},
		{
			name: "watermark with default values",
			annotation: &Watermark{
				Common: Common{
					Rect: pdf.Rectangle{LLx: 50, LLy: 50, URx: 250, URy: 100},
				},
				FixedPrint: &FixedPrint{
					Matrix: []float64{1, 0, 0, 1, 0, 0}, // Identity matrix (default)
					H:      0.0,                         // Default horizontal position
					V:      0.0,                         // Default vertical position
				},
			},
		},
		{
			name: "watermark bottom-right corner",
			annotation: &Watermark{
				Common: Common{
					Rect:     pdf.Rectangle{LLx: 0, LLy: 0, URx: 100, URy: 20},
					Contents: "© 2023 Company Name",
				},
				FixedPrint: &FixedPrint{
					Matrix: []float64{1, 0, 0, 1, -20, 20}, // Offset from edge for margins
					H:      0.95,                           // 95% from left (near right edge)
					V:      0.05,                           // 5% from bottom (near bottom edge)
				},
			},
		},
		{
			name: "minimal watermark annotation",
			annotation: &Watermark{
				Common: Common{
					Rect: pdf.Rectangle{LLx: 0, LLy: 0, URx: 100, URy: 50},
				},
				// No FixedPrint dictionary - should be drawn without special media consideration
			},
		},
	},
	"3D": {
		{
			name: "basic 3D annotation",
			annotation: &Annot3D{
				Common: Common{
					Rect: pdf.Rectangle{LLx: 100, LLy: 100, URx: 400, URy: 300},
				},
				D: pdf.NewReference(100, 0), // 3D stream reference
				I: true,                     // Default value for interactive flag
			},
		},
		{
			name: "3D annotation with view specification",
			annotation: &Annot3D{
				Common: Common{
					Rect:     pdf.Rectangle{LLx: 50, LLy: 50, URx: 350, URy: 250},
					Contents: "Interactive 3D Model",
					Name:     "3d-model-001",
				},
				D: pdf.NewReference(200, 0), // 3D stream reference
				V: pdf.Integer(0),           // View index
				I: true,                     // Default value for interactive flag
			},
		},
		{
			name: "3D annotation with activation dictionary",
			annotation: &Annot3D{
				Common: Common{
					Rect: pdf.Rectangle{LLx: 0, LLy: 0, URx: 300, URy: 200},
				},
				D: pdf.NewReference(300, 0), // 3D stream reference
				A: &Annot3DActivation{
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
			annotation: &Annot3D{
				Common: Common{
					Rect: pdf.Rectangle{LLx: 0, LLy: 0, URx: 400, URy: 300},
				},
				D: pdf.NewReference(400, 0),                                 // 3D stream reference
				B: &pdf.Rectangle{LLx: -150, LLy: -100, URx: 150, URy: 100}, // View box in target coordinates
				I: true,                                                     // Default value for interactive flag
			},
		},
		{
			name: "3D annotation with PDF 2.0 features",
			annotation: &Annot3D{
				Common: Common{
					Rect: pdf.Rectangle{LLx: 100, LLy: 100, URx: 500, URy: 400},
					Name: "advanced-3d-model",
				},
				D: pdf.NewReference(500, 0), // 3D stream reference
				A: &Annot3DActivation{
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
			annotation: &Annot3D{
				Common: Common{
					Rect: pdf.Rectangle{LLx: 200, LLy: 200, URx: 600, URy: 500},
				},
				D: pdf.NewReference(600, 0),  // 3D stream reference
				V: pdf.String("DefaultView"), // View name
				I: false,                     // Not interactive (non-default)
			},
		},
		{
			name: "3D annotation with name view reference",
			annotation: &Annot3D{
				Common: Common{
					Rect: pdf.Rectangle{LLx: 0, LLy: 0, URx: 400, URy: 300},
				},
				D: pdf.NewReference(700, 0), // 3D stream reference
				V: pdf.Name("F"),            // First view in VA array
				I: true,                     // Default value for interactive flag
			},
		},
		{
			name: "comprehensive 3D annotation",
			annotation: &Annot3D{
				Common: Common{
					Rect:     pdf.Rectangle{LLx: 50, LLy: 50, URx: 450, URy: 350},
					Contents: "Complex 3D Scene with Multiple Features",
					Name:     "complex-3d-scene",
					Flags:    1, // Invisible flag
				},
				D: pdf.NewReference(800, 0), // 3D stream reference
				V: pdf.Integer(2),           // Third view (index 2)
				A: &Annot3DActivation{
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
			annotation: &Annot3D{
				Common: Common{
					Rect: pdf.Rectangle{LLx: 0, LLy: 0, URx: 200, URy: 150},
				},
				D: pdf.NewReference(900, 0), // 3D stream reference (required)
				I: true,                     // Default interactive flag
			},
		},
	},
	"Redact": {
		{
			name: "basic redaction annotation",
			annotation: &Redact{
				Common: Common{
					Rect: pdf.Rectangle{LLx: 100, LLy: 100, URx: 300, URy: 120},
				},
			},
		},
		{
			name: "redaction with QuadPoints",
			annotation: &Redact{
				Common: Common{
					Rect:     pdf.Rectangle{LLx: 50, LLy: 200, URx: 400, URy: 250},
					Contents: "Redacted content area",
				},
				QuadPoints: []float64{
					100, 210, 200, 210, 200, 240, 100, 240, // First quadrilateral
					250, 210, 350, 210, 350, 240, 250, 240, // Second quadrilateral
				},
			},
		},
		{
			name: "redaction with interior color",
			annotation: &Redact{
				Common: Common{
					Rect: pdf.Rectangle{LLx: 0, LLy: 0, URx: 200, URy: 50},
					Name: "redact-001",
				},
				IC: []float64{0.0, 0.0, 0.0}, // Black interior color
			},
		},
		{
			name: "redaction with overlay text",
			annotation: &Redact{
				Common: Common{
					Rect: pdf.Rectangle{LLx: 100, LLy: 300, URx: 500, URy: 350},
				},
				OverlayText: "CONFIDENTIAL",
				DA:          "/Helvetica 12 Tf 1 0 0 rg", // Red text appearance
				Q:           1,                           // Centered
			},
		},
		{
			name: "redaction with repeating overlay text",
			annotation: &Redact{
				Common: Common{
					Rect:     pdf.Rectangle{LLx: 50, LLy: 400, URx: 550, URy: 450},
					Contents: "Large redacted area",
				},
				OverlayText: "REDACTED",
				DA:          "/Arial 10 Tf 0.5 0.5 0.5 rg", // Gray text
				Repeat:      true,
				Q:           2, // Right-justified
			},
		},
		{
			name: "redaction with form XObject overlay",
			annotation: &Redact{
				Common: Common{
					Rect: pdf.Rectangle{LLx: 200, LLy: 500, URx: 400, URy: 550},
					Name: "redact-with-xobject",
				},
				RO: pdf.NewReference(100, 0), // Form XObject reference
			},
		},
		{
			name: "comprehensive redaction annotation",
			annotation: &Redact{
				Common: Common{
					Rect:     pdf.Rectangle{LLx: 50, LLy: 600, URx: 450, URy: 700},
					Contents: "Sensitive information requiring redaction",
					Name:     "comprehensive-redact",
					Flags:    4, // Print flag
				},
				Markup: Markup{
					User:      "Redaction Tool",
					RC:        pdf.String("This content contains sensitive information"),
					InReplyTo: pdf.NewReference(200, 0), // In reply to reference
				},
				QuadPoints: []float64{
					75, 620, 175, 620, 175, 640, 75, 640, // First quad
					200, 620, 300, 620, 300, 640, 200, 640, // Second quad
					325, 620, 425, 620, 425, 640, 325, 640, // Third quad
				},
				IC:          []float64{0.8, 0.8, 0.8}, // Light gray background
				OverlayText: "CLASSIFIED",
				DA:          "/Times-Bold 14 Tf 1 0 0 rg", // Bold red text
				Repeat:      false,
				Q:           1, // Centered
			},
		},
		{
			name: "redaction with markup properties",
			annotation: &Redact{
				Common: Common{
					Rect:  pdf.Rectangle{LLx: 100, LLy: 750, URx: 300, URy: 780},
					Flags: 1, // Invisible flag
				},
				Markup: Markup{
					User:    "Security Officer",
					Subject: "Personal Information Redaction",
				},
				IC: []float64{1.0, 1.0, 0.0}, // Yellow background
			},
		},
		{
			name: "minimal redaction annotation",
			annotation: &Redact{
				Common: Common{
					Rect: pdf.Rectangle{LLx: 0, LLy: 0, URx: 100, URy: 20},
				},
				// Only required fields (Subtype is automatically set)
			},
		},
	},
	"Projection": {
		{
			name: "basic projection annotation",
			annotation: &Projection{
				Common: Common{
					Rect:     pdf.Rectangle{LLx: 100, LLy: 100, URx: 200, URy: 150},
					Contents: "3D measurement comment",
				},
				Markup: Markup{
					User:    "3D Analyst",
					Subject: "Measurement annotation",
				},
			},
		},
		{
			name: "projection with external data dictionary",
			annotation: &Projection{
				Common: Common{
					Rect: pdf.Rectangle{LLx: 50, LLy: 200, URx: 300, URy: 250},
					Name: "proj-001",
				},
				Markup: Markup{
					User:         "Measurement Tool",
					CreationDate: time.Date(2023, 5, 10, 9, 15, 0, 0, time.UTC),
					Subject:      "3D measurement association",
				},
				ExData: pdf.NewReference(500, 0), // Reference to external data dictionary
			},
		},
		{
			name: "projection with zero rect (no AP)",
			annotation: &Projection{
				Common: Common{
					Rect:     pdf.Rectangle{LLx: 100, LLy: 300, URx: 100, URy: 350}, // Zero width
					Contents: "Measurement with zero width rect",
				},
				Markup: Markup{
					User:    "Specialist",
					Subject: "Zero-dimension measurement",
				},
				ExData: pdf.NewReference(600, 0),
			},
		},
		{
			name: "comprehensive projection annotation",
			annotation: &Projection{
				Common: Common{
					Rect:                    pdf.Rectangle{LLx: 200, LLy: 400, URx: 500, URy: 500},
					Contents:                "Comprehensive 3D measurement annotation",
					Name:                    "comprehensive-projection",
					Flags:                   2, // Print flag
					Color:                   color.DeviceGray(0.8),
					StrokingTransparency:    0.9,
					NonStrokingTransparency: 0.7,
				},
				Markup: Markup{
					User:         "3D Measurement System",
					Subject:      "Complex geospatial measurement",
					RC:           pdf.String("Associated with 3D measurement data"),
					CreationDate: time.Date(2023, 8, 20, 14, 30, 0, 0, time.UTC),
					InReplyTo:    pdf.NewReference(300, 0), // In reply to reference
					Intent:       "Group",
				},
				ExData: pdf.NewReference(700, 0), // External data dictionary with 3DM subtype
			},
		},
		{
			name: "minimal projection annotation",
			annotation: &Projection{
				Common: Common{
					Rect: pdf.Rectangle{LLx: 0, LLy: 0, URx: 50, URy: 25},
				},
				Markup: Markup{
					User: "User",
				},
				// No ExData - optional for projection annotations
			},
		},
	},
	"RichMedia": {
		{
			name: "basic rich media annotation",
			annotation: &RichMedia{
				Common: Common{
					Rect:     pdf.Rectangle{LLx: 100, LLy: 100, URx: 400, URy: 300},
					Contents: "3D model annotation",
				},
				RichMediaContent: pdf.NewReference(500, 0), // Required reference
			},
		},
		{
			name: "rich media with settings",
			annotation: &RichMedia{
				Common: Common{
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
			annotation: &RichMedia{
				Common: Common{
					Rect:                    pdf.Rectangle{LLx: 200, LLy: 500, URx: 600, URy: 700},
					Contents:                "Interactive 3D content with video and sound",
					Name:                    "comprehensive-richmedia",
					Flags:                   0, // No flags
					Color:                   color.DeviceGray(0.5),
					StrokingTransparency:    1.0,
					NonStrokingTransparency: 0.8,
					Border: &Border{
						HCornerRadius: 5.0,
						VCornerRadius: 5.0,
						Width:         2.0,
					},
				},
				RichMediaContent:  pdf.NewReference(800, 0), // Content with 3D, Sound, Video instances
				RichMediaSettings: pdf.NewReference(900, 0), // Activation/deactivation settings
			},
		},
		{
			name: "minimal rich media annotation",
			annotation: &RichMedia{
				Common: Common{
					Rect: pdf.Rectangle{LLx: 0, LLy: 0, URx: 200, URy: 150},
				},
				RichMediaContent: pdf.NewReference(400, 0), // Only required field
				// No RichMediaSettings - uses defaults
			},
		},
		{
			name: "rich media with other fields",
			annotation: &RichMedia{
				Common: Common{
					Rect:                    pdf.Rectangle{LLx: 300, LLy: 800, URx: 500, URy: 900},
					Contents:                "Rich media with additional fields",
					Name:                    "richmedia-advanced",
					StrokingTransparency:    0.9,
					NonStrokingTransparency: 0.7,
				},
				RichMediaContent:  pdf.NewReference(1100, 0),
				RichMediaSettings: pdf.NewReference(1200, 0),
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

func TestRoundTrip(t *testing.T) {
	for annotationType, cases := range testCases {
		for _, tc := range cases {
			t.Run(fmt.Sprintf("%s-%s", annotationType, tc.name), func(t *testing.T) {
				a := shallowCopy(tc.annotation)
				roundTripTest(t, a)
			})
		}
	}
}

func shallowCopy(iface Annotation) Annotation {
	origVal := reflect.ValueOf(iface)

	if origVal.Kind() != reflect.Ptr {
		return iface
	}

	elemType := origVal.Elem().Type()
	newPtr := reflect.New(elemType)
	newPtr.Elem().Set(origVal.Elem())

	return newPtr.Interface().(Annotation)
}

func TestRoundTripDict(t *testing.T) {
	for i, dict := range testDicts {
		t.Run(fmt.Sprintf("dict-%d", i), func(t *testing.T) {
			// step 1: make sure Extract does not crash or hang
			a, err := Extract(mock.Getter, dict)
			if err != nil {
				t.Error(err)
				return
			}

			// step 2: if we managed to extract an annotation, do a round-trip test
			roundTripTest(t, a)
		})
	}
}

// makeAppearance creates a simple appearance dictionary for testing
func makeAppearance(rm *pdf.ResourceManager, rect pdf.Rectangle) *appearance.Dict {
	// Create a simple form XObject with a transparent rectangle
	formObj := &form.Form{
		BBox: rect,
		Draw: func(gw *graphics.Writer) error {
			// Draw nothing - just a transparent appearance
			return nil
		},
	}

	// Embed the form and get its reference
	ref, _, err := pdf.ResourceManagerEmbed(rm, formObj)
	if err != nil {
		// Fallback to a simple appearance if embedding fails
		return &appearance.Dict{
			Normal:    nil,
			SingleUse: false,
		}
	}

	return &appearance.Dict{
		Normal:    ref,
		SingleUse: false,
	}
}

// roundTripTest performs a round-trip test for any annotation type
func roundTripTest(t *testing.T, a1 Annotation) {
	buf, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	rm := pdf.NewResourceManager(buf)

	// Ensure appearance dictionary compliance for PDF 2.0
	common := a1.GetCommon()
	common.Appearance = makeAppearance(rm, a1.GetCommon().Rect)
	common.AppearanceState = pdf.Name("Normal")

	// Embed the annotation
	embedded, err := a1.AsDict(rm)
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
		cmpopts.EquateComparable(language.Tag{}),
		// Ignore appearance dictionary in comparison since we add it artificially for testing
		cmpopts.IgnoreFields(Common{}, "Appearance"),
	}

	// For Unknown annotations, we don't expect perfect round-trip
	// because the embedding process may add common annotation fields
	if _, isUnknown := a1.(*Custom); isUnknown {
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

	unknown, ok := annotation.(*Custom)
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

	_, err = unknown.AsDict(rm2)
	if err != nil {
		t.Errorf("failed to embed unknown annotation: %v", err)
	}
}

func TestAnnotationTypes(t *testing.T) {
	tests := []struct {
		annotation   Annotation
		expectedType pdf.Name
	}{
		{&Text{}, "Text"},
		{&Link{}, "Link"},
		{&FreeText{}, "FreeText"},
		{&Line{}, "Line"},
		{&Custom{Type: "Custom", Data: pdf.Dict{"Subtype": pdf.Name("Custom")}}, "Custom"},
		{&Custom{Type: "Unknown", Data: pdf.Dict{}}, "Unknown"},
	}

	for _, tt := range tests {
		t.Run(string(tt.expectedType), func(t *testing.T) {
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

	// Add appearance dictionary for PDF 2.0 compliance
	annotation.Common.Appearance = makeAppearance(rm, annotation.Common.Rect)

	embedded, err := annotation.AsDict(rm)
	if err != nil {
		t.Fatal(err)
	}

	err = rm.Close()
	if err != nil {
		t.Fatal(err)
	}

	dict, err := pdf.GetDict(buf, embedded)
	if err != nil {
		t.Fatal(err)
	}

	// Border should not be present since it's the default value
	if _, exists := dict["Border"]; exists {
		t.Error("default border should not be written to PDF")
	}
}

func TestOpacityHandling(t *testing.T) {
	tests := []struct {
		name                  string
		strokeTransparency    float64
		nonStrokeTransparency float64
		expectCA              bool
		expectCa              bool
	}{
		{
			name:                  "default transparencies",
			strokeTransparency:    0.0,
			nonStrokeTransparency: 0.0,
			expectCA:              false,
			expectCa:              false,
		},
		{
			name:                  "custom stroke transparency",
			strokeTransparency:    0.5,
			nonStrokeTransparency: 0.5,
			expectCA:              true,
			expectCa:              false,
		},
		{
			name:                  "different transparencies",
			strokeTransparency:    0.2,
			nonStrokeTransparency: 0.4,
			expectCA:              true,
			expectCa:              true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			annotation := &Text{
				Common: Common{
					Rect:                    pdf.Rectangle{LLx: 0, LLy: 0, URx: 100, URy: 50},
					StrokingTransparency:    tt.strokeTransparency,
					NonStrokingTransparency: tt.nonStrokeTransparency,
				},
			}

			buf, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
			rm := pdf.NewResourceManager(buf)

			// Add appearance dictionary for PDF 2.0 compliance
			annotation.Common.Appearance = makeAppearance(rm, annotation.Common.Rect)

			embedded, err := annotation.AsDict(rm)
			if err != nil {
				t.Fatal(err)
			}

			err = rm.Close()
			if err != nil {
				t.Fatal(err)
			}

			dict, err := pdf.GetDict(rm.Out, embedded)
			if err != nil {
				t.Fatal(err)
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

			a := shallowCopy(tc.annotation)
			common := a.GetCommon()
			common.Appearance = makeAppearance(rm, a.GetCommon().Rect)
			common.AppearanceState = pdf.Name("Normal")

			embedded, err := a.AsDict(rm)
			if err != nil {
				continue
			}

			w.GetMeta().Trailer["Quir:X"] = embedded

			err = rm.Close()
			if err != nil {
				continue
			}

			err = w.Close()
			if err != nil {
				continue
			}

			f.Add(out.Data)
		}
	}
	for _, dict := range testDicts {
		opt := &pdf.WriterOptions{
			HumanReadable: true,
		}
		w, out := memfile.NewPDFWriter(pdf.V2_0, opt)
		rm := pdf.NewResourceManager(w)
		ref := w.Alloc()
		w.Put(ref, dict)
		err := rm.Close()
		if err != nil {
			continue
		}

		w.GetMeta().Trailer["Quir:X"] = ref
		err = w.Close()
		if err != nil {
			continue
		}

		f.Add(out.Data)
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
		if _, isUnknown := annotation.(*Custom); !isUnknown && annotationType == "" {
			t.Error("annotation type should not be empty for known types")
		}

		// Test embedding doesn't panic
		buf, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
		rm := pdf.NewResourceManager(buf)

		_, err = annotation.AsDict(rm)
		if err != nil {
			t.Errorf("failed to embed annotation: %v", err)
		}
	})
}
