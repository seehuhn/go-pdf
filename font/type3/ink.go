// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package type3

import (
	"bytes"
	"io"
	"math"

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/geom/path"
	"seehuhn.de/go/geom/rect"
	"seehuhn.de/go/geom/vec"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/form"
	"seehuhn.de/go/pdf/graphics/shading"
)

// A glyph procedure inherits the graphics state, apart from the CTM, from
// the text-showing operator which invokes it.  A parameter which the
// procedure does not set itself can therefore take any value.  The ink
// bounds cover every such value where a bound exists, and are infinite
// where none does.

const (
	// textAdvanceBits are the parameters which determine how far a
	// text-showing operator moves the text position.
	textAdvanceBits = graphics.StateTextFont | graphics.StateTextMatrix |
		graphics.StateTextCharacterSpacing | graphics.StateTextWordSpacing |
		graphics.StateTextHorizontalScaling

	// textPlaceBits are the parameters which place the glyphs a
	// text-showing operator paints.
	textPlaceBits = textAdvanceBits | graphics.StateTextRise

	// extGStateInkBits are the parameters which an ExtGState can set and
	// which change where ink falls.
	extGStateInkBits = graphics.StateLineWidth | graphics.StateLineCap |
		graphics.StateLineJoin | graphics.StateMiterLimit |
		graphics.StateLineDash | graphics.StateTextFont
)

// infiniteRect is the extent of ink which has no bound.
var infiniteRect = rect.Rect{
	LLx: math.Inf(-1), LLy: math.Inf(-1),
	URx: math.Inf(+1), URy: math.Inf(+1),
}

// isInfinite reports whether r is [infiniteRect].
func isInfinite(r rect.Rect) bool {
	return math.IsInf(r.LLx, -1)
}

// region is a part of glyph space: empty, the whole plane, or a rectangle.
type region struct {
	r         rect.Rect
	empty     bool
	unbounded bool
}

var wholePlane = region{unbounded: true}

// rectRegion returns the region r covers.  A rectangle with a side which
// is not a finite number, as a transformation which overflows can give,
// covers the whole plane.
func rectRegion(r rect.Rect) region {
	if !r.IsFinite() {
		return wholePlane
	}
	return region{r: r}
}

// boundedRegion returns the region a bounding box reports, where ok is
// false for a box around nothing.
func boundedRegion(r rect.Rect, ok bool) region {
	if !ok {
		return region{empty: true}
	}
	return rectRegion(r)
}

// intersect returns the part of glyph space which lies in both a and b.
func (a region) intersect(b region) region {
	switch {
	case a.empty || b.empty:
		return region{empty: true}
	case a.unbounded:
		return b
	case b.unbounded:
		return a
	}
	r := rect.Rect{
		LLx: max(a.r.LLx, b.r.LLx), LLy: max(a.r.LLy, b.r.LLy),
		URx: min(a.r.URx, b.r.URx), URy: min(a.r.URy, b.r.URy),
	}
	if r.LLx > r.URx || r.LLy > r.URy {
		return region{empty: true}
	}
	return region{r: r}
}

// union returns the smallest region containing a and b.
func union(a, b region) region {
	switch {
	case a.empty:
		return b
	case b.empty:
		return a
	case a.unbounded || b.unbounded:
		return wholePlane
	}
	a.r.Extend(b.r)
	return a
}

// inkBounds accumulates the ink a glyph's content stream paints, in glyph
// space.
type inkBounds struct {
	st *content.State

	// resUnknown is set where the glyph's resources are inherited from the
	// page, so that named resources cannot be resolved.
	resUnknown bool

	// clips caches, for a prefix of the clipping paths in the graphics
	// state, the region each path and those before it confine the ink to
	clips []clipEntry

	// fontExtents caches the union of all glyph extents of a font.
	fontExtents map[font.Instance]rect.Rect

	ink region
}

func newInkBounds(res *content.Resources) *inkBounds {
	return &inkBounds{
		st:          content.NewState(content.Glyph, res),
		resUnknown:  res == nil,
		fontExtents: make(map[font.Instance]rect.Rect),
		ink:         region{empty: true},
	}
}

