// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2024  Jochen Voss <voss@seehuhn.de>
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

package ghostscript

import (
	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/geom/vec"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font/type3"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/content/builder"
)

// FindTextPos finds the approximate text position in user coordinates,
// after the f function has been called to draw some text.
// When f is called, the content stream is at the beginning of a new text object.
// The function should not end the text object.
func FindTextPos(v pdf.Version, paper *pdf.Rectangle, setup func(page *document.Page) error) (x, y float64, err error) {
	if !isAvailable() {
		return 0, 0, ErrNoGhostscript
	}

	r, err := newGSRenderer(paper, v)
	if err != nil {
		return 0, 0, err
	}

	r.Page.TextBegin()
	err = setup(r.Page)
	if err != nil {
		return 0, 0, err
	}

	// Make a new font to draw a red marker at the current position.
	// We adjust the marker to compensate for variations in the font metrics
	// and font size, so that the marker is always the same size.
	markerFontSize := 10.0
	param := r.Page.State.GState
	M := matrix.Matrix{markerFontSize * param.TextHorizontalScaling, 0, 0, markerFontSize, 0, param.TextRise}
	M = M.Mul(param.TextMatrix)
	M = M.Mul(param.CTM)
	c := M.Apply(vec.Vec2{})

	// Build the marker glyph content stream
	b := builder.New(content.Glyph, nil)
	b.Type3ColoredGlyph(0, 0) // d0: colored glyph with zero width
	b.SetFillColor(color.DeviceRGB{1.0, 0, 0})
	A := M.Inv()
	pq := A.Apply(vec.Vec2{X: c.X - 1, Y: c.Y - 1})
	b.MoveTo(pq.X*1000, pq.Y*1000)
	pq = A.Apply(vec.Vec2{X: c.X + 1, Y: c.Y - 1})
	b.LineTo(pq.X*1000, pq.Y*1000)
	pq = A.Apply(vec.Vec2{X: c.X + 1, Y: c.Y + 1})
	b.LineTo(pq.X*1000, pq.Y*1000)
	pq = A.Apply(vec.Vec2{X: c.X - 1, Y: c.Y + 1})
	b.LineTo(pq.X*1000, pq.Y*1000)
	b.Fill()

	stream, err := b.Harvest()
	if err != nil {
		return 0, 0, err
	}

	markerFont := &type3.Font{
		Glyphs: []*type3.Glyph{
			{}, // .notdef
			{
				Name:    "x",
				Content: stream,
			},
		},
		FontMatrix: matrix.Matrix{0.001, 0, 0, 0.001, 0, 0},
	}
	X, err := markerFont.New()
	if err != nil {
		return 0, 0, err
	}

	r.Page.TextSetFont(X, 10)
	r.Page.TextShow("x")
	r.Page.TextEnd()

	img, err := r.Close()
	if err != nil {
		return 0, 0, err
	}

	var xSum, ySum float64
	var weight float64
	bounds := img.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, a := img.At(x, y).RGBA()
			if r > g && r > b && a > 0 {
				xSum += float64(x)
				ySum += float64(y)
				weight++
			}
		}
	}
	if weight == 0 {
		panic("marker not found")
	}
	xPix := xSum / weight
	yPix := ySum / weight

	// xPix = b.Min.X-0.5 correspond to xUser=paper.LLx
	// xPix = b.Max.X-0.5 correspond to xUser=paper.URx
	xUser := paper.LLx + (xPix-float64(bounds.Min.X)+0.5)*(paper.URx-paper.LLx)/float64(bounds.Max.X-bounds.Min.X)
	yUser := paper.LLy + (float64(bounds.Max.Y)-yPix-0.5)*(paper.URy-paper.LLy)/float64(bounds.Max.Y-bounds.Min.Y)

	return xUser, yUser, nil
}
