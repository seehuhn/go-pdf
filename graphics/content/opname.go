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

package content

import "seehuhn.de/go/pdf"

type OpName pdf.Name

const (
	// General Graphics State
	OpPushGraphicsState    OpName = "q"
	OpPopGraphicsState     OpName = "Q"
	OpTransform            OpName = "cm"
	OpSetLineWidth         OpName = "w"
	OpSetLineCap           OpName = "J"
	OpSetLineJoin          OpName = "j"
	OpSetMiterLimit        OpName = "M"
	OpSetLineDash          OpName = "d"
	OpSetRenderingIntent   OpName = "ri"
	OpSetFlatnessTolerance OpName = "i"
	OpSetExtGState         OpName = "gs"

	// Path Construction
	OpMoveTo    OpName = "m"
	OpLineTo    OpName = "l"
	OpCurveTo   OpName = "c"
	OpCurveToV  OpName = "v"
	OpCurveToY  OpName = "y"
	OpClosePath OpName = "h"
	OpRectangle OpName = "re"

	// Path Painting
	OpStroke                    OpName = "S"
	OpCloseAndStroke            OpName = "s"
	OpFill                      OpName = "f"
	OpFillCompat                OpName = "F"
	OpFillEvenOdd               OpName = "f*"
	OpFillAndStroke             OpName = "B"
	OpFillAndStrokeEvenOdd      OpName = "B*"
	OpCloseFillAndStroke        OpName = "b"
	OpCloseFillAndStrokeEvenOdd OpName = "b*"
	OpEndPath                   OpName = "n"

	// Clipping Paths
	OpClipNonZero OpName = "W"
	OpClipEvenOdd OpName = "W*"

	// Text Objects
	OpTextBegin OpName = "BT"
	OpTextEnd   OpName = "ET"

	// Text State
	OpTextSetCharacterSpacing  OpName = "Tc"
	OpTextSetWordSpacing       OpName = "Tw"
	OpTextSetHorizontalScaling OpName = "Tz"
	OpTextSetLeading           OpName = "TL"
	OpTextSetFont              OpName = "Tf"
	OpTextSetRenderingMode     OpName = "Tr"
	OpTextSetRise              OpName = "Ts"

	// Text Positioning
	OpTextMoveOffset           OpName = "Td"
	OpTextMoveOffsetSetLeading OpName = "TD"
	OpTextSetMatrix            OpName = "Tm"
	OpTextNextLine             OpName = "T*"

	// Text Showing
	OpTextShow                       OpName = "Tj"
	OpTextShowArray                  OpName = "TJ"
	OpTextShowMoveNextLine           OpName = "'"
	OpTextShowMoveNextLineSetSpacing OpName = "\""

	// Type 3 Fonts
	OpType3SetWidthOnly           OpName = "d0"
	OpType3SetWidthAndBoundingBox OpName = "d1"

	// Color Spaces
	OpSetStrokeColorSpace OpName = "CS"
	OpSetFillColorSpace   OpName = "cs"

	// Generic Color
	OpSetStrokeColor  OpName = "SC"
	OpSetStrokeColorN OpName = "SCN"
	OpSetFillColor    OpName = "sc"
	OpSetFillColorN   OpName = "scn"

	// Device Colors
	OpSetStrokeGray OpName = "G"
	OpSetFillGray   OpName = "g"
	OpSetStrokeRGB  OpName = "RG"
	OpSetFillRGB    OpName = "rg"
	OpSetStrokeCMYK OpName = "K"
	OpSetFillCMYK   OpName = "k"

	// Shading Patterns
	OpShading OpName = "sh"

	// XObjects
	OpXObject OpName = "Do"

	// Marked Content
	OpMarkedContentPoint               OpName = "MP"
	OpMarkedContentPointWithProperties OpName = "DP"
	OpBeginMarkedContent               OpName = "BMC"
	OpBeginMarkedContentWithProperties OpName = "BDC"
	OpEndMarkedContent                 OpName = "EMC"

	// Compatibility
	OpBeginCompatibility OpName = "BX"
	OpEndCompatibility   OpName = "EX"

	// Pseudo-operators used internally
	OpRawContent  OpName = "%raw%"
	OpInlineImage OpName = "%image%"

	// Inline image operators (unexported).
	// BI, ID, EI are parsed as a unit and converted to OpInlineImage.
	opBeginInlineImage OpName = "BI"
	opInlineImageData  OpName = "ID"
	opEndInlineImage   OpName = "EI"
)
