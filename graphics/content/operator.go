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
	"seehuhn.de/go/pdf/graphics"
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

// Equal determines whether two operators are equal.
func (o Operator) Equal(other Operator) bool {
	if o.Name != other.Name {
		return false
	}
	if len(o.Args) != len(other.Args) {
		return false
	}
	for i := range o.Args {
		if !pdf.Equal(o.Args[i], other.Args[i]) {
			return false
		}
	}
	return true
}

// isValidName checks whether the operator name is valid for the given PDF version.
// It returns ErrUnknown if the operator is not recognized, ErrDeprecated if
// the operator is deprecated in the given version, or ErrVersion if the
// operator was not yet available in the given version.
func (name OpName) isValidName(v pdf.Version) error {
	info, ok := operators[name]
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
	Since      pdf.Version   // PDF version when introduced
	Deprecated pdf.Version   // PDF version when deprecated (0 if not deprecated)
	Allowed    Object        // bitmask of allowed object states
	Transition Object        // new state after operator (0 = no change)
	Sets       graphics.Bits // state bits this operator sets
	Requires   graphics.Bits // state bits required before this operator
}

// Common requirement sets for operators
const (
	// reqStroke is the set of state bits required for stroke operations
	reqStroke = graphics.StateLineWidth | graphics.StateLineCap | graphics.StateLineJoin |
		graphics.StateLineDash | graphics.StateStrokeColor

	// reqFill is the set of state bits required for fill operations
	reqFill = graphics.StateFillColor

	// reqTextShow is the set of state bits required for text showing operations
	reqTextShow = graphics.StateTextFont | graphics.StateTextMatrix |
		graphics.StateTextHorizontalScaling | graphics.StateTextRise |
		graphics.StateTextWordSpacing | graphics.StateTextCharacterSpacing

	// reqTextShowSetSpacing is for the " operator which sets word/char spacing
	reqTextShowSetSpacing = graphics.StateTextFont | graphics.StateTextMatrix |
		graphics.StateTextHorizontalScaling | graphics.StateTextRise
)

