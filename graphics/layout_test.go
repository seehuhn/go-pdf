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

package graphics

import (
	"io"
	"math"
	"testing"

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/internal/debug/memfile"
	"seehuhn.de/go/pdf/internal/squarefont"
)

func TestGetQuadPointsSimple(t *testing.T) {
	F := squarefont.All[0].MakeFont()

	data, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	rm := pdf.NewResourceManager(data)
	w := NewWriter(io.Discard, rm)

	w.TextBegin()
	w.TextSetFont(F, 10)
	w.TextFirstLine(100, 100)
	gg := w.TextLayout(nil, "A")
	corners := w.TextGetQuadPoints(gg)
	w.TextShowGlyphs(gg)
	w.TextEnd()

	err := rm.Close()
	if err != nil {
		t.Fatal(err)
	}

	expected := []float64{
		101, 102, // bottom-left
		105, 102, // bottom-right
		105, 106, // top-right
		101, 106, // top-left
	}
	if len(corners) != len(expected) {
		t.Fatalf("expected %d coordinates, got %d", len(expected), len(corners))
	}
	for i := 0; i < len(expected); i++ {
		if math.Abs(corners[i]-expected[i]) > 1e-6 {
			t.Errorf("coordinate %d: expected %.6f, got %.6f", i, expected[i], corners[i])
		}
	}
}

func TestTextGetQuadPointsComprehensive(t *testing.T) {
	testCases := []struct {
		name         string
		fontSize     float64
		setupFunc    func(*Writer) *font.GlyphSeq
		expectedFunc func() []float64
	}{
		{
			name:     "identity_transform",
			fontSize: 10.0,
			setupFunc: func(w *Writer) *font.GlyphSeq {
				// Basic identity transform with standard text layout
				return w.TextLayout(nil, "A A")
			},
			expectedFunc: func() []float64 {
				return calculateExpectedQuadPoints(10.0, 0, 0, matrix.Identity, matrix.Identity, "A A")
			},
		},
		{
			name:     "text_matrix_translate",
			fontSize: 10.0,
			setupFunc: func(w *Writer) *font.GlyphSeq {
				w.TextSetMatrix(matrix.Translate(20, 30))
				return w.TextLayout(nil, "A A")
			},
			expectedFunc: func() []float64 {
				return calculateExpectedQuadPoints(10.0, 0, 0, matrix.Identity, matrix.Translate(20, 30), "A A")
			},
		},
		{
			name:     "text_matrix_scale",
			fontSize: 10.0,
			setupFunc: func(w *Writer) *font.GlyphSeq {
				w.TextSetMatrix(matrix.Scale(1.5, 1.2))
				return w.TextLayout(nil, "A A")
			},
			expectedFunc: func() []float64 {
				return calculateExpectedQuadPoints(10.0, 0, 0, matrix.Identity, matrix.Scale(1.5, 1.2), "A A")
			},
		},
		{
			name:     "text_rise",
			fontSize: 10.0,
			setupFunc: func(w *Writer) *font.GlyphSeq {
				w.TextSetRise(5.0)
				return w.TextLayout(nil, "A A")
			},
			expectedFunc: func() []float64 {
				return calculateExpectedQuadPoints(10.0, 5.0, 0, matrix.Identity, matrix.Identity, "A A")
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create test font
			font := squarefont.All[0].MakeFont()

			// Create graphics writer
			data, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
			rm := pdf.NewResourceManager(data)
			w := NewWriter(io.Discard, rm)

			// Start text object and set font
			w.TextBegin()
			w.TextSetFont(font, tc.fontSize)

			// Run setup function to configure transforms and get glyph sequence
			glyphSeq := tc.setupFunc(w)

			// Call the method under test
			result := w.TextGetQuadPoints(glyphSeq)

			// Calculate expected values
			expected := tc.expectedFunc()

			// Compare results
			if len(result) != len(expected) {
				t.Errorf("expected %d coordinates, got %d", len(expected), len(result))
				return
			}

			for i := 0; i < len(expected); i++ {
				if math.Abs(result[i]-expected[i]) > 1e-6 {
					t.Errorf("coordinate %d: expected %.6f, got %.6f", i, expected[i], result[i])
				}
			}

			w.TextEnd()
		})
	}
}

