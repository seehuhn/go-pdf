// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package content

import (
	"bytes"
	"io"
	"iter"

	"seehuhn.de/go/geom/rect"

	"seehuhn.de/go/pdf"
)

// GlyphMetrics holds what the d0 or d1 operator of a Type 3 glyph
// procedure declares.
type GlyphMetrics struct {
	// Width is the horizontal displacement wx, in glyph space.
	Width float64

	// BBox is the glyph bounding box, in glyph space.
	// It is only set for d1.
	BBox rect.Rect

	// Uncolored is true for d1 and false for d0.
	Uncolored bool
}

// ParseGlyphMetrics interprets one operator as the d0 or d1 operator of a
// Type 3 glyph procedure.  The result is false if the operator is not d0 or
// d1, if it has too few numeric arguments (2 for d0, 6 for d1), or if the
// vertical displacement wy is not zero.
func ParseGlyphMetrics(name OpName, args []pdf.Object) (GlyphMetrics, bool) {
	m, ok := parseGlyphMetrics(name, args)
	if !ok || m.wy != 0 {
		return GlyphMetrics{}, false
	}
	return m.GlyphMetrics, true
}

// rawGlyphMetrics is a d0 or d1 operator with its vertical displacement.
type rawGlyphMetrics struct {
	GlyphMetrics
	wy float64
}

// parseGlyphMetrics is like [ParseGlyphMetrics], but accepts any wy.
func parseGlyphMetrics(name OpName, args []pdf.Object) (rawGlyphMetrics, bool) {
	var n int
	switch name {
	case OpType3ColoredGlyph:
		n = 2
	case OpType3UncoloredGlyph:
		n = 6
	default:
		return rawGlyphMetrics{}, false
	}

	var v [6]float64
	for i := range n {
		x, ok := getNumber(args, i)
		if !ok {
			return rawGlyphMetrics{}, false
		}
		v[i] = x
	}

	m := rawGlyphMetrics{Width: v[0], wy: v[1]}
	if name == OpType3UncoloredGlyph {
		m.Uncolored = true
		m.BBox = rect.Rect{LLx: v[2], LLy: v[3], URx: v[4], URy: v[5]}
	}
	return m, true
}

// ReadGlyphMetrics returns the metrics declared at the start of the glyph
// procedure s.  Leading comments are skipped.  The result is false if the
// first operator is not a well-formed d0 or d1 operator, or if s is nil or
// empty.
func ReadGlyphMetrics(s Stream) (GlyphMetrics, bool) {
	if s == nil {
		return GlyphMetrics{}, false
	}
	for name, args := range s.NewIter().All() {
		if name == OpRawContent {
			continue
		}
		return ParseGlyphMetrics(name, args)
	}
	return GlyphMetrics{}, false
}

// RepairGlyph returns a stream which starts with a well-formed d0 or d1
// operator and contains no other d0 or d1 operator, as a Type 3 glyph
// procedure must.
//
// The leading operator is the first d0 or d1 operator of s which has enough
// numeric arguments, with wy set to 0 and any further arguments left out.
// If s has no such operator, the result starts with "0 0 d0"; the glyph
// advance is given by the Widths array of the font in any case.  All other
// d0 and d1 operators of s are left out.
//
// RepairGlyph reads s up to the operator it chooses, or all of s if there is
// none.  The result reads s again each time it is iterated.
func RepairGlyph(s Stream) Stream {
	switch s := s.(type) {
	case nil:
		return &repairedGlyph{inner: &Operators{}, metrics: defaultGlyphMetrics()}
	case *repairedGlyph:
		return s
	}

	g := &repairedGlyph{inner: s, metrics: defaultGlyphMetrics()}
	for name, args := range s.NewIter().All() {
		if _, ok := parseGlyphMetrics(name, args); ok {
			n := 2
			if name == OpType3UncoloredGlyph {
				n = 6
			}
			a := make([]pdf.Object, n)
			copy(a, args)
			a[1] = pdf.Integer(0)
			g.metrics = Operator{Name: name, Args: a}
			break
		}
	}
	return g
}

// defaultGlyphMetrics returns the operator "0 0 d0".
func defaultGlyphMetrics() Operator {
	return Operator{Name: OpType3ColoredGlyph, Args: []pdf.Object{pdf.Integer(0), pdf.Integer(0)}}
}

// repairedGlyph is the [Stream] returned by [RepairGlyph].
type repairedGlyph struct {
	inner   Stream
	metrics Operator
}

var _ Stream = (*repairedGlyph)(nil)

func (g *repairedGlyph) NewIter() Iter {
	return &repairedIter{g: g}
}

// RawBytes formats the operators lazily, in PDF 1.2 and later syntax.
func (g *repairedGlyph) RawBytes() (io.ReadCloser, error) {
	it := g.NewIter()
	next, stop := iter.Pull2(it.All())
	return &iterReader{it: it, next: next, stop: stop}, nil
}

type repairedIter struct {
	g     *repairedGlyph
	inner Iter
}

func (ri *repairedIter) All() iter.Seq2[OpName, []pdf.Object] {
	return func(yield func(OpName, []pdf.Object) bool) {
		if !yield(ri.g.metrics.Name, ri.g.metrics.Args) {
			return
		}
		ri.inner = ri.g.inner.NewIter()
		for name, args := range ri.inner.All() {
			if name == OpType3ColoredGlyph || name == OpType3UncoloredGlyph {
				continue
			}
			if !yield(name, args) {
				return
			}
		}
	}
}

func (ri *repairedIter) Err() error {
	if ri.inner == nil {
		return nil
	}
	return ri.inner.Err()
}

// iterReader formats the operators of a pulled iterator one at a time.
type iterReader struct {
	it      Iter
	next    func() (OpName, []pdf.Object, bool)
	stop    func()
	pending bytes.Buffer
	done    bool
}

func (r *iterReader) Read(p []byte) (n int, err error) {
	for n < len(p) {
		if r.pending.Len() > 0 {
			k, _ := r.pending.Read(p[n:])
			n += k
			continue
		}
		if r.done {
			if n > 0 {
				return n, nil
			}
			if err := r.it.Err(); err != nil {
				return 0, err
			}
			return 0, io.EOF
		}
		name, args, ok := r.next()
		if !ok {
			r.done = true
			continue
		}
		if err := (Operator{Name: name, Args: args}).Format(&r.pending, 0); err != nil {
			return n, err
		}
	}
	return n, nil
}

func (r *iterReader) Close() error {
	r.stop()
	return nil
}
