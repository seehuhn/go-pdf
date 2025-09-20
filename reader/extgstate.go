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

package reader

import (
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/graphics"
)

// readExtGState reads an graphics state parameter dictionary from a PDF file.
func (r *Reader) readExtGState(ref pdf.Object) (graphics.State, error) {
	var zero graphics.State

	extGState, err := graphics.ExtractExtGState(r.x, ref)
	if err != nil {
		return zero, err
	}

	// Convert ExtGState to graphics.State by copying all fields to Parameters
	param := &graphics.Parameters{
		TextFontSize:           extGState.TextFontSize,
		TextKnockout:           extGState.TextKnockout,
		LineWidth:              extGState.LineWidth,
		LineCap:                extGState.LineCap,
		LineJoin:               extGState.LineJoin,
		MiterLimit:             extGState.MiterLimit,
		DashPattern:            extGState.DashPattern,
		DashPhase:              extGState.DashPhase,
		RenderingIntent:        extGState.RenderingIntent,
		StrokeAdjustment:       extGState.StrokeAdjustment,
		BlendMode:              extGState.BlendMode,
		SoftMask:               extGState.SoftMask,
		StrokeAlpha:            extGState.StrokeAlpha,
		FillAlpha:              extGState.FillAlpha,
		AlphaSourceFlag:        extGState.AlphaSourceFlag,
		BlackPointCompensation: extGState.BlackPointCompensation,
		OverprintStroke:        extGState.OverprintStroke,
		OverprintFill:          extGState.OverprintFill,
		OverprintMode:          extGState.OverprintMode,
		BlackGeneration:        extGState.BlackGeneration,
		UndercolorRemoval:      extGState.UndercolorRemoval,
		TransferFunction:       extGState.TransferFunction,
		Halftone:               extGState.Halftone,
		HalftoneOriginX:        extGState.HalftoneOriginX,
		HalftoneOriginY:        extGState.HalftoneOriginY,
		FlatnessTolerance:      extGState.FlatnessTolerance,
		SmoothnessTolerance:    extGState.SmoothnessTolerance,
	}

	// Handle TextFont conversion - ExtGState.TextFont is font.Font, but
	// Parameters.TextFont expects font.Embedded Since the font came from a PDF
	// file, it should be font.FromFile which implements font.Embedded.
	//
	// TODO(voss): this should be modelled on Writer.TextSetFont instead.
	if extGState.TextFont != nil {
		if embeddedFont, ok := extGState.TextFont.(font.Embedded); ok {
			param.TextFont = embeddedFont
		}
	}

	res := graphics.State{
		Parameters: param,
		Set:        extGState.Set,
	}
	return res, nil
}
