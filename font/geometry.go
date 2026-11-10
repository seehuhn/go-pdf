// Copyright 2024 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package font

import (
	"seehuhn.de/go/geom/rect"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/sfnt/glyph"
)

// Geometry collects the various dimensions connected to a font and to
// the individual glyphs. All dimensions are in text space units.
//
// Leading, CapHeight and XHeight are always positive.  Fonts which report
// none, such as Type 1 fonts embedded without metrics, have them measured
// from the glyphs where possible and estimated otherwise.
type Geometry struct {
	Ascent             float64 // text space units
	Descent            float64 // negative, text space units
	Leading            float64 // text space units, always positive
	CapHeight          float64 // text space units, always positive
	XHeight            float64 // text space units, always positive
	UnderlinePosition  float64 // text space units
	UnderlineThickness float64 // text space units

	GlyphExtents []rect.Rect // indexed by GID, text space units

	Widths []float64 // indexed by GID, text space units
}

// GetGeometry returns the geometry of a font.
func (g *Geometry) GetGeometry() *Geometry {
	return g
}

// BoundingBox returns the bounding box of a glyph sequence,
// assuming that it is typeset at point (0, 0) using the given font size.
func (g *Geometry) BoundingBox(fontSize float64, gg *GlyphSeq) *pdf.Rectangle {
	res := &pdf.Rectangle{}
	if gg == nil || len(gg.Seq) == 0 {
		return res
	}

	xPos := gg.Skip
	for _, glyph := range gg.Seq {
		bbox := g.GlyphExtents[glyph.GID]
		if !bbox.IsZero() {
			res.Extend(&pdf.Rectangle{
				LLx: bbox.LLx*fontSize + xPos,
				LLy: bbox.LLy*fontSize + glyph.Rise,
				URx: bbox.URx*fontSize + xPos,
				URy: bbox.URy*fontSize + glyph.Rise,
			})
		}
		xPos += glyph.Advance
	}
	return res
}

// IsFixedPitch returns true if all glyphs in the font have the same width.
func (g *Geometry) IsFixedPitch() bool {
	ww := g.Widths
	if len(ww) == 0 {
		return false
	}

	var width float64
	for _, w := range ww {
		if w == 0 {
			continue
		}
		if width == 0 {
			width = w
		} else if width != w {
			return false
		}
	}

	return true
}

// IsBlank reports whether the glyph is blank.
func (g *Geometry) IsBlank(gid glyph.ID) bool {
	return g.GlyphExtents[gid].IsZero()
}
