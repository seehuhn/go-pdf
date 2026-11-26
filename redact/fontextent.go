// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package redact

import (
	"math"

	georect "seehuhn.de/go/geom/rect"
	"seehuhn.de/go/geom/vec"

	"seehuhn.de/go/postscript/cid"

	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/dict"
	"seehuhn.de/go/pdf/internal/stdmtx"
)

// fontDescriptor returns the font descriptor of f, which may be nil, and the
// PostScript name of the font.
func fontDescriptor(f font.Instance) (desc *font.Descriptor, psName string) {
	switch d := fontDict(f).(type) {
	case *dict.Type1:
		return d.Descriptor, d.PostScriptName
	case *dict.TrueType:
		return d.Descriptor, d.PostScriptName
	case *dict.CIDFontType0:
		return d.Descriptor, d.PostScriptName
	case *dict.CIDFontType2:
		return d.Descriptor, d.PostScriptName
	}
	return nil, ""
}

// fontExtent returns the vertical extent of the glyphs of f, as the descent
// (never positive) and the ascent (positive) of the glyph boxes, in text
// space units where 1 is the font size.  The values come from the font
// descriptor, from the y-range of its font bounding box, and from the
// metrics of the standard 14 fonts, in this order.  A positive descent is
// taken to be a sign error.  A source is skipped if its values are not
// finite or if its ascent is not positive.  If no source is usable, the
// descent and the ascent are zero and the result is false.
func fontExtent(f font.Instance) (descent, ascent float64, ok bool) {
	desc, psName := fontDescriptor(f)
	if desc != nil {
		if d, a, ok := usableExtent(-math.Abs(desc.Descent), desc.Ascent); ok {
			return d, a, true
		}
		if d, a, ok := usableExtent(desc.FontBBox.LLy, desc.FontBBox.URy); ok {
			return d, a, true
		}
	}
	if m, ok := stdmtx.Metrics[psName]; ok {
		if d, a, ok := usableExtent(-math.Abs(m.Descent), m.Ascent); ok {
			return d, a, true
		}
		if d, a, ok := usableExtent(m.FontBBox.LLy, m.FontBBox.URy); ok {
			return d, a, true
		}
	}
	return 0, 0, false
}

// usableExtent converts a descent and an ascent from glyph space units
// (1000 per em) to em, extends the range down to the baseline, and checks
// that the result is usable.
func usableExtent(descent, ascent float64) (d, a float64, ok bool) {
	d, a = min(descent/1000, 0), ascent/1000
	if !isFinite(d) || !isFinite(a) || a <= 0 {
		return 0, 0, false
	}
	return d, a, true
}

// infiniteRect is the extent of ink which has no bound.
var infiniteRect = georect.Rect{
	LLx: math.Inf(-1), LLy: math.Inf(-1),
	URx: math.Inf(+1), URy: math.Inf(+1),
}

// fontBBox returns the font bounding box of f in text space units, from the
// font descriptor or, failing that, from the metrics of the standard 14
// fonts.  The result is false if there is no usable box.
func fontBBox(f font.Instance) (georect.Rect, bool) {
	desc, psName := fontDescriptor(f)
	if desc != nil {
		if b, ok := usableBBox(desc.FontBBox); ok {
			return b, true
		}
	}
	if m, ok := stdmtx.Metrics[psName]; ok {
		return usableBBox(m.FontBBox)
	}
	return georect.Rect{}, false
}

// usableBBox converts a bounding box from glyph space units (1000 per em)
// to em, extends its y-range to contain the baseline, and checks that the
// result is usable.
func usableBBox(b georect.Rect) (georect.Rect, bool) {
	b.Scale(1.0 / 1000)
	b.LLy, b.URy = min(b.LLy, 0), max(b.URy, 0)
	ok := b.IsFinite() && b.URx > b.LLx && b.URy > b.LLy
	return b, ok
}

// glyphBoxes computes the boxes of the glyphs of a single font.
type glyphBoxes struct {
	vertical bool

	// descent and ascent give the vertical extent of the advance box, in
	// text space units
	descent, ascent float64

	// xRange, if set, widens the box of every glyph horizontally.  It is
	// used where the font gives no bounds for individual glyphs.
	xRange *[2]float64

	// substituted is set for a font whose glyphs a viewer draws with a
	// substitute font, see [substLo]
	substituted bool

	// ink gives the ink bounds of individual glyphs, or is nil
	ink inkFunc

	// inks caches ink by CID
	inks map[cid.CID]georect.Rect
}

// A font which is neither embedded nor one of the standard 14 fonts is drawn
// with a substitute font, whose glyphs cannot be known.  The box of such a
// glyph covers the range from substLo to substHi em around the horizontal
// glyph origin, horizontally and vertically.  The glyphs of text fonts lie
// well inside this range: the font bounding boxes of the standard 14 fonts,
// for example, lie within [-0.2, 1.2] em horizontally and [-0.3, 1.1] em
// vertically.  A viewer may scale the substitute glyphs horizontally to the
// width of the glyph, so for glyphs wider than 1 em the horizontal range is
// scaled by the width.  For a negative width, the range also covers its
// mirror image, in case a viewer mirrors the glyph.  In vertical writing
// mode, the substitute glyph is placed by the position vector of the font
// dictionary like any other glyph, so the range is relative to the horizontal
// origin there as well.
const (
	substLo = -0.5
	substHi = 1.5
)

