// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package graphics_test

import (
	"maps"
	"slices"
	"testing"

	"seehuhn.de/go/geom/matrix"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/extract"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

// readVerticalFont writes a Type 0 font with the Identity-V encoding and a
// CIDFontType2 descendant with widths [600 500] for CIDs 1 and 2, adds the
// given entries to the CIDFont dictionary, and reads the font back.
func readVerticalFont(t *testing.T, extra pdf.Dict) font.Instance {
	t.Helper()

	w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
	cidFont := pdf.Dict{
		"Type":     pdf.Name("Font"),
		"Subtype":  pdf.Name("CIDFontType2"),
		"BaseFont": pdf.Name("Test"),
		"CIDSystemInfo": pdf.Dict{
			"Registry":   pdf.String("Adobe"),
			"Ordering":   pdf.String("Identity"),
			"Supplement": pdf.Integer(0),
		},
		"FontDescriptor": pdf.Dict{
			"Type":        pdf.Name("FontDescriptor"),
			"FontName":    pdf.Name("Test"),
			"Flags":       pdf.Integer(4),
			"FontBBox":    pdf.Array{pdf.Integer(0), pdf.Integer(-200), pdf.Integer(1000), pdf.Integer(800)},
			"ItalicAngle": pdf.Integer(0),
			"Ascent":      pdf.Integer(800),
			"Descent":     pdf.Integer(-200),
			"CapHeight":   pdf.Integer(700),
			"StemV":       pdf.Integer(80),
		},
		"W":           pdf.Array{pdf.Integer(1), pdf.Array{pdf.Integer(600), pdf.Integer(500)}},
		"CIDToGIDMap": pdf.Name("Identity"),
	}
	maps.Copy(cidFont, extra)
	cidRef := w.Alloc()
	if err := w.Put(cidRef, cidFont); err != nil {
		t.Fatal(err)
	}
	fontRef := w.Alloc()
	fontDict := pdf.Dict{
		"Type":            pdf.Name("Font"),
		"Subtype":         pdf.Name("Type0"),
		"BaseFont":        pdf.Name("Test"),
		"Encoding":        pdf.Name("Identity-V"),
		"DescendantFonts": pdf.Array{cidRef},
	}
	if err := w.Put(fontRef, fontDict); err != nil {
		t.Fatal(err)
	}

	d, err := extract.Dict(pdf.CursorAt(pdf.NewExtractor(w), nil), fontRef, false)
	if err != nil {
		t.Fatal(err)
	}
	F := d.MakeFont()
	if F.WritingMode() != font.Vertical {
		t.Fatal("font is not for vertical writing")
	}
	return F
}

// cids12 shows CIDs 1 and 2 under the Identity-V encoding.
var cids12 = pdf.String{0, 1, 0, 2}

// textState returns a graphics state which shows F at size 10, starting at
// (100, 700).
func textState(F font.Instance) *graphics.State {
	s := graphics.NewState()
	s.TextFont = F
	s.TextFontSize = 10
	s.TextHorizontalScaling = 1
	s.TextMatrix = matrix.Translate(100, 700)
	return &s
}

// TestAdvanceVerticalZero checks that an explicit vertical displacement of
// zero, from DW2 or from W2, leaves the text position unchanged.
func TestAdvanceVerticalZero(t *testing.T) {
	F := readVerticalFont(t, pdf.Dict{
		"DW2": pdf.Array{pdf.Integer(880), pdf.Integer(0)},
		"W2":  pdf.Array{pdf.Integer(2), pdf.Array{pdf.Integer(0), pdf.Integer(250), pdf.Integer(772)}},
	})
	s := textState(F)
	start := s.TextMatrix
	for c := range F.Codes(cids12) {
		if c.VerticalAdvance != 0 {
			t.Errorf("CID %d: VerticalAdvance %g, want 0", c.CID, c.VerticalAdvance)
		}
		s.AdvanceTextMatrix(&c)
		if s.TextMatrix != start {
			t.Errorf("CID %d: text matrix moved to %v", c.CID, s.TextMatrix)
		}
	}
}

// TestAdvanceVerticalDefault checks that a font without DW2 and W2 advances
// by one em downward per glyph.
func TestAdvanceVerticalDefault(t *testing.T) {
	F := readVerticalFont(t, nil)
	s := textState(F)
	var got []matrix.Matrix
	for c := range F.Codes(cids12) {
		s.AdvanceTextMatrix(&c)
		got = append(got, s.TextMatrix)
	}
	want := []matrix.Matrix{matrix.Translate(100, 690), matrix.Translate(100, 680)}
	if !slices.Equal(got, want) {
		t.Errorf("text matrices %v, want %v", got, want)
	}
}

// TestVerticalPositionVector checks that the position vector of a glyph
// reaches font.Code: from W2, from DW2 with half the glyph width, and from the
// default DW2 [880 -1000] when the font dictionary gives none.
func TestVerticalPositionVector(t *testing.T) {
	type vert struct{ advance, vx, vy float64 }
	for _, tc := range []struct {
		name  string
		extra pdf.Dict
		want  []vert
	}{
		{"spec default", nil, []vert{{-1, 0.3, 0.88}, {-1, 0.25, 0.88}}},
		{"DW2", pdf.Dict{
			"DW2": pdf.Array{pdf.Integer(700), pdf.Integer(-900)},
		}, []vert{{-0.9, 0.3, 0.7}, {-0.9, 0.25, 0.7}}},
		{"W2", pdf.Dict{
			"DW2": pdf.Array{pdf.Integer(880), pdf.Integer(0)},
			"W2":  pdf.Array{pdf.Integer(2), pdf.Array{pdf.Integer(0), pdf.Integer(200), pdf.Integer(772)}},
		}, []vert{{0, 0.3, 0.88}, {0, 0.2, 0.772}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			F := readVerticalFont(t, tc.extra)
			var got []vert
			for c := range F.Codes(cids12) {
				got = append(got, vert{c.VerticalAdvance, c.VerticalOriginX, c.VerticalOriginY})
			}
			if !slices.Equal(got, tc.want) {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

// TestGlyphMatrixVertical checks that in vertical writing mode the glyph
// outline is shifted by minus the position vector before font size,
// horizontal scaling and rise are applied.
func TestGlyphMatrixVertical(t *testing.T) {
	F := readVerticalFont(t, nil)
	s := textState(F)
	s.TextHorizontalScaling = 0.5
	s.TextRise = 3
	for c := range F.Codes(cids12) {
		got := s.GlyphMatrix(&c)
		// x: 100 - vx·Tfs·Th, y: 700 - vy·Tfs + Trise
		want := matrix.Matrix{5, 0, 0, 10, 100 - c.VerticalOriginX*5, 700 - c.VerticalOriginY*10 + 3}
		for i := range got {
			if d := got[i] - want[i]; d > 1e-9 || d < -1e-9 {
				t.Errorf("CID %d: glyph matrix %v, want %v", c.CID, got, want)
				break
			}
		}
	}
}

// TestGlyphMatrixHorizontal checks that in horizontal writing mode the glyph
// matrix is the text rendering matrix without the CTM, bit for bit, whatever
// the vertical fields of the code hold.
func TestGlyphMatrixHorizontal(t *testing.T) {
	s := graphics.NewState()
	s.TextFont = horizontalFont{}
	s.TextFontSize = 7.3
	s.TextHorizontalScaling = 0.9
	s.TextRise = -1.7
	s.TextMatrix = matrix.Matrix{0.6, 0.8, -0.8, 0.6, 123.4, 567.8}
	s.CTM = matrix.Matrix{1.1, 0.3, -0.2, 0.9, 10.5, -20.25}

	c := font.Code{Width: 0.5, VerticalAdvance: -1, VerticalOriginX: 0.25, VerticalOriginY: 0.88}
	if got, want := s.GlyphMatrix(&c).Mul(s.CTM), s.TextRenderingMatrix(); got != want {
		t.Errorf("glyph matrix %v, want %v", got, want)
	}
}

// horizontalFont is a font.Instance for horizontal writing with no codes.
type horizontalFont struct{ font.Instance }

func (horizontalFont) WritingMode() font.WritingMode { return font.Horizontal }
