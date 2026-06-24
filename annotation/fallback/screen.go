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

package fallback

import (
	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/geom/vec"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/content/builder"
	"seehuhn.de/go/pdf/graphics/form"
)

// addScreenAppearance generates a fallback appearance for a screen annotation.
// When the appearance characteristics dictionary supplies an icon, that icon
// form is drawn stretched to fill the Rect (the icon is the spec's
// appearance-generation input, Table 190); otherwise a generic media
// placeholder is drawn (see [drawMediaPlaceholder]).
//
// This generator is for writers that must supply the appearance PDF 2.0
// requires.  Readers must not call it for a screen annotation lacking an
// appearance stream: §12.5.6.18 says such an annotation has no default visual
// appearance.
func (s *Style) addScreenAppearance(a *annotation.Screen) (*form.Form, error) {
	rect := a.Rect
	w := rect.Dx()
	h := rect.Dy()
	if w <= 0 || h <= 0 {
		return &form.Form{Content: nil, Res: &content.Resources{}, BBox: rect}, nil
	}

	b := builder.New(content.Form, nil, s.version)
	b.SetExtGState(s.reset)
	mediaAlpha(b, a.StrokingTransparency, a.NonStrokingTransparency)

	if a.Style != nil && a.Style.Icon != nil {
		b.PushGraphicsState()
		b.Transform(fitToRect(a.Style.Icon, rect))
		b.DrawXObject(a.Style.Icon)
		b.PopGraphicsState()
	} else {
		drawMediaPlaceholder(b, rect)
	}

	return harvest(b, rect)
}

// fitToRect returns the transform that maps the form's bounding box, after the
// form's own Matrix, onto rect (anamorphic, filling the whole rect).  The form
// mechanism applies the form Matrix itself, so the returned matrix maps the
// already-transformed box.
func fitToRect(f *form.Form, rect pdf.Rectangle) matrix.Matrix {
	fm := f.Matrix
	if fm == (matrix.Matrix{}) {
		fm = matrix.Identity
	}

	bbox := f.BBox
	ll := fm.Apply(vec.Vec2{X: bbox.LLx, Y: bbox.LLy})
	lr := fm.Apply(vec.Vec2{X: bbox.URx, Y: bbox.LLy})
	ul := fm.Apply(vec.Vec2{X: bbox.LLx, Y: bbox.URy})
	ur := fm.Apply(vec.Vec2{X: bbox.URx, Y: bbox.URy})
	tbLLx := min(ll.X, lr.X, ul.X, ur.X)
	tbLLy := min(ll.Y, lr.Y, ul.Y, ur.Y)
	tbURx := max(ll.X, lr.X, ul.X, ur.X)
	tbURy := max(ll.Y, lr.Y, ul.Y, ur.Y)
	tbW := tbURx - tbLLx
	tbH := tbURy - tbLLy
	if tbW == 0 || tbH == 0 {
		return matrix.Identity
	}

	m := matrix.Translate(-tbLLx, -tbLLy)
	m = m.Mul(matrix.Scale(rect.Dx()/tbW, rect.Dy()/tbH))
	m = m.Mul(matrix.Translate(rect.LLx, rect.LLy))
	return m
}
