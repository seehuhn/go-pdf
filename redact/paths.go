// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package redact

import (
	"slices"

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/geom/vec"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/content"
)

// A pathOp is a buffered path operator.
type pathOp struct {
	idx  int
	name content.OpName
	args []pdf.Object
}

// A subpath is a run of path-construction operators which draw one
// connected piece of the path.
type subpath struct {
	ops []pathOp

	// pts are the control points, in user space
	pts []vec.Vec2

	// implicit is set if the subpath starts without "m", after "h" or "re"
	implicit bool

	// the start point, as vector and as operands of an "m" operator
	start  vec.Vec2
	sx, sy pdf.Object
}

// A pathBuf collects the operators of the current path.
type pathBuf struct {
	subs []*subpath

	// cur is the subpath which new segments extend, or nil
	cur *subpath

	// closed is set if cur was ended by "h" or "re"
	closed bool

	// clips are the "W" and "W*" operators of the path
	clips []pathOp
}

// numbers converts n operands to numbers.
func numbers(args []pdf.Object, n int) ([]float64, bool) {
	if len(args) != n {
		return nil, false
	}
	res := make([]float64, n)
	for i, a := range args {
		v, ok := getNumber(a)
		if !ok {
			return nil, false
		}
		res[i] = v
	}
	return res, true
}

// add buffers a path-construction or clipping operator.  Operators which
// are malformed or which have no current point are left out of the
// buffer, and so are never changed.
func (pb *pathBuf) add(idx int, name content.OpName, args []pdf.Object) {
	op := pathOp{idx: idx, name: name, args: slices.Clone(args)}
	pt := func(v []float64, i int) vec.Vec2 { return vec.Vec2{X: v[2*i], Y: v[2*i+1]} }

	switch name {
	case content.OpClipNonZero, content.OpClipEvenOdd:
		pb.clips = append(pb.clips, op)

	case content.OpMoveTo:
		v, ok := numbers(args, 2)
		if !ok {
			return
		}
		s := &subpath{ops: []pathOp{op}, pts: []vec.Vec2{pt(v, 0)}, start: pt(v, 0), sx: op.args[0], sy: op.args[1]}
		pb.subs = append(pb.subs, s)
		pb.cur, pb.closed = s, false

	case content.OpRectangle:
		v, ok := numbers(args, 4)
		if !ok {
			return
		}
		x, y, w, h := v[0], v[1], v[2], v[3]
		s := &subpath{
			ops: []pathOp{op},
			pts: []vec.Vec2{{X: x, Y: y}, {X: x + w, Y: y}, {X: x + w, Y: y + h}, {X: x, Y: y + h}},

			start: vec.Vec2{X: x, Y: y}, sx: op.args[0], sy: op.args[1],
		}
		pb.subs = append(pb.subs, s)
		pb.cur, pb.closed = s, true

	case content.OpLineTo, content.OpCurveTo, content.OpCurveToV, content.OpCurveToY:
		var n int
		switch name {
		case content.OpLineTo:
			n = 2
		case content.OpCurveTo:
			n = 6
		default:
			n = 4
		}
		v, ok := numbers(args, n)
		if !ok || pb.cur == nil {
			return
		}
		if pb.closed {
			s := &subpath{
				implicit: true,
				pts:      []vec.Vec2{pb.cur.start},
				start:    pb.cur.start, sx: pb.cur.sx, sy: pb.cur.sy,
			}
			pb.subs = append(pb.subs, s)
			pb.cur, pb.closed = s, false
		}
		pb.cur.ops = append(pb.cur.ops, op)
		for i := range n / 2 {
			pb.cur.pts = append(pb.cur.pts, pt(v, i))
		}

	case content.OpClosePath:
		if pb.cur == nil {
			return
		}
		pb.cur.ops = append(pb.cur.ops, op)
		pb.closed = true
	}
}

