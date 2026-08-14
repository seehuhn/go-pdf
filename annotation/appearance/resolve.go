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

package appearance

import (
	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/geom/vec"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics/form"
)

// Kind selects an entry of an annotation appearance dictionary.
type Kind int

const (
	Normal   Kind = iota // shown while the annotation is not interacting with the user
	RollOver             // shown while the pointer rests inside the active area
	Down                 // shown while the pointer button is held down
)

// Resolve returns the appearance form of the given kind, honouring the
// appearance state.  Where the appearance is given per state, the result is nil
// unless state names one of them, so that an annotation with no matching state
// shows nothing.  Resolve is nil-safe: a nil dictionary yields nil.
func (d *Dict) Resolve(state pdf.Name, kind Kind) *form.Form {
	if d == nil {
		return nil
	}
	var single *form.Form
	var byState map[pdf.Name]*form.Form
	switch kind {
	case Normal:
		single, byState = d.Normal, d.NormalMap
	case RollOver:
		single, byState = d.RollOver, d.RollOverMap
	case Down:
		single, byState = d.Down, d.DownMap
	}
	if single == nil && len(byState) == 0 {
		single, byState = d.Normal, d.NormalMap
	}
	if state != "" && len(byState) > 0 {
		return byState[state]
	}
	return single
}

// ToRect returns the matrix mapping an appearance form's coordinates into the
// annotation rectangle: the form matrix is applied to the form's bounding box,
// and the result is scaled and translated to align with rect.  The second
// return value is false when the appearance has no content or its transformed
// bounding box is degenerate, in which case there is nothing to draw.
//
// This is the matrix for drawing the form's content stream directly.  A caller
// which instead invokes the form as a form XObject needs [XObjectToRect].
func ToRect(ap *form.Form, rect pdf.Rectangle) (matrix.Matrix, bool) {
	formMatrix, a, ok := rectMapping(ap, rect)
	if !ok {
		return matrix.Matrix{}, false
	}
	return formMatrix.Mul(a), true
}

// XObjectToRect returns the matrix for placing an appearance form in the
// annotation rectangle when the form is drawn as a form XObject, with the
// Do operator.  It leaves out the form matrix, which Do applies of its own
// accord; [ToRect] here would apply that matrix twice.  The second return
// value is false in the same cases as for [ToRect].
func XObjectToRect(ap *form.Form, rect pdf.Rectangle) (matrix.Matrix, bool) {
	_, a, ok := rectMapping(ap, rect)
	if !ok {
		return matrix.Matrix{}, false
	}
	return a, true
}

// rectMapping splits the appearance-to-rectangle mapping of §12.5.5 into the
// form matrix and the matrix A which carries the transformed appearance box
// onto rect.  Their product maps form coordinates into the rectangle.
func rectMapping(ap *form.Form, rect pdf.Rectangle) (formMatrix, a matrix.Matrix, ok bool) {
	if ap == nil || ap.Content == nil {
		return matrix.Matrix{}, matrix.Matrix{}, false
	}

	formMatrix = ap.Matrix
	if formMatrix == (matrix.Matrix{}) {
		formMatrix = matrix.Identity
	}

	// transform BBox through the form matrix to get the transformed appearance box
	bbox := ap.BBox
	ll := formMatrix.Apply(vec.Vec2{X: bbox.LLx, Y: bbox.LLy})
	lr := formMatrix.Apply(vec.Vec2{X: bbox.URx, Y: bbox.LLy})
	ul := formMatrix.Apply(vec.Vec2{X: bbox.LLx, Y: bbox.URy})
	ur := formMatrix.Apply(vec.Vec2{X: bbox.URx, Y: bbox.URy})
	tbLLx := min(ll.X, lr.X, ul.X, ur.X)
	tbLLy := min(ll.Y, lr.Y, ul.Y, ur.Y)
	tbURx := max(ll.X, lr.X, ul.X, ur.X)
	tbURy := max(ll.Y, lr.Y, ul.Y, ur.Y)
	tbW := tbURx - tbLLx
	tbH := tbURy - tbLLy
	if tbW == 0 || tbH == 0 {
		return matrix.Matrix{}, matrix.Matrix{}, false
	}

	// matrix A maps the transformed appearance box to the annotation rectangle
	sx := (rect.URx - rect.LLx) / tbW
	sy := (rect.URy - rect.LLy) / tbH
	tx := rect.LLx - tbLLx*sx
	ty := rect.LLy - tbLLy*sy

	return formMatrix, matrix.Matrix{sx, 0, 0, sy, tx, ty}, true
}
