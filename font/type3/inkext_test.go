// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package type3_test

import (
	"math"
	"testing"

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/geom/rect"
	"seehuhn.de/go/geom/vec"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/type3"
	"seehuhn.de/go/pdf/function"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/content/builder"
	"seehuhn.de/go/pdf/graphics/extgstate"
	"seehuhn.de/go/pdf/graphics/form"
	"seehuhn.de/go/pdf/graphics/image"
	"seehuhn.de/go/pdf/graphics/shading"
	"seehuhn.de/go/pdf/internal/squarefont"
)

// glyphMatrix is the font matrix of the test fonts, which makes 1000 glyph
// space units one em
var glyphMatrix = matrix.Matrix{0.001, 0, 0, 0.001, 0, 0}

// inGlyphSpace returns a glyph extent from a font with [glyphMatrix], given
// in text space, in glyph space units.
func inGlyphSpace(ext rect.Rect) rect.Rect {
	return rect.Rect{
		LLx: ext.LLx * 1000, LLy: ext.LLy * 1000,
		URx: ext.URx * 1000, URy: ext.URy * 1000,
	}
}

// glyphExtent returns the extent of a d0 glyph drawn by draw, in glyph
// space units of a font with [glyphMatrix].  The glyph's resources are given
// to the font unless inherit is set, in which case they come from the page.
func glyphExtent(t *testing.T, inherit bool, draw func(b *builder.Builder)) rect.Rect {
	t.Helper()
	b := builder.New(content.Glyph, nil, pdf.V2_0)
	b.Type3ColoredGlyph(1000, 0)
	draw(b)
	stream, err := b.Harvest()
	if err != nil {
		t.Fatal(err)
	}
	F := &type3.Font{
		Glyphs:     []*type3.Glyph{{}, {Name: "A", Content: stream}},
		FontMatrix: glyphMatrix,
	}
	if !inherit {
		F.Resources = b.Resources
	}
	inst, err := F.New()
	if err != nil {
		t.Fatal(err)
	}
	return inGlyphSpace(inst.GetGeometry().GlyphExtents[1])
}

func isInfinite(r rect.Rect) bool {
	return math.IsInf(r.LLx, -1) && math.IsInf(r.LLy, -1) &&
		math.IsInf(r.URx, 1) && math.IsInf(r.URy, 1)
}

// tolerance for comparisons of computed coordinates
const eps = 1e-9

// A checker tests the extent of a glyph.
type checker func(t *testing.T, got rect.Rect)

// grown returns r grown by a tenth of its larger side, the slack allowed for
// ink whose extent is not fixed by the contract.
func grown(r rect.Rect) rect.Rect {
	return r.Grow(0.1 * max(r.Dx(), r.Dy()))
}

// within returns a checker for extents which contain ink and lie inside
// bound.  Infinite and NaN extents fail.
func within(ink, bound rect.Rect) checker {
	return func(t *testing.T, got rect.Rect) {
		t.Helper()
		// a comparison with NaN is false, so NaN fails
		ok := got.IsFinite() &&
			got.LLx <= ink.LLx+eps && got.LLy <= ink.LLy+eps &&
			ink.URx-eps <= got.URx && ink.URy-eps <= got.URy &&
			bound.LLx-eps <= got.LLx && bound.LLy-eps <= got.LLy &&
			got.URx <= bound.URx+eps && got.URy <= bound.URy+eps
		if !ok {
			t.Errorf("extent %v, want between %v and %v", got, ink, bound)
		}
	}
}

// tight returns a checker for an extent which covers ink and is not much
// larger.
func tight(ink rect.Rect) checker {
	return within(ink, grown(ink))
}

// strokeWithin returns a checker for stroked ink.  The bound allows twice the
// reach of the stroke, where hw is half the line width in glyph space and k
// the largest factor by which joins and caps reach beyond hw.
func strokeWithin(ink rect.Rect, hw, k float64) checker {
	return within(ink, ink.Grow(2*hw*k))
}

// infinite is a checker for ink without bound.
func infinite(t *testing.T, got rect.Rect) {
	t.Helper()
	if !isInfinite(got) {
		t.Errorf("extent %v, want infinite", got)
	}
}

// empty is a checker for a glyph without ink.
func empty(t *testing.T, got rect.Rect) {
	t.Helper()
	if !got.IsZero() {
		t.Errorf("extent %v, want the zero rectangle", got)
	}
}

// textInk returns the ink of the glyph "A" of F, shown at the given size with
// the text position at (x, y).
func textInk(F font.Layouter, size, x, y float64) rect.Rect {
	gid := F.Layout(nil, size, "A").Seq[0].GID
	ext := F.GetGeometry().GlyphExtents[gid]
	return rect.Rect{
		LLx: x + ext.LLx*size, LLy: y + ext.LLy*size,
		URx: x + ext.URx*size, URy: y + ext.URy*size,
	}
}

// bboxOf returns the bounding box of the points.
func bboxOf(pts ...vec.Vec2) rect.Rect {
	r := rect.Rect{LLx: pts[0].X, LLy: pts[0].Y, URx: pts[0].X, URy: pts[0].Y}
	for _, p := range pts[1:] {
		r.Add(p.X, p.Y)
	}
	return r
}

// segmentInk returns the bounding box of the line segment from a to b,
// stroked with half line width hw and caps which extend by capExt.
func segmentInk(a, b vec.Vec2, hw, capExt float64) rect.Rect {
	d := b.Sub(a)
	d = d.Mul(1 / math.Hypot(d.X, d.Y))
	n := vec.Vec2{X: -d.Y, Y: d.X}
	a = a.Sub(d.Mul(capExt))
	b = b.Add(d.Mul(capExt))
	return bboxOf(
		a.Add(n.Mul(hw)), a.Sub(n.Mul(hw)),
		b.Add(n.Mul(hw)), b.Sub(n.Mul(hw)),
	)
}

func axialShading() *shading.Type2 {
	return &shading.Type2{
		ColorSpace: color.SpaceDeviceGray,
		P0:         vec.Vec2{X: 0, Y: 0},
		P1:         vec.Vec2{X: 100, Y: 0},
		F: &function.Type2{
			XMin: 0, XMax: 1,
			C0: []float64{0}, C1: []float64{1},
			N: 1,
		},
		TMax:      1,
		SingleUse: true,
	}
}

// setStroke sets all stroke parameters, so that the glyph inherits none of
// them.
func setStroke(b *builder.Builder, width float64, lineCap graphics.LineCapStyle) {
	b.SetLineWidth(width)
	b.SetLineCap(lineCap)
	b.SetLineJoin(graphics.LineJoinMiter)
	b.SetMiterLimit(10)
	b.SetLineDash(nil, 0)
}

// setText sets the font and all text state parameters which place and paint
// glyphs, so that the glyph inherits none of them.
func setText(b *builder.Builder, F font.Instance, size float64) {
	b.TextSetFont(F, size)
	b.TextSetCharacterSpacing(0)
	b.TextSetWordSpacing(0)
	b.TextSetHorizontalScaling(1)
	b.TextSetRise(0)
	b.TextSetRenderingMode(graphics.TextRenderingModeFill)
}

