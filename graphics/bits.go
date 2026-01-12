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

package graphics

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

	StateStrokeColor Bits = 1 << iota
	StateFillColor

	StateTextCharacterSpacing
	StateTextWordSpacing
	StateTextHorizontalScaling
	StateTextLeading
	StateTextFont // includes size
	StateTextRenderingMode
	StateTextRise
	StateTextKnockout

	StateTextMatrix // text matrix and text line matrix

	StateLineWidth
	StateLineCap
	StateLineJoin
	StateMiterLimit
	StateLineDash // pattern and phase

	StateRenderingIntent
	StateStrokeAdjustment
	StateBlendMode
	StateSoftMask
	StateStrokeAlpha
	StateFillAlpha
	StateAlphaSourceFlag
	StateBlackPointCompensation

	StateOverprint
	StateOverprintMode
	StateBlackGeneration
	StateUndercolorRemoval
	StateTransferFunction
	StateHalftone
	StateHalftoneOrigin
	StateFlatnessTolerance
	StateSmoothnessTolerance

	firstUnused
	AllBits = firstUnused - 1
)

var names = []string{
	"StateStrokeColor",
	"StateFillColor",
	"StateTextCharacterSpacing",
	"StateTextWordSpacing",
	"StateTextHorizontalScaling",
	"StateTextLeading",
	"StateTextFont",
	"StateTextRenderingMode",
	"StateTextRise",
	"StateTextKnockout",
	"StateTextMatrix",
	"StateLineWidth",
	"StateLineCap",
	"StateLineJoin",
	"StateMiterLimit",
	"StateLineDash",
	"StateRenderingIntent",
	"StateStrokeAdjustment",
	"StateBlendMode",
	"StateSoftMask",
	"StateStrokeAlpha",
	"StateFillAlpha",
	"StateAlphaSourceFlag",
	"StateBlackPointCompensation",
	"StateOverprint",
	"StateOverprintMode",
	"StateBlackGeneration",
	"StateUndercolorRemoval",
	"StateTransferFunction",
	"StateHalftone",
	"StateHalftoneOrigin",
	"StateFlatnessTolerance",
	"StateSmoothnessTolerance",
}

// ErrMissing is returned when a required state parameter is not set.
type ErrMissing Bits

func (e ErrMissing) Error() string {
	k := bits.TrailingZeros64(uint64(e))
	return names[k] + " not set"
}
