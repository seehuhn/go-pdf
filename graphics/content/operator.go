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

import (
	"errors"

	"seehuhn.de/go/pdf"
)

var (
	// ErrUnknown is returned when an operator is not recognized.
	ErrUnknown = errors.New("unknown operator")

	// ErrVersion is returned when an operator is only available in later PDF versions.
	ErrVersion = errors.New("operator not available in PDF version")

	// ErrDeprecated is returned when an operator has been deprecated in the target PDF version.
	ErrDeprecated = errors.New("deprecated operator")
)

// Operator represents a content stream operator with its arguments
type Operator struct {
	Name OpName
	Args []pdf.Object
}

// isValidName checks whether the operator name is valid for the given PDF version.
// It returns ErrUnknown if the operator is not recognized, ErrDeprecated if
// the operator is deprecated in the given version, or ErrVersion if the
// operator was not yet available in the given version.
func (o Operator) isValidName(v pdf.Version) error {
	info, ok := operators[o.Name]
	if !ok {
		return ErrUnknown
	}

	if v < info.Since {
		return ErrVersion
	}

	if info.Deprecated != 0 && v >= info.Deprecated {
		return ErrDeprecated
	}

	return nil
}

// opInfo contains metadata about a content stream operator
type opInfo struct {
	Since      pdf.Version // PDF version when introduced
	Deprecated pdf.Version // PDF version when deprecated (0 if not deprecated)
	Allowed    Object      // bitmask of allowed object states
	Transition Object      // new state after operator (0 = no change)
}

