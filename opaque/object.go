// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2026  Jochen Voss <voss@seehuhn.de>
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package opaque

import (
	"errors"

	"seehuhn.de/go/pdf"
)

// Object holds an arbitrary PDF value that can be embedded into PDF files.
//
// Object implements [pdf.Embedder].
type Object struct {
	src *pdf.Extractor // nil for in-memory values
	obj pdf.Object
}

// Extract wraps a PDF value read through x.  References inside obj refer to
// objects in the file x reads from.  x must remain usable for as long as the
// Object is used.
func Extract(x *pdf.Extractor, obj pdf.Object) *Object {
	return &Object{src: x, obj: obj}
}

// Direct wraps an in-memory PDF value.  obj must contain no indirect objects;
// on [Object.Embed] the value is written verbatim without reference
// translation.
func Direct(obj pdf.Object) *Object {
	return &Object{obj: obj}
}

// Embed implements [pdf.Embedder].
//
// For an Object built via [Extract], references in the wrapped value
// are translated to the destination file via [pdf.Copier].  For an
// Object built via [Direct], the value is written verbatim.
func (o *Object) Embed(e *pdf.EmbedHelper) (pdf.Native, error) {
	native := o.obj.AsPDF(e.Out().GetOptions())
	if o.src == nil {
		return native, nil
	}
	return e.CopierFrom(o.src).Copy(native)
}

// Equal reports whether o and other denote the same PDF value.
// Streams and placeholders are compared by pointer identity.
func (o *Object) Equal(other *Object) bool {
	if o == nil || other == nil {
		return o == other
	}

	// Two values are equal if they unfold to the same tree of dictionaries,
	// arrays and scalars, whether the parts are direct or indirect objects.
	// The unfolding can be infinite, so the comparison uses the algorithm
	// of Hopcroft and Karp (1971) for the equivalence of deterministic
	// automata: each dictionary or array met on either side is a state, and
	// a pair of states is expanded only when the two are not yet known to be
	// equivalent.  Every expansion merges two classes, so the work is linear
	// in the size of the two values.
	var next int
	a := newWalker(o.src, &next)
	b := newWalker(other.src, &next)
	var classes unionFind

	// Each task compares one pair of values.  Both values are found under
	// the same key of their parents, and the parent positions identify
	// where they sit inside their objects.
	type task struct {
		a, b             pdf.Object
		parentA, parentB int
		key              step
	}
	todo := []task{{a: o.obj, b: other.obj, parentA: -1, parentB: -1}}
	for len(todo) > 0 {
		t := todo[len(todo)-1]
		todo = todo[:len(todo)-1]

		valA, posA, err := a.visit(t.a, t.parentA, t.key)
		if err != nil {
			return false
		}
		valB, posB, err := b.visit(t.b, t.parentB, t.key)
		if err != nil {
			return false
		}

		switch va := valA.(type) {
		case pdf.Dict:
			vb, ok := valB.(pdf.Dict)
			if !ok || (va == nil) != (vb == nil) {
				return false
			}
			if !classes.union(posA, posB) {
				continue
			}
			for k, val := range va {
				todo = append(todo, task{val, vb[k], posA, posB, step{key: k}})
			}
			for k, val := range vb {
				if _, exists := va[k]; !exists {
					todo = append(todo, task{nil, val, posA, posB, step{key: k}})
				}
			}
		case pdf.Array:
			vb, ok := valB.(pdf.Array)
			if !ok || (va == nil) != (vb == nil) || len(va) != len(vb) {
				return false
			}
			if !classes.union(posA, posB) {
				continue
			}
			for i := range va {
				todo = append(todo, task{va[i], vb[i], posA, posB, step{idx: i}})
			}
		default:
			if !pdf.Equal(valA, valB) {
				return false
			}
		}
	}
	return true
}

// unionFind partitions position numbers into classes of values which are
// assumed to be equal.  Positions not yet mentioned form classes of their
// own.
type unionFind []int

