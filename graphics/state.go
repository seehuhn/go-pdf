// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2023  Jochen Voss <voss@seehuhn.de>
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

package graphics

import (
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/color"
)

// State collects all graphical parameters of the PDF processor.
//
// See section 8.4 of PDF 32000-1:2008.
type State struct {
	// CTM is the "current transformation matrix", which maps positions from
	// user coordinates to device coordinates
	CTM Matrix

	// TODO(voss): clipping path

	StrokeColor color.Color
	FillColor   color.Color

	// Text State parameters:
	Tm           Matrix
	Tlm          Matrix
	Tc           float64 // character spacing
	Tw           float64 // word spacing
	Th           float64 // horizonal scaling
	Tl           float64 // leading
	Font         pdf.Name
	FontSize     float64
	Tmode        int
	TextTrise    float64
	TextKnockout bool

	LineWidth   float64 // thickness of paths to be stroked
	LineCap     LineCapStyle
	LineJoin    LineJoinStyle
	MiterLimit  float64
	DashPattern []float64

	RenderingIntent pdf.Name

	// StrokeAdjustment is a flag specifying whether to compensate for possible
	// rasterization effects when stroking a path with a line width that is
	// small relative to the pixel resolution of the output device.
	StrokeAdjustment bool

	BlendMode              pdf.Name
	SoftMask               pdf.Dict
	StrokeAlpha            float64
	FillAlpha              float64
	AlphaSourceFlag        bool
	BlackPointCompensation pdf.Name

	// device-dependent parameters:
	OverprintStroke bool
	OverprintFill   bool
	OverprintMode   int
	// TODO(voss): black generation
	// TODO(voss): undercolor removal
	// TODO(voss): transfer function
	// TODO(voss): halftone
	// TODO(voss): flatness
	// TODO(voss): smoothness
}

func NewState() *State {
	res := &State{}
	res.CTM = IdentityMatrix
	res.FillColor = color.Gray(0)
	res.StrokeColor = color.Gray(0)

	res.Tc = 0
	res.Tw = 0
	res.Th = 1
	res.Tl = 0
	// no default for Font
	// no default for FontSize
	res.Tmode = 0
	res.TextTrise = 0
	res.TextKnockout = true

	res.LineWidth = 1
	res.LineCap = LineCapButt
	res.LineJoin = LineJoinMiter
	res.MiterLimit = 10
	res.RenderingIntent = pdf.Name("RelativeColorimetric")
	res.StrokeAdjustment = false
	res.BlendMode = pdf.Name("Normal")
	res.SoftMask = nil
	res.StrokeAlpha = 1
	res.FillAlpha = 1
	res.AlphaSourceFlag = false
	res.BlackPointCompensation = pdf.Name("Default")

	res.OverprintStroke = false
	res.OverprintFill = false
	res.OverprintMode = 0

	return res
}

// Clone returns a shallow copy of the GraphicsState.
func (g *State) Clone() *State {
	res := *g
	return &res
}

// Matrix contains a PDF transformation matrix.
// The elements are stored in the same order as for the "cm" operator.
//
// If M = [a b c d e f] is a matrix, then M corresponds to the following
// 3x3 matrix:
//
//	/ a b 0 \
//	| c d 0 |
//	\ e f 1 /
//
// A vector (x, y, 1) is transformed by M into
//
//	(x y 1) * M = (a*x+c*y+e, b*x+d*y+f, 1)
type Matrix [6]float64

func (A Matrix) Apply(x, y float64) (float64, float64) {
	return A[0]*x + A[2]*y + A[4], A[1]*x + A[3]*y + A[5]
}

func (A Matrix) Mul(B Matrix) Matrix {
	// / A0 A1 0 \  / B0 B1 0 \   / A0*B0+A1*B2    A0*B1+A1*B3    0 \
	// | A2 A3 0 |  | B2 B3 0 | = | A2*B0+A3*B2    A2*B1+A3*B3    0 |
	// \ A4 A5 1 /  \ B4 B5 1 /   \ A4*B0+A5*B2+B4 A4*B1+A5*B3+B5 1 /
	return Matrix{
		A[0]*B[0] + A[1]*B[2],
		A[0]*B[1] + A[1]*B[3],
		A[2]*B[0] + A[3]*B[2],
		A[2]*B[1] + A[3]*B[3],
		A[4]*B[0] + A[5]*B[2] + B[4],
		A[4]*B[1] + A[5]*B[3] + B[5],
	}
}

var IdentityMatrix = Matrix{1, 0, 0, 1, 0, 0}