// TestGlyphExtentInk checks that the extent of d0 glyphs which paint bounded
// ink contains the ink and is not much larger.
func TestGlyphExtentInk(t *testing.T) {
	cases := []struct {
		name  string
		draw  func(b *builder.Builder)
		check checker
	}{
		{"fill under cm", func(b *builder.Builder) {
			b.Transform(matrix.Translate(10, 20))
			b.Rectangle(0, 0, 5, 5)
			b.Fill()
		}, tight(rect.Rect{LLx: 10, LLy: 20, URx: 15, URy: 25})},
		{"curve, not control points", func(b *builder.Builder) {
			b.MoveTo(0, 0)
			b.CurveTo(0, 10, 10, 10, 10, 0)
			b.Fill()
		}, tight(rect.Rect{URx: 10, URy: 7.5})},
		{"move after move", func(b *builder.Builder) {
			b.MoveTo(100, 100)
			b.MoveTo(0, 0)
			b.LineTo(10, 20)
			b.Fill()
		}, tight(rect.Rect{URx: 10, URy: 20})},
		{"trailing move", func(b *builder.Builder) {
			b.MoveTo(0, 0)
			b.LineTo(10, 20)
			b.MoveTo(50, 60)
			b.Fill()
		}, tight(rect.Rect{URx: 10, URy: 20})},
		{"move and close", func(b *builder.Builder) {
			b.MoveTo(0, 0)
			b.LineTo(10, 20)
			b.MoveTo(-5, -5)
			b.ClosePath()
			b.Fill()
		}, tight(rect.Rect{LLx: -5, LLy: -5, URx: 10, URy: 20})},
		{"move only", func(b *builder.Builder) {
			b.MoveTo(100, 100)
			b.Fill()
		}, empty},
		{"stroke width", func(b *builder.Builder) {
			setStroke(b, 4, graphics.LineCapRound)
			b.MoveTo(0, 0)
			b.LineTo(10, 0)
			b.Stroke()
		}, strokeWithin(rect.Rect{LLx: -2, LLy: -2, URx: 12, URy: 2}, 2, math.Sqrt2)},
		{"stroke under scaling", func(b *builder.Builder) {
			b.Transform(matrix.Scale(2, 1))
			setStroke(b, 2, graphics.LineCapRound)
			b.MoveTo(0, 0)
			b.LineTo(0, 10)
			b.Stroke()
		}, strokeWithin(rect.Rect{LLx: -2, LLy: -1, URx: 2, URy: 11}, 2, math.Sqrt2)},
		{"stroke under rotation", func(b *builder.Builder) {
			b.Transform(matrix.RotateDeg(45))
			setStroke(b, 2, graphics.LineCapButt)
			b.MoveTo(0, 0)
			b.LineTo(10, 0)
			b.Stroke()
		}, strokeWithin(rect.Rect{
			LLx: -math.Sqrt2 / 2, LLy: -math.Sqrt2 / 2,
			URx: 5*math.Sqrt2 + math.Sqrt2/2, URy: 5*math.Sqrt2 + math.Sqrt2/2,
		}, 1, math.Sqrt2)},
		{"clipping path", func(b *builder.Builder) {
			b.Rectangle(0, 0, 10, 10)
			b.ClipNonZero()
			b.EndPath()
			b.Rectangle(5, 5, 100, 100)
			b.Fill()
		}, tight(rect.Rect{LLx: 5, LLy: 5, URx: 10, URy: 10})},
		{"clipped shading", func(b *builder.Builder) {
			b.Rectangle(0, 0, 10, 10)
			b.ClipNonZero()
			b.EndPath()
			b.DrawShading(axialShading())
		}, tight(rect.Rect{URx: 10, URy: 10})},
		{"shading bbox", func(b *builder.Builder) {
			sh := axialShading()
			sh.BBox = &pdf.Rectangle{LLx: 1, LLy: 2, URx: 3, URy: 4}
			b.DrawShading(sh)
		}, tight(rect.Rect{LLx: 1, LLy: 2, URx: 3, URy: 4})},
		{"radial shading", func(b *builder.Builder) {
			b.DrawShading(&shading.Type3{
				ColorSpace: color.SpaceDeviceGray,
				Center1:    vec.Vec2{X: 0, Y: 0}, R1: 1,
				Center2: vec.Vec2{X: 10, Y: 0}, R2: 2,
				F: &function.Type2{
					XMin: 0, XMax: 1,
					C0: []float64{0}, C1: []float64{1},
					N: 1,
				},
				TMax:        1,
				ExtendStart: true,
				SingleUse:   true,
			})
		}, tight(rect.Rect{LLx: -10, LLy: -2, URx: 12, URy: 2})},
		{"image", func(b *builder.Builder) {
			b.Transform(matrix.Matrix{20, 0, 0, 10, 5, 5})
			b.DrawXObject(&image.Mask{Width: 1, Height: 1})
		}, tight(rect.Rect{LLx: 5, LLy: 5, URx: 25, URy: 15})},
		{"form", func(b *builder.Builder) {
			b.Transform(matrix.Scale(2, 2))
			b.DrawXObject(&form.Form{
				Content: &content.Operators{},
				Res:     &content.Resources{},
				BBox:    pdf.Rectangle{URx: 10, URy: 10},
				Matrix:  matrix.Translate(5, 0),
			})
		}, tight(rect.Rect{LLx: 10, URx: 30, URy: 20})},
		{"invisible text", func(b *builder.Builder) {
			b.TextBegin()
			b.TextSetFont(squarefont.All[0].MakeFont(), 10)
			b.TextSetRenderingMode(graphics.TextRenderingModeInvisible)
			b.TextFirstLine(0, 0)
			b.TextShow("A")
			b.TextEnd()
		}, empty},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			c.check(t, glyphExtent(t, false, c.draw))
		})
	}
}

// TestGlyphExtentUnbounded checks that d0 glyphs whose ink has no bound get
// an infinite extent.
func TestGlyphExtentUnbounded(t *testing.T) {
	cases := []struct {
		name    string
		inherit bool
		draw    func(b *builder.Builder)
	}{
		{"unclipped shading", false, func(b *builder.Builder) {
			b.DrawShading(axialShading())
		}},
		{"clip restored by Q", false, func(b *builder.Builder) {
			b.PushGraphicsState()
			b.Rectangle(0, 0, 10, 10)
			b.ClipNonZero()
			b.EndPath()
			b.PopGraphicsState()
			b.DrawShading(axialShading())
		}},
		{"resources from the page", true, func(b *builder.Builder) {
			b.DrawXObject(&image.Mask{Width: 1, Height: 1})
		}},
		{"text in an unbounded font", false, func(b *builder.Builder) {
			inner := builder.New(content.Glyph, nil, pdf.V2_0)
			inner.Type3ColoredGlyph(1000, 0)
			inner.DrawShading(axialShading())
			stream, err := inner.Harvest()
			if err != nil {
				t.Fatal(err)
			}
			F := &type3.Font{
				Glyphs:     []*type3.Glyph{{}, {Name: "A", Content: stream}},
				Resources:  inner.Resources,
				FontMatrix: matrix.Matrix{0.001, 0, 0, 0.001, 0, 0},
			}
			inst, err := F.New()
			if err != nil {
				t.Fatal(err)
			}
			b.TextBegin()
			setText(b, inst, 10)
			b.TextFirstLine(0, 0)
			b.TextShow("A")
			b.TextEnd()
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := glyphExtent(t, c.inherit, c.draw); !isInfinite(got) {
				t.Errorf("extent %v, want infinite", got)
			}
		})
	}
}

