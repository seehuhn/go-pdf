// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package redact

import (
	"math"

	"seehuhn.de/go/geom/matrix"
	georect "seehuhn.de/go/geom/rect"
	"seehuhn.de/go/geom/vec"

	"seehuhn.de/go/postscript/cid"

	"seehuhn.de/go/sfnt"
	"seehuhn.de/go/sfnt/cff"
	"seehuhn.de/go/sfnt/glyf"
	sglyph "seehuhn.de/go/sfnt/glyph"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font/dict"
	"seehuhn.de/go/pdf/font/encoding"
	"seehuhn.de/go/pdf/font/glyphdata"
	"seehuhn.de/go/pdf/font/glyphdata/cffglyphs"
	"seehuhn.de/go/pdf/font/glyphdata/sfntglyphs"
	"seehuhn.de/go/pdf/font/glyphdata/type1glyphs"
	"seehuhn.de/go/pdf/font/standard"
	"seehuhn.de/go/pdf/font/type3"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/internal/stdmtx"
)

// An inkFunc returns a rectangle in text space which contains the ink of the
// glyph a CID selects.  The rectangle is zero for a glyph which paints
// nothing, and not finite where the ink has no bound.  The function charges
// the work of an analysis to the work budget with charge, and returns
// [ErrTooComplex] if the budget is used up.
type inkFunc func(c cid.CID, charge func(int64) error) (georect.Rect, error)

// A plainInk is an inkFunc which needs no work accounting.
type plainInk func(c cid.CID) georect.Rect

// lookupInk adapts a plainInk to an inkFunc.
func lookupInk(f plainInk) inkFunc {
	if f == nil {
		return nil
	}
	return func(c cid.CID, _ func(int64) error) (georect.Rect, error) {
		return f(c), nil
	}
}

// unboundedInk is the inkFunc of a font whose embedded program cannot be
// analysed: every glyph has ink without a bound.
func unboundedInk(cid.CID, func(int64) error) (georect.Rect, error) {
	return infiniteRect, nil
}

// orUnbounded returns f, or unboundedInk if f is nil.
func orUnbounded(f inkFunc) inkFunc {
	if f == nil {
		return unboundedInk
	}
	return f
}

// chargedInk adapts a plainInk to an inkFunc which charges the work
// for the analysis of a glyph to the work budget.  The function cost gives
// the number of units of work for a glyph, in proportion to the size of its
// outline.
func chargedInk(cost func(cid.CID) int64, f plainInk) inkFunc {
	return func(c cid.CID, charge func(int64) error) (georect.Rect, error) {
		if err := charge(cost(c)); err != nil {
			return georect.Rect{}, err
		}
		return f(c), nil
	}
}

// newInkFunc returns the per-glyph ink bounds of a font, or nil if the font
// gives none.  The argument is the result of the FontInfo method of the font.
// Fonts without an embedded program give bounds only if they are one of the
// standard 14 fonts.  Where the embedded program cannot be read or is of an
// unsupported type, every glyph has ink without a bound.
func newInkFunc(info any) inkFunc {
	switch fi := info.(type) {
	case *dict.FontInfoSimple:
		if fi.FontFile == nil {
			return lookupInk(standardInk(fi))
		}
		return orUnbounded(simpleInk(fi))
	case *dict.FontInfoCID:
		if fi.FontFile == nil {
			return nil
		}
		return orUnbounded(cidInk(fi))
	case *dict.FontInfoGlyfEmbedded:
		if fi.FontFile == nil {
			return nil
		}
		return orUnbounded(glyfInk(fi))
	case *dict.FontInfoType3:
		return type3Ink(fi)
	}
	return nil
}

// simpleGlyphName returns the glyph name a simple font selects for a CID,
// where builtin is the built-in encoding of the font program.
func simpleGlyphName(enc encoding.Simple, c cid.CID, builtin []string) string {
	if c == 0 || c > 256 {
		return ".notdef"
	}
	code := byte(c - 1)
	name, ok := enc(code)
	if !ok {
		return ".notdef"
	}
	if name == encoding.UseBuiltin {
		if int(code) >= len(builtin) {
			return ".notdef"
		}
		return builtin[code]
	}
	return name
}

