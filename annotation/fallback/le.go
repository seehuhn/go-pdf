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
}

// lineEnding is a line ending, laid out at one end of a line
type lineEnding interface {
	// connection returns the point where the line meets the ending,
	// for line width lw
	connection(lw float64) vec.Vec2
	// drawShape draws the ending itself, complete with its own
	// fill/stroke painting operator; it leaves no open path
	drawShape(b *builder.Builder)
}

// newLineEnding returns the ending of the given style; a direction
// shorter than 0.1 gives no ending (style None), otherwise Dir is normalised
func newLineEnding(style annotation.LineEndingStyle, info lineEndingInfo) lineEnding {
	if info.Dir.Length() < 0.1 {
		return none(info)
	}
	info.Dir = info.Dir.Normalize()

	switch style {
	case annotation.LineEndingStyleSquare:
		return square(info)
	case annotation.LineEndingStyleCircle:
		return circle(info)
	case annotation.LineEndingStyleDiamond:
		return diamond(info)
	case annotation.LineEndingStyleOpenArrow:
		return arrow{lineEndingInfo: info}
	case annotation.LineEndingStyleClosedArrow:
		return arrow{lineEndingInfo: info, closed: true}
	case annotation.LineEndingStyleButt:
		return butt(info)
	case annotation.LineEndingStyleROpenArrow:
		return arrow{lineEndingInfo: info, reverse: true}
	case annotation.LineEndingStyleRClosedArrow:
		return arrow{lineEndingInfo: info, closed: true, reverse: true}
	case annotation.LineEndingStyleSlash:
		return slash(info)
	default: // annotation.LineEndingStyleNone
		return none(info)
	}
}

// setFill selects the fill colour, if the ending is filled.
func (le lineEndingInfo) setFill(b *builder.Builder) {
	if le.FillColor != nil {
		b.SetFillColor(le.FillColor)
	}
}

// paint paints the current path, filling it if the ending is filled.
// If closed is true, the path is closed first.
func (le lineEndingInfo) paint(b *builder.Builder, closed bool) {
	switch {
	case le.FillColor != nil && closed:
		b.CloseFillAndStroke()
	case le.FillColor != nil:
		b.FillAndStroke()
	case closed:
		b.CloseAndStroke()
	default:
		b.Stroke()
	}
}

// strokeSegment strokes the line from At+n to At-n.
func (le lineEndingInfo) strokeSegment(b *builder.Builder, n vec.Vec2) {
	p1 := le.At.Add(n)
	p2 := le.At.Sub(n)
	b.MoveTo(pdf.Round(p1.X, 2), pdf.Round(p1.Y, 2))
	b.LineTo(pdf.Round(p2.X, 2), pdf.Round(p2.Y, 2))
	b.Stroke()
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

func (le none) connection(lw float64) vec.Vec2 {
	return le.At
}

func (le none) drawShape(b *builder.Builder) {}

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

func (le butt) connection(lw float64) vec.Vec2 {
	return le.At
}

func (le butt) drawShape(b *builder.Builder) {
	n := le.Dir.Normal()
	n.IMul(le.size(b.State.GState.LineWidth) / 2)
	lineEndingInfo(le).strokeSegment(b, n)
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

func (le slash) connection(lw float64) vec.Vec2 {
	return le.At
}

func (le slash) drawShape(b *builder.Builder) {
	a := 0.5              // cos(60°)
	c := math.Sqrt(3) / 2 // sin(60°)
	n := vec.Vec2{X: a*le.Dir.X - c*le.Dir.Y, Y: a*le.Dir.Y + c*le.Dir.X}
	n.IMul(le.size(b.State.GState.LineWidth) / 2)
	lineEndingInfo(le).strokeSegment(b, n)
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

func (le square) connection(lw float64) vec.Vec2 {
	a := (le.size(lw) / 2) / max(math.Abs(le.Dir.X), math.Abs(le.Dir.Y))
	return le.At.Sub(le.Dir.Mul(a))
}

func (le square) drawShape(b *builder.Builder) {
	size := le.size(b.State.GState.LineWidth)
	info := lineEndingInfo(le)
	info.setFill(b)
	b.Rectangle(pdf.Round(le.At.X-size/2, 2), pdf.Round(le.At.Y-size/2, 2), pdf.Round(size, 2), pdf.Round(size, 2))
	info.paint(b, false)
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

func (le circle) connection(lw float64) vec.Vec2 {
	return le.At.Sub(le.Dir.Mul(0.5 * le.size(lw)))
}

func (le circle) drawShape(b *builder.Builder) {
	size := le.size(b.State.GState.LineWidth)
	info := lineEndingInfo(le)
	info.setFill(b)
	b.Circle(pdf.Round(le.At.X, 2), pdf.Round(le.At.Y, 2), pdf.Round(0.5*size, 2))
	info.paint(b, false)
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

func (le diamond) connection(lw float64) vec.Vec2 {
	a := le.size(lw) / (math.Abs(le.Dir.X) + math.Abs(le.Dir.Y)) / 2
	return le.At.Sub(le.Dir.Mul(a))
}

func (le diamond) drawShape(b *builder.Builder) {
	L := le.size(b.State.GState.LineWidth)
	info := lineEndingInfo(le)
	info.setFill(b)
	b.MoveTo(pdf.Round(le.At.X+L/2, 2), pdf.Round(le.At.Y, 2))
	b.LineTo(pdf.Round(le.At.X, 2), pdf.Round(le.At.Y+L/2, 2))
	b.LineTo(pdf.Round(le.At.X-L/2, 2), pdf.Round(le.At.Y, 2))
	b.LineTo(pdf.Round(le.At.X, 2), pdf.Round(le.At.Y-L/2, 2))
	info.paint(b, true)
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

func (le arrow) connection(lw float64) vec.Vec2 {
	switch {
	case le.reverse:
		return le.At
	case le.closed:
		_, base1, base2 := le.corners(lw)
		return vec.Middle(base1, base2)
	default:
		tip, base1, base2 := le.corners(lw)
		v, _ := linalg.Miter(base1, tip, base2, lw, false)
		return v
	}
}

func (le arrow) drawShape(b *builder.Builder) {
	tip, base1, base2 := le.corners(b.State.GState.LineWidth)
	if le.closed {
		le.setFill(b)
	}
	b.MoveTo(pdf.Round(base1.X, 2), pdf.Round(base1.Y, 2))
	b.LineTo(pdf.Round(tip.X, 2), pdf.Round(tip.Y, 2))
	b.LineTo(pdf.Round(base2.X, 2), pdf.Round(base2.Y, 2))
	if le.closed {
		le.paint(b, true)
	} else {
		b.Stroke()
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
