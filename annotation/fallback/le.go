// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2025  Jochen Voss <voss@seehuhn.de>
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
	"math"

	"seehuhn.de/go/geom/linalg"
	"seehuhn.de/go/geom/vec"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
)

type LineEnding interface {
	// Enlarge enlarges an existing bounding box to include the line ending.
	Enlarge(*pdf.Rectangle)

	// Draw finishes the existing path by adding the line ending and
	// stroking the complete path.
	Draw(w *graphics.Writer, fillColor color.Color)
}

func NewLineEnding(atX, atY, fromX, fromY float64, lw float64, style annotation.LineEndingStyle) LineEnding {
	dX := atX - fromX
	dY := atY - fromY
	D := math.Sqrt(dX*dX + dY*dY)
	if D < 0.1 {
		return &tooShort{}
	}
	dX /= D
	dY /= D

	switch style {
	case annotation.LineEndingStyleSquare:
		return &square{
			atX: atX,
			atY: atY,
			dX:  dX,
			dY:  dY,
			lw:  lw,
		}
	case annotation.LineEndingStyleCircle:
		return &circle{
			atX: atX,
			atY: atY,
			dX:  dX,
			dY:  dY,
			lw:  lw,
		}
	case annotation.LineEndingStyleDiamond:
		return &diamond{
			atX: atX,
			atY: atY,
			dX:  dX,
			dY:  dY,
			lw:  lw,
		}
	case annotation.LineEndingStyleOpenArrow:
		return &arrow{
			atX: atX,
			atY: atY,
			dX:  dX,
			dY:  dY,
			lw:  lw,
		}
	case annotation.LineEndingStyleClosedArrow:
		return &arrow{
			atX:    atX,
			atY:    atY,
			dX:     dX,
			dY:     dY,
			lw:     lw,
			closed: true,
		}
	case annotation.LineEndingStyleButt:
		return &butt{
			atX: atX,
			atY: atY,
			dX:  dX,
			dY:  dY,
			lw:  lw,
		}
	case annotation.LineEndingStyleROpenArrow:
		return &arrow{
			atX:     atX,
			atY:     atY,
			dX:      dX,
			dY:      dY,
			lw:      lw,
			reverse: true,
		}
	case annotation.LineEndingStyleRClosedArrow:
		return &arrow{
			atX:     atX,
			atY:     atY,
			dX:      dX,
			dY:      dY,
			lw:      lw,
			closed:  true,
			reverse: true,
		}
	case annotation.LineEndingStyleSlash:
		return &slash{
			atX: atX,
			atY: atY,
			dX:  dX,
			dY:  dY,
			lw:  lw,
		}
	default: // annotation.LineEndingStyleNone
		return &none{
			atX: atX,
			atY: atY,
		}
	}
}

// ---------------------------------------------------------------------------

type none struct {
	atX, atY float64
}

func (le *none) Enlarge(b *pdf.Rectangle) {
	// do nothing, no line ending to expand the bounding box
}

func (le *none) Draw(w *graphics.Writer, col color.Color) {
	w.LineTo(le.atX, le.atY)
	w.Stroke()
}

// ---------------------------------------------------------------------------

type butt struct {
	atX, atY float64
	dX, dY   float64
	lw       float64
}

func (le *butt) Enlarge(bbox *pdf.Rectangle) {
	n := vec.Vec2{X: le.dX, Y: le.dY}.Normal()
	n.IMul(le.size() / 2)
	corners := []float64{
		le.atX + n.X + le.dX, le.atY + n.Y + le.dY,
		le.atX - n.X + le.dX, le.atY - n.Y + le.dY,
		le.atX + n.X - le.dX, le.atY + n.Y - le.dY,
		le.atX - n.X - le.dX, le.atY - n.Y - le.dY,
	}

	first := bbox.IsZero()
	for i := 0; i < len(corners); i += 2 {
		x := corners[i]
		y := corners[i+1]
		if first || x < bbox.LLx {
			bbox.LLx = x
		}
		if first || y < bbox.LLy {
			bbox.LLy = y
		}
		if first || x > bbox.URx {
			bbox.URx = x
		}
		if first || y > bbox.URy {
			bbox.URy = y
		}
		first = false
	}
}

