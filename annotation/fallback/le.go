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
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/content/builder"
)

// lineEndingInfo contains the parameters needed to draw a line ending.
type lineEndingInfo struct {
	// At is the connection point between the line and the line ending.
	At vec.Vec2

	// Dir is the direction vector, pointing in the direction that
	// the line ending should face (away from the line body).
	Dir vec.Vec2

	// For filling line endings, FillColor is used for the filled area.
	FillColor color.Color

	// If IsStart is true, the current point is set to the connection point
	// after drawing the line ending. If IsStart is false, a LineTo() to the
	// connection point is appended to the current path before drawing the line
	// ending.
	IsStart bool
}

// drawLineEndingBuilder draws a line ending using builder.Builder.
func drawLineEndingBuilder(b *builder.Builder, style annotation.LineEndingStyle, info lineEndingInfo) {
	// normalize direction vectors
	if info.Dir.Length() < 0.1 {
		style = annotation.LineEndingStyleNone
	} else {
		info.Dir = info.Dir.Normalize()
	}

	switch style {
	case annotation.LineEndingStyleSquare:
		square(info).drawBuilder(b)
	case annotation.LineEndingStyleCircle:
		circle(info).drawBuilder(b)
	case annotation.LineEndingStyleDiamond:
		diamond(info).drawBuilder(b)
	case annotation.LineEndingStyleOpenArrow:
		a := arrow{lineEndingInfo: info}
		a.drawBuilder(b)
	case annotation.LineEndingStyleClosedArrow:
		a := arrow{lineEndingInfo: info, closed: true}
		a.drawBuilder(b)
	case annotation.LineEndingStyleButt:
		butt(info).drawBuilder(b)
	case annotation.LineEndingStyleROpenArrow:
		a := arrow{lineEndingInfo: info, reverse: true}
		a.drawBuilder(b)
	case annotation.LineEndingStyleRClosedArrow:
		a := arrow{lineEndingInfo: info, closed: true, reverse: true}
		a.drawBuilder(b)
	case annotation.LineEndingStyleSlash:
		slash(info).drawBuilder(b)
	default: // annotation.LineEndingStyleNone
		none(info).drawBuilder(b)
	}
}

// lineEndingBBox enlarges a bounding box to include the line ending.
func lineEndingBBox(bbox *pdf.Rectangle, style annotation.LineEndingStyle, info lineEndingInfo, lw float64) {
	// normalize direction vectors
	if info.Dir.Length() < 0.1 {
		return // too short to enlarge
	}
	info.Dir = info.Dir.Normalize()

	switch style {
	case annotation.LineEndingStyleSquare:
		square(info).extend(bbox, lw)
	case annotation.LineEndingStyleCircle:
		circle(info).extend(bbox, lw)
	case annotation.LineEndingStyleDiamond:
		diamond(info).extend(bbox, lw)
	case annotation.LineEndingStyleOpenArrow:
		a := arrow{lineEndingInfo: info}
		a.extend(bbox, lw)
	case annotation.LineEndingStyleClosedArrow:
		a := arrow{lineEndingInfo: info, closed: true}
		a.extend(bbox, lw)
	case annotation.LineEndingStyleButt:
		butt(info).extend(bbox, lw)
	case annotation.LineEndingStyleROpenArrow:
		a := arrow{lineEndingInfo: info, reverse: true}
		a.extend(bbox, lw)
	case annotation.LineEndingStyleRClosedArrow:
		a := arrow{lineEndingInfo: info, closed: true, reverse: true}
		a.extend(bbox, lw)
	case annotation.LineEndingStyleSlash:
		slash(info).extend(bbox, lw)
	default: // annotation.LineEndingStyleNone
		// none style doesn't change the bbox
	}
}

// ---------------------------------------------------------------------------

type none lineEndingInfo

func (le none) drawBuilder(b *builder.Builder) {
	if !le.IsStart {
		b.LineTo(pdf.Round(le.At.X, 2), pdf.Round(le.At.Y, 2))
		b.Stroke()
	}
	if le.IsStart {
		b.MoveTo(pdf.Round(le.At.X, 2), pdf.Round(le.At.Y, 2))
	}
}

