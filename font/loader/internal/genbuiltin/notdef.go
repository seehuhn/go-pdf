// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"fmt"
	"math"

	"seehuhn.de/go/postscript/funit"

	"seehuhn.de/go/sfnt"
	"seehuhn.de/go/sfnt/cff"
	"seehuhn.de/go/sfnt/glyf"
)

// The .notdef glyph of every Quire font is a hollow upright rectangle,
// centred in its advance width, which stands on the baseline and reaches the
// cap height.  The proportions below are fractions of the em.
const (
	notdefAdvance   = 0.6  // advance width, for fonts which are not fixed-pitch
	notdefBoxWidth  = 0.5  // outer width of the box
	notdefStroke    = 0.05 // thickness of the box outline
	notdefCapHeight = 0.7  // box height, for fonts without a cap height
)

// notdefBox gives the outer and inner rectangle of the .notdef glyph, in font
// design units.
type notdefBox struct {
	advance      float64
	outer, inner funit.Rect16
}

// makeNotdefBox computes the .notdef rectangles for a font.  An advance of
// zero selects the default advance width.
func makeNotdefBox(unitsPerEm, advance, capHeight float64) notdefBox {
	if advance <= 0 {
		advance = math.Round(notdefAdvance * unitsPerEm)
	}
	if capHeight <= 0 {
		capHeight = notdefCapHeight * unitsPerEm
	}
	stroke := math.Round(notdefStroke * unitsPerEm)
	llx := math.Round((advance - notdefBoxWidth*unitsPerEm) / 2)
	urx := llx + math.Round(notdefBoxWidth*unitsPerEm)
	ury := math.Round(capHeight)
	return notdefBox{
		advance: advance,
		outer: funit.Rect16{
			LLx: funit.Int16(llx), LLy: 0,
			URx: funit.Int16(urx), URy: funit.Int16(ury),
		},
		inner: funit.Rect16{
			LLx: funit.Int16(llx + stroke), LLy: funit.Int16(stroke),
			URx: funit.Int16(urx - stroke), URy: funit.Int16(ury - stroke),
		},
	}
}

// setNotdefGlyf replaces glyph 0 of a TrueType font by the .notdef box.
// A fixed-pitch font keeps its advance width.
func setNotdefGlyf(f *sfnt.Font) error {
	outl, ok := f.Outlines.(*glyf.Outlines)
	if !ok {
		return fmt.Errorf("unexpected outline type %T", f.Outlines)
	}
	var advance float64
	if f.IsFixedPitch() {
		advance = float64(outl.Widths[0])
	}
	box := makeNotdefBox(float64(f.UnitsPerEm), advance, float64(f.CapHeight))

	// TrueType outer contours run clockwise
	o, i := box.outer, box.inner
	sg := glyf.SimpleUnpacked{
		Contours: []glyf.Contour{
			{{X: o.LLx, Y: o.LLy, OnCurve: true}, {X: o.LLx, Y: o.URy, OnCurve: true},
				{X: o.URx, Y: o.URy, OnCurve: true}, {X: o.URx, Y: o.LLy, OnCurve: true}},
			{{X: i.LLx, Y: i.LLy, OnCurve: true}, {X: i.URx, Y: i.LLy, OnCurve: true},
				{X: i.URx, Y: i.URy, OnCurve: true}, {X: i.LLx, Y: i.URy, OnCurve: true}},
		},
	}
	g := sg.AsGlyph()
	outl.Glyphs[0] = &g
	outl.Widths[0] = funit.Uint16(box.advance)
	return nil
}

// setNotdefCFF replaces glyph 0 of a CFF font by the .notdef box.
func setNotdefCFF(outlines *cff.Outlines, unitsPerEm, capHeight float64) {
	box := makeNotdefBox(unitsPerEm, 0, capHeight)

	// CFF outer contours run counter-clockwise
	o, i := box.outer, box.inner
	g := cff.NewGlyph(".notdef", box.advance)
	g.MoveTo(float64(o.LLx), float64(o.LLy))
	g.LineTo(float64(o.URx), float64(o.LLy))
	g.LineTo(float64(o.URx), float64(o.URy))
	g.LineTo(float64(o.LLx), float64(o.URy))
	g.MoveTo(float64(i.LLx), float64(i.LLy))
	g.LineTo(float64(i.LLx), float64(i.URy))
	g.LineTo(float64(i.URx), float64(i.URy))
	g.LineTo(float64(i.URx), float64(i.LLy))
	outlines.Glyphs[0] = g
}
