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

// Package state provides the Bits type for tracking graphics state parameters.
package state

import (
	"fmt"
	"math/bits"
	"strings"
)

// Bits is a bit mask for graphics state parameters.
type Bits uint64

// Names returns a string representation of the set bits.
func (b Bits) Names() string {
	var parts []string

	for i := 0; i < len(names); i++ {
		if b&(1<<i) != 0 {
			parts = append(parts, names[i])
		}
	}
	b = b & ^AllBits
	if b != 0 {
		parts = append(parts, fmt.Sprintf("0x%x", b))
	}

	return strings.Join(parts, "|")
}

// Possible values for Bits.
const (
	// CTM is always set, so it is not included in the bit mask.
	// ClippingPath is always set, so it is not included in the bit mask.

	StrokeColor Bits = 1 << iota
	FillColor

	TextCharacterSpacing
	TextWordSpacing
	TextHorizontalScaling
	TextLeading
	TextFont // includes size
	TextRenderingMode
	TextRise
	TextKnockout

	TextMatrix // text matrix and text line matrix

	LineWidth
	LineCap
	LineJoin
	MiterLimit
	LineDash // pattern and phase

	RenderingIntent
	StrokeAdjustment
	BlendMode
	SoftMask
	StrokeAlpha
	FillAlpha
	AlphaSourceFlag
	BlackPointCompensation

	Overprint
	OverprintMode
	BlackGeneration
	UndercolorRemoval
	TransferFunction
	Halftone
	HalftoneOrigin
	FlatnessTolerance
	SmoothnessTolerance

	firstUnused
	AllBits = firstUnused - 1
)

var names = []string{
	"StrokeColor",
	"FillColor",
	"TextCharacterSpacing",
	"TextWordSpacing",
	"TextHorizontalScaling",
	"TextLeading",
	"TextFont",
	"TextRenderingMode",
	"TextRise",
	"TextKnockout",
	"TextMatrix",
	"LineWidth",
	"LineCap",
	"LineJoin",
	"MiterLimit",
	"LineDash",
	"RenderingIntent",
	"StrokeAdjustment",
	"BlendMode",
	"SoftMask",
	"StrokeAlpha",
	"FillAlpha",
	"AlphaSourceFlag",
	"BlackPointCompensation",
	"Overprint",
	"OverprintMode",
	"BlackGeneration",
	"UndercolorRemoval",
	"TransferFunction",
	"Halftone",
	"HalftoneOrigin",
	"FlatnessTolerance",
	"SmoothnessTolerance",
}

// ErrMissing is returned when a required state parameter is not set.
type ErrMissing Bits

func (e ErrMissing) Error() string {
	k := bits.TrailingZeros64(uint64(e))
	return names[k] + " not set"
}