// result returns the rectangle which contains all ink seen so far:
// [infiniteRect] for ink without bound, and the zero rectangle where there
// is no ink.
func (b *inkBounds) result() rect.Rect {
	switch {
	case b.ink.empty:
		return rect.Rect{}
	case b.ink.unbounded:
		return infiniteRect
	}
	return b.ink.r
}

// done reports whether the ink has no bound, so that no further operator
// can change the result.
func (b *inkBounds) done() bool {
	return b.ink.unbounded
}

// add records ink which lies in r, before clipping.
func (b *inkBounds) add(r region) {
	b.addClipped(r, b.st.GState.ClipPaths)
}

// addClipped records ink which lies in r, before clipping by cps.
func (b *inkBounds) addClipped(r region, cps []graphics.ClipPath) {
	b.ink = union(b.ink, r.intersect(b.clipRegion(cps)))
}

// clipEntry is one entry of [inkBounds.clips].
type clipEntry struct {
	path   *path.Data
	region region
}

// clipRegion returns the region the clipping paths cps confine ink to.
//
// The clipping paths form a stack: q saves them, Q restores a saved prefix,
// and each clipping operator appends a newly allocated path.  The paths
// which cps shares with the cache are therefore those up to the last one
// with the same pointer, and each path is intersected only once.
func (b *inkBounds) clipRegion(cps []graphics.ClipPath) region {
	n := min(len(b.clips), len(cps))
	for n > 0 && b.clips[n-1].path != cps[n-1].Path {
		n--
	}
	b.clips = b.clips[:n]
	for _, cp := range cps[n:] {
		r := boundedRegion(cp.Path.Iter().Transform(cp.CTM).TightBBox())
		if len(b.clips) > 0 {
			r = r.intersect(b.clips[len(b.clips)-1].region)
		}
		b.clips = append(b.clips, clipEntry{path: cp.Path, region: r})
	}
	if len(b.clips) == 0 {
		return wholePlane
	}
	return b.clips[len(b.clips)-1].region
}

// addRect records ink which lies in r, given in a coordinate system which M
// maps to glyph space.  An infinite r stands for ink without bound.
func (b *inkBounds) addRect(r rect.Rect, M matrix.Matrix) {
	if isInfinite(r) {
		b.add(wholePlane)
		return
	}
	b.add(rectRegion(r.Transform(M)))
}

// apply processes one operator of the glyph's content stream.
func (b *inkBounds) apply(name content.OpName, args []pdf.Object) {
	st := b.st

	// an uncoloured glyph ignores shadings and images other than masks
	if st.Ignored(name, args) {
		return
	}

	numClips := len(st.GState.ClipPaths)
	leadingSet := st.IsSet(graphics.StateTextLeading)

	// state changes first: for painting operators this moves the current
	// path to the painted path, and ' and " move to the next line
	_ = st.ApplyStateChanges(name, args)
	gs := st.GState

	if pp, ok := content.PathPainting(name); ok {
		// the clipping path changes after the path is painted
		cps := gs.ClipPaths[:numClips]
		p := st.PaintedPath().Iter()
		if pp.Fill {
			b.addClipped(boundedRegion(p.Transform(gs.CTM).TightBBox()), cps)
		}
		if pp.Stroke {
			b.addClipped(b.strokeRegion(p), cps)
		}
		return
	}

	switch name {
	case content.OpRawContent:
		// Bytes inserted into the stream verbatim.  Read from a file, these
		// are comments.
		if raw, ok := getString(args, 0); ok {
			b.applyRaw(raw)
		}

	case content.OpSetExtGState:
		if b.resUnknown {
			gs.Set &^= extGStateInkBits
		}

	case content.OpTextNextLine:
		if !leadingSet {
			gs.Set &^= graphics.StateTextMatrix
		}

	case content.OpInlineImage:
		b.addRect(rect.Rect{URx: 1, URy: 1}, gs.CTM)

	case content.OpXObject:
		name, ok := getName(args)
		if !ok {
			return
		}
		if b.resUnknown {
			b.add(wholePlane)
			return
		}
		if x := st.Resources.XObject[name]; x != nil {
			b.addXObject(x, gs.CTM)
		}

	case content.OpShading:
		name, ok := getName(args)
		if !ok {
			return
		}
		if b.resUnknown {
			b.add(wholePlane)
			return
		}
		if sh := st.Resources.Shading[name]; sh != nil {
			b.add(shadingRegion(sh, gs.CTM))
		}

	case content.OpTextShow, content.OpTextShowMoveNextLine,
		content.OpTextShowMoveNextLineSetSpacing, content.OpTextShowArray:
		if name != content.OpTextShow && name != content.OpTextShowArray && !leadingSet {
			// ' and " move to a new line by the inherited leading
			gs.Set &^= graphics.StateTextMatrix
		}
		b.showText(name, args)
		if !st.IsSet(textAdvanceBits) {
			// the text position moved by an amount which is not known
			gs.Set &^= graphics.StateTextMatrix
		}
	}
}

