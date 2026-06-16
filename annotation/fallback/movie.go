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
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/content/builder"
	"seehuhn.de/go/pdf/graphics/extgstate"
	"seehuhn.de/go/pdf/graphics/form"
	"seehuhn.de/go/pdf/movie"
)

// addMovieAppearance generates a fallback appearance for a movie annotation.
// When the movie dictionary supplies an embedded poster image, that image is
// drawn stretched to fill the Rect; otherwise a generic media placeholder is
// drawn (see [drawMediaPlaceholder]).
func (s *Style) addMovieAppearance(a *annotation.Movie) (*form.Form, error) {
	rect := a.Rect
	w := rect.Dx()
	h := rect.Dy()
	if w <= 0 || h <= 0 {
		return &form.Form{Content: nil, Res: &content.Resources{}, BBox: rect}, nil
	}

	b := builder.New(content.Form, nil, s.version)
	b.SetExtGState(s.reset)
	mediaAlpha(b, a.StrokingTransparency, a.NonStrokingTransparency)

	// PosterFromMovieFile lives inside an undecoded movie container, so we can
	// only draw a poster that is an embedded image XObject.
	if a.Movie != nil && a.Movie.Poster != nil && a.Movie.Poster != movie.PosterFromMovieFile {
		b.PushGraphicsState()
		b.Transform(matrix.Matrix{w, 0, 0, h, rect.LLx, rect.LLy})
		b.DrawXObject(a.Movie.Poster)
		b.PopGraphicsState()
	} else {
		drawMediaPlaceholder(b, rect)
	}

	return harvest(b, rect)
}

// mediaAlpha applies the annotation's stroking and non-stroking transparency
// to the builder's graphics state.  The transparency fields are zero for fully
// opaque, so a non-zero value sets the corresponding alpha.
func mediaAlpha(b *builder.Builder, strokeTransparency, fillTransparency float64) {
	if strokeTransparency == 0 && fillTransparency == 0 {
		return
	}
	b.SetExtGState(&extgstate.ExtGState{
		Set:         graphics.StateStrokeAlpha | graphics.StateFillAlpha,
		StrokeAlpha: 1 - strokeTransparency,
		FillAlpha:   1 - fillTransparency,
		SingleUse:   true,
	})
}

// drawMediaPlaceholder fills rect with a light chrome panel and a centred,
// right-pointing play triangle.  It is the shared fallback used by movie and
// screen annotations that have no poster or icon to display.
func drawMediaPlaceholder(b *builder.Builder, rect pdf.Rectangle) {
	w := rect.Dx()
	h := rect.Dy()

	// chrome panel
	b.SetFillColor(quireSlate1)
	b.Rectangle(rect.LLx, rect.LLy, w, h)
	b.Fill()

	// hairline border, inset by half the line width so it stays inside the Rect
	const lw = 0.5
	b.SetLineWidth(lw)
	b.SetStrokeColor(quireSlate3)
	b.Rectangle(rect.LLx+lw/2, rect.LLy+lw/2, w-lw, h-lw)
	b.Stroke()

	// centred right-pointing play triangle (symmetric in y)
	size := min(w, h) * 0.16
	cx := rect.LLx + w/2
	cy := rect.LLy + h/2
	b.SetFillColor(quireInk3)
	b.MoveTo(cx-0.5*size, cy-0.7*size)
	b.LineTo(cx-0.5*size, cy+0.7*size)
	b.LineTo(cx+0.8*size, cy)
	b.ClosePath()
	b.Fill()
}