// operators maps operator names to their metadata
var operators = map[OpName]*opInfo{
	// General Graphics State (allowed in page and text contexts)
	OpPushGraphicsState:    {Since: pdf.V1_0, Allowed: ObjPage | ObjText},
	OpPopGraphicsState:     {Since: pdf.V1_0, Allowed: ObjPage | ObjText},
	OpTransform:            {Since: pdf.V1_0, Allowed: ObjPage | ObjText},
	OpSetLineWidth:         {Since: pdf.V1_0, Allowed: ObjPage | ObjText},
	OpSetLineCap:           {Since: pdf.V1_0, Allowed: ObjPage | ObjText},
	OpSetLineJoin:          {Since: pdf.V1_0, Allowed: ObjPage | ObjText},
	OpSetMiterLimit:        {Since: pdf.V1_0, Allowed: ObjPage | ObjText},
	OpSetLineDash:          {Since: pdf.V1_0, Allowed: ObjPage | ObjText},
	OpSetRenderingIntent:   {Since: pdf.V1_1, Allowed: ObjPage | ObjText},
	OpSetFlatnessTolerance: {Since: pdf.V1_0, Allowed: ObjPage | ObjText},
	OpSetExtGState:         {Since: pdf.V1_2, Allowed: ObjPage | ObjText},

	// Path Construction
	OpMoveTo:    {Since: pdf.V1_0, Allowed: ObjPage | ObjPath, Transition: ObjPath},
	OpLineTo:    {Since: pdf.V1_0, Allowed: ObjPath},
	OpCurveTo:   {Since: pdf.V1_0, Allowed: ObjPath},
	OpCurveToV:  {Since: pdf.V1_0, Allowed: ObjPath},
	OpCurveToY:  {Since: pdf.V1_0, Allowed: ObjPath},
	OpClosePath: {Since: pdf.V1_0, Allowed: ObjPath},
	OpRectangle: {Since: pdf.V1_0, Allowed: ObjPage | ObjPath, Transition: ObjPath},

	// Path Painting (transitions back to page context)
	OpStroke:                    {Since: pdf.V1_0, Allowed: ObjPath | ObjClippingPath, Transition: ObjPage},
	OpCloseAndStroke:            {Since: pdf.V1_0, Allowed: ObjPath | ObjClippingPath, Transition: ObjPage},
	OpFill:                      {Since: pdf.V1_0, Allowed: ObjPath | ObjClippingPath, Transition: ObjPage},
	OpFillCompat:                {Since: pdf.V1_0, Deprecated: pdf.V2_0, Allowed: ObjPath | ObjClippingPath, Transition: ObjPage},
	OpFillEvenOdd:               {Since: pdf.V1_0, Allowed: ObjPath | ObjClippingPath, Transition: ObjPage},
	OpFillAndStroke:             {Since: pdf.V1_0, Allowed: ObjPath | ObjClippingPath, Transition: ObjPage},
	OpFillAndStrokeEvenOdd:      {Since: pdf.V1_0, Allowed: ObjPath | ObjClippingPath, Transition: ObjPage},
	OpCloseFillAndStroke:        {Since: pdf.V1_0, Allowed: ObjPath | ObjClippingPath, Transition: ObjPage},
	OpCloseFillAndStrokeEvenOdd: {Since: pdf.V1_0, Allowed: ObjPath | ObjClippingPath, Transition: ObjPage},
	OpEndPath:                   {Since: pdf.V1_0, Allowed: ObjPath | ObjClippingPath, Transition: ObjPage},

	// Clipping Paths (transitions to clipping path context)
	OpClipNonZero: {Since: pdf.V1_0, Allowed: ObjPath, Transition: ObjClippingPath},
	OpClipEvenOdd: {Since: pdf.V1_0, Allowed: ObjPath, Transition: ObjClippingPath},

	// Text Objects
	OpTextBegin: {Since: pdf.V1_0, Allowed: ObjPage, Transition: ObjText},
	OpTextEnd:   {Since: pdf.V1_0, Allowed: ObjText, Transition: ObjPage},

	// Text State (allowed in any context)
	OpTextSetCharacterSpacing:  {Since: pdf.V1_0, Allowed: ObjAny},
	OpTextSetWordSpacing:       {Since: pdf.V1_0, Allowed: ObjAny},
	OpTextSetHorizontalScaling: {Since: pdf.V1_0, Allowed: ObjAny},
	OpTextSetLeading:           {Since: pdf.V1_0, Allowed: ObjAny},
	OpTextSetFont:              {Since: pdf.V1_0, Allowed: ObjAny},
	OpTextSetRenderingMode:     {Since: pdf.V1_0, Allowed: ObjAny},
	OpTextSetRise:              {Since: pdf.V1_0, Allowed: ObjAny},

	// Text Positioning (only in text context)
	OpTextMoveOffset:           {Since: pdf.V1_0, Allowed: ObjText},
	OpTextMoveOffsetSetLeading: {Since: pdf.V1_0, Allowed: ObjText},
	OpTextSetMatrix:            {Since: pdf.V1_0, Allowed: ObjText},
	OpTextNextLine:             {Since: pdf.V1_0, Allowed: ObjText},

	// Text Showing (only in text context)
	OpTextShow:                       {Since: pdf.V1_0, Allowed: ObjText},
	OpTextShowArray:                  {Since: pdf.V1_0, Allowed: ObjText},
	OpTextShowMoveNextLine:           {Since: pdf.V1_0, Allowed: ObjText},
	OpTextShowMoveNextLineSetSpacing: {Since: pdf.V1_0, Allowed: ObjText},

	// Type 3 Fonts (only at start of Type 3 glyph)
	OpType3ColoredGlyph:   {Since: pdf.V1_0, Allowed: ObjType3Start, Transition: ObjPage},
	OpType3UncoloredGlyph: {Since: pdf.V1_0, Allowed: ObjType3Start, Transition: ObjPage},

	// Colour (allowed in page and text contexts)
	OpSetStrokeColorSpace: {Since: pdf.V1_1, Allowed: ObjPage | ObjText},
	OpSetFillColorSpace:   {Since: pdf.V1_1, Allowed: ObjPage | ObjText},
	OpSetStrokeColor:      {Since: pdf.V1_1, Allowed: ObjPage | ObjText},
	OpSetStrokeColorN:     {Since: pdf.V1_2, Allowed: ObjPage | ObjText},
	OpSetFillColor:        {Since: pdf.V1_1, Allowed: ObjPage | ObjText},
	OpSetFillColorN:       {Since: pdf.V1_2, Allowed: ObjPage | ObjText},
	OpSetStrokeGray:       {Since: pdf.V1_0, Allowed: ObjPage | ObjText},
	OpSetFillGray:         {Since: pdf.V1_0, Allowed: ObjPage | ObjText},
	OpSetStrokeRGB:        {Since: pdf.V1_0, Allowed: ObjPage | ObjText},
	OpSetFillRGB:          {Since: pdf.V1_0, Allowed: ObjPage | ObjText},
	OpSetStrokeCMYK:       {Since: pdf.V1_0, Allowed: ObjPage | ObjText},
	OpSetFillCMYK:         {Since: pdf.V1_0, Allowed: ObjPage | ObjText},

	// Shading Patterns (allowed in page and text contexts)
	OpShading: {Since: pdf.V1_3, Allowed: ObjPage | ObjText},

	// Inline Images (internal use, allowed in page and text)
	opBeginInlineImage: {Since: pdf.V1_0, Allowed: ObjPage | ObjText},
	opInlineImageData:  {Since: pdf.V1_0, Allowed: ObjPage | ObjText},
	opEndInlineImage:   {Since: pdf.V1_0, Allowed: ObjPage | ObjText},

	// XObjects (allowed in page and text contexts)
	OpXObject: {Since: pdf.V1_0, Allowed: ObjPage | ObjText},

	// Marked Content (allowed in any context)
	OpMarkedContentPoint:               {Since: pdf.V1_2, Allowed: ObjAny},
	OpMarkedContentPointWithProperties: {Since: pdf.V1_2, Allowed: ObjAny},
	OpBeginMarkedContent:               {Since: pdf.V1_2, Allowed: ObjAny},
	OpBeginMarkedContentWithProperties: {Since: pdf.V1_2, Allowed: ObjAny},
	OpEndMarkedContent:                 {Since: pdf.V1_2, Allowed: ObjAny},

	// Compatibility (allowed in any context)
	OpBeginCompatibility: {Since: pdf.V1_1, Allowed: ObjAny},
	OpEndCompatibility:   {Since: pdf.V1_1, Allowed: ObjAny},

	// Pseudo-operators (internal use, allowed in any context)
	OpRawContent:  {Since: pdf.V1_0, Allowed: ObjAny},
	OpInlineImage: {Since: pdf.V1_0, Allowed: ObjPage | ObjText},
}

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
	OpType3ColoredGlyph   OpName = "d0"
	OpType3UncoloredGlyph OpName = "d1"

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
	OpRawContent  OpName = "%raw%"   // comments, whitespace and unparsed data
	OpInlineImage OpName = "%image%" // BI...ID...EI inline image

	// Inline image operators (unexported).
	// BI, ID, EI are parsed as a unit and converted to OpInlineImage.
	opBeginInlineImage OpName = "BI"
	opInlineImageData  OpName = "ID"
	opEndInlineImage   OpName = "EI"
)