func (le *butt) Draw(w *graphics.Writer, col color.Color) {
	w.LineTo(le.atX, le.atY)
	w.Stroke()

	n := vec.Vec2{X: le.dX, Y: le.dY}.Normal()
	n.IMul(le.size() / 2)

	w.MoveTo(le.atX+n.X, le.atY+n.Y)
	w.LineTo(le.atX-n.X, le.atY-n.Y)
	w.Stroke()
}

func (le *butt) size() float64 {
	return max(3, 6*le.lw)
}

// ---------------------------------------------------------------------------

type slash struct {
	atX, atY float64
	dX, dY   float64
	lw       float64
}

func (le *slash) Enlarge(bbox *pdf.Rectangle) {
	a := 0.5              // cos(60°)
	b := math.Sqrt(3) / 2 // sin(60°)
	n := vec.Vec2{X: a*le.dX - b*le.dY, Y: a*le.dY + b*le.dX}
	n.IMul(le.size() / 2)
	corners := []float64{
		le.atX + n.X + le.dX, le.atY + n.Y + le.dY,
		le.atX - n.X + le.dX, le.atY - n.Y + le.dY,
		le.atX + n.X - le.dX, le.atY + n.Y - le.dY,
		le.atX - n.X - le.dX, le.atY - n.Y - le.dY,
	}

	first := bbox.IsZero()
	for i := 0; i < len(corners); i += 2 {
		x := corners[i]
		y := corners[i+1]
		if first || x < bbox.LLx {
			bbox.LLx = x
		}
		if first || y < bbox.LLy {
			bbox.LLy = y
		}
		if first || x > bbox.URx {
			bbox.URx = x
		}
		if first || y > bbox.URy {
			bbox.URy = y
		}
		first = false
	}
}

func (le *slash) Draw(w *graphics.Writer, col color.Color) {
	w.LineTo(le.atX, le.atY)
	w.Stroke()

	a := 0.5              // cos(60°)
	b := math.Sqrt(3) / 2 // sin(60°)
	n := vec.Vec2{X: a*le.dX - b*le.dY, Y: a*le.dY + b*le.dX}
	n.IMul(le.size() / 2)

	w.MoveTo(le.atX+n.X, le.atY+n.Y)
	w.LineTo(le.atX-n.X, le.atY-n.Y)
	w.Stroke()
}

func (le *slash) size() float64 {
	return max(5, 10*le.lw)
}

// ---------------------------------------------------------------------------

type square struct {
	atX, atY float64
	dX, dY   float64
	lw       float64
}

func (le *square) Enlarge(bbox *pdf.Rectangle) {
	size := max(3, 6*le.lw)
	L := size + le.lw
	corners := []float64{
		le.atX + 0.5*L*le.dX - 0.5*L*le.dY, le.atY + 0.5*L*le.dY + 0.5*L*le.dX,
		le.atX - 0.5*L*le.dX - 0.5*L*le.dY, le.atY - 0.5*L*le.dY + 0.5*L*le.dX,
		le.atX - 0.5*L*le.dX + 0.5*L*le.dY, le.atY - 0.5*L*le.dY - 0.5*L*le.dX,
		le.atX + 0.5*L*le.dX + 0.5*L*le.dY, le.atY + 0.5*L*le.dY - 0.5*L*le.dX,
	}

	first := bbox.IsZero()
	for i := 0; i < len(corners); i += 2 {
		x := corners[i]
		y := corners[i+1]
		if first || x < bbox.LLx {
			bbox.LLx = x
		}
		if first || y < bbox.LLy {
			bbox.LLy = y
		}
		if first || x > bbox.URx {
			bbox.URx = x
		}
		if first || y > bbox.URy {
			bbox.URy = y
		}
		first = false
	}
}