// standardInk returns the ink bounds of one of the standard 14 fonts, which
// the font data in this library gives per glyph.  A glyph name which the font
// lacks has the range of a substitute glyph.
func standardInk(fi *dict.FontInfoSimple) plainInk {
	sf, ok := standard.ByName(fi.PostScriptName)
	if !ok {
		return nil
	}
	inst, err := sf.New()
	if err != nil || inst.Geometry == nil || len(inst.GlyphExtents) != len(inst.GlyphNames) {
		return nil
	}
	byName := make(map[string]georect.Rect, len(inst.GlyphNames))
	for i, name := range inst.GlyphNames {
		byName[name] = inst.GlyphExtents[i]
	}
	var builtin []string
	if m, ok := stdmtx.Metrics[sf.PostScriptName()]; ok {
		builtin = m.Encoding
	}
	return func(c cid.CID) georect.Rect {
		name := simpleGlyphName(fi.Encoding, c, builtin)
		if r, ok := byName[name]; ok {
			return r
		}
		// the viewer can take the glyph from a font with more glyphs
		return substRange
	}
}

// substRange is the range of the glyphs of a substitute font, see [substLo].
var substRange = georect.Rect{LLx: substLo, LLy: substLo, URx: substHi, URy: substHi}

// simpleInk returns the ink bounds of a simple font with an embedded program.
func simpleInk(fi *dict.FontInfoSimple) inkFunc {
	switch fi.FontFile.Type {
	case glyphdata.Type1:
		t1, err := type1glyphs.FromStream(fi.FontFile)
		if err != nil {
			return nil
		}
		return chargedInk(func(c cid.CID) int64 {
			g := t1.Glyphs[simpleGlyphName(fi.Encoding, c, t1.Encoding)]
			if g == nil {
				g = t1.Glyphs[".notdef"]
			}
			if g == nil || g.Outline == nil {
				return 1
			}
			return 1 + int64(len(g.Outline.Cmds))
		}, func(c cid.CID) georect.Rect {
			name := simpleGlyphName(fi.Encoding, c, t1.Encoding)
			b := t1.GlyphBBoxPDF(name)
			b.Scale(1.0 / 1000)
			return b
		})

	case glyphdata.CFFSimple:
		cf, err := cffglyphs.FromStream(fi.FontFile)
		if err != nil || cf.Outlines == nil {
			return nil
		}
		sel := cffglyphs.NewSimpleSelector(cf, fi.Encoding)
		return cffInk(cf, func(c cid.CID) sglyph.ID {
			gid, _ := sel(c)
			return gid
		})

	case glyphdata.OpenTypeCFFSimple, glyphdata.TrueType, glyphdata.OpenTypeGlyf:
		sf, err := sfntglyphs.FromStream(fi.FontFile)
		if err != nil {
			return nil
		}
		sel := sfntglyphs.NewSimpleSelector(sf, fi.IsSymbolic, fi.Encoding)
		return sfntInk(sf, func(c cid.CID) sglyph.ID {
			gid, _ := sel(c)
			return gid
		})
	}
	return nil
}

// cffInk returns the ink bounds of the glyphs of a CFF font program, where
// toGID selects the glyph for a CID.
func cffInk(cf *cff.Font, toGID func(cid.CID) sglyph.ID) inkFunc {
	fm := matrix.Matrix(cf.FontMatrix)
	gidOf := func(c cid.CID) sglyph.ID {
		gid := toGID(c)
		if int(gid) >= cf.NumGlyphs() {
			gid = 0
		}
		return gid
	}
	return chargedInk(func(c cid.CID) int64 {
		return 1 + int64(len(cf.Glyphs[gidOf(c)].Cmds))
	}, func(c cid.CID) georect.Rect {
		gid := gidOf(c)
		return cf.GlyphBBox(cf.GlyphMatrix(fm, gid), gid)
	})
}

// cidInk returns the ink bounds of a CID-keyed font with an embedded program.
func cidInk(fi *dict.FontInfoCID) inkFunc {
	switch fi.FontFile.Type {
	case glyphdata.CFF:
		cf, err := cffglyphs.FromStream(fi.FontFile)
		if err != nil || cf.Outlines == nil {
			return nil
		}
		return cffInk(cf, cffglyphs.CIDToGID(cf))

	case glyphdata.OpenTypeCFF:
		sf, err := sfntglyphs.FromStream(fi.FontFile)
		if err != nil {
			return nil
		}
		toGID := func(c cid.CID) sglyph.ID { return sglyph.ID(c) }
		if cf := sf.AsCFF(); cf != nil {
			toGID = cffglyphs.CIDToGID(cf)
		}
		return sfntInk(sf, toGID)
	}
	return nil
}