// TestGlyphExtentText checks that the extent of a d0 glyph which shows text
// contains the text's glyphs.
func TestGlyphExtentText(t *testing.T) {
	inner := squarefont.All[0].MakeFont()
	const size = 10
	got := glyphExtent(t, false, func(b *builder.Builder) {
		b.TextBegin()
		setText(b, inner, size)
		b.TextFirstLine(100, 200)
		b.TextShow("A")
		b.TextEnd()
	})
	tight(textInk(inner, size, 100, 200))(t, got)
}

// TestGlyphExtentHeights checks that a glyph without bound does not give
// the font an infinite cap height.
func TestGlyphExtentHeights(t *testing.T) {
	b := builder.New(content.Glyph, nil, pdf.V2_0)
	b.Type3ColoredGlyph(1000, 0)
	b.DrawShading(axialShading())
	stream, err := b.Harvest()
	if err != nil {
		t.Fatal(err)
	}
	F := &type3.Font{
		Glyphs:     []*type3.Glyph{{}, {Name: "H", Content: stream}},
		Resources:  b.Resources,
		FontMatrix: matrix.Matrix{0.001, 0, 0, 0.001, 0, 0},
	}
	inst, err := F.New()
	if err != nil {
		t.Fatal(err)
	}
	geom := inst.GetGeometry()
	if !isInfinite(geom.GlyphExtents[1]) {
		t.Errorf("extent %v, want infinite", geom.GlyphExtents[1])
	}
	if h := geom.CapHeight; math.IsInf(h, 0) || !(h > 0) {
		t.Errorf("cap height %g, want a positive finite value", h)
	}
}

// fakeXObject is an XObject of a subtype the font does not know.
type fakeXObject struct{}

func (fakeXObject) Subtype() pdf.Name                          { return "Unknown" }
func (fakeXObject) ResourceName() pdf.Name                     { return "" }
func (fakeXObject) Embed(*pdf.EmbedHelper) (pdf.Native, error) { return nil, nil }

// coons evaluates the Coons patch with the given boundary control points,
// in the order of a Type 6 shading, at (u, v).
func coons(cp [12]vec.Vec2, u, v float64) vec.Vec2 {
	bez := func(p0, p1, p2, p3 vec.Vec2, t float64) vec.Vec2 {
		s := 1 - t
		return p0.Mul(s * s * s).Add(p1.Mul(3 * s * s * t)).Add(p2.Mul(3 * s * t * t)).Add(p3.Mul(t * t * t))
	}
	// pij with i along u and j along v
	p00, p01, p02, p03 := cp[0], cp[1], cp[2], cp[3]
	p13, p23, p33, p32 := cp[4], cp[5], cp[6], cp[7]
	p31, p30, p20, p10 := cp[8], cp[9], cp[10], cp[11]
	d1 := bez(p00, p01, p02, p03, v) // u = 0
	d2 := bez(p30, p31, p32, p33, v) // u = 1
	c1 := bez(p00, p10, p20, p30, u) // v = 0
	c2 := bez(p03, p13, p23, p33, u) // v = 1
	s := c1.Mul(1 - v).Add(c2.Mul(v)).Add(d1.Mul(1 - u)).Add(d2.Mul(u))
	corners := p00.Mul((1 - u) * (1 - v)).Add(p03.Mul((1 - u) * v)).
		Add(p30.Mul(u * (1 - v))).Add(p33.Mul(u * v))
	return s.Sub(corners)
}

// tensor evaluates the bicubic patch with the given control points, in the
// order of a Type 7 shading, at (u, v).
func tensor(cp [16]vec.Vec2, u, v float64) vec.Vec2 {
	// pij with i along u and j along v
	p := [4][4]vec.Vec2{
		{cp[0], cp[1], cp[2], cp[3]},
		{cp[11], cp[12], cp[13], cp[4]},
		{cp[10], cp[15], cp[14], cp[5]},
		{cp[9], cp[8], cp[7], cp[6]},
	}
	bern := func(t float64) [4]float64 {
		s := 1 - t
		return [4]float64{s * s * s, 3 * s * s * t, 3 * s * t * t, t * t * t}
	}
	bu, bv := bern(u), bern(v)
	var res vec.Vec2
	for i := range p {
		for j := range p[i] {
			res = res.Add(p[i][j].Mul(bu[i] * bv[j]))
		}
	}
	return res
}

// sampleInk returns the bounding box of the points f(u, v) for u and v in
// the unit interval.
func sampleInk(f func(u, v float64) vec.Vec2) rect.Rect {
	const n = 50
	var pts []vec.Vec2
	for i := 0; i <= n; i++ {
		for j := 0; j <= n; j++ {
			pts = append(pts, f(float64(i)/n, float64(j)/n))
		}
	}
	return bboxOf(pts...)
}

// TestGlyphExtentMeshes checks the extent of glyphs which paint shadings
// with a finite extent of their own.
func TestGlyphExtentMeshes(t *testing.T) {
	tensorPoints := [16]vec.Vec2{
		{X: 0, Y: 0}, {X: 0, Y: 1}, {X: 0, Y: 2}, {X: 0, Y: 3},
		{X: 1, Y: 3}, {X: 2, Y: 3}, {X: 3, Y: 3}, {X: 3, Y: 2},
		{X: 3, Y: 1}, {X: 3, Y: 0}, {X: 2, Y: 0}, {X: 1, Y: 0},
		{X: 1, Y: 1}, {X: 1, Y: 2}, {X: 2, Y: 2}, {X: 9, Y: 1},
	}
	// the patch lies in the hull of its control points
	tensorHull := bboxOf(tensorPoints[:]...)
	tensorInk := sampleInk(func(u, v float64) vec.Vec2 { return tensor(tensorPoints, u, v) })

	cases := []struct {
		name  string
		sh    graphics.Shading
		check checker
	}{
		{"function-based", &shading.Type1{
			ColorSpace: color.SpaceDeviceGray,
			F:          &function.Type4{Domain: []float64{0, 1, 0, 1}, Range: []float64{0, 1}, Program: "pop"},
			Domain:     []float64{0, 2, 0, 1},
			Matrix:     []float64{1, 0, 0, 1, 10, 20},
		}, tight(rect.Rect{LLx: 10, LLy: 20, URx: 12, URy: 21})},
		{"free-form triangles", &shading.Type4{
			ColorSpace:        color.SpaceDeviceGray,
			BitsPerCoordinate: 16, BitsPerComponent: 8, BitsPerFlag: 8,
			Decode: []float64{0, 100, 0, 100, 0, 1},
			Vertices: []shading.Type4Vertex{
				{X: 1, Y: 2, Color: []float64{0}},
				{X: 5, Y: 3, Color: []float64{0}},
				{X: 2, Y: 8, Color: []float64{0}},
			},
		}, tight(rect.Rect{LLx: 1, LLy: 2, URx: 5, URy: 8})},
		{"lattice", &shading.Type5{
			ColorSpace:        color.SpaceDeviceGray,
			BitsPerCoordinate: 16, BitsPerComponent: 8, VerticesPerRow: 2,
			Decode: []float64{0, 100, 0, 100, 0, 1},
			Vertices: []shading.Type5Vertex{
				{X: 0, Y: 0, Color: []float64{0}}, {X: 4, Y: 0, Color: []float64{0}},
				{X: 0, Y: 3, Color: []float64{0}}, {X: 4, Y: 3, Color: []float64{0}},
			},
		}, tight(rect.Rect{URx: 4, URy: 3})},
		{"tensor patch", &shading.Type7{
			ColorSpace:        color.SpaceDeviceGray,
			BitsPerCoordinate: 16, BitsPerComponent: 8, BitsPerFlag: 8,
			Decode: []float64{0, 100, 0, 100, 0, 1},
			Patches: []shading.Type7Patch{{
				ControlPoints: tensorPoints,
				CornerColors:  [][]float64{{0}, {0}, {0}, {0}},
			}},
		}, within(tensorInk, grown(tensorHull))},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			c.check(t, glyphExtent(t, false, func(b *builder.Builder) { b.DrawShading(c.sh) }))
		})
	}
}