func (le *square) Draw(w *graphics.Writer, col color.Color) {
	size := max(3, 6*le.lw)
	w.LineTo(le.atX-0.5*size*le.dX, le.atY-0.5*size*le.dY)
	w.Stroke()
	L := size
	if col != nil {
		w.SetFillColor(col)
		w.MoveTo(le.atX+0.5*L*le.dX-0.5*L*le.dY, le.atY+0.5*L*le.dY+0.5*L*le.dX)
		w.LineTo(le.atX-0.5*L*le.dX-0.5*L*le.dY, le.atY-0.5*L*le.dY+0.5*L*le.dX)
		w.LineTo(le.atX-0.5*L*le.dX+0.5*L*le.dY, le.atY-0.5*L*le.dY-0.5*L*le.dX)
		w.LineTo(le.atX+0.5*L*le.dX+0.5*L*le.dY, le.atY+0.5*L*le.dY-0.5*L*le.dX)
		w.CloseFillAndStroke()
	} else {
		w.MoveTo(le.atX+0.5*L*le.dX-0.5*L*le.dY, le.atY+0.5*L*le.dY+0.5*L*le.dX)
		w.LineTo(le.atX-0.5*L*le.dX-0.5*L*le.dY, le.atY-0.5*L*le.dY+0.5*L*le.dX)
		w.LineTo(le.atX-0.5*L*le.dX+0.5*L*le.dY, le.atY-0.5*L*le.dY-0.5*L*le.dX)
		w.LineTo(le.atX+0.5*L*le.dX+0.5*L*le.dY, le.atY+0.5*L*le.dY-0.5*L*le.dX)
		w.CloseAndStroke()
	}
}

// ---------------------------------------------------------------------------

type circle struct {
	atX, atY float64
	dX, dY   float64
	lw       float64
}

func (c *circle) Enlarge(b *pdf.Rectangle) {
	size := max(3, 6*c.lw)
	L := size + c.lw
	first := b.IsZero()
	xMin := c.atX - 0.5*L
	xMax := c.atX + 0.5*L
	yMin := c.atY - 0.5*L
	yMax := c.atY + 0.5*L

	if first || xMin < b.LLx {
		b.LLx = xMin
	}
	if first || yMin < b.LLy {
		b.LLy = yMin
	}
	if first || xMax > b.URx {
		b.URx = xMax
	}
	if first || yMax > b.URy {
		b.URy = yMax
	}
}

func (c *circle) Draw(w *graphics.Writer, col color.Color) {
	size := max(3, 6*c.lw)
	w.LineTo(c.atX-0.5*size*c.dX, c.atY-0.5*size*c.dY)
	w.Stroke()
	if col != nil {
		w.SetFillColor(col)
		w.Circle(c.atX, c.atY, 0.5*size)
		w.FillAndStroke()
	} else {
		w.Circle(c.atX, c.atY, 0.5*size)
		w.Stroke()
	}
}

// ---------------------------------------------------------------------------

type diamond struct {
	atX, atY float64
	dX, dY   float64
	lw       float64
}

func (d *diamond) Enlarge(b *pdf.Rectangle) {
	L := d.size() + d.lw
	corners := []float64{
		d.atX + 0.5*L*d.dX, d.atY + 0.5*L*d.dY,
		d.atX - 0.5*L*d.dY, d.atY + 0.5*L*d.dX,
		d.atX - 0.5*L*d.dX, d.atY - 0.5*L*d.dY,
		d.atX + 0.5*L*d.dY, d.atY - 0.5*L*d.dX,
	}

	first := b.IsZero()
	for i := 0; i < len(corners); i += 2 {
		x := corners[i]
		y := corners[i+1]
		if first || x < b.LLx {
			b.LLx = x
		}
		if first || y < b.LLy {
			b.LLy = y
		}
		if first || x > b.URx {
			b.URx = x
		}
		if first || y > b.URy {
			b.URy = y
		}
		first = false
	}
}

func (d *diamond) Draw(w *graphics.Writer, col color.Color) {
	size := d.size()
	w.LineTo(d.atX-0.5*size*d.dX, d.atY-0.5*size*d.dY)
	w.Stroke()
	L := size
	if col != nil {
		w.SetFillColor(col)
		w.MoveTo(d.atX+0.5*L*d.dX, d.atY+0.5*L*d.dY)
		w.LineTo(d.atX-0.5*L*d.dY, d.atY+0.5*L*d.dX)
		w.LineTo(d.atX-0.5*L*d.dX, d.atY-0.5*L*d.dY)
		w.LineTo(d.atX+0.5*L*d.dY, d.atY-0.5*L*d.dX)
		w.CloseFillAndStroke()
	} else {
		w.MoveTo(d.atX+0.5*L*d.dX, d.atY+0.5*L*d.dY)
		w.LineTo(d.atX-0.5*L*d.dY, d.atY+0.5*L*d.dX)
		w.LineTo(d.atX-0.5*L*d.dX, d.atY-0.5*L*d.dY)
		w.LineTo(d.atX+0.5*L*d.dY, d.atY-0.5*L*d.dX)
		w.CloseAndStroke()
	}
}