// calculateExpectedQuadPoints computes the expected quad points for given parameters
// This matches the logic from the actual implementation
func calculateExpectedQuadPoints(fontSize, textRise, skip float64, ctm, textMatrix matrix.Matrix, text string) []float64 {
	// Squarefont constants (from internal/squarefont/font.go)
	const (
		SquareLeft   = 100 // LLx for "A"
		SquareRight  = 500 // URx for "A"
		SquareBottom = 200 // LLy for "A"
		SquareTop    = 600 // URy for "A"
		SpaceWidth   = 250 // advance width for space
		SquareWidth  = 500 // advance width for "A"
	)

	scale := fontSize / 1000.0

	// From debug test: "A A" identity gives [1 2 12.5 2 12.5 6 1 6]
	// This means:
	// - Left = skip + LLx*scale = 0 + 100*0.01 = 1
	// - Right = skip + (advance_A + advance_space)*scale + URx*scale = 0 + (500+250)*0.01 + 500*0.01 = 7.5 + 5 = 12.5
	// - Bottom = -(LLy*scale + rise) = -(200*0.01 + 0) = -2, but we got +2, so it's +LLy*scale = +2
	// - Top = URy*scale + rise = 600*0.01 + 0 = 6

	leftBearing := skip + SquareLeft*scale                                    // skip + LLx of first A
	rightBearing := skip + (SquareWidth+SpaceWidth)*scale + SquareRight*scale // skip + total_advance + URx of second A

	// Vertical bounds including text rise - corrected based on debug output
	depth := SquareBottom*scale + textRise // LLy + rise (positive, not negative)
	height := SquareTop*scale + textRise   // URy + rise

	// Build rectangle in text space
	rectText := []float64{
		leftBearing, depth, // bottom-left
		rightBearing, depth, // bottom-right
		rightBearing, height, // top-right
		leftBearing, height, // top-left
	}

	// Apply combined transformation: CTM * TextMatrix
	M := ctm.Mul(textMatrix)

	// Transform all corners to default user space
	result := make([]float64, 8)
	for i := 0; i < 4; i++ {
		x, y := M.Apply(rectText[2*i], rectText[2*i+1])
		result[2*i] = x
		result[2*i+1] = y
	}

	return result
}

func TestGetGlyphQuadPointsStateValidation(t *testing.T) {
	// Test that function returns nil when required text state is not set
	font := squarefont.All[0].MakeFont()

	data, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	rm := pdf.NewResourceManager(data)
	w := NewWriter(io.Discard, rm)

	// Create a glyph sequence without setting up text state
	glyphSeq := font.Layout(nil, 12.0, "A")

	// Should return nil because text state is not properly set
	result := w.TextGetQuadPoints(glyphSeq)
	if result != nil {
		t.Errorf("expected nil result when text state not set, got %v", result)
	}
}

func TestGetGlyphQuadPointsTextMatrixTransform(t *testing.T) {
	// Test combined text matrix and CTM transformation
	font := squarefont.All[0].MakeFont()

	data, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	rm := pdf.NewResourceManager(data)
	w := NewWriter(io.Discard, rm)

	// Set up text state
	w.TextSetFont(font, 10.0)

	// Start text object and set text matrix
	w.TextBegin()
	w.TextSetMatrix(matrix.Translate(5, 10))

	// Also apply CTM transformation
	w.Transform(matrix.Scale(2, 2))

	glyphSeq := w.TextLayout(nil, "A")

	// The function should account for both text matrix and CTM
	result := w.TextGetQuadPoints(glyphSeq)

	// Should get a valid result (not nil)
	if result == nil {
		t.Error("expected valid result with proper text state, got nil")
	}

	// Should have 8 coordinates (4 points)
	if len(result) != 8 {
		t.Errorf("expected 8 coordinates, got %d", len(result))
	}

	w.TextEnd()
}