// showText processes the operands of a text-showing operator.
func (b *inkBounds) showText(name content.OpName, args []pdf.Object) {
	switch name {
	case content.OpTextShow, content.OpTextShowMoveNextLine:
		if s, ok := getString(args, 0); ok {
			b.addText(s)
		}
	case content.OpTextShowMoveNextLineSetSpacing:
		if s, ok := getString(args, 2); ok {
			b.addText(s)
		}
	case content.OpTextShowArray:
		if len(args) == 0 {
			return
		}
		a, _ := args[0].(pdf.Array)
		for _, obj := range a {
			if s, ok := obj.(pdf.String); ok {
				b.addText(s)
			} else if x, ok := getNumber(obj); ok {
				b.st.GState.ApplyTextKern(x)
			}
		}
	}
}

// applyRaw processes the operators in raw content stream bytes.
func (b *inkBounds) applyRaw(raw pdf.String) {
	stm := content.NewScanner(func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(raw)), nil
	})
	it := stm.NewIter()
	for name, args := range it.All() {
		if name != content.OpRawContent {
			b.apply(name, args)
		}
	}
}

// addXObject records the ink of an XObject painted with the given CTM.
func (b *inkBounds) addXObject(x graphics.XObject, ctm matrix.Matrix) {
	if f, ok := x.(*form.Form); ok {
		// the form is clipped to its bounding box
		if f.BBox.IsZero() {
			return
		}
		M := f.Matrix
		if M.IsZero() {
			M = matrix.Identity
		}
		b.addRect(f.BBox.ToRect(), M.Mul(ctm))
		return
	}
	switch x.Subtype() {
	case "Image":
		// an image fills the unit square of user space
		b.addRect(rect.Rect{URx: 1, URy: 1}, ctm)
	case "PS":
		// PostScript XObjects are not painted
	default:
		b.add(wholePlane)
	}
}

// addText records the ink of the glyphs a text-showing operator paints, and
// advances the text matrix past them.
func (b *inkBounds) addText(s pdf.String) {
	st := b.st
	gs := st.GState

	// an inherited rendering mode can both fill and stroke
	mode := graphics.TextRenderingModeFillStroke
	if st.IsSet(graphics.StateTextRenderingMode) {
		mode = gs.TextRenderingMode
	}
	visible := mode != graphics.TextRenderingModeInvisible && mode != graphics.TextRenderingModeClip
	if visible && !st.IsSet(textPlaceBits) {
		// the glyphs are placed by inherited parameters
		b.add(wholePlane)
		visible = false
	}
	F := gs.TextFont
	if F == nil {
		return
	}
	stroked := mode == graphics.TextRenderingModeStroke ||
		mode == graphics.TextRenderingModeFillStroke ||
		mode == graphics.TextRenderingModeStrokeClip ||
		mode == graphics.TextRenderingModeFillStrokeClip

	geomFont, hasGeom := F.(interface{ GetGeometry() *font.Geometry })
	outliner, hasOutlines := F.(font.Outliner)
	for code := range F.Codes(s) {
		if visible {
			var ext rect.Rect
			switch {
			case !hasGeom:
				ext = infiniteRect
			case hasOutlines:
				gid, ok := outliner.GlyphID(code.CID)
				if !ok {
					gid, _ = outliner.GlyphID(code.Notdef)
				}
				if g := geomFont.GetGeometry(); int(gid) < len(g.GlyphExtents) {
					ext = g.GlyphExtents[gid]
				}
			default:
				ext = b.allExtents(F, geomFont.GetGeometry())
			}
			if !ext.IsZero() {
				b.addGlyph(ext, gs.GlyphMatrix(&code).Mul(gs.CTM), stroked)
			}
		}
		gs.AdvanceTextMatrix(&code)
	}
}