// glyfInk returns the ink bounds of an embedded TrueType font used as a
// CID-keyed font.
func glyfInk(fi *dict.FontInfoGlyfEmbedded) inkFunc {
	sf, err := sfntglyphs.FromStream(fi.FontFile)
	if err != nil {
		return nil
	}
	return sfntInk(sf, func(c cid.CID) sglyph.ID {
		if len(fi.CIDToGID) == 0 {
			return sglyph.ID(c)
		}
		if int(c) >= len(fi.CIDToGID) {
			return 0
		}
		return fi.CIDToGID[c]
	})
}

// sfntInk returns the ink bounds of the glyphs of an sfnt font, where toGID
// selects the glyph for a CID.
func sfntInk(sf *sfnt.Font, toGID func(cid.CID) sglyph.ID) inkFunc {
	upem := float64(sf.UnitsPerEm)
	if !(upem > 0) {
		return nil
	}
	n := sf.NumGlyphs()
	gidOf := func(c cid.CID) sglyph.ID {
		gid := toGID(c)
		if int(gid) >= n {
			gid = 0
		}
		return gid
	}
	if o, ok := sf.Outlines.(*glyf.Outlines); ok {
		gi := &glyfInkBoxes{glyphs: o.Glyphs, boxes: map[sglyph.ID]georect.Rect{}}
		return func(c cid.CID, charge func(int64) error) (georect.Rect, error) {
			b, err := gi.box(gidOf(c), charge)
			if err != nil {
				return georect.Rect{}, err
			}
			b.Scale(1 / upem)
			return b, nil
		}
	}
	cost := func(c cid.CID) int64 {
		if o, ok := sf.Outlines.(*cff.Outlines); ok {
			return 1 + int64(len(o.Glyphs[gidOf(c)].Cmds))
		}
		return 1
	}
	return chargedInk(cost, func(c cid.CID) georect.Rect {
		b := sf.GlyphBBox(gidOf(c))
		b.Scale(1 / upem)
		return b
	})
}

// glyfInkBoxes computes the ink bounds of the glyphs of a TrueType font, in
// font design units.  The bounding box in the header of a glyph is not
// trusted to contain the outline: the bounds of a simple glyph contain the
// header box and all points of the outline, before hinting.  The bounds of a
// composite glyph contain its header box and the bounds of its components,
// for every way of placing the components which the flags allow.
type glyfInkBoxes struct {
	glyphs glyf.Glyphs

	// boxes caches the bounds by glyph
	boxes map[sglyph.ID]georect.Rect

	// active holds the composite glyphs whose bounds are being computed
	active map[sglyph.ID]bool
}

// box returns the ink bounds of the glyph gid.  The bounds are not finite for
// a glyph whose outline cannot be decoded or which contains itself.  The
// work for each glyph is charged once.
func (gi *glyfInkBoxes) box(gid sglyph.ID, charge func(int64) error) (georect.Rect, error) {
	if b, ok := gi.boxes[gid]; ok {
		return b, nil
	}
	if int(gid) >= len(gi.glyphs) || gi.glyphs[gid] == nil {
		return georect.Rect{}, nil
	}
	if gi.active[gid] {
		return infiniteRect, nil
	}
	g := gi.glyphs[gid]
	b := georect.Rect{
		LLx: float64(g.LLx), LLy: float64(g.LLy),
		URx: float64(g.URx), URy: float64(g.URy),
	}

	switch d := g.Data.(type) {
	case glyf.SimpleGlyph:
		if err := charge(1 + int64(len(d.Encoded))); err != nil {
			return georect.Rect{}, err
		}
		u, err := d.Unpack()
		if err != nil {
			b = infiniteRect
			break
		}
		npts := 0
		for _, cc := range u.Contours {
			npts += len(cc)
		}
		if err := charge(int64(npts)); err != nil {
			return georect.Rect{}, err
		}
		for _, cc := range u.Contours {
			for _, p := range cc {
				b.Extend(pointRect(float64(p.X), float64(p.Y)))
			}
		}

	case glyf.CompositeGlyph:
		if err := charge(1 + int64(len(d.Components))); err != nil {
			return georect.Rect{}, err
		}
		if gi.active == nil {
			gi.active = map[sglyph.ID]bool{}
		}
		gi.active[gid] = true
		placed, err := gi.composite(d, charge)
		delete(gi.active, gid)
		if err != nil {
			return georect.Rect{}, err
		}
		b.Extend(placed)
	}
	gi.boxes[gid] = b
	return b, nil
}