func (d *diamond) size() float64 {
	return max(4, 8*d.lw)
}

// ---------------------------------------------------------------------------

type arrow struct {
	atX, atY float64
	dX, dY   float64
	lw       float64
	closed   bool
	reverse  bool
}

func (a *arrow) Size() float64 {
	return max(4, 8*a.lw)
}

func (a *arrow) Enlarge(b *pdf.Rectangle) {
	xy := a.outerCorners()

	first := b.IsZero()
	for i := 0; i+1 < len(xy); i += 2 {
		x := xy[i]
		y := xy[i+1]
		if first || x < b.LLx {
			b.LLx = x
		}
		if first || y < b.LLy {
			b.LLy = y
		}
		if first || x > b.URx {
			b.URx = x
		}
		if first || y > b.URy {
			b.URy = y
		}
		first = false
	}
}

func (a *arrow) Draw(w *graphics.Writer, col color.Color) {
	tip, base1, base2 := a.corners()

	if a.reverse {
		w.LineTo(a.atX, a.atY)
	} else if a.closed {
		m := vec.Middle(base1, base2)
		w.LineTo(m.X, m.Y)
	} else {
		v, _ := linalg.Miter(base1, tip, base2, a.lw, false)
		w.LineTo(v.X, v.Y)
	}
	w.Stroke()

	if a.closed {
		if col != nil {
			w.SetFillColor(col)
			w.MoveTo(base1.X, base1.Y)
			w.LineTo(tip.X, tip.Y)
			w.LineTo(base2.X, base2.Y)
			w.CloseFillAndStroke()
		} else {
			w.MoveTo(base1.X, base1.Y)
			w.LineTo(tip.X, tip.Y)
			w.LineTo(base2.X, base2.Y)
			w.CloseAndStroke()
		}
	} else {
		w.MoveTo(base1.X, base1.Y)
		w.LineTo(tip.X, tip.Y)
		w.LineTo(base2.X, base2.Y)
		w.Stroke()
	}
}

func (a *arrow) corners() (vec.Vec2, vec.Vec2, vec.Vec2) {
	size := a.Size()
	width := 0.9 * size

	var dir vec.Vec2
	if !a.reverse {
		dir = vec.Vec2{X: a.dX, Y: a.dY}
	} else {
		dir = vec.Vec2{X: -a.dX, Y: -a.dY}
	}
	n := dir.Normal()

	// slope: -width/2/size
	// we need shift*width/2/size=a.lw/2
	shift := size * a.lw / width

	at := vec.Vec2{X: a.atX, Y: a.atY}

	tip := at.Sub(dir.Mul(shift))
	base := tip.Sub(dir.Mul(size))
	base1 := base.Add(n.Mul(0.5 * width))
	base2 := base.Sub(n.Mul(0.5 * width))

	return tip, base1, base2
}

func (a *arrow) outerCorners() []float64 {
	tip, base1, base2 := a.corners()
	oTip, _ := linalg.Miter(base2, tip, base1, a.lw, true)
	oBase1, _ := linalg.Miter(tip, base1, base2, a.lw, true)
	oBase2, _ := linalg.Miter(base1, base2, tip, a.lw, true)
	return []float64{
		oTip.X, oTip.Y,
		oBase1.X, oBase1.Y,
		oBase2.X, oBase2.Y,
	}
}

// ---------------------------------------------------------------------------

type tooShort struct{}

func (m *tooShort) Enlarge(*pdf.Rectangle) {
	// do nothing, the line ending is too short to be drawn
}

func (m *tooShort) Draw(w *graphics.Writer, col color.Color) {
	w.Stroke()
}
