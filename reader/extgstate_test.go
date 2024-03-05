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

package reader

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"seehuhn.de/go/sfnt/cff"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/internal/dummyfont"
)

func TestExtGState(t *testing.T) {
	data := pdf.NewData(pdf.V1_7)

	F := dummyfont.Embed(data, "")

	s1 := graphics.State{Parameters: &graphics.Parameters{}}
	s1.TextFont = F
	s1.TextFontSize = 12
	s1.Set |= graphics.StateTextFont
	s1.TextKnockout = true
	s1.Set |= graphics.StateTextKnockout
	s1.LineWidth = 13
	s1.Set |= graphics.StateLineWidth
	s1.LineCap = graphics.LineCapSquare
	s1.Set |= graphics.StateLineCap
	s1.LineJoin = graphics.LineJoinRound
	s1.Set |= graphics.StateLineJoin
	s1.MiterLimit = 14
	s1.Set |= graphics.StateMiterLimit
	s1.DashPattern = []float64{1, 2, 3}
	s1.DashPhase = 4
	s1.Set |= graphics.StateLineDash
	s1.RenderingIntent = "dangerously ambiguous"
	s1.Set |= graphics.StateRenderingIntent
	s1.StrokeAdjustment = true
	s1.Set |= graphics.StateStrokeAdjustment
	s1.BlendMode = pdf.Name("SoftLight")
	s1.Set |= graphics.StateBlendMode
	s1.SoftMask = pdf.Dict{
		"Type": pdf.Name("Mask"),
		"S":    pdf.Name("Alpha"),
	}
	s1.Set |= graphics.StateSoftMask
	s1.StrokeAlpha = 0.4
	s1.Set |= graphics.StateStrokeAlpha
	s1.FillAlpha = 0.6
	s1.Set |= graphics.StateFillAlpha
	s1.AlphaSourceFlag = true
	s1.Set |= graphics.StateAlphaSourceFlag
	s1.BlackPointCompensation = pdf.Name("OFF")
	s1.Set |= graphics.StateBlackPointCompensation
	s1.OverprintFill = false
	s1.OverprintStroke = true
	s1.Set |= graphics.StateOverprint
	s1.OverprintMode = 1
	s1.Set |= graphics.StateOverprintMode
	s1.BlackGeneration = pdf.Name("Default")
	s1.Set |= graphics.StateBlackGeneration
	s1.UndercolorRemoval = pdf.Dict{
		"FunctionType": pdf.Integer(0),
		"Domain":       pdf.Array{pdf.Number(0), pdf.Number(1)},
		"Range":        pdf.Array{pdf.Number(0), pdf.Number(1)},
	}
	s1.Set |= graphics.StateUndercolorRemoval
	s1.TransferFunction = pdf.Name("Default")
	s1.Set |= graphics.StateTransferFunction
	s1.Halftone = pdf.Dict{
		"Type":         pdf.Name("Halftone"),
		"HalftoneType": pdf.Integer(1),
		"Frequency":    pdf.Number(120),
		"Angle":        pdf.Number(30),
		"SpotFunction": pdf.Name("Round"),
	}
	s1.Set |= graphics.StateHalftone
	s1.HalftoneOriginX = 12
	s1.HalftoneOriginY = 34
	s1.Set |= graphics.StateHalftoneOrigin
	s1.FlatnessTolerance = 0.5
	s1.Set |= graphics.StateFlatnessTolerance
	s1.SmoothnessTolerance = 0.6
	s1.Set |= graphics.StateSmoothnessTolerance

	ext1, err := graphics.NewExtGState(s1, "X")
	if err != nil {
		t.Fatal(err)
	}

	ext1, err = ext1.Embed(data)
	if err != nil {
		t.Fatal(err)
	}

	qqq := New(data, nil)
	ext2, err := qqq.ReadExtGState(ext1.Res.Data, "X")
	if err != nil {
		t.Fatal(err)
	}

	cmpFDSelectFn := cmp.Comparer(func(fn1, fn2 cff.FDSelectFn) bool {
		return true
	})
	cmpFont := cmp.Comparer(func(f1, f2 font.Embedded) bool {
		if f1.PDFObject() != f2.PDFObject() {
			return false
		}
		if f1.DefaultName() != f2.DefaultName() {
			return false
		}
		if f1.WritingMode() != f2.WritingMode() {
			return false
		}
		// TODO(voss): add more checks?
		return true
	})

	if d := cmp.Diff(ext1, ext2, cmpFDSelectFn, cmpFont); d != "" {
		t.Error(d)
	}

	s3 := graphics.State{Parameters: &graphics.Parameters{}}
	ext2.Value.CopyTo(&s3)
	if d := cmp.Diff(s1, s3, cmpFDSelectFn, cmpFont); d != "" {
		t.Error(d)
	}
}
