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
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/color"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font/type3"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/postscript/funit"
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

	r.Page.TextStart()
	err = setup(r.Page)
	if err != nil {
		return 0, 0, err
	}

	// Make a new font to draw a red marker at the current position.
	// We adjust the marker to compensate for variations in the font metrics
	// and font size, so that the marker is always the same size.
	markerFontSize := 10.0
	s := r.Page.State
	M := graphics.Matrix{markerFontSize * s.TextHorizontalScaling, 0, 0, markerFontSize, 0, s.TextRise}
	M = M.Mul(s.TextMatrix)
	M = M.Mul(s.CTM)
	xc, yc := M.Apply(0, 0)

	XFont := type3.New(1000)
	g, err := XFont.AddGlyph("x", 0, funit.Rect16{LLx: -100, LLy: 1000, URx: 100, URy: 100}, false)
	if err != nil {
		return 0, 0, err
	}
	g.SetFillColor(color.RGB(1.0, 0, 0))
	A := M.Inv()
	p, q := A.Apply(xc-1, yc-1)
	g.MoveTo(p*1000, q*1000)
	p, q = A.Apply(xc+1, yc-1)
	g.LineTo(p*1000, q*1000)
	p, q = A.Apply(xc+1, yc+1)
	g.LineTo(p*1000, q*1000)
	p, q = A.Apply(xc-1, yc+1)
	g.LineTo(p*1000, q*1000)
	g.Fill()
	err = g.Close()
	if err != nil {
		return 0, 0, err
	}
	X, err := XFont.Embed(r.Page.Out, nil)
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
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
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
	xUser := paper.LLx + (xPix-float64(b.Min.X)+0.5)*(paper.URx-paper.LLx)/float64(b.Max.X-b.Min.X)
	yUser := paper.LLy + (float64(b.Max.Y)-yPix-0.5)*(paper.URy-paper.LLy)/float64(b.Max.Y-b.Min.Y)

	return xUser, yUser, nil
}
