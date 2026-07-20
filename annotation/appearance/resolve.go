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
	Normal Kind = iota
	RollOver
	Down
)

// Resolve returns the appearance form of the given kind, honouring the
// appearance state.  It is nil-safe: a nil dictionary yields nil.
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
	if state != "" && byState != nil {
		return byState[state]
	}
	return single
}

// AppearanceToRect returns the matrix mapping an appearance form's coordinates
// into the annotation rectangle, following the algorithm of §12.5.5: the
// form matrix is applied to the form's bounding box, and the result is scaled
// and translated to align with rect.  The second return value is false when
// the appearance has no content or its transformed bounding box is degenerate,
// in which case there is nothing to draw.
func AppearanceToRect(ap *form.Form, rect pdf.Rectangle) (matrix.Matrix, bool) {
	if ap == nil || ap.Content == nil {
		return matrix.Matrix{}, false
	}

	formMatrix := ap.Matrix
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
		return matrix.Matrix{}, false
	}

	// matrix A maps the transformed appearance box to the annotation rectangle
	sx := (rect.URx - rect.LLx) / tbW
	sy := (rect.URy - rect.LLy) / tbH
	tx := rect.LLx - tbLLx*sx
	ty := rect.LLy - tbLLy*sy
	a2 := matrix.Matrix{sx, 0, 0, sy, tx, ty}

	return formMatrix.Mul(a2), true
}