// A glyphBoxCache holds the glyph boxes of the fonts of one document, so that
// the font program of a font is parsed once however many pages use it.  The
// key is the font instance, which the extractor of the document shares
// between all uses of one font dictionary.
type glyphBoxCache map[font.Instance]*glyphBoxes

// get returns the glyph boxes of f, creating them on first use.
func (c glyphBoxCache) get(f font.Instance) *glyphBoxes {
	gb, ok := c[f]
	if !ok {
		gb = newGlyphBoxes(f)
		c[f] = gb
	}
	return gb
}

// newGlyphBoxes returns the glyph boxes of f.
//
// The box of a glyph is the union of the advance box and the ink bounds of
// the glyph.  The advance box extends over the advance of the glyph and
// from the descent to the ascent of the font, or lies on the baseline if the
// font gives no usable ascent.  A Type 3 font has no ascent and descent.
// Where the font program does not give ink bounds for individual glyphs,
// since the font is substituted, the box also covers the range of substitute
// glyphs and the font bounding box.  A font whose embedded program cannot be
// analysed has no bound.
func newGlyphBoxes(f font.Instance) *glyphBoxes {
	info := f.FontInfo()
	g := &glyphBoxes{
		vertical: f.WritingMode() == font.Vertical,
		ink:      newInkFunc(info),
		inks:     map[cid.CID]georect.Rect{},
	}
	if _, isType3 := info.(*dict.FontInfoType3); isType3 {
		return g
	}
	g.descent, g.ascent, _ = fontExtent(f)
	if g.ink == nil {
		g.substituted = true
		g.descent, g.ascent = min(g.descent, substLo), max(g.ascent, substHi)
		if b, ok := fontBBox(f); ok {
			g.descent, g.ascent = min(g.descent, b.LLy), max(g.ascent, b.URy)
			g.xRange = &[2]float64{b.LLx, b.URx}
		}
	}
	return g
}

// points returns the corners of the box of the glyph for a code, in the
// text space of the glyph, relative to its horizontal origin.  The result is
// false if the box has no bound.  The analysis of a glyph is charged to the
// work budget with charge, once per glyph.
//
// Where the CMap of a CID-keyed font gives a notdef CID for the code, the
// box also covers the ink of that glyph, which a viewer shows if the font
// lacks the glyph of the CID.
func (g *glyphBoxes) points(code *font.Code, charge func(int64) error) ([]vec.Vec2, bool, error) {
	x0, x1 := min(0, code.Width), max(0, code.Width)
	if g.xRange != nil {
		x0, x1 = min(x0, g.xRange[0]), max(x1, g.xRange[1])
	}
	if g.substituted {
		s := max(1, math.Abs(code.Width))
		x0, x1 = min(x0, substLo*s), max(x1, substHi*s)
		if code.Width < 0 {
			x0 = min(x0, -substHi*s)
		}
	}
	pts := []vec.Vec2{{X: x0, Y: g.descent}, {X: x1, Y: g.descent}, {X: x1, Y: g.ascent}, {X: x0, Y: g.ascent}}
	if g.vertical {
		// the box together with the displacement of the glyph
		pts = append(pts,
			vec.Vec2{X: code.VerticalOriginX, Y: code.VerticalOriginY},
			vec.Vec2{X: code.VerticalOriginX, Y: code.VerticalOriginY + code.VerticalAdvance})
	}

	if g.ink == nil {
		return pts, true, nil
	}
	cids := []cid.CID{code.CID}
	if code.Notdef != 0 && code.Notdef != code.CID {
		cids = append(cids, code.Notdef)
	}
	for _, c := range cids {
		ink, err := g.inkOf(c, charge)
		switch {
		case err != nil:
			return nil, false, err
		case !ink.IsFinite():
			return nil, false, nil
		case !ink.IsZero():
			pts = append(pts,
				vec.Vec2{X: ink.LLx, Y: ink.LLy}, vec.Vec2{X: ink.URx, Y: ink.LLy},
				vec.Vec2{X: ink.URx, Y: ink.URy}, vec.Vec2{X: ink.LLx, Y: ink.URy})
		}
	}
	return pts, true, nil
}

// inkOf returns the ink bounds of the glyph for the CID c, analysing the
// glyph on first use.
func (g *glyphBoxes) inkOf(c cid.CID, charge func(int64) error) (georect.Rect, error) {
	if ink, ok := g.inks[c]; ok {
		return ink, nil
	}
	ink, err := g.ink(c, charge)
	if err != nil {
		return georect.Rect{}, err
	}
	g.inks[c] = ink
	return ink, nil
}