// find returns the representative of the class containing x.
func (u *unionFind) find(x int) int {
	for len(*u) <= x {
		*u = append(*u, len(*u))
	}
	p := *u
	for p[x] != x {
		p[x] = p[p[x]]
		x = p[x]
	}
	return x
}

// union merges the classes of x and y, and reports whether they were
// distinct before.
func (u *unionFind) union(x, y int) bool {
	x, y = u.find(x), u.find(y)
	if x == y {
		return false
	}
	(*u)[x] = y
	return true
}

// step identifies a child of a dictionary (by key) or of an array (by index),
// together with the position number of the parent.  A parent is either a
// dictionary or an array, so key and idx are never both in use.
type step struct {
	parent int
	key    pdf.Name
	idx    int
}

// walker follows the references inside one of the two values compared by
// [Object.Equal], and numbers the dictionaries and arrays it meets.  The
// two walkers of a comparison draw their numbers from a shared counter.
//
// A dictionary or array reached through a reference is numbered by that
// reference, so that each visit of the reference gives the same number.
// A direct dictionary or array is numbered by its parent and its position
// in the parent.  Each number thus stands for a fixed value, and a pair of
// numbers which comes up again during the comparison can be skipped.
type walker struct {
	c     pdf.Cursor
	src   bool
	refs  map[pdf.Reference]int
	steps map[step]int
	next  *int
}

func newWalker(x *pdf.Extractor, next *int) *walker {
	return &walker{
		c:     pdf.CursorAt(x, nil),
		src:   x != nil,
		refs:  make(map[pdf.Reference]int),
		steps: make(map[step]int),
		next:  next,
	}
}

// visit resolves obj, found at the given step from the parent position, and
// returns the resulting value together with its position number.  The
// position is only meaningful for dictionaries and arrays.
func (w *walker) visit(obj pdf.Object, parent int, s step) (pdf.Native, int, error) {
	if obj == nil {
		return nil, -1, nil
	}
	native := obj.AsPDF(0)
	if ref, ok := native.(pdf.Reference); ok && w.src {
		// The cursor carries no path, so cycle detection covers only the
		// chain of references itself.  A looping or over-long chain is
		// malformed, and reads as null like any other unreadable object.
		resolved, err := w.c.Resolve(ref)
		if pdf.IsMalformed(err) {
			return nil, -1, nil
		} else if err != nil {
			return nil, -1, err
		}
		pos, ok := w.refs[ref]
		if !ok {
			pos = w.newPos()
			w.refs[ref] = pos
		}
		return resolved, pos, nil
	}

	switch native.(type) {
	case pdf.Dict, pdf.Array:
		s.parent = parent
		pos, ok := w.steps[s]
		if !ok {
			pos = w.newPos()
			w.steps[s] = pos
		}
		return native, pos, nil
	default:
		return native, -1, nil
	}
}

func (w *walker) newPos() int {
	pos := *w.next
	*w.next++
	return pos
}

// AsDirectDict checks whether the wrapped value is a PDF dictionary,
// which is a direct object and does not contain any indirect objects.
// If the wrapped value has this form, the dictionary is returned.
// Otherwise, nil is returned.
func (o *Object) AsDirectDict() pdf.Dict {
	dict, ok := o.obj.(pdf.Dict)
	if !ok || !pdf.IsDirect(dict) {
		return nil
	}
	return dict
}

// As decodes the value wrapped by o using extract, with path for cycle
// detection.  It returns an error if o was created by [Direct].
func (o *Object) As[T any](
	path *pdf.CycleCheck,
	extract func(pdf.Cursor, pdf.Object, bool) (T, error),
) (T, error) {
	if o.src == nil {
		var zero T
		return zero, errors.New("opaque: Object.As requires an Object built via Extract")
	}
	return pdf.CursorAt(o.src, path).Decode(o.obj, extract)
}