// composite returns the bounds of the components of a composite glyph.
func (gi *glyfInkBoxes) composite(d glyf.CompositeGlyph, charge func(int64) error) (georect.Rect, error) {
	var res georect.Rect
	for _, comp := range d.Components {
		cu, err := comp.Unpack()
		if err != nil {
			return infiniteRect, nil
		}
		child, err := gi.box(cu.Child, charge)
		if err != nil {
			return georect.Rect{}, err
		}
		if !child.IsFinite() {
			return infiniteRect, nil
		}
		if child.IsZero() {
			continue
		}
		lin := cu.Trfm
		lin[4], lin[5] = 0, 0
		t := child.Transform(lin)

		if cu.AlignPoints {
			// The offset moves a point of the child onto a point of the
			// components before it.  The bound also covers point indices
			// which are out of range, by including the origin.
			prev, own := res, t
			prev.Extend(pointRect(0, 0))
			own.Extend(pointRect(0, 0))
			res.Extend(georect.Rect{
				LLx: t.LLx + prev.LLx - own.URx, LLy: t.LLy + prev.LLy - own.URy,
				URx: t.URx + prev.URx - own.LLx, URy: t.URy + prev.URy - own.LLy,
			})
			continue
		}

		// Where neither of the offset flags is set, whether the offset is
		// scaled depends on the rasterizer, so both placements are covered.
		dx, dy := cu.Trfm[4], cu.Trfm[5]
		res.Extend(child.Transform(lin.Mul(matrix.Translate(dx, dy))))
		scaled := lin.Apply(vec.Vec2{X: dx, Y: dy})
		res.Extend(child.Transform(lin.Mul(matrix.Translate(scaled.X, scaled.Y))))
	}
	return res, nil
}

// pointRect returns a rectangle which contains the point (x, y) and which is
// not the zero rectangle, which [georect.Rect.Extend] treats as empty.
func pointRect(x, y float64) georect.Rect {
	r := georect.Rect{LLx: x, LLy: y, URx: x, URy: y}
	if r.IsZero() {
		r.URx = math.SmallestNonzeroFloat64
	}
	return r
}

// type3Ink returns the ink bounds of the glyphs of a Type 3 font.  The
// bounds of a glyph contain the box of its d1 operator and the ink of its
// glyph procedure, mapped to text space.  The box is not trusted to confine
// the ink.  The work for analysing a glyph procedure is proportional to its
// decoded length and is charged to the work budget.
func type3Ink(fi *dict.FontInfoType3) inkFunc {
	return func(c cid.CID, charge func(int64) error) (georect.Rect, error) {
		if c == 0 || c > 256 {
			return georect.Rect{}, nil
		}
		name, ok := fi.Encoding(byte(c - 1))
		if !ok {
			return georect.Rect{}, nil
		}
		cp := fi.CharProcs[pdf.Name(name)]
		if cp == nil {
			return georect.Rect{}, nil
		}
		if err := chargeStream(cp.Content, charge); err != nil {
			return georect.Rect{}, err
		}
		res := cp.Resources
		if res == nil {
			res = fi.Resources
		}
		b := type3.GlyphPaintBounds(cp.Content, res)
		if !b.IsFinite() {
			return infiniteRect, nil
		}
		if b.IsZero() {
			return b, nil
		}
		return b.Transform(fi.FontMatrix), nil
	}
}

// chargeStream charges the decoded length of a content stream to the work
// budget.  The stream is read in blocks, so that a stream which expands
// beyond the budget is not read to the end.
func chargeStream(s content.Stream, charge func(int64) error) error {
	rc, err := s.RawBytes()
	if err != nil {
		// the glyph analysis meets the same error and gives no bound
		return nil
	}
	defer rc.Close()
	buf := make([]byte, 32*1024)
	for {
		n, err := rc.Read(buf)
		if n > 0 {
			if cerr := charge(int64(n)); cerr != nil {
				return cerr
			}
		}
		if err != nil {
			return nil
		}
	}
}