// TestGlyphExtentCoons checks that the extent of a Coons patch shading
// covers the patch, which can bulge beyond its boundary control points.
func TestGlyphExtentCoons(t *testing.T) {
	cp := [12]vec.Vec2{
		{X: 0, Y: 0}, {X: -3, Y: 4}, {X: 5, Y: 6}, {X: 0, Y: 10},
		{X: 4, Y: 14}, {X: 7, Y: 3}, {X: 10, Y: 10},
		{X: 14, Y: 7}, {X: 6, Y: 4}, {X: 10, Y: 0},
		{X: 6, Y: -5}, {X: 3, Y: 4},
	}
	sh := &shading.Type6{
		ColorSpace:        color.SpaceDeviceGray,
		BitsPerCoordinate: 16, BitsPerComponent: 8, BitsPerFlag: 8,
		Decode: []float64{-100, 100, -100, 100, 0, 1},
		Patches: []shading.Type6Patch{{
			ControlPoints: cp,
			CornerColors:  [][]float64{{0}, {0}, {0}, {0}},
		}},
	}
	got := glyphExtent(t, false, func(b *builder.Builder) { b.DrawShading(sh) })
	ink := sampleInk(func(u, v float64) vec.Vec2 { return coons(cp, u, v) })
	// the bound is the hull of the boundary points, grown by the amount
	// the patch can bulge beyond it
	bound := bboxOf(cp[:]...)
	bound = bound.Grow(0.1 * max(bound.Dx(), bound.Dy()))
	within(ink, bound)(t, got)
}

// TestGlyphExtentOperators checks the extent of d0 glyphs which use further
// painting operators.
func TestGlyphExtentOperators(t *testing.T) {
	squareFont := func() *squarefont.Sample {
		for _, s := range squarefont.All {
			if s.Label == "Type3-1000" {
				return s
			}
		}
		t.Fatal("no Type 3 sample font")
		return nil
	}()
	kernedFont := squarefont.All[0].MakeFont()
	noOutlines := squareFont.MakeFont()

	cases := []struct {
		name  string
		draw  func(b *builder.Builder)
		check checker
	}{
		{"inline image", func(b *builder.Builder) {
			b.Transform(matrix.Matrix{4, 0, 0, 2, 1, 1})
			b.DrawInlineImageRaw(pdf.Dict{"W": pdf.Integer(1), "H": pdf.Integer(1), "IM": pdf.Boolean(true)}, []byte{0})
		}, tight(rect.Rect{LLx: 1, LLy: 1, URx: 5, URy: 3})},
		{"raw content", func(b *builder.Builder) {
			b.Stream = append(b.Stream, content.Operator{
				Name: content.OpRawContent,
				Args: []pdf.Object{pdf.String("1 2 3 4 re f")},
			})
		}, tight(rect.Rect{LLx: 1, LLy: 2, URx: 4, URy: 6})},
		{"comment", func(b *builder.Builder) {
			b.Stream = append(b.Stream, content.Operator{
				Name: content.OpRawContent,
				Args: []pdf.Object{pdf.String("% 1 2 3 4 re f")},
			})
		}, empty},
		{"empty form", func(b *builder.Builder) {
			b.DrawXObject(&form.Form{Content: &content.Operators{}, Res: &content.Resources{}})
		}, empty},
		{"unknown XObject", func(b *builder.Builder) {
			b.DrawXObject(fakeXObject{})
		}, infinite},
		{"kerned text", func(b *builder.Builder) {
			// the kern moves the text position right by 1000
			b.TextBegin()
			setText(b, kernedFont, 10)
			b.TextFirstLine(0, 0)
			b.TextShowKernedRaw(pdf.Integer(-100000))
			b.TextShow("A")
			b.TextEnd()
		}, tight(textInk(kernedFont, 10, 1000, 0))},
		{"font without outlines", func(b *builder.Builder) {
			b.TextBegin()
			setText(b, noOutlines, 10)
			b.TextFirstLine(0, 0)
			b.TextShow("A")
			b.TextEnd()
		}, tight(textInk(noOutlines, 10, 0, 0))},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			c.check(t, glyphExtent(t, false, c.draw))
		})
	}
}

// TestGlyphExtentStrokedText checks that stroked text reaches out by half
// the line width beyond the glyphs.
func TestGlyphExtentStrokedText(t *testing.T) {
	F := squarefont.All[0].MakeFont()
	got := glyphExtent(t, false, func(b *builder.Builder) {
		setStroke(b, 10, graphics.LineCapRound)
		b.SetLineJoin(graphics.LineJoinRound)
		b.TextBegin()
		setText(b, F, 10)
		b.TextSetRenderingMode(graphics.TextRenderingModeStroke)
		b.TextFirstLine(0, 0)
		b.TextShow("A")
		b.TextEnd()
	})
	ink := textInk(F, 10, 0, 0).Grow(5)
	strokeWithin(ink, 5, 1)(t, got)
}

// TestGlyphExtentPageResources checks glyphs whose resources come from the
// page, so that named resources are unknown.
func TestGlyphExtentPageResources(t *testing.T) {
	line := func(b *builder.Builder) {
		b.MoveTo(0, 0)
		b.LineTo(10, 0)
		b.Stroke()
	}
	F := squarefont.All[0].MakeFont()
	cases := []struct {
		name  string
		draw  func(b *builder.Builder)
		check checker
	}{
		{"stroke", func(b *builder.Builder) {
			setStroke(b, 2, graphics.LineCapButt)
			line(b)
		}, strokeWithin(rect.Rect{LLy: -1, URx: 10, URy: 1}, 1, math.Sqrt2)},
		{"stroke after gs", func(b *builder.Builder) {
			// the unknown ExtGState can set the stroke parameters
			setStroke(b, 2, graphics.LineCapButt)
			b.SetExtGState(&extgstate.ExtGState{Set: graphics.StateLineWidth, LineWidth: 50})
			line(b)
		}, infinite},
		{"text in unknown font", func(b *builder.Builder) {
			b.TextBegin()
			setText(b, F, 10)
			b.TextFirstLine(0, 0)
			b.TextShow("A")
			b.TextEnd()
		}, infinite},
		{"clipped shading", func(b *builder.Builder) {
			b.Rectangle(0, 0, 10, 10)
			b.ClipNonZero()
			b.EndPath()
			b.DrawShading(axialShading())
		}, tight(rect.Rect{URx: 10, URy: 10})},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			c.check(t, glyphExtent(t, true, c.draw))
		})
	}
}

