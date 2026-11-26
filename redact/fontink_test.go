// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package redact

import (
	"errors"
	"io"
	"testing"

	"seehuhn.de/go/geom/matrix"
	georect "seehuhn.de/go/geom/rect"
	"seehuhn.de/go/geom/vec"
	"seehuhn.de/go/postscript/cid"
	"seehuhn.de/go/postscript/funit"
	"seehuhn.de/go/sfnt"
	"seehuhn.de/go/sfnt/glyf"
	sglyph "seehuhn.de/go/sfnt/glyph"

	"seehuhn.de/go/pdf/font/dict"
	"seehuhn.de/go/pdf/font/glyphdata"
	"seehuhn.de/go/pdf/font/glyphdata/cffglyphs"
	"seehuhn.de/go/pdf/font/glyphdata/sfntglyphs"
	"seehuhn.de/go/pdf/internal/debug/makefont"
)

// glyfSimple returns a simple glyph with the given points, and with the
// header box hdr.
func glyfSimple(hdr funit.Rect16, pts ...glyf.Point) *glyf.Glyph {
	u := &glyf.SimpleUnpacked{Contours: []glyf.Contour{pts}}
	g := u.AsGlyph()
	g.Rect16 = hdr
	return &g
}

// glyfComposite returns a composite glyph with a zero header box.
func glyfComposite(comps ...glyf.ComponentUnpacked) *glyf.Glyph {
	var c glyf.CompositeGlyph
	for _, cu := range comps {
		c.Components = append(c.Components, cu.Pack())
	}
	return &glyf.Glyph{Data: c}
}

// covers reports whether b contains the point p.
func covers(b georect.Rect, p vec.Vec2) bool {
	return b.LLx <= p.X && p.X <= b.URx && b.LLy <= p.Y && p.Y <= b.URy
}

// simplePoints returns the points of a simple glyph.
func simplePoints(t *testing.T, g *glyf.Glyph) []vec.Vec2 {
	t.Helper()
	u, err := g.Data.(glyf.SimpleGlyph).Unpack()
	if err != nil {
		t.Fatal(err)
	}
	var res []vec.Vec2
	for _, cc := range u.Contours {
		for _, p := range cc {
			res = append(res, vec.Vec2{X: float64(p.X), Y: float64(p.Y)})
		}
	}
	return res
}

func noCharge(int64) error { return nil }

func TestGlyfInkPoints(t *testing.T) {
	// the points extend beyond the header box
	small := funit.Rect16{LLx: 400, LLy: 0, URx: 600, URy: 500}
	tri := glyfSimple(small,
		glyf.Point{X: 100, Y: -50, OnCurve: true},
		glyf.Point{X: 900, Y: -50, OnCurve: true},
		glyf.Point{X: 500, Y: 1200})
	// the header box extends beyond the points
	big := funit.Rect16{LLx: -300, LLy: -400, URx: 1300, URy: 1500}
	dot := glyfSimple(big, glyf.Point{X: 0, Y: 0, OnCurve: true}, glyf.Point{X: 10, Y: 10, OnCurve: true})

	gi := &glyfInkBoxes{glyphs: glyf.Glyphs{nil, tri, dot}, boxes: map[sglyph.ID]georect.Rect{}}
	for gid, g := range map[sglyph.ID]*glyf.Glyph{1: tri, 2: dot} {
		b, err := gi.box(gid, noCharge)
		if err != nil {
			t.Fatal(err)
		}
		hdr := g.Rect16
		pts := append(simplePoints(t, g),
			vec.Vec2{X: float64(hdr.LLx), Y: float64(hdr.LLy)},
			vec.Vec2{X: float64(hdr.URx), Y: float64(hdr.URy)})
		for _, p := range pts {
			if !covers(b, p) {
				t.Errorf("glyph %d: bounds %v miss %v", gid, b, p)
			}
		}
	}

	// a blank glyph paints nothing
	if b, err := gi.box(0, noCharge); err != nil || !b.IsZero() {
		t.Errorf("blank glyph: got %v, %v", b, err)
	}
}