// addGlyph records the ink of a glyph with extent ext, which M maps to glyph
// space.
func (b *inkBounds) addGlyph(ext rect.Rect, M matrix.Matrix, stroked bool) {
	if isInfinite(ext) {
		b.add(wholePlane)
		return
	}
	r := ext.Transform(M)
	if stroked {
		reach, ok := b.strokeReach()
		if !ok {
			b.add(wholePlane)
			return
		}
		C := b.st.GState.CTM
		hx := reach * math.Hypot(C[0], C[2])
		hy := reach * math.Hypot(C[1], C[3])
		r = rect.Rect{LLx: r.LLx - hx, LLy: r.LLy - hy, URx: r.URx + hx, URy: r.URy + hy}
	}
	b.add(rectRegion(r))
}

// strokeReach returns how far, in user space, the stroke of an outline
// whose corners can be of any sharpness reaches beyond the outline.  The
// result is false where there is no bound.
//
// The pen is half the line width across.  A miter join reaches out by at
// most the miter limit times this, and a square cap by the square root of
// two times this.
func (b *inkBounds) strokeReach() (float64, bool) {
	st := b.st
	gs := st.GState
	if !st.IsSet(graphics.StateLineWidth) {
		return 0, false
	}
	k := 1.0
	if !st.IsSet(graphics.StateLineJoin) || gs.LineJoin == graphics.LineJoinMiter {
		if !st.IsSet(graphics.StateMiterLimit) {
			return 0, false
		}
		k = max(k, gs.MiterLimit)
	}
	if !st.IsSet(graphics.StateLineCap) || gs.LineCap == graphics.LineCapSquare {
		k = max(k, math.Sqrt2)
	}
	return k * strokeWidth(gs.LineWidth) / 2, true
}

// strokeWidth returns the width a line of the given width parameter is
// bounded as.  A width which is not positive asks for the thinnest line the
// device can paint.  This is bounded as a line of no width: it reaches less
// than a device pixel beyond the path, as the edge of a filled area can.
func strokeWidth(w float64) float64 {
	if w <= 0 {
		return 0
	}
	return w
}

// strokeRegion returns the region of glyph space which the stroke of p, a
// path in user space, can cover.
func (b *inkBounds) strokeRegion(p path.Path) region {
	st := b.st
	gs := st.GState
	if !st.IsSet(graphics.StateLineWidth) {
		return wholePlane
	}

	// A square cap covers a butt or round one, except on a sub-path of
	// length zero, which only a round cap paints.  A miter join covers a
	// bevel, and a round join where it is not itself bevelled.  A miter
	// without limit is never bevelled, apart from a corner which doubles
	// back.
	caps := []graphics.LineCapStyle{gs.LineCap}
	if !st.IsSet(graphics.StateLineCap) {
		caps = []graphics.LineCapStyle{graphics.LineCapSquare, graphics.LineCapRound}
	}
	joins := []graphics.LineJoinStyle{gs.LineJoin}
	if !st.IsSet(graphics.StateLineJoin) {
		joins = []graphics.LineJoinStyle{graphics.LineJoinMiter, graphics.LineJoinRound}
	}
	miterLimit := gs.MiterLimit
	if !st.IsSet(graphics.StateMiterLimit) {
		miterLimit = math.Inf(1)
	}
	// the bound for a dashed stroke covers every dash pattern, including
	// none
	dashed := !st.IsSet(graphics.StateLineDash) || len(gs.DashPattern) > 0

	res := region{empty: true}
	for _, lineCap := range caps {
		for _, join := range joins {
			opt := path.StrokeOptions{
				Width:      strokeWidth(gs.LineWidth),
				Cap:        lineCap,
				Join:       join,
				MiterLimit: miterLimit,
				Dashed:     dashed,
			}
			res = union(res, boundedRegion(strokeBBox(p, gs.CTM, opt)))
		}
	}
	return res
}