// TestGlyphExtentD1 checks that a d1 glyph keeps the box of its d1 operator,
// and that Width reports the advance width.
func TestGlyphExtentD1(t *testing.T) {
	b := builder.New(content.Glyph, nil, pdf.V2_0)
	b.Type3UncoloredGlyph(700, 0, 10, 20, 600, 800)
	b.Rectangle(-1000, -1000, 5000, 5000)
	b.Fill()
	stream, err := b.Harvest()
	if err != nil {
		t.Fatal(err)
	}
	g := &type3.Glyph{Name: "A", Content: stream}
	F := &type3.Font{Glyphs: []*type3.Glyph{{}, g}, FontMatrix: glyphMatrix}
	inst, err := F.New()
	if err != nil {
		t.Fatal(err)
	}
	box := rect.Rect{LLx: 10, LLy: 20, URx: 600, URy: 800}
	within(box, box)(t, inGlyphSpace(inst.GetGeometry().GlyphExtents[1]))
	if w := g.Width(); w != 700 {
		t.Errorf("width %g, want 700", w)
	}
}

// TestGlyphExtentInherited checks d0 glyphs which leave stroke or text
// parameters to be inherited from where the glyph is shown.  Each case
// leaves one parameter unset, and is followed by a case which sets it.
func TestGlyphExtentInherited(t *testing.T) {
	F := squarefont.All[0].MakeFont()
	const size = 10
	advance := F.Layout(nil, size, "A").Seq[0].Advance

	// a horizontal line of width 2
	line := func(b *builder.Builder) {
		b.MoveTo(0, 0)
		b.LineTo(10, 0)
		b.Stroke()
	}
	lineInk := rect.Rect{LLx: 0, LLy: -1, URx: 10, URy: 1}

	// A corner at (100, 10), drawn with line width 2.  Half the line width
	// is one, the offsets of the segment ends from the path are (nx, ny) in
	// absolute value, and a miter without limit reaches miterTip.
	corner := func(b *builder.Builder) {
		b.MoveTo(0, 0)
		b.LineTo(100, 10)
		b.LineTo(0, 20)
		b.Stroke()
	}
	norm := math.Hypot(100, 10)
	nx, ny := 10/norm, 100/norm
	miterTip := 100 + norm/10
	cornerInk := func(urx float64) rect.Rect {
		return rect.Rect{LLx: -nx, LLy: -ny, URx: urx, URy: 20 + ny}
	}

	// text shown after a line feed by the leading 12
	shown := textInk(F, size, 0, 0)
	shownBelow := textInk(F, size, 0, -12)

	cases := []struct {
		name    string
		inherit bool
		draw    func(b *builder.Builder)
		check   checker
	}{
		{"line width", false, func(b *builder.Builder) {
			b.SetLineCap(graphics.LineCapButt)
			b.SetLineJoin(graphics.LineJoinRound)
			b.SetMiterLimit(10)
			b.SetLineDash(nil, 0)
			line(b)
		}, infinite},
		{"line width set", false, func(b *builder.Builder) {
			b.SetLineWidth(2)
			b.SetLineCap(graphics.LineCapButt)
			b.SetLineJoin(graphics.LineJoinRound)
			b.SetMiterLimit(10)
			b.SetLineDash(nil, 0)
			line(b)
		}, strokeWithin(lineInk, 1, math.Sqrt2)},
		{"line cap", false, func(b *builder.Builder) {
			b.SetLineWidth(2)
			b.SetLineJoin(graphics.LineJoinRound)
			b.SetMiterLimit(10)
			b.SetLineDash(nil, 0)
			line(b)
		}, strokeWithin(rect.Rect{LLx: -1, LLy: -1, URx: 11, URy: 1}, 1, math.Sqrt2)},
		{"line cap set", false, func(b *builder.Builder) {
			b.SetLineWidth(2)
			b.SetLineCap(graphics.LineCapButt)
			b.SetLineJoin(graphics.LineJoinRound)
			b.SetMiterLimit(10)
			b.SetLineDash(nil, 0)
			line(b)
		}, strokeWithin(lineInk, 1, math.Sqrt2)},
		{"miter limit", false, func(b *builder.Builder) {
			b.SetLineWidth(2)
			b.SetLineCap(graphics.LineCapButt)
			b.SetLineJoin(graphics.LineJoinMiter)
			b.SetLineDash(nil, 0)
			corner(b)
		}, strokeWithin(cornerInk(miterTip), norm/10, 1)},
		{"miter limit set", false, func(b *builder.Builder) {
			// the miter exceeds the limit and is bevelled
			b.SetLineWidth(2)
			b.SetLineCap(graphics.LineCapButt)
			b.SetLineJoin(graphics.LineJoinMiter)
			b.SetMiterLimit(2)
			b.SetLineDash(nil, 0)
			corner(b)
		}, strokeWithin(cornerInk(100+nx), 1, 2)},
		{"line join", false, func(b *builder.Builder) {
			// the miter is bevelled, and a round join reaches furthest
			b.SetLineWidth(2)
			b.SetLineCap(graphics.LineCapButt)
			b.SetMiterLimit(10)
			b.SetLineDash(nil, 0)
			corner(b)
		}, strokeWithin(cornerInk(101), 1, 10)},
		{"line join set", false, func(b *builder.Builder) {
			b.SetLineWidth(2)
			b.SetLineCap(graphics.LineCapButt)
			b.SetLineJoin(graphics.LineJoinBevel)
			b.SetMiterLimit(10)
			b.SetLineDash(nil, 0)
			corner(b)
		}, strokeWithin(cornerInk(100+nx), 1, math.Sqrt2)},
		{"font", true, func(b *builder.Builder) {
			// the resources are unknown, so the font is not
			b.TextBegin()
			setText(b, F, size)
			b.TextShow("A")
			b.TextEnd()
		}, infinite},
		{"font set", false, func(b *builder.Builder) {
			b.TextBegin()
			setText(b, F, size)
			b.TextShow("A")
			b.TextEnd()
		}, tight(shown)},
		{"character spacing", false, func(b *builder.Builder) {
			b.TextBegin()
			b.TextSetFont(F, size)
			b.TextSetWordSpacing(0)
			b.TextSetHorizontalScaling(1)
			b.TextSetRise(0)
			b.TextSetRenderingMode(graphics.TextRenderingModeFill)
			b.TextShow("A")
			b.TextEnd()
		}, infinite},
		{"character spacing set", false, func(b *builder.Builder) {
			b.TextBegin()
			setText(b, F, size)
			b.TextSetCharacterSpacing(2)
			b.TextShow("A")
			b.TextEnd()
		}, tight(shown)},
		{"leading", false, func(b *builder.Builder) {
			b.TextBegin()
			setText(b, F, size)
			b.TextNextLine()
			b.TextShow("A")
			b.TextEnd()
		}, infinite},
		{"leading of '", false, func(b *builder.Builder) {
			b.TextBegin()
			setText(b, F, size)
			b.TextShowNextLineRaw(pdf.String("A"))
			b.TextEnd()
		}, infinite},
		{"leading set", false, func(b *builder.Builder) {
			b.TextBegin()
			setText(b, F, size)
			b.TextSetLeading(12)
			b.TextNextLine()
			b.TextShow("A")
			b.TextEnd()
		}, tight(shownBelow)},
		{"leading of ' set", false, func(b *builder.Builder) {
			b.TextBegin()
			setText(b, F, size)
			b.TextSetLeading(12)
			b.TextShowNextLineRaw(pdf.String("A"))
			b.TextEnd()
		}, tight(shownBelow)},
		{"spacing of earlier text", false, func(b *builder.Builder) {
			// the invisible text moves the text position by an amount
			// which depends on the inherited character spacing
			b.TextBegin()
			b.TextSetFont(F, size)
			b.TextSetRenderingMode(graphics.TextRenderingModeInvisible)
			b.TextShow("A")
			setText(b, F, size)
			b.TextShow("A")
			b.TextEnd()
		}, infinite},
		{"spacing of earlier text set", false, func(b *builder.Builder) {
			b.TextBegin()
			setText(b, F, size)
			b.TextSetRenderingMode(graphics.TextRenderingModeInvisible)
			b.TextShow("A")
			setText(b, F, size)
			b.TextShow("A")
			b.TextEnd()
		}, tight(textInk(F, size, advance, 0))},
		{"invisible text", false, func(b *builder.Builder) {
			b.TextBegin()
			b.TextSetFont(F, size)
			b.TextSetRenderingMode(graphics.TextRenderingModeInvisible)
			b.TextShow("A")
			b.TextEnd()
		}, empty},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			c.check(t, glyphExtent(t, c.inherit, c.draw))
		})
	}
}

