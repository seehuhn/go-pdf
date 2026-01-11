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

package extract

import (
	"fmt"

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/pattern"
	"seehuhn.de/go/pdf/graphics/shading"
)

// Pattern extracts a pattern from a PDF file and returns a color.Pattern.
func Pattern(x *pdf.Extractor, obj pdf.Object) (color.Pattern, error) {
	// check if original object was a reference before resolving
	_, isIndirect := obj.(pdf.Reference)

	// resolve references
	resolved, err := x.Resolve(obj)
	if err != nil {
		return nil, err
	} else if resolved == nil {
		return nil, &pdf.MalformedFileError{
			Err: fmt.Errorf("missing pattern object"),
		}
	}

	// extract dict (works for both Dict and Stream)
	var dict pdf.Dict
	switch resolved := resolved.(type) {
	case pdf.Dict:
		dict = resolved
	case *pdf.Stream:
		dict = resolved.Dict
	default:
		return nil, &pdf.MalformedFileError{
			Err: fmt.Errorf("pattern must be dictionary or stream"),
		}
	}

	// read PatternType (required)
	patternType, err := x.GetInteger(dict["PatternType"])
	if err != nil {
		return nil, &pdf.MalformedFileError{
			Err: fmt.Errorf("missing or invalid PatternType"),
		}
	}

	// dispatch based on type
	switch patternType {
	case 1:
		stream, ok := resolved.(*pdf.Stream)
		if !ok {
			return nil, &pdf.MalformedFileError{
				Err: fmt.Errorf("type 1 pattern must be a stream"),
			}
		}
		return extractType1(x, stream)
	case 2:
		return extractType2(x, dict, isIndirect)
	default:
		return nil, &pdf.MalformedFileError{
			Err: fmt.Errorf("unsupported pattern type %d", patternType),
		}
	}
}

// extractType2 extracts a Type2 (shading) pattern from a PDF dictionary.
func extractType2(x *pdf.Extractor, dict pdf.Dict, isIndirect bool) (*pattern.Type2, error) {
	// extract required Shading
	shadingObj := dict["Shading"]
	if shadingObj == nil {
		return nil, &pdf.MalformedFileError{
			Err: fmt.Errorf("missing Shading entry in type 2 pattern"),
		}
	}

	sh, err := shading.Extract(x, shadingObj)
	if err != nil {
		return nil, err
	}

	pat := &pattern.Type2{
		Shading:   sh,
		SingleUse: !isIndirect && !x.IsIndirect,
	}

	// extract optional Matrix
	pat.Matrix, err = pdf.GetMatrix(x.R, dict["Matrix"])
	if err != nil {
		pat.Matrix = matrix.Identity
	}

	// extract optional ExtGState
	if extGState, err := pdf.Optional(ExtGState(x, dict["ExtGState"])); err != nil {
		return nil, err
	} else {
		pat.ExtGState = extGState
	}

	return pat, nil
}

// extractType1 extracts a Type1 (tiling) pattern from a PDF stream.
func extractType1(x *pdf.Extractor, stream *pdf.Stream) (*pattern.Type1, error) {
	dict := stream.Dict

	// extract required PaintType
	paintType, err := x.GetInteger(dict["PaintType"])
	if err != nil {
		return nil, &pdf.MalformedFileError{
			Err: fmt.Errorf("missing or invalid PaintType"),
		}
	}
	if paintType != 1 && paintType != 2 {
		return nil, pdf.Errorf("invalid PaintType: %d (must be 1 or 2)", paintType)
	}

	// extract required TilingType
	tilingType, err := x.GetInteger(dict["TilingType"])
	if err != nil {
		return nil, &pdf.MalformedFileError{
			Err: fmt.Errorf("missing or invalid TilingType"),
		}
	}
	if tilingType < 1 || tilingType > 3 {
		return nil, pdf.Errorf("invalid TilingType: %d (must be 1, 2, or 3)", tilingType)
	}

	// extract required BBox
	bbox, err := pdf.GetRectangle(x.R, dict["BBox"])
	if err != nil || bbox == nil || bbox.IsZero() {
		return nil, &pdf.MalformedFileError{
			Err: fmt.Errorf("missing or invalid BBox"),
		}
	}

	// extract required XStep
	xStep, err := x.GetNumber(dict["XStep"])
	if err != nil {
		return nil, &pdf.MalformedFileError{
			Err: fmt.Errorf("missing or invalid XStep"),
		}
	}
	if xStep == 0 {
		return nil, pdf.Errorf("XStep cannot be zero")
	}

	// extract required YStep
	yStep, err := x.GetNumber(dict["YStep"])
	if err != nil {
		return nil, &pdf.MalformedFileError{
			Err: fmt.Errorf("missing or invalid YStep"),
		}
	}
	if yStep == 0 {
		return nil, pdf.Errorf("YStep cannot be zero")
	}

	pat := &pattern.Type1{
		TilingType: int(tilingType),
		BBox:       bbox,
		XStep:      xStep,
		YStep:      yStep,
		Color:      paintType == 1,
	}

	// extract optional Matrix
	pat.Matrix, err = pdf.GetMatrix(x.R, dict["Matrix"])
	if err != nil {
		pat.Matrix = matrix.Identity
	}

	// extract resources (required)
	pat.Res = &content.Resources{}
	if resObj := dict["Resources"]; resObj != nil {
		res, err := Resources(x, resObj)
		if err != nil {
			return nil, err
		}
		if res != nil {
			pat.Res = res
		}
	}

	// read content stream
	stmType := content.PatternColored
	if !pat.Color {
		stmType = content.PatternUncolored
	}
	stm, err := pdf.DecodeStream(x.R, stream, 0)
	if err != nil {
		return nil, err
	}

	pat.Content, err = content.ReadStream(stm, pdf.GetVersion(x.R), stmType, pat.Res)
	closeErr := stm.Close()
	if err != nil {
		return nil, err
	}
	if closeErr != nil {
		return nil, closeErr
	}

	return pat, nil
}