// allExtents returns the union of the extents of all glyphs of a font, for
// fonts which cannot map a character to its glyph.
func (b *inkBounds) allExtents(F font.Instance, g *font.Geometry) rect.Rect {
	if r, ok := b.fontExtents[F]; ok {
		return r
	}
	var r rect.Rect
	for _, ext := range g.GlyphExtents {
		if isInfinite(ext) {
			r = infiniteRect
			break
		}
		r.Extend(ext)
	}
	b.fontExtents[F] = r
	return r
}

// strokeBBox returns the rectangle in glyph space which contains the stroke
// of p, a path in user space, drawn with the options opt under the CTM C.
//
// The pen is a disc in user space, which the CTM can distort.  Each glyph
// space coordinate is a linear function of the user space position, so its
// range over the stroke is found by mapping user space with a rotation and
// uniform scaling, which keeps the pen a disc, onto an x-axis along which
// that coordinate grows.
func strokeBBox(p path.Path, C matrix.Matrix, opt path.StrokeOptions) (rect.Rect, bool) {
	xRange := func(a, c float64) (lo, hi float64, ok bool) {
		o := opt
		o.Width = opt.Width * math.Hypot(a, c)
		r, ok := p.Transform(matrix.Matrix{a, -c, c, a, 0, 0}).StrokeBBox(o)
		return r.LLx, r.URx, ok
	}
	llx, urx, okX := xRange(C[0], C[2])
	lly, ury, okY := xRange(C[1], C[3])
	if !okX || !okY {
		return rect.Rect{}, false
	}
	return rect.Rect{LLx: llx + C[4], LLy: lly + C[5], URx: urx + C[4], URy: ury + C[5]}, true
}

// shadingRegion returns the region of glyph space which a shading painted
// with the sh operator and the given CTM can cover, before clipping.
func shadingRegion(sh graphics.Shading, ctm matrix.Matrix) region {
	var r region
	switch sh := sh.(type) {
	case *shading.Type1:
		domain := []float64{0, 1, 0, 1}
		if len(sh.Domain) == 4 {
			domain = sh.Domain
		}
		M := matrix.Identity
		if len(sh.Matrix) == 6 {
			M = matrix.Matrix(sh.Matrix)
		}
		d := rect.Rect{LLx: domain[0], URx: domain[1], LLy: domain[2], URy: domain[3]}
		r = rectRegion(d.Transform(M.Mul(ctm)))
	case *shading.Type3:
		r = radialRegion(sh, ctm)
	case *shading.Type4:
		pts := make([]vec.Vec2, len(sh.Vertices))
		for i, v := range sh.Vertices {
			pts[i] = vec.Vec2{X: v.X, Y: v.Y}
		}
		r = pointsRegion(pts, ctm)
	case *shading.Type5:
		pts := make([]vec.Vec2, len(sh.Vertices))
		for i, v := range sh.Vertices {
			pts[i] = vec.Vec2{X: v.X, Y: v.Y}
		}
		r = pointsRegion(pts, ctm)
	case *shading.Type6:
		var pts []vec.Vec2
		for _, patch := range sh.Patches {
			pts = append(pts, patch.ControlPoints[:]...)
			pts = append(pts, coonsInterior(&patch.ControlPoints)...)
		}
		r = pointsRegion(pts, ctm)
	case *shading.Type7:
		var pts []vec.Vec2
		for _, patch := range sh.Patches {
			pts = append(pts, patch.ControlPoints[:]...)
		}
		r = pointsRegion(pts, ctm)
	case *shading.Type2:
		// an axial shading covers a strip which is infinite across its axis
		r = wholePlane
	default:
		// a shading whose shape this does not know
		r = wholePlane
	}

	if bb := sh.GetShadingCommon().BBox; bb != nil {
		r = r.intersect(rectRegion(bb.ToRect().Transform(ctm)))
	}
	return r
}

