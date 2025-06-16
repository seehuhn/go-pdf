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
	"github.com/google/go-cmp/cmp/cmpopts"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/standard"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

// TestExtGState verifies that external graphics states are correctly read.
func TestExtGState(t *testing.T) {
	testFont := standard.Helvetica.New()

	// We start by creating a graphics state with all possible parameters set.
	ext1 := &graphics.ExtGState{}
	ext1.TextFont = testFont
	ext1.TextFontSize = 12
	ext1.Set |= graphics.StateTextFont
	ext1.TextKnockout = true
	ext1.Set |= graphics.StateTextKnockout
	ext1.LineWidth = 13
	ext1.Set |= graphics.StateLineWidth
	ext1.LineCap = graphics.LineCapSquare
	ext1.Set |= graphics.StateLineCap
	ext1.LineJoin = graphics.LineJoinRound
	ext1.Set |= graphics.StateLineJoin
	ext1.MiterLimit = 14
	ext1.Set |= graphics.StateMiterLimit
	ext1.DashPattern = []float64{1, 2, 3}
	ext1.DashPhase = 4
	ext1.Set |= graphics.StateLineDash
	ext1.RenderingIntent = "dangerously ambiguous"
	ext1.Set |= graphics.StateRenderingIntent
	ext1.StrokeAdjustment = true
	ext1.Set |= graphics.StateStrokeAdjustment
	ext1.BlendMode = pdf.Name("SoftLight")
	ext1.Set |= graphics.StateBlendMode
	ext1.SoftMask = pdf.Dict{
		"Type": pdf.Name("Mask"),
		"S":    pdf.Name("Alpha"),
	}
	ext1.Set |= graphics.StateSoftMask
	ext1.StrokeAlpha = 0.4
	ext1.Set |= graphics.StateStrokeAlpha
	ext1.FillAlpha = 0.6
	ext1.Set |= graphics.StateFillAlpha
	ext1.AlphaSourceFlag = true
	ext1.Set |= graphics.StateAlphaSourceFlag
	ext1.BlackPointCompensation = pdf.Name("OFF")
	ext1.Set |= graphics.StateBlackPointCompensation
	ext1.OverprintFill = false
	ext1.OverprintStroke = true
	ext1.Set |= graphics.StateOverprint
	ext1.OverprintMode = 1
	ext1.Set |= graphics.StateOverprintMode
	ext1.BlackGeneration = pdf.Name("Default")
	ext1.Set |= graphics.StateBlackGeneration
	ext1.UndercolorRemoval = pdf.Dict{
		"FunctionType": pdf.Integer(0),
		"Domain":       pdf.Array{pdf.Integer(0), pdf.Integer(1)},
		"Range":        pdf.Array{pdf.Integer(0), pdf.Integer(1)},
	}
	ext1.Set |= graphics.StateUndercolorRemoval
	ext1.TransferFunction = pdf.Name("Default")
	ext1.Set |= graphics.StateTransferFunction
	ext1.Halftone = pdf.Dict{
		"Type":         pdf.Name("Halftone"),
		"HalftoneType": pdf.Integer(1),
		"Frequency":    pdf.Integer(120),
		"Angle":        pdf.Integer(30),
		"SpotFunction": pdf.Name("Round"),
	}
	ext1.Set |= graphics.StateHalftone
	ext1.HalftoneOriginX = 12
	ext1.HalftoneOriginY = 34
	ext1.Set |= graphics.StateHalftoneOrigin
	ext1.FlatnessTolerance = 0.5
	ext1.Set |= graphics.StateFlatnessTolerance
	ext1.SmoothnessTolerance = 0.6
	ext1.Set |= graphics.StateSmoothnessTolerance

	// check that we have set all possible parameters
	if ext1.Set != graphics.ExtGStateBits {
		t.Error("test is broken: some parameters are not set")
	}

	// step 1: embed this graphics state into a PDF file
	data, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	rm := pdf.NewResourceManager(data)

	ext1Ref, ext1embedded, err := pdf.ResourceManagerEmbed(rm, ext1)
	if err != nil {
		t.Fatal(err)
	}

	err = rm.Close()
	if err != nil {
		t.Fatal(err)
	}

	// step 2: read back the embedded graphics state
	reader := New(data, nil)
	ext2, err := reader.readExtGState(ext1Ref)
	if err != nil {
		t.Fatal(err)
	}

	// step 3: check that the embedded and read back graphics states are equal
	cmpFont := cmp.Comparer(func(a, b font.Embedded) bool {
		if a == nil || b == nil {
			return a == b
		}
		// TODO(voss): update this once we have a way of comparing a loaded
		// font to an original font.  Maybe we can use the font name?
		return true
	})
	fixObj := cmpopts.AcyclicTransformer("fixObj", func(x pdf.Object) pdf.Native {
		return x.AsPDF(0)
	})
	if d := cmp.Diff(ext1embedded, ext2, cmpFont, fixObj); d != "" {
		t.Error(d)
	}
}