// ---------------------------------------------------------------------------

type butt lineEndingInfo

func (le butt) extend(bbox *pdf.Rectangle, lw float64) {
	n := le.Dir.Normal()
	n.IMul(le.size(lw) / 2)
	p1 := le.At.Add(n).Add(le.Dir)
	p2 := le.At.Sub(n).Add(le.Dir)
	p3 := le.At.Add(n).Sub(le.Dir)
	p4 := le.At.Sub(n).Sub(le.Dir)
	corners := []float64{
		p1.X, p1.Y,
		p2.X, p2.Y,
		p3.X, p3.Y,
		p4.X, p4.Y,
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

func (le butt) size(lw float64) float64 {
	return max(3.5, 7*lw)
}

func (le butt) drawBuilder(b *builder.Builder) {
	n := le.Dir.Normal()
	n.IMul(le.size(b.State.GState.LineWidth) / 2)

	if le.IsStart {
		p1 := le.At.Add(n)
		p2 := le.At.Sub(n)
		b.MoveTo(pdf.Round(p1.X, 2), pdf.Round(p1.Y, 2))
		b.LineTo(pdf.Round(p2.X, 2), pdf.Round(p2.Y, 2))
		b.MoveTo(pdf.Round(le.At.X, 2), pdf.Round(le.At.Y, 2))
	} else {
		b.LineTo(pdf.Round(le.At.X, 2), pdf.Round(le.At.Y, 2))
		p1 := le.At.Add(n)
		p2 := le.At.Sub(n)
		b.MoveTo(pdf.Round(p1.X, 2), pdf.Round(p1.Y, 2))
		b.LineTo(pdf.Round(p2.X, 2), pdf.Round(p2.Y, 2))
		b.Stroke()
	}
}

// ---------------------------------------------------------------------------

type slash lineEndingInfo

func (le slash) extend(bbox *pdf.Rectangle, lw float64) {
	a := 0.5              // cos(60°)
	b := math.Sqrt(3) / 2 // sin(60°)
	n := vec.Vec2{X: a*le.Dir.X - b*le.Dir.Y, Y: a*le.Dir.Y + b*le.Dir.X}
	n.IMul(le.size(lw) / 2)
	p1 := le.At.Add(n).Add(le.Dir)
	p2 := le.At.Sub(n).Add(le.Dir)
	p3 := le.At.Add(n).Sub(le.Dir)
	p4 := le.At.Sub(n).Sub(le.Dir)
	corners := []float64{
		p1.X, p1.Y,
		p2.X, p2.Y,
		p3.X, p3.Y,
		p4.X, p4.Y,
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

func (le slash) size(lw float64) float64 {
	return max(5, 10*lw)
}

func (le slash) drawBuilder(b *builder.Builder) {
	a := 0.5              // cos(60°)
	c := math.Sqrt(3) / 2 // sin(60°)
	n := vec.Vec2{X: a*le.Dir.X - c*le.Dir.Y, Y: a*le.Dir.Y + c*le.Dir.X}
	n.IMul(le.size(b.State.GState.LineWidth) / 2)

	if le.IsStart {
		p1 := le.At.Add(n)
		p2 := le.At.Sub(n)
		b.MoveTo(pdf.Round(p1.X, 2), pdf.Round(p1.Y, 2))
		b.LineTo(pdf.Round(p2.X, 2), pdf.Round(p2.Y, 2))
		b.MoveTo(pdf.Round(le.At.X, 2), pdf.Round(le.At.Y, 2))
	} else {
		b.LineTo(pdf.Round(le.At.X, 2), pdf.Round(le.At.Y, 2))
		p1 := le.At.Add(n)
		p2 := le.At.Sub(n)
		b.MoveTo(pdf.Round(p1.X, 2), pdf.Round(p1.Y, 2))
		b.LineTo(pdf.Round(p2.X, 2), pdf.Round(p2.Y, 2))
		b.Stroke()
	}
}

// ---------------------------------------------------------------------------

type square lineEndingInfo

func (le square) extend(bbox *pdf.Rectangle, lw float64) {
	L := le.size(lw) + lw
	corners := []float64{
		le.At.X + L/2, le.At.Y + L/2,
		le.At.X - L/2, le.At.Y - L/2,
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

func (le square) size(lw float64) float64 {
	return max(3, 6*lw)
}

func (le square) drawBuilder(b *builder.Builder) {
	size := le.size(b.State.GState.LineWidth)
	a := (size / 2) / max(math.Abs(le.Dir.X), math.Abs(le.Dir.Y))

	if le.IsStart {
		if le.FillColor != nil {
			b.SetFillColor(le.FillColor)
			b.Rectangle(pdf.Round(le.At.X-size/2, 2), pdf.Round(le.At.Y-size/2, 2), pdf.Round(size, 2), pdf.Round(size, 2))
			b.FillAndStroke()
		} else {
			b.Rectangle(pdf.Round(le.At.X-size/2, 2), pdf.Round(le.At.Y-size/2, 2), pdf.Round(size, 2), pdf.Round(size, 2))
			b.Stroke()
		}
		pos := le.At.Sub(le.Dir.Mul(a))
		b.MoveTo(pdf.Round(pos.X, 2), pdf.Round(pos.Y, 2))
	} else {
		pos := le.At.Sub(le.Dir.Mul(a))
		b.LineTo(pdf.Round(pos.X, 2), pdf.Round(pos.Y, 2))
		b.Stroke()
		if le.FillColor != nil {
			b.SetFillColor(le.FillColor)
			b.Rectangle(pdf.Round(le.At.X-size/2, 2), pdf.Round(le.At.Y-size/2, 2), pdf.Round(size, 2), pdf.Round(size, 2))
			b.FillAndStroke()
		} else {
			b.Rectangle(pdf.Round(le.At.X-size/2, 2), pdf.Round(le.At.Y-size/2, 2), pdf.Round(size, 2), pdf.Round(size, 2))
			b.Stroke()
		}
	}
}

// ---------------------------------------------------------------------------

type circle lineEndingInfo

func (le circle) extend(b *pdf.Rectangle, lw float64) {
	L := le.size(lw) + lw
	first := b.IsZero()
	xMin := le.At.X - 0.5*L
	xMax := le.At.X + 0.5*L
	yMin := le.At.Y - 0.5*L
	yMax := le.At.Y + 0.5*L

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

func (le circle) size(lw float64) float64 {
	return max(3.5, 7*lw)
}

func (le circle) drawBuilder(b *builder.Builder) {
	size := le.size(b.State.GState.LineWidth)

	if le.IsStart {
		if le.FillColor != nil {
			b.SetFillColor(le.FillColor)
			b.Circle(pdf.Round(le.At.X, 2), pdf.Round(le.At.Y, 2), pdf.Round(0.5*size, 2))
			b.FillAndStroke()
		} else {
			b.Circle(pdf.Round(le.At.X, 2), pdf.Round(le.At.Y, 2), pdf.Round(0.5*size, 2))
			b.Stroke()
		}
		pos := le.At.Sub(le.Dir.Mul(0.5 * size))
		b.MoveTo(pdf.Round(pos.X, 2), pdf.Round(pos.Y, 2))
	} else {
		pos := le.At.Sub(le.Dir.Mul(0.5 * size))
		b.LineTo(pdf.Round(pos.X, 2), pdf.Round(pos.Y, 2))
		b.Stroke()
		if le.FillColor != nil {
			b.SetFillColor(le.FillColor)
			b.Circle(pdf.Round(le.At.X, 2), pdf.Round(le.At.Y, 2), pdf.Round(0.5*size, 2))
			b.FillAndStroke()
		} else {
			b.Circle(pdf.Round(le.At.X, 2), pdf.Round(le.At.Y, 2), pdf.Round(0.5*size, 2))
			b.Stroke()
		}
	}
}

// ---------------------------------------------------------------------------

type diamond lineEndingInfo

func (le diamond) extend(b *pdf.Rectangle, lw float64) {
	L := le.size(lw) + lw
	corners := []float64{
		le.At.X - L/2, le.At.Y - L/2,
		le.At.X + L/2, le.At.Y + L/2,
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

func (le diamond) size(lw float64) float64 {
	return max(4, 8*lw)
}

func (le diamond) drawBuilder(b *builder.Builder) {
	size := le.size(b.State.GState.LineWidth)
	a := size / (math.Abs(le.Dir.X) + math.Abs(le.Dir.Y)) / 2
	L := size

	if le.IsStart {
		if le.FillColor != nil {
			b.SetFillColor(le.FillColor)
			b.MoveTo(pdf.Round(le.At.X+L/2, 2), pdf.Round(le.At.Y, 2))
			b.LineTo(pdf.Round(le.At.X, 2), pdf.Round(le.At.Y+L/2, 2))
			b.LineTo(pdf.Round(le.At.X-L/2, 2), pdf.Round(le.At.Y, 2))
			b.LineTo(pdf.Round(le.At.X, 2), pdf.Round(le.At.Y-L/2, 2))
			b.CloseFillAndStroke()
		} else {
			b.MoveTo(pdf.Round(le.At.X+L/2, 2), pdf.Round(le.At.Y, 2))
			b.LineTo(pdf.Round(le.At.X, 2), pdf.Round(le.At.Y+L/2, 2))
			b.LineTo(pdf.Round(le.At.X-L/2, 2), pdf.Round(le.At.Y, 2))
			b.LineTo(pdf.Round(le.At.X, 2), pdf.Round(le.At.Y-L/2, 2))
			b.CloseAndStroke()
		}
		pos := le.At.Sub(le.Dir.Mul(a))
		b.MoveTo(pdf.Round(pos.X, 2), pdf.Round(pos.Y, 2))
	} else {
		pos := le.At.Sub(le.Dir.Mul(a))
		b.LineTo(pdf.Round(pos.X, 2), pdf.Round(pos.Y, 2))
		b.Stroke()
		if le.FillColor != nil {
			b.SetFillColor(le.FillColor)
			b.MoveTo(pdf.Round(le.At.X+L/2, 2), pdf.Round(le.At.Y, 2))
			b.LineTo(pdf.Round(le.At.X, 2), pdf.Round(le.At.Y+L/2, 2))
			b.LineTo(pdf.Round(le.At.X-L/2, 2), pdf.Round(le.At.Y, 2))
			b.LineTo(pdf.Round(le.At.X, 2), pdf.Round(le.At.Y-L/2, 2))
			b.CloseFillAndStroke()
		} else {
			b.MoveTo(pdf.Round(le.At.X+L/2, 2), pdf.Round(le.At.Y, 2))
			b.LineTo(pdf.Round(le.At.X, 2), pdf.Round(le.At.Y+L/2, 2))
			b.LineTo(pdf.Round(le.At.X-L/2, 2), pdf.Round(le.At.Y, 2))
			b.LineTo(pdf.Round(le.At.X, 2), pdf.Round(le.At.Y-L/2, 2))
			b.CloseAndStroke()
		}
	}
}

// ---------------------------------------------------------------------------

type arrow struct {
	lineEndingInfo
	closed  bool
	reverse bool
}

func (le arrow) size(lw float64) float64 {
	return max(4, 8*lw)
}

func (le arrow) extend(b *pdf.Rectangle, lw float64) {
	xy := le.outerCorners(lw)

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

func (le arrow) drawBuilder(b *builder.Builder) {
	tip, base1, base2 := le.corners(b.State.GState.LineWidth)

	var connectX, connectY float64
	if le.reverse {
		connectX, connectY = le.At.X, le.At.Y
	} else if le.closed {
		m := vec.Middle(base1, base2)
		connectX, connectY = m.X, m.Y
	} else {
		v, _ := linalg.Miter(base1, tip, base2, b.State.GState.LineWidth, false)
		connectX, connectY = v.X, v.Y
	}

	if le.IsStart {
		if le.closed {
			if le.FillColor != nil {
				b.SetFillColor(le.FillColor)
				b.MoveTo(pdf.Round(base1.X, 2), pdf.Round(base1.Y, 2))
				b.LineTo(pdf.Round(tip.X, 2), pdf.Round(tip.Y, 2))
				b.LineTo(pdf.Round(base2.X, 2), pdf.Round(base2.Y, 2))
				b.CloseFillAndStroke()
			} else {
				b.MoveTo(pdf.Round(base1.X, 2), pdf.Round(base1.Y, 2))
				b.LineTo(pdf.Round(tip.X, 2), pdf.Round(tip.Y, 2))
				b.LineTo(pdf.Round(base2.X, 2), pdf.Round(base2.Y, 2))
				b.CloseAndStroke()
			}
		} else {
			b.MoveTo(pdf.Round(base1.X, 2), pdf.Round(base1.Y, 2))
			b.LineTo(pdf.Round(tip.X, 2), pdf.Round(tip.Y, 2))
			b.LineTo(pdf.Round(base2.X, 2), pdf.Round(base2.Y, 2))
		}
		b.MoveTo(pdf.Round(connectX, 2), pdf.Round(connectY, 2))
	} else {
		b.LineTo(pdf.Round(connectX, 2), pdf.Round(connectY, 2))
		b.Stroke()
		if le.closed {
			if le.FillColor != nil {
				b.SetFillColor(le.FillColor)
				b.MoveTo(pdf.Round(base1.X, 2), pdf.Round(base1.Y, 2))
				b.LineTo(pdf.Round(tip.X, 2), pdf.Round(tip.Y, 2))
				b.LineTo(pdf.Round(base2.X, 2), pdf.Round(base2.Y, 2))
				b.CloseFillAndStroke()
			} else {
				b.MoveTo(pdf.Round(base1.X, 2), pdf.Round(base1.Y, 2))
				b.LineTo(pdf.Round(tip.X, 2), pdf.Round(tip.Y, 2))
				b.LineTo(pdf.Round(base2.X, 2), pdf.Round(base2.Y, 2))
				b.CloseAndStroke()
			}
		} else {
			b.MoveTo(pdf.Round(base1.X, 2), pdf.Round(base1.Y, 2))
			b.LineTo(pdf.Round(tip.X, 2), pdf.Round(tip.Y, 2))
			b.LineTo(pdf.Round(base2.X, 2), pdf.Round(base2.Y, 2))
			b.Stroke()
		}
	}
}

func (le arrow) corners(lw float64) (vec.Vec2, vec.Vec2, vec.Vec2) {
	size := le.size(lw)
	width := 0.9 * size

	var dir vec.Vec2
	if !le.reverse {
		dir = le.Dir
	} else {
		dir = le.Dir.Neg()
	}
	n := dir.Normal()

	// slope: -width/2/size
	// we need shift*width/2/size=a.lw/2
	shift := size * lw / width

	at := le.At

	tip := at.Sub(dir.Mul(shift))
	base := tip.Sub(dir.Mul(size))
	base1 := base.Add(n.Mul(0.5 * width))
	base2 := base.Sub(n.Mul(0.5 * width))

	return tip, base1, base2
}

func (le arrow) outerCorners(lw float64) []float64 {
	tip, base1, base2 := le.corners(lw)
	oTip, _ := linalg.Miter(base2, tip, base1, lw, true)
	oBase1, _ := linalg.Miter(tip, base1, base2, lw, true)
	oBase2, _ := linalg.Miter(base1, base2, tip, lw, true)
	return []float64{
		oTip.X, oTip.Y,
		oBase1.X, oBase1.Y,
		oBase2.X, oBase2.Y,
	}
}