// radialRegion returns the region a radial shading can cover.  Without
// extension, the shading is the union of circles interpolating between the
// two given ones, which lies in the convex hull of these.  An extension
// towards shrinking circles ends where the radius reaches zero.  An
// extension towards growing or equal circles has no bound.
func radialRegion(sh *shading.Type3, ctm matrix.Matrix) region {
	c1, c2 := sh.Center1, sh.Center2
	r1, r2 := sh.R1, sh.R2

	var pts []vec.Vec2
	if sh.ExtendStart {
		if r1 >= r2 {
			return wholePlane
		}
		pts = append(pts, c1.Add(c1.Sub(c2).Mul(r1/(r2-r1))))
	}
	if sh.ExtendEnd {
		if r2 >= r1 {
			return wholePlane
		}
		pts = append(pts, c2.Add(c2.Sub(c1).Mul(r2/(r1-r2))))
	}

	// a circle maps to an ellipse, whose half extents along the axes are
	// the radius times the lengths of the rows of the CTM's linear part
	sx, sy := math.Hypot(ctm[0], ctm[2]), math.Hypot(ctm[1], ctm[3])
	res := pointsRegion(pts, ctm)
	for _, c := range []struct {
		centre vec.Vec2
		radius float64
	}{{c1, r1}, {c2, r2}} {
		p := ctm.Apply(c.centre)
		hx, hy := c.radius*sx, c.radius*sy
		circle := rectRegion(rect.Rect{LLx: p.X - hx, LLy: p.Y - hy, URx: p.X + hx, URy: p.Y + hy})
		res = union(res, circle)
	}
	return res
}

// pointsRegion returns the bounding box of the given points, mapped by M.
func pointsRegion(pts []vec.Vec2, M matrix.Matrix) region {
	res := region{empty: true}
	for _, p := range pts {
		q := M.Apply(p)
		res = union(res, rectRegion(rect.Rect{LLx: q.X, LLy: q.Y, URx: q.X, URy: q.Y}))
	}
	return res
}

// coonsInterior returns the four interior control points of the bicubic
// patch which equals the Coons patch with the given boundary control
// points.  The patch lies in the convex hull of its sixteen control points,
// but the interior points can lie outside the hull of the boundary.
//
// The boundary points run round the patch in the order of a Type 6
// shading: p00, p01, p02, p03, p13, p23, p33, p32, p31, p30, p20, p10.
func coonsInterior(cp *[12]vec.Vec2) []vec.Vec2 {
	p00, p01, p02, p03 := cp[0], cp[1], cp[2], cp[3]
	p13, p23, p33, p32 := cp[4], cp[5], cp[6], cp[7]
	p31, p30, p20, p10 := cp[8], cp[9], cp[10], cp[11]

	// the interior point next to corner a, as given in the PDF specification
	interior := func(a, b1, b2, c1, c2, d1, d2, e vec.Vec2) vec.Vec2 {
		s := a.Mul(-4).
			Add(b1.Add(b2).Mul(6)).
			Add(c1.Add(c2).Mul(-2)).
			Add(d1.Add(d2).Mul(3)).
			Sub(e)
		return s.Mul(1.0 / 9)
	}
	return []vec.Vec2{
		interior(p00, p01, p10, p03, p30, p31, p13, p33), // p11
		interior(p03, p02, p13, p00, p33, p32, p10, p30), // p12
		interior(p30, p31, p20, p33, p00, p01, p23, p03), // p21
		interior(p33, p32, p23, p30, p03, p02, p20, p00), // p22
	}
}

func getName(args []pdf.Object) (pdf.Name, bool) {
	if len(args) == 0 {
		return "", false
	}
	n, ok := args[0].(pdf.Name)
	return n, ok
}

func getString(args []pdf.Object, i int) (pdf.String, bool) {
	if len(args) <= i {
		return nil, false
	}
	s, ok := args[i].(pdf.String)
	return s, ok
}