// TestGlyphExtentInheritedDash checks that an inherited dash pattern is
// bounded by the stroke for every dash pattern, including none.
func TestGlyphExtentInheritedDash(t *testing.T) {
	const hw = 2
	// a thin triangle, whose sharp corner a dash can end at
	tri := []vec.Vec2{{X: 0, Y: 0}, {X: 100, Y: 3}, {X: 0, Y: 6}}
	triangle := func(setDash bool, pattern []float64) func(b *builder.Builder) {
		return func(b *builder.Builder) {
			b.SetLineWidth(2 * hw)
			b.SetLineCap(graphics.LineCapSquare)
			b.SetLineJoin(graphics.LineJoinBevel)
			if setDash {
				b.SetLineDash(pattern, 0)
			}
			b.MoveTo(0, 0)
			b.LineTo(100, 3)
			b.LineTo(0, 6)
			b.ClosePath()
			b.Stroke()
		}
	}

	// the outline of the three sides, with caps which extend by capExt
	outline := func(capExt float64) rect.Rect {
		var r rect.Rect
		for i, a := range tri {
			b := tri[(i+1)%len(tri)]
			if i == 0 {
				r = segmentInk(a, b, hw, capExt)
			} else {
				r.Extend(segmentInk(a, b, hw, capExt))
			}
		}
		return r
	}
	// a dash which ends at a corner reaches furthest with a square cap
	anyDash := outline(hw)
	// the first dash of the pattern 1 1 starts at the first corner
	d := tri[1].Sub(tri[0])
	firstDash := segmentInk(tri[0], d.Mul(1/math.Hypot(d.X, d.Y)), hw, hw)

	cases := []struct {
		name    string
		setDash bool
		pattern []float64
		ink     rect.Rect
	}{
		{"inherited", false, nil, anyDash},
		{"solid", true, nil, outline(0)},
		{"dashed", true, []float64{1, 1}, firstDash},
	}
	bound := anyDash.Grow(2 * hw * math.Sqrt2)
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := glyphExtent(t, false, triangle(c.setDash, c.pattern))
			within(c.ink, bound)(t, got)
		})
	}
}

// TestGlyphExtentInheritedRenderingMode checks that an inherited text
// rendering mode is bounded by filling and stroking the glyphs.
func TestGlyphExtentInheritedRenderingMode(t *testing.T) {
	F := squarefont.All[0].MakeFont()
	text := func(mode graphics.TextRenderingMode, setMode bool) rect.Rect {
		return glyphExtent(t, false, func(b *builder.Builder) {
			setStroke(b, 10, graphics.LineCapRound)
			b.SetLineJoin(graphics.LineJoinRound)
			b.TextBegin()
			b.TextSetFont(F, 10)
			b.TextSetCharacterSpacing(0)
			b.TextSetWordSpacing(0)
			b.TextSetHorizontalScaling(1)
			b.TextSetRise(0)
			if setMode {
				b.TextSetRenderingMode(mode)
			}
			b.TextShow("A")
			b.TextEnd()
		})
	}
	glyph := textInk(F, 10, 0, 0)
	t.Run("inherited", func(t *testing.T) {
		// the stroke has the line width 10
		within(glyph.Grow(5), glyph.Grow(10))(t, text(0, false))
	})
	t.Run("set", func(t *testing.T) {
		tight(glyph)(t, text(graphics.TextRenderingModeFill, true))
	})
}

// d1Extent returns the extent of a glyph drawn by draw after a d1 operator
// whose bounding box has no interior, in glyph space units of a font with
// [glyphMatrix].
func d1Extent(t *testing.T, draw func(b *builder.Builder)) rect.Rect {
	t.Helper()
	b := builder.New(content.Glyph, nil, pdf.V2_0)
	b.Type3UncoloredGlyph(700, 0, 0, 0, 0, 0)
	draw(b)
	stream, err := b.Harvest()
	if err != nil {
		t.Fatal(err)
	}
	F := &type3.Font{
		Glyphs:     []*type3.Glyph{{}, {Name: "A", Content: stream}},
		Resources:  b.Resources,
		FontMatrix: glyphMatrix,
	}
	inst, err := F.New()
	if err != nil {
		t.Fatal(err)
	}
	return inGlyphSpace(inst.GetGeometry().GlyphExtents[1])
}

// TestGlyphExtentD1NoBox checks d1 glyphs whose box has no interior.  The
// box does not confine these, and like all uncoloured glyphs they ignore
// shadings and images other than image masks.
func TestGlyphExtentD1NoBox(t *testing.T) {
	cases := []struct {
		name  string
		draw  func(b *builder.Builder)
		check checker
	}{
		{"fill", func(b *builder.Builder) {
			b.Rectangle(1, 2, 3, 4)
			b.Fill()
		}, tight(rect.Rect{LLx: 1, LLy: 2, URx: 4, URy: 6})},
		{"shading", func(b *builder.Builder) {
			// the builder refuses to write this, but a file can contain it
			b.Resources.Shading = map[pdf.Name]graphics.Shading{"S": axialShading()}
			b.Stream = append(b.Stream, content.Operator{
				Name: content.OpShading,
				Args: []pdf.Object{pdf.Name("S")},
			})
		}, empty},
		{"image mask", func(b *builder.Builder) {
			b.Transform(matrix.Matrix{4, 0, 0, 2, 1, 1})
			b.DrawInlineImageRaw(pdf.Dict{"W": pdf.Integer(1), "H": pdf.Integer(1), "IM": pdf.Boolean(true)}, []byte{0})
		}, tight(rect.Rect{LLx: 1, LLy: 1, URx: 5, URy: 3})},
		{"image", func(b *builder.Builder) {
			b.Stream = append(b.Stream, content.Operator{
				Name: content.OpRawContent,
				Args: []pdf.Object{pdf.String("BI /W 1 /H 1 /BPC 8 /CS /G ID \x00 EI")},
			})
		}, empty},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			c.check(t, d1Extent(t, c.draw))
		})
	}
}