func TestGlyfInkComposite(t *testing.T) {
	small := funit.Rect16{LLx: 400, LLy: 0, URx: 600, URy: 500}
	tri := glyfSimple(small,
		glyf.Point{X: 100, Y: -50, OnCurve: true},
		glyf.Point{X: 900, Y: -50, OnCurve: true},
		glyf.Point{X: 500, Y: 1200})
	base := glyfSimple(funit.Rect16{},
		glyf.Point{X: -200, Y: 0, OnCurve: true},
		glyf.Point{X: 0, Y: 700, OnCurve: true})
	triPts, basePts := simplePoints(t, tri), simplePoints(t, base)

	type placed struct {
		child []vec.Vec2
		m     matrix.Matrix
	}
	type testCase struct {
		name  string
		comps []glyf.ComponentUnpacked
		// want lists the placements of components which the bounds must cover
		want []placed
	}
	cases := []testCase{
		{
			name:  "offset",
			comps: []glyf.ComponentUnpacked{{Child: 1, Trfm: matrix.Matrix{1, 0, 0, 1, 500, 300}}},
			want:  []placed{{triPts, matrix.Matrix{1, 0, 0, 1, 500, 300}}},
		},
		{
			name: "scaled offset",
			comps: []glyf.ComponentUnpacked{{
				Child: 1, Trfm: matrix.Matrix{0, 1.5, -1.5, 0, 100, 200}, ScaledComponentOffset: true,
			}},
			want: []placed{{triPts, matrix.Matrix{0, 1.5, -1.5, 0, -300, 150}}},
		},
		{
			// whether the offset is scaled is up to the rasterizer
			name:  "unspecified offset",
			comps: []glyf.ComponentUnpacked{{Child: 1, Trfm: matrix.Matrix{1.5, 0, 0, 1.5, 100, -100}}},
			want: []placed{
				{triPts, matrix.Matrix{1.5, 0, 0, 1.5, 100, -100}},
				{triPts, matrix.Matrix{1.5, 0, 0, 1.5, 150, -150}},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gi := &glyfInkBoxes{
				glyphs: glyf.Glyphs{nil, tri, base, glyfComposite(tc.comps...)},
				boxes:  map[sglyph.ID]georect.Rect{},
			}
			b, err := gi.box(3, noCharge)
			if err != nil {
				t.Fatal(err)
			}
			for _, pl := range tc.want {
				for _, p := range pl.child {
					if q := pl.m.Apply(p); !covers(b, q) {
						t.Errorf("bounds %v miss %v", b, q)
					}
				}
			}
		})
	}

	t.Run("point matching", func(t *testing.T) {
		// the second component is placed so that one of its points meets
		// one of the points of the first component
		comps := []glyf.ComponentUnpacked{
			{Child: 2, Trfm: matrix.Matrix{1, 0, 0, 1, 1000, 0}},
			{Child: 1, Trfm: matrix.Matrix{0.5, 0, 0, 0.5, 0, 0}, AlignPoints: true, OurPoint: 1, TheirPoint: 2},
		}
		gi := &glyfInkBoxes{
			glyphs: glyf.Glyphs{nil, tri, base, glyfComposite(comps...)},
			boxes:  map[sglyph.ID]georect.Rect{},
		}
		b, err := gi.box(3, noCharge)
		if err != nil {
			t.Fatal(err)
		}
		for _, our := range basePts {
			our = our.Add(vec.Vec2{X: 1000})
			for _, their := range triPts {
				d := our.Sub(their.Mul(0.5))
				for _, p := range triPts {
					if q := p.Mul(0.5).Add(d); !covers(b, q) {
						t.Fatalf("bounds %v miss %v", b, q)
					}
				}
			}
		}
	})

	t.Run("cycle", func(t *testing.T) {
		gi := &glyfInkBoxes{
			glyphs: glyf.Glyphs{nil, tri, glyfComposite(
				glyf.ComponentUnpacked{Child: 1, Trfm: matrix.Identity},
				glyf.ComponentUnpacked{Child: 2, Trfm: matrix.Identity})},
			boxes: map[sglyph.ID]georect.Rect{},
		}
		b, err := gi.box(2, noCharge)
		if err != nil {
			t.Fatal(err)
		}
		if b.IsFinite() {
			t.Errorf("glyph which contains itself has bounds %v", b)
		}
	})
}

func TestGlyfInkBudget(t *testing.T) {
	tri := glyfSimple(funit.Rect16{},
		glyf.Point{X: 100, Y: -50, OnCurve: true},
		glyf.Point{X: 900, Y: -50, OnCurve: true},
		glyf.Point{X: 500, Y: 1200})
	comp := glyfComposite(glyf.ComponentUnpacked{Child: 1, Trfm: matrix.Identity})
	errBudget := errors.New("budget used up")
	var charged int64
	gi := &glyfInkBoxes{glyphs: glyf.Glyphs{nil, tri, comp}, boxes: map[sglyph.ID]georect.Rect{}}
	_, err := gi.box(2, func(n int64) error {
		charged += n
		return errBudget
	})
	if !errors.Is(err, errBudget) {
		t.Errorf("got error %v", err)
	}
	if charged <= 0 {
		t.Error("no work was charged")
	}
}

func TestSfntInkGlyf(t *testing.T) {
	sf := makefont.TrueType()
	o := sf.Outlines.(*glyf.Outlines)
	o.Glyphs[1] = glyfSimple(funit.Rect16{LLx: 1, LLy: 1, URx: 2, URy: 2},
		glyf.Point{X: -100, Y: -400, OnCurve: true},
		glyf.Point{X: 1500, Y: 2000, OnCurve: true})
	ink := sfntInk(sf, func(c cid.CID) sglyph.ID { return sglyph.ID(c) })
	got, err := ink(1, noCharge)
	if err != nil {
		t.Fatal(err)
	}
	upem := float64(sf.UnitsPerEm)
	for _, p := range []vec.Vec2{{X: -100 / upem, Y: -400 / upem}, {X: 1500 / upem, Y: 2000 / upem}} {
		if !covers(got, p) {
			t.Errorf("bounds %v miss %v", got, p)
		}
	}
}

