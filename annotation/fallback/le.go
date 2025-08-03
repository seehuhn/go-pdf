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
	"fmt"
	"math"

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
	case annotation.LineEndingStyleClosedArrow:
		return &closedArrow{
			atX:     atX,
			atY:     atY,
			dX:      dX,
			dY:      dY,
			lw:      lw,
			reverse: false,
		}
	case annotation.LineEndingStyleButt:
	case annotation.LineEndingStyleROpenArrow:
	case annotation.LineEndingStyleRClosedArrow:
		return &closedArrow{
			atX:     atX,
			atY:     atY,
			dX:      dX,
			dY:      dY,
			lw:      lw,
			reverse: true,
		}
	case annotation.LineEndingStyleSlash:
	default: // annotation.LineEndingStyleNone
		return &none{
			atX: atX,
			atY: atY,
		}
	}
	panic(fmt.Sprintf("not implemented: %s", style))
}

// ---------------------------------------------------------------------------

type square struct {
	atX, atY float64
	dX, dY   float64
	lw       float64
}

func (s *square) Enlarge(b *pdf.Rectangle) {
	size := max(3, 6*s.lw)
	L := size + s.lw
	corners := []float64{
		s.atX + 0.5*L*s.dX - 0.5*L*s.dY, s.atY + 0.5*L*s.dY + 0.5*L*s.dX,
		s.atX - 0.5*L*s.dX - 0.5*L*s.dY, s.atY - 0.5*L*s.dY + 0.5*L*s.dX,
		s.atX - 0.5*L*s.dX + 0.5*L*s.dY, s.atY - 0.5*L*s.dY - 0.5*L*s.dX,
		s.atX + 0.5*L*s.dX + 0.5*L*s.dY, s.atY + 0.5*L*s.dY - 0.5*L*s.dX,
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

func (s *square) Draw(w *graphics.Writer, col color.Color) {
	size := max(3, 6*s.lw)
	w.LineTo(s.atX-0.5*size*s.dX, s.atY-0.5*size*s.dY)
	w.Stroke()
	L := size
	if col != nil {
		w.SetFillColor(col)
		w.MoveTo(s.atX+0.5*L*s.dX-0.5*L*s.dY, s.atY+0.5*L*s.dY+0.5*L*s.dX)
		w.LineTo(s.atX-0.5*L*s.dX-0.5*L*s.dY, s.atY-0.5*L*s.dY+0.5*L*s.dX)
		w.LineTo(s.atX-0.5*L*s.dX+0.5*L*s.dY, s.atY-0.5*L*s.dY-0.5*L*s.dX)
		w.LineTo(s.atX+0.5*L*s.dX+0.5*L*s.dY, s.atY+0.5*L*s.dY-0.5*L*s.dX)
		w.CloseFillAndStroke()
	} else {
		w.MoveTo(s.atX+0.5*L*s.dX-0.5*L*s.dY, s.atY+0.5*L*s.dY+0.5*L*s.dX)
		w.LineTo(s.atX-0.5*L*s.dX-0.5*L*s.dY, s.atY-0.5*L*s.dY+0.5*L*s.dX)
		w.LineTo(s.atX-0.5*L*s.dX+0.5*L*s.dY, s.atY-0.5*L*s.dY-0.5*L*s.dX)
		w.LineTo(s.atX+0.5*L*s.dX+0.5*L*s.dY, s.atY+0.5*L*s.dY-0.5*L*s.dX)
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
	size := max(3, 6*d.lw)
	L := size + d.lw
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
	size := max(3, 6*d.lw)
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

// ---------------------------------------------------------------------------

type none struct {
	atX, atY float64
}

func (n *none) Enlarge(b *pdf.Rectangle) {
	// do nothing, no line ending to expand the bounding box
}

func (n *none) Draw(w *graphics.Writer, col color.Color) {
	w.LineTo(n.atX, n.atY)
	w.Stroke()
}

// ---------------------------------------------------------------------------

type closedArrow struct {
	atX, atY float64
	dX, dY   float64
	lw       float64
	reverse  bool
}

func (a *closedArrow) Enlarge(b *pdf.Rectangle) {
	xy := a.corners()

	// Update bounding box with visual corners (same loop as square.Enlarge)
	first := b.IsZero()
	for i := 0; i < len(xy); i += 2 {
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

func (a *closedArrow) Draw(w *graphics.Writer, col color.Color) {
	size := max(3, 6*a.lw)

	var tipX, tipY float64
	var dirX, dirY float64

	if a.reverse {
		// For reverse arrow: line goes to atX,atY, triangle points backward
		dirX, dirY = -a.dX, -a.dY
		tipX, tipY = a.atX, a.atY

		// Draw line all the way to the endpoint
		w.LineTo(a.atX, a.atY)
		w.Stroke()
	} else {
		// For normal arrow: calculate shift and draw line to triangle base
		dirX, dirY = a.dX, a.dY

		// Calculate shift to account for stroke width at triangle tip
		// Edge vectors from tip to base corners
		edge1X, edge1Y := -size*dirX-0.5*size*dirY, -size*dirY+0.5*size*dirX
		edge2X, edge2Y := -size*dirX+0.5*size*dirY, -size*dirY-0.5*size*dirX
		// Angle between edges: cos(θ) = (edge1 · edge2) / (|edge1| * |edge2|)
		dot := edge1X*edge2X + edge1Y*edge2Y
		mag1 := math.Sqrt(edge1X*edge1X + edge1Y*edge1Y)
		mag2 := math.Sqrt(edge2X*edge2X + edge2Y*edge2Y)
		cosTheta := dot / (mag1 * mag2)
		// Stroke miter extension = strokeWidth / (2 * sin(θ/2))
		// Using identity: sin(θ/2) = sqrt((1 - cos(θ)) / 2)
		sinHalfTheta := math.Sqrt((1 - cosTheta) / 2)
		shift := a.lw / (2 * sinHalfTheta)

		tipX = a.atX - shift*dirX
		tipY = a.atY - shift*dirY

		// Draw line to triangle base (where line naturally connects to arrow)
		w.LineTo(tipX-size*dirX, tipY-size*dirY)
		w.Stroke()
	}

	// Draw triangle arrowhead using direction vectors
	if col != nil {
		w.SetFillColor(col)
		w.MoveTo(tipX, tipY)                                                 // tip
		w.LineTo(tipX-size*dirX-0.5*size*dirY, tipY-size*dirY+0.5*size*dirX) // base corner 1
		w.LineTo(tipX-size*dirX+0.5*size*dirY, tipY-size*dirY-0.5*size*dirX) // base corner 2
		w.CloseFillAndStroke()
	} else {
		w.MoveTo(tipX, tipY)                                                 // tip
		w.LineTo(tipX-size*dirX-0.5*size*dirY, tipY-size*dirY+0.5*size*dirX) // base corner 1
		w.LineTo(tipX-size*dirX+0.5*size*dirY, tipY-size*dirY-0.5*size*dirX) // base corner 2
		w.CloseAndStroke()
	}

	xx := a.corners()
	w.SetLineWidth(0.2)
	w.SetStrokeColor(color.Red)
	w.MoveTo(xx[0], xx[1])
	for i := 2; i < len(xx); i += 2 {
		w.LineTo(xx[i], xx[i+1])
	}
	w.CloseAndStroke()
}

func (a *closedArrow) corners() []float64 {
	// Calculate arrowhead size
	size := max(3, 6*a.lw)

	var tipX, tipY float64
	var dirX, dirY float64

	if a.reverse {
		// For reverse arrow: tip at atX,atY, triangle points backward
		dirX, dirY = -a.dX, -a.dY
		tipX, tipY = a.atX, a.atY
	} else {
		// For normal arrow: calculate shift for tip positioning
		dirX, dirY = a.dX, a.dY

		// Calculate shift for tip (same as original code)
		edge1X, edge1Y := -size*dirX-0.5*size*dirY, -size*dirY+0.5*size*dirX
		edge2X, edge2Y := -size*dirX+0.5*size*dirY, -size*dirY-0.5*size*dirX
		dot := edge1X*edge2X + edge1Y*edge2Y
		mag1 := math.Sqrt(edge1X*edge1X + edge1Y*edge1Y)
		mag2 := math.Sqrt(edge2X*edge2X + edge2Y*edge2Y)
		cosTheta := dot / (mag1 * mag2)
		sinHalfTheta := math.Sqrt((1 - cosTheta) / 2)
		shift := a.lw / (2 * sinHalfTheta)

		tipX = a.atX - shift*dirX
		tipY = a.atY - shift*dirY
	}

	// Drawn base corner positions using direction vectors
	base1X := tipX - size*dirX - 0.5*size*dirY
	base1Y := tipY - size*dirY + 0.5*size*dirX
	base2X := tipX - size*dirX + 0.5*size*dirY
	base2Y := tipY - size*dirY - 0.5*size*dirX

	// Calculate miter extension for base corner 1
	v1X, v1Y := tipX-base1X, tipY-base1Y     // to tip
	v2X, v2Y := base2X-base1X, base2Y-base1Y // to base2
	extend1, bisect1X, bisect1Y := miterExtension(v1X, v1Y, v2X, v2Y, a.lw)
	visualBase1X := base1X - extend1*bisect1X
	visualBase1Y := base1Y - extend1*bisect1Y

	// Calculate miter extension for base corner 2
	v3X, v3Y := base1X-base2X, base1Y-base2Y // to base1
	v4X, v4Y := tipX-base2X, tipY-base2Y     // to tip
	extend2, bisect2X, bisect2Y := miterExtension(v3X, v3Y, v4X, v4Y, a.lw)
	visualBase2X := base2X - extend2*bisect2X
	visualBase2Y := base2Y - extend2*bisect2Y

	return []float64{
		tipX, tipY, // tip (visual position)
		visualBase1X, visualBase1Y, // base corner 1
		visualBase2X, visualBase2Y, // base corner 2
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

// miterExtension calculates miter extension for stroke width compensation
// at a corner where two edges meet. The corner is treated as origin (0,0).
//
// Parameters:
//
//	v1X, v1Y: vector from corner to first adjacent point
//	v2X, v2Y: vector from corner to second adjacent point
//	strokeWidth: line width for miter calculation
//
// Returns:
//
//	extension: distance to extend along bisector direction
//	bisectX, bisectY: normalized bisector direction (pointing outward from corner)
func miterExtension(v1X, v1Y, v2X, v2Y, strokeWidth float64) (extension, bisectX, bisectY float64) {
	length1 := math.Sqrt(v1X*v1X + v1Y*v1Y)
	length2 := math.Sqrt(v2X*v2X + v2Y*v2Y)

	if length1 < 1e-10 || length2 < 1e-10 {
		return 0, 1, 0 // no extension, default bisector
	}

	// angle between edges: cos(θ) = (v1 · v2) / (|v1| * |v2|)
	dot := v1X*v2X + v1Y*v2Y
	cosTheta := dot / (length1 * length2)

	// clamp cosTheta to [-1,1] to handle numerical errors
	if cosTheta > 1 {
		cosTheta = 1
	} else if cosTheta < -1 {
		cosTheta = -1
	}

	// Calculate sin(θ/2) using identity: sin(θ/2) = sqrt((1 - cos(θ)) / 2)
	sinHalfTheta := math.Sqrt((1 - cosTheta) / 2)

	// Handle parallel vectors (very small angle)
	if sinHalfTheta < 1e-10 {
		return 0, 1, 0 // no extension needed for parallel edges
	}

	// Miter extension = strokeWidth / (2 * sin(θ/2))
	extension = strokeWidth / (2 * sinHalfTheta)

	// Calculate normalized bisector direction (sum of unit vectors)
	bisectX = v1X/length1 + v2X/length2
	bisectY = v1Y/length1 + v2Y/length2
	bisectMag := math.Sqrt(bisectX*bisectX + bisectY*bisectY)

	// Normalize bisector (should never be zero for non-parallel edges)
	if bisectMag > 1e-10 {
		bisectX /= bisectMag
		bisectY /= bisectMag
	} else {
		bisectX, bisectY = 1, 0 // fallback direction
	}

	return extension, bisectX, bisectY
}