// TestGlyphExtentClip checks that clipping paths confine the ink while they
// are in effect.
func TestGlyphExtentClip(t *testing.T) {
	cases := []struct {
		name  string
		draw  func(b *builder.Builder)
		ink   rect.Rect
		bound rect.Rect
	}{
		{"stroke of the clipping path", func(b *builder.Builder) {
			// the clip takes effect after the path is stroked
			setStroke(b, 10, graphics.LineCapButt)
			b.Rectangle(0, 0, 100, 100)
			b.ClipNonZero()
			b.Stroke()
		}, rect.Rect{LLx: -5, LLy: -5, URx: 105, URy: 105},
			rect.Rect{LLx: -50, LLy: -50, URx: 150, URy: 150}},
		{"clip replaced after Q", func(b *builder.Builder) {
			b.PushGraphicsState()
			b.Rectangle(0, 0, 10, 10)
			b.ClipNonZero()
			b.EndPath()
			b.PopGraphicsState()
			b.Rectangle(20, 20, 10, 10)
			b.ClipNonZero()
			b.EndPath()
			b.DrawShading(axialShading())
		}, rect.Rect{LLx: 20, LLy: 20, URx: 30, URy: 30},
			rect.Rect{LLx: 19, LLy: 19, URx: 31, URy: 31}},
		{"nested clips", func(b *builder.Builder) {
			b.Rectangle(0, 0, 100, 100)
			b.ClipNonZero()
			b.EndPath()
			b.PushGraphicsState()
			b.Rectangle(50, 50, 100, 100)
			b.ClipNonZero()
			b.EndPath()
			b.Rectangle(-10, -10, 200, 200)
			b.Fill()
			b.PopGraphicsState()
			b.Rectangle(-10, -10, 20, 20)
			b.Fill()
		}, rect.Rect{LLx: 0, LLy: 0, URx: 100, URy: 100},
			rect.Rect{LLx: 0, LLy: 0, URx: 100, URy: 100}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := glyphExtent(t, false, c.draw)
			within(c.ink, c.bound)(t, got)
		})
	}
}

// TestGlyphExtentD1Overflow checks that a d1 box which overflows when mapped
// to text space gives an extent without bound.
func TestGlyphExtentD1Overflow(t *testing.T) {
	b := builder.New(content.Glyph, nil, pdf.V2_0)
	b.Type3UncoloredGlyph(700, 0, -1e300, -1e300, 1e300, 1e300)
	stream, err := b.Harvest()
	if err != nil {
		t.Fatal(err)
	}
	F := &type3.Font{
		Glyphs:     []*type3.Glyph{{}, {Name: "A", Content: stream}},
		FontMatrix: matrix.Matrix{1e10, 0, 0, 1, 0, 0},
	}
	inst, err := F.New()
	if err != nil {
		t.Fatal(err)
	}
	if got := inst.GetGeometry().GlyphExtents[1]; !isInfinite(got) {
		t.Errorf("extent %v, want infinite", got)
	}
}

// TestGlyphExtentReach checks that a glyph whose ink reaches far beyond its
// em square has an extent without bound, whether the ink is painted or
// declared by d1, while ink a few em across keeps its extent.
func TestGlyphExtentReach(t *testing.T) {
	cases := []struct {
		name  string
		d1    bool
		box   rect.Rect // glyph space
		check checker
	}{
		{"fill nearby", false, rect.Rect{LLx: -2000, LLy: -1000, URx: 5000, URy: 3000}, nil},
		{"fill far away", false, rect.Rect{LLx: 1e7, LLy: 0, URx: 1e7 + 500, URy: 500}, infinite},
		{"fill far across", false, rect.Rect{LLx: -1e7, LLy: 0, URx: 1e7, URy: 500}, infinite},
		{"d1 nearby", true, rect.Rect{LLx: -2000, LLy: -1000, URx: 5000, URy: 3000}, nil},
		{"d1 far away", true, rect.Rect{LLx: 0, LLy: 0, URx: 500, URy: 1e7}, infinite},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			check := c.check
			if check == nil {
				check = within(c.box, c.box)
			}
			b := builder.New(content.Glyph, nil, pdf.V2_0)
			if c.d1 {
				b.Type3UncoloredGlyph(700, 0, c.box.LLx, c.box.LLy, c.box.URx, c.box.URy)
			} else {
				b.Type3ColoredGlyph(700, 0)
				b.Rectangle(c.box.LLx, c.box.LLy, c.box.Dx(), c.box.Dy())
				b.Fill()
			}
			stream, err := b.Harvest()
			if err != nil {
				t.Fatal(err)
			}
			F := &type3.Font{
				Glyphs:     []*type3.Glyph{{}, {Name: "A", Content: stream}},
				FontMatrix: glyphMatrix,
			}
			inst, err := F.New()
			if err != nil {
				t.Fatal(err)
			}
			check(t, inGlyphSpace(inst.GetGeometry().GlyphExtents[1]))
		})
	}
}

// fakePS is a PostScript XObject, which is not painted.
type fakePS struct{ fakeXObject }

func (fakePS) Subtype() pdf.Name { return "PS" }