// operators maps operator names to their metadata
var operators = map[OpName]*opInfo{
	// General Graphics State (allowed in page and text contexts)
	OpPushGraphicsState:    {Since: pdf.V1_0, Allowed: ObjPage | ObjText},
	OpPopGraphicsState:     {Since: pdf.V1_0, Allowed: ObjPage | ObjText},
	OpTransform:            {Since: pdf.V1_0, Allowed: ObjPage}, // special graphics state
	OpSetLineWidth:         {Since: pdf.V1_0, Allowed: ObjPage | ObjText, Sets: graphics.StateLineWidth},
	OpSetLineCap:           {Since: pdf.V1_0, Allowed: ObjPage | ObjText, Sets: graphics.StateLineCap},
	OpSetLineJoin:          {Since: pdf.V1_0, Allowed: ObjPage | ObjText, Sets: graphics.StateLineJoin},
	OpSetMiterLimit:        {Since: pdf.V1_0, Allowed: ObjPage | ObjText, Sets: graphics.StateMiterLimit},
	OpSetLineDash:          {Since: pdf.V1_0, Allowed: ObjPage | ObjText, Sets: graphics.StateLineDash},
	OpSetRenderingIntent:   {Since: pdf.V1_1, Allowed: ObjPage | ObjText, Sets: graphics.StateRenderingIntent},
	OpSetFlatnessTolerance: {Since: pdf.V1_0, Allowed: ObjPage | ObjText, Sets: graphics.StateFlatnessTolerance},
	OpSetExtGState:         {Since: pdf.V1_2, Allowed: ObjPage | ObjText}, // Sets determined from resource

	// Path Construction
	OpMoveTo:    {Since: pdf.V1_0, Allowed: ObjPage | ObjPath, Transition: ObjPath},
	OpLineTo:    {Since: pdf.V1_0, Allowed: ObjPath},
	OpCurveTo:   {Since: pdf.V1_0, Allowed: ObjPath},
	OpCurveToV:  {Since: pdf.V1_0, Allowed: ObjPath},
	OpCurveToY:  {Since: pdf.V1_0, Allowed: ObjPath},
	OpClosePath: {Since: pdf.V1_0, Allowed: ObjPath},
	OpRectangle: {Since: pdf.V1_0, Allowed: ObjPage | ObjPath, Transition: ObjPath},

	// Path Painting (transitions back to page context)
	OpStroke:                    {Since: pdf.V1_0, Allowed: ObjPath | ObjClippingPath, Transition: ObjPage, Requires: reqStroke},
	OpCloseAndStroke:            {Since: pdf.V1_0, Allowed: ObjPath | ObjClippingPath, Transition: ObjPage, Requires: reqStroke},
	OpFill:                      {Since: pdf.V1_0, Allowed: ObjPath | ObjClippingPath, Transition: ObjPage, Requires: reqFill},
	OpFillCompat:                {Since: pdf.V1_0, Deprecated: pdf.V2_0, Allowed: ObjPath | ObjClippingPath, Transition: ObjPage, Requires: reqFill},
	OpFillEvenOdd:               {Since: pdf.V1_0, Allowed: ObjPath | ObjClippingPath, Transition: ObjPage, Requires: reqFill},
	OpFillAndStroke:             {Since: pdf.V1_0, Allowed: ObjPath | ObjClippingPath, Transition: ObjPage, Requires: reqStroke | reqFill},
	OpFillAndStrokeEvenOdd:      {Since: pdf.V1_0, Allowed: ObjPath | ObjClippingPath, Transition: ObjPage, Requires: reqStroke | reqFill},
	OpCloseFillAndStroke:        {Since: pdf.V1_0, Allowed: ObjPath | ObjClippingPath, Transition: ObjPage, Requires: reqStroke | reqFill},
	OpCloseFillAndStrokeEvenOdd: {Since: pdf.V1_0, Allowed: ObjPath | ObjClippingPath, Transition: ObjPage, Requires: reqStroke | reqFill},
	OpEndPath:                   {Since: pdf.V1_0, Allowed: ObjPath | ObjClippingPath, Transition: ObjPage},

	// Clipping Paths (transitions to clipping path context)
	OpClipNonZero: {Since: pdf.V1_0, Allowed: ObjPath, Transition: ObjClippingPath},
	OpClipEvenOdd: {Since: pdf.V1_0, Allowed: ObjPath, Transition: ObjClippingPath},

	// Text Objects
	OpTextBegin: {Since: pdf.V1_0, Allowed: ObjPage, Transition: ObjText, Sets: graphics.StateTextMatrix},
	OpTextEnd:   {Since: pdf.V1_0, Allowed: ObjText, Transition: ObjPage},

	// Text State (allowed in any context)
	OpTextSetCharacterSpacing:  {Since: pdf.V1_0, Allowed: ObjAny, Sets: graphics.StateTextCharacterSpacing},
	OpTextSetWordSpacing:       {Since: pdf.V1_0, Allowed: ObjAny, Sets: graphics.StateTextWordSpacing},
	OpTextSetHorizontalScaling: {Since: pdf.V1_0, Allowed: ObjAny, Sets: graphics.StateTextHorizontalScaling},
	OpTextSetLeading:           {Since: pdf.V1_0, Allowed: ObjAny, Sets: graphics.StateTextLeading},
	OpTextSetFont:              {Since: pdf.V1_0, Allowed: ObjAny, Sets: graphics.StateTextFont},
	OpTextSetRenderingMode:     {Since: pdf.V1_0, Allowed: ObjAny, Sets: graphics.StateTextRenderingMode},
	OpTextSetRise:              {Since: pdf.V1_0, Allowed: ObjAny, Sets: graphics.StateTextRise},

	// Text Positioning (only in text context)
	OpTextMoveOffset:           {Since: pdf.V1_0, Allowed: ObjText},
	OpTextMoveOffsetSetLeading: {Since: pdf.V1_0, Allowed: ObjText, Sets: graphics.StateTextLeading},
	OpTextSetMatrix:            {Since: pdf.V1_0, Allowed: ObjText, Sets: graphics.StateTextMatrix},
	OpTextNextLine:             {Since: pdf.V1_0, Allowed: ObjText, Requires: graphics.StateTextMatrix | graphics.StateTextLeading},

	// Text Showing (only in text context)
	OpTextShow:                       {Since: pdf.V1_0, Allowed: ObjText, Requires: reqTextShow},
	OpTextShowArray:                  {Since: pdf.V1_0, Allowed: ObjText, Requires: reqTextShow},
	OpTextShowMoveNextLine:           {Since: pdf.V1_0, Allowed: ObjText, Requires: reqTextShow | graphics.StateTextLeading},
	OpTextShowMoveNextLineSetSpacing: {Since: pdf.V1_0, Allowed: ObjText, Requires: reqTextShowSetSpacing, Sets: graphics.StateTextWordSpacing | graphics.StateTextCharacterSpacing},

	// Type 3 Fonts (only at start of Type 3 glyph)
	OpType3ColoredGlyph:   {Since: pdf.V1_0, Allowed: ObjType3Start, Transition: ObjPage},
	OpType3UncoloredGlyph: {Since: pdf.V1_0, Allowed: ObjType3Start, Transition: ObjPage},

	// Colour (allowed in page and text contexts)
	OpSetStrokeColorSpace: {Since: pdf.V1_1, Allowed: ObjPage | ObjText, Sets: graphics.StateStrokeColor},
	OpSetFillColorSpace:   {Since: pdf.V1_1, Allowed: ObjPage | ObjText, Sets: graphics.StateFillColor},
	OpSetStrokeColor:      {Since: pdf.V1_1, Allowed: ObjPage | ObjText, Sets: graphics.StateStrokeColor},
	OpSetStrokeColorN:     {Since: pdf.V1_2, Allowed: ObjPage | ObjText, Sets: graphics.StateStrokeColor},
	OpSetFillColor:        {Since: pdf.V1_1, Allowed: ObjPage | ObjText, Sets: graphics.StateFillColor},
	OpSetFillColorN:       {Since: pdf.V1_2, Allowed: ObjPage | ObjText, Sets: graphics.StateFillColor},
	OpSetStrokeGray:       {Since: pdf.V1_0, Allowed: ObjPage | ObjText, Sets: graphics.StateStrokeColor},
	OpSetFillGray:         {Since: pdf.V1_0, Allowed: ObjPage | ObjText, Sets: graphics.StateFillColor},
	OpSetStrokeRGB:        {Since: pdf.V1_0, Allowed: ObjPage | ObjText, Sets: graphics.StateStrokeColor},
	OpSetFillRGB:          {Since: pdf.V1_0, Allowed: ObjPage | ObjText, Sets: graphics.StateFillColor},
	OpSetStrokeCMYK:       {Since: pdf.V1_0, Allowed: ObjPage | ObjText, Sets: graphics.StateStrokeColor},
	OpSetFillCMYK:         {Since: pdf.V1_0, Allowed: ObjPage | ObjText, Sets: graphics.StateFillColor},

	// Shading Patterns (allowed in page and text contexts)
	OpShading: {Since: pdf.V1_3, Allowed: ObjPage | ObjText},

	// Inline Images (internal use, allowed in page and text)
	opBeginInlineImage: {Since: pdf.V1_0, Allowed: ObjPage | ObjText},
	opInlineImageData:  {Since: pdf.V1_0, Allowed: ObjPage | ObjText},
	opEndInlineImage:   {Since: pdf.V1_0, Allowed: ObjPage | ObjText},

	// XObjects (allowed in page and text contexts)
	OpXObject: {Since: pdf.V1_0, Allowed: ObjPage | ObjText},

	// Marked Content (allowed in page and text contexts)
	OpMarkedContentPoint:               {Since: pdf.V1_2, Allowed: ObjPage | ObjText},
	OpMarkedContentPointWithProperties: {Since: pdf.V1_2, Allowed: ObjPage | ObjText},
	OpBeginMarkedContent:               {Since: pdf.V1_2, Allowed: ObjPage | ObjText},
	OpBeginMarkedContentWithProperties: {Since: pdf.V1_2, Allowed: ObjPage | ObjText},
	OpEndMarkedContent:                 {Since: pdf.V1_2, Allowed: ObjPage | ObjText},

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