// TestInkGlyphSelection checks that the ink of a CID is the bounding box of
// the glyph which a CID-keyed font selects for it, and that a CID beyond the
// glyphs of the font selects glyph 0.
func TestInkGlyphSelection(t *testing.T) {
	tt, ot, otCID := makefont.TrueType(), makefont.OpenType(), makefont.OpenTypeCID()
	type sel struct {
		c   cid.CID
		gid sglyph.ID
	}
	const beyond = 60000

	// the charset gives glyph g the CID g+100
	var byCharset []sel
	toCID := otCID.AsCFF().GIDToCID
	for gid := range toCID {
		if gid == 0 {
			continue
		}
		toCID[gid] = cid.CID(gid + 100)
		if len(byCharset) < 5 && !otCID.GlyphBBox(sglyph.ID(gid)).IsZero() {
			byCharset = append(byCharset, sel{toCID[gid], sglyph.ID(gid)})
		}
	}
	if len(byCharset) == 0 {
		t.Fatal("test font has no glyphs with ink")
	}
	byCharset = append(byCharset, sel{beyond, 0}, sel{byCharset[0].c - 100, 0})
	identity := []sel{{1, 1}, {2, 2}, {5, 5}, {beyond, 0}}

	cases := []struct {
		name string
		info any
		src  *sfnt.Font
		sels []sel
	}{
		{"CFF", &dict.FontInfoCID{FontFile: cffglyphs.ToStream(otCID.AsCFF(), glyphdata.CFF)}, otCID, byCharset},
		{"OpenType CFF", &dict.FontInfoCID{FontFile: sfntglyphs.ToStream(otCID, glyphdata.OpenTypeCFF)}, otCID, byCharset},
		{"OpenType CFF, not CID-keyed", &dict.FontInfoCID{FontFile: sfntglyphs.ToStream(ot, glyphdata.OpenTypeCFF)}, ot, identity},
		{"TrueType", &dict.FontInfoGlyfEmbedded{FontFile: sfntglyphs.ToStream(tt, glyphdata.TrueType)}, tt, identity},
		{"TrueType with CIDToGID", &dict.FontInfoGlyfEmbedded{
			FontFile: sfntglyphs.ToStream(tt, glyphdata.TrueType),
			CIDToGID: []sglyph.ID{0, 5, 3, 1},
		}, tt, []sel{{1, 5}, {2, 3}, {3, 1}, {4, 0}, {beyond, 0}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ink := newInkFunc(c.info)
			if ink == nil {
				t.Fatal("no glyph bounds")
			}
			upem := float64(c.src.UnitsPerEm)
			var charged int64
			for _, s := range c.sels {
				got, err := ink(s.c, func(n int64) error {
					charged += n
					return nil
				})
				if err != nil {
					t.Fatal(err)
				}
				want := c.src.GlyphBBox(s.gid)
				want.Scale(1 / upem)
				if !got.Covers(want) || got.Dx() > want.Dx()+2e-3 || got.Dy() > want.Dy()+2e-3 {
					t.Errorf("CID %d: got %v, want the box %v of glyph %d", s.c, got, want, s.gid)
				}
			}
			if charged <= 0 {
				t.Error("glyph analysis was not charged to the budget")
			}
		})
	}
}

// TestInkBadProgram checks that the glyphs of a CID-keyed font whose
// embedded program cannot be read have no bound.
func TestInkBadProgram(t *testing.T) {
	garbage := func(typ glyphdata.Type) *glyphdata.Stream {
		return &glyphdata.Stream{Type: typ, WriteTo: func(w io.Writer, _ *glyphdata.Lengths) error {
			_, err := w.Write([]byte("not a font program"))
			return err
		}}
	}
	infos := map[string]any{
		"CFF":          &dict.FontInfoCID{FontFile: garbage(glyphdata.CFF)},
		"OpenType CFF": &dict.FontInfoCID{FontFile: garbage(glyphdata.OpenTypeCFF)},
		"TrueType":     &dict.FontInfoGlyfEmbedded{FontFile: garbage(glyphdata.TrueType)},
	}
	for name, info := range infos {
		ink := newInkFunc(info)
		if ink == nil {
			t.Errorf("%s: no glyph bounds", name)
			continue
		}
		got, err := ink(1, noCharge)
		if err != nil {
			t.Fatal(err)
		}
		if got.IsFinite() {
			t.Errorf("%s: glyph of a broken program has the bound %v", name, got)
		}
	}
}