// TestGlyphExtentMore checks operators and parameters not covered above.
func TestGlyphExtentMore(t *testing.T) {
	F := squarefont.All[0].MakeFont()
	gid := F.Layout(nil, 10, "A").Seq[0].GID
	code, ok := F.Encode(gid, "A")
	if !ok {
		t.Fatal("cannot encode A")
	}
	A := pdf.String(F.Codec().AppendCode(nil, code))

	radial := func(r1, r2 float64, extendEnd bool) *shading.Type3 {
		return &shading.Type3{
			ColorSpace: color.SpaceDeviceGray,
			Center1:    vec.Vec2{X: 0, Y: 0}, R1: r1,
			Center2: vec.Vec2{X: 10, Y: 0}, R2: r2,
			F: &function.Type2{
				XMin: 0, XMax: 1,
				C0: []float64{0}, C1: []float64{1},
				N: 1,
			},
			TMax:      1,
			ExtendEnd: extendEnd,
			SingleUse: true,
		}
	}
	boxed := axialShading()
	boxed.BBox = &pdf.Rectangle{LLx: 1, LLy: 2, URx: 3, URy: 4}

	stroked := func(b *builder.Builder) {
		b.TextBegin()
		setText(b, F, 10)
		b.TextSetRenderingMode(graphics.TextRenderingModeStroke)
		b.TextFirstLine(0, 0)
		b.TextShow("A")
		b.TextEnd()
	}
	textStroke := textInk(F, 10, 0, 0).Grow(5)

	cases := []struct {
		name  string
		draw  func(b *builder.Builder)
		check checker
	}{
		{"thinnest line", func(b *builder.Builder) {
			setStroke(b, 0, graphics.LineCapButt)
			b.MoveTo(0, 0)
			b.LineTo(10, 0)
			b.Stroke()
		}, tight(rect.Rect{URx: 10})},
		{"radial, shrinking end", func(b *builder.Builder) {
			// the circles shrink to a point at (20, 0)
			b.DrawShading(radial(2, 1, true))
		}, tight(rect.Rect{LLx: -2, LLy: -2, URx: 20, URy: 2})},
		{"radial, growing end", func(b *builder.Builder) {
			b.DrawShading(radial(1, 2, true))
		}, infinite},
		{"shading with bounding box", func(b *builder.Builder) {
			b.DrawShading(boxed)
		}, tight(rect.Rect{LLx: 1, LLy: 2, URx: 3, URy: 4})},
		{"PostScript XObject", func(b *builder.Builder) {
			b.DrawXObject(fakePS{})
		}, empty},
		{"form", func(b *builder.Builder) {
			b.DrawXObject(&form.Form{
				Content: &content.Operators{},
				Res:     &content.Resources{},
				BBox:    pdf.Rectangle{LLx: 1, LLy: 2, URx: 3, URy: 4},
			})
		}, tight(rect.Rect{LLx: 1, LLy: 2, URx: 3, URy: 4})},
		{"\" operator", func(b *builder.Builder) {
			b.TextBegin()
			setText(b, F, 10)
			b.TextSetLeading(-20)
			b.TextShowSpacedRaw(0, 0, A)
			b.TextEnd()
		}, tight(textInk(F, 10, 0, 20))},
		{"TJ with strings", func(b *builder.Builder) {
			// the kern moves the second glyph right by 1000
			b.TextBegin()
			setText(b, F, 10)
			b.TextFirstLine(0, 0)
			b.TextShowKernedRaw(A, pdf.Integer(-100000), A)
			b.TextEnd()
		}, func(t *testing.T, got rect.Rect) {
			ink := textInk(F, 10, 0, 0)
			ink.Extend(textInk(F, 10, 1000+F.Layout(nil, 10, "A").Seq[0].Advance, 0))
			tight(ink)(t, got)
		}},
		{"stroked text, inherited join", func(b *builder.Builder) {
			b.SetLineWidth(10)
			b.SetLineCap(graphics.LineCapButt)
			b.SetMiterLimit(4)
			stroked(b)
		}, strokeWithin(textStroke, 5, 4)},
		{"stroked text, inherited cap", func(b *builder.Builder) {
			b.SetLineWidth(10)
			b.SetLineJoin(graphics.LineJoinRound)
			stroked(b)
		}, strokeWithin(textStroke, 5, math.Sqrt2)},
		{"stroked text, inherited miter limit", func(b *builder.Builder) {
			b.SetLineWidth(10)
			b.SetLineJoin(graphics.LineJoinMiter)
			b.SetLineCap(graphics.LineCapButt)
			stroked(b)
		}, infinite},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			c.check(t, glyphExtent(t, false, c.draw))
		})
	}
}

// TestGlyphInkBounds checks the ink bounds of glyph procedures.
func TestGlyphInkBounds(t *testing.T) {
	harvest := func(b *builder.Builder) content.Stream {
		t.Helper()
		stream, err := b.Harvest()
		if err != nil {
			t.Fatal(err)
		}
		return stream
	}

	t.Run("d1 box", func(t *testing.T) {
		b := builder.New(content.Glyph, nil, pdf.V2_0)
		b.Type3UncoloredGlyph(700, 0, 10, 20, 600, 800)
		b.Rectangle(-1000, -1000, 5000, 5000)
		b.Fill()
		got := type3.GlyphInkBounds(harvest(b), nil)
		want := rect.Rect{LLx: 10, LLy: 20, URx: 600, URy: 800}
		if got != want {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("d0 path", func(t *testing.T) {
		b := builder.New(content.Glyph, nil, pdf.V2_0)
		b.Type3ColoredGlyph(500, 0)
		b.Rectangle(30, 40, 100, 900)
		b.Fill()
		got := type3.GlyphInkBounds(harvest(b), nil)
		tight(rect.Rect{LLx: 30, LLy: 40, URx: 130, URy: 940})(t, got)
	})

	t.Run("no ink", func(t *testing.T) {
		b := builder.New(content.Glyph, nil, pdf.V2_0)
		b.Type3ColoredGlyph(500, 0)
		if got := type3.GlyphInkBounds(harvest(b), nil); !got.IsZero() {
			t.Errorf("got %v, want the zero rectangle", got)
		}
	})

	t.Run("unbounded", func(t *testing.T) {
		b := builder.New(content.Glyph, nil, pdf.V2_0)
		b.Type3ColoredGlyph(500, 0)
		b.Rectangle(0, 0, 10, 10)
		b.Stroke() // the line width is inherited
		if got := type3.GlyphInkBounds(harvest(b), nil); !isInfinite(got) {
			t.Errorf("got %v, want an infinite rectangle", got)
		}
	})
}

// TestGlyphPaintBounds checks that the bounds contain both the d1 box and
// the ink outside it.
func TestGlyphPaintBounds(t *testing.T) {
	b := builder.New(content.Glyph, nil, pdf.V2_0)
	b.Type3UncoloredGlyph(700, 0, 10, 20, 600, 800)
	b.Rectangle(-100, 30, 200, 1000)
	b.Fill()
	stream, err := b.Harvest()
	if err != nil {
		t.Fatal(err)
	}
	got := type3.GlyphPaintBounds(stream, nil)
	for _, r := range []rect.Rect{
		{LLx: 10, LLy: 20, URx: 600, URy: 800},
		{LLx: -100, LLy: 30, URx: 100, URy: 1030},
	} {
		if !got.Covers(r) {
			t.Errorf("bounds %v do not cover %v", got, r)
		}
	}
}

// TestGlyphPaintBoundsZeroBox checks that a d1 box without interior adds
// nothing to the bounds.
func TestGlyphPaintBoundsZeroBox(t *testing.T) {
	ink := rect.Rect{LLx: 100, LLy: 200, URx: 300, URy: 400}
	for _, fill := range []bool{false, true} {
		b := builder.New(content.Glyph, nil, pdf.V2_0)
		b.Type3UncoloredGlyph(700, 0, 0, 0, 0, 0)
		if fill {
			b.Rectangle(ink.LLx, ink.LLy, ink.Dx(), ink.Dy())
			b.Fill()
		}
		stream, err := b.Harvest()
		if err != nil {
			t.Fatal(err)
		}
		got := type3.GlyphPaintBounds(stream, nil)
		if !fill {
			if !got.IsZero() {
				t.Errorf("got %v, want the zero rectangle", got)
			}
			continue
		}
		tight(ink)(t, got)
	}
}

// TestGlyphPaintBoundsUnpainted checks the bounds of procedures which paint
// nothing, and of a procedure which cannot be analysed.
func TestGlyphPaintBoundsUnpainted(t *testing.T) {
	empty := &content.Operators{}
	if got := type3.GlyphPaintBounds(nil, nil); !got.IsZero() {
		t.Errorf("nil procedure: got %v, want the zero rectangle", got)
	}
	if got := type3.GlyphPaintBounds(empty, nil); !got.IsZero() {
		t.Errorf("empty procedure: got %v, want the zero rectangle", got)
	}
	malformed := &content.Operators{Ops: []content.Operator{
		{Name: content.OpFill},
	}}
	infinite(t, type3.GlyphPaintBounds(malformed, nil))
}
