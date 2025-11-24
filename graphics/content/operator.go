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
}

// operators maps operator names to their metadata
var operators = map[OpName]*opInfo{
	// General Graphics State
	OpPushGraphicsState:    {Since: pdf.V1_0},
	OpPopGraphicsState:     {Since: pdf.V1_0},
	OpTransform:            {Since: pdf.V1_0},
	OpSetLineWidth:         {Since: pdf.V1_0},
	OpSetLineCap:           {Since: pdf.V1_0},
	OpSetLineJoin:          {Since: pdf.V1_0},
	OpSetMiterLimit:        {Since: pdf.V1_0},
	OpSetLineDash:          {Since: pdf.V1_0},
	OpSetRenderingIntent:   {Since: pdf.V1_1},
	OpSetFlatnessTolerance: {Since: pdf.V1_0},
	OpSetExtGState:         {Since: pdf.V1_2},

	// Path Construction
	OpMoveTo:    {Since: pdf.V1_0},
	OpLineTo:    {Since: pdf.V1_0},
	OpCurveTo:   {Since: pdf.V1_0},
	OpCurveToV:  {Since: pdf.V1_0},
	OpCurveToY:  {Since: pdf.V1_0},
	OpClosePath: {Since: pdf.V1_0},
	OpRectangle: {Since: pdf.V1_0},

	// Path Painting
	OpStroke:                    {Since: pdf.V1_0},
	OpCloseAndStroke:            {Since: pdf.V1_0},
	OpFill:                      {Since: pdf.V1_0},
	OpFillCompat:                {Since: pdf.V1_0, Deprecated: pdf.V2_0},
	OpFillEvenOdd:               {Since: pdf.V1_0},
	OpFillAndStroke:             {Since: pdf.V1_0},
	OpFillAndStrokeEvenOdd:      {Since: pdf.V1_0},
	OpCloseFillAndStroke:        {Since: pdf.V1_0},
	OpCloseFillAndStrokeEvenOdd: {Since: pdf.V1_0},
	OpEndPath:                   {Since: pdf.V1_0},

	// Clipping Paths
	OpClipNonZero: {Since: pdf.V1_0},
	OpClipEvenOdd: {Since: pdf.V1_0},

	// Text Objects
	OpTextBegin: {Since: pdf.V1_0},
	OpTextEnd:   {Since: pdf.V1_0},

	// Text State
	OpTextSetCharacterSpacing:  {Since: pdf.V1_0},
	OpTextSetWordSpacing:       {Since: pdf.V1_0},
	OpTextSetHorizontalScaling: {Since: pdf.V1_0},
	OpTextSetLeading:           {Since: pdf.V1_0},
	OpTextSetFont:              {Since: pdf.V1_0},
	OpTextSetRenderingMode:     {Since: pdf.V1_0},
	OpTextSetRise:              {Since: pdf.V1_0},

	// Text Positioning
	OpTextMoveOffset:           {Since: pdf.V1_0},
	OpTextMoveOffsetSetLeading: {Since: pdf.V1_0},
	OpTextSetMatrix:            {Since: pdf.V1_0},
	OpTextNextLine:             {Since: pdf.V1_0},

	// Text Showing
	OpTextShow:                       {Since: pdf.V1_0},
	OpTextShowArray:                  {Since: pdf.V1_0},
	OpTextShowMoveNextLine:           {Since: pdf.V1_0},
	OpTextShowMoveNextLineSetSpacing: {Since: pdf.V1_0},

	// Type 3 Fonts
	OpType3SetWidthOnly:           {Since: pdf.V1_0},
	OpType3SetWidthAndBoundingBox: {Since: pdf.V1_0},

	// Colour
	OpSetStrokeColorSpace: {Since: pdf.V1_1},
	OpSetFillColorSpace:   {Since: pdf.V1_1},
	OpSetStrokeColor:      {Since: pdf.V1_1},
	OpSetStrokeColorN:     {Since: pdf.V1_2},
	OpSetFillColor:        {Since: pdf.V1_1},
	OpSetFillColorN:       {Since: pdf.V1_2},
	OpSetStrokeGray:       {Since: pdf.V1_0},
	OpSetFillGray:         {Since: pdf.V1_0},
	OpSetStrokeRGB:        {Since: pdf.V1_0},
	OpSetFillRGB:          {Since: pdf.V1_0},
	OpSetStrokeCMYK:       {Since: pdf.V1_0},
	OpSetFillCMYK:         {Since: pdf.V1_0},

	// Shading Patterns
	OpShading: {Since: pdf.V1_3},

	// Inline Images
	opBeginInlineImage: {Since: pdf.V1_0},
	opInlineImageData:  {Since: pdf.V1_0},
	opEndInlineImage:   {Since: pdf.V1_0},

	// XObjects
	OpXObject: {Since: pdf.V1_0},

	// Marked Content
	OpMarkedContentPoint:               {Since: pdf.V1_2},
	OpMarkedContentPointWithProperties: {Since: pdf.V1_2},
	OpBeginMarkedContent:               {Since: pdf.V1_2},
	OpBeginMarkedContentWithProperties: {Since: pdf.V1_2},
	OpEndMarkedContent:                 {Since: pdf.V1_2},

	// Compatibility
	OpBeginCompatibility: {Since: pdf.V1_1},
	OpEndCompatibility:   {Since: pdf.V1_1},
}