// paint classifies the subpaths of the buffered path at its painting
// operator, and records in p how the operators are rewritten.  The buffer
// is emptied.
func (wk *walker) paint(p *plan, pb *pathBuf, gs *graphics.State, idx int, name content.OpName) error {
	subs, clips := pb.subs, pb.clips
	*pb = pathBuf{}
	if len(subs) == 0 {
		return nil
	}

	pp, _ := content.PathPainting(name)
	userToPage := gs.CTM.Mul(wk.base)
	reach := 0.0
	if pp.Stroke {
		reach = strokeReach(gs.LineWidth, gs.MiterLimit, gs.LineJoin, gs.LineCap, userToPage)
	}

	// the tests are linear in the number of points
	npts := 0
	for _, s := range subs {
		npts += len(s.pts)
	}
	if err := wk.chargeTests(npts); err != nil {
		return err
	}
	deleted := make([]bool, len(subs))
	all := true
	for i, s := range subs {
		pts := transformPoints(s.pts, userToPage)
		for _, h := range wk.hulls {
			if inside(pts, h, reach) {
				deleted[i] = true
				break
			}
		}
		if !deleted[i] {
			// TODO(voss): subtract regions geometrically
			all = false
		}
	}

	if err := wk.paintPattern(p, subs, pp, gs, userToPage, reach); err != nil {
		return err
	}

	removed := false
	for i, s := range subs {
		if !deleted[i] {
			if s.implicit && i > 0 && deleted[i-1] {
				// the segment must keep its start point
				first := s.ops[0]
				p.ops[first.idx] = &opPlan{repl: []content.Operator{
					{Name: content.OpMoveTo, Args: []pdf.Object{s.sx, s.sy}},
					{Name: first.name, Args: first.args},
				}}
			}
			continue
		}
		removed = true
		for _, op := range s.ops {
			p.ops[op.idx] = &opPlan{drop: true}
		}
		p.lose()
	}
	if !removed || !all {
		return nil
	}

	// nothing is left of the path
	if len(clips) == 0 {
		p.ops[idx] = &opPlan{drop: true}
		return nil
	}
	for _, c := range clips {
		p.ops[c.idx] = &opPlan{drop: true}
	}
	zero := pdf.Integer(0)
	p.ops[idx] = &opPlan{repl: []content.Operator{
		{Name: content.OpRectangle, Args: []pdf.Object{zero, zero, zero, zero}},
		{Name: clips[len(clips)-1].name},
		{Name: content.OpEndPath},
	}}
	return nil
}

// paintPattern rewrites the tiling patterns which the painted path uses,
// if the path meets a region, including the paths which are removed.
func (wk *walker) paintPattern(p *plan, subs []*subpath, pp content.PathPaint, gs *graphics.State, userToPage matrix.Matrix, reach float64) error {
	fill := pp.Fill && wk.pat.fill.name != "" && color.IsPattern(gs.FillColor.ColorSpace())
	stroke := pp.Stroke && wk.pat.stroke.name != "" && color.IsPattern(gs.StrokeColor.ColorSpace())
	if !fill && !stroke {
		return nil
	}
	var pts []vec.Vec2
	for _, s := range subs {
		pts = append(pts, transformPoints(s.pts, userToPage)...)
	}
	hull := convexHull(pts)
	// the hull, and the grown hull which has up to eight points for each
	// point of the hull, are tested against every region
	if err := wk.chargeOp(int64(len(pts))); err != nil {
		return err
	}
	if err := wk.chargeTests(9 * len(hull)); err != nil {
		return err
	}
	meets := func(h poly) bool {
		for _, region := range wk.hulls {
			if intersects(h, region, touchTol) {
				return true
			}
		}
		return false
	}
	if fill && meets(hull) {
		if err := wk.hitPattern(p, wk.pat.fill); err != nil {
			return err
		}
	}
	if stroke && meets(hull.grow(reach)) {
		return wk.hitPattern(p, wk.pat.stroke)
	}
	return nil
}
