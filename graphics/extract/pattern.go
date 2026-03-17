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
	"io"

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/pattern"
	"seehuhn.de/go/pdf/graphics/shading"
)

// Pattern extracts a pattern from a PDF file and returns a color.Pattern.
func Pattern(x *pdf.Extractor, path *pdf.CycleCheck, obj pdf.Object, isDirect bool) (color.Pattern, error) {

	// resolve references
	resolved, err := x.Resolve(path, obj)
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
	patternType, err := x.GetInteger(path, dict["PatternType"])
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
		return extractType1(x, path, stream)
	case 2:
		return extractType2(x, path, dict, isDirect)
	default:
		return nil, &pdf.MalformedFileError{
			Err: fmt.Errorf("unsupported pattern type %d", patternType),
		}
	}
}

// extractType2 extracts a Type2 (shading) pattern from a PDF dictionary.
func extractType2(x *pdf.Extractor, path *pdf.CycleCheck, dict pdf.Dict, isDirect bool) (*pattern.Type2, error) {
	// extract required Shading
	shadingObj := dict["Shading"]
	if shadingObj == nil {
		return nil, &pdf.MalformedFileError{
			Err: fmt.Errorf("missing Shading entry in type 2 pattern"),
		}
	}

	sh, err := pdf.ExtractorGet(x, path, shadingObj, shading.Extract)
	if err != nil {
		return nil, err
	}

	pat := &pattern.Type2{
		Shading:   sh,
		SingleUse: isDirect,
	}

	// extract optional Matrix
	pat.Matrix, err = pdf.GetMatrix(x.R, dict["Matrix"])
	if err != nil {
		pat.Matrix = matrix.Identity
	}

	// extract optional ExtGState
	if extGState, err := pdf.Optional(ExtGState(x, path, dict["ExtGState"], false)); err != nil {
		return nil, err
	} else {
		pat.ExtGState = extGState
	}

	return pat, nil
}

// extractType1 extracts a Type1 (tiling) pattern from a PDF stream.
func extractType1(x *pdf.Extractor, path *pdf.CycleCheck, stream *pdf.Stream) (*pattern.Type1, error) {
	dict := stream.Dict

	// extract required PaintType
	paintType, err := x.GetInteger(path, dict["PaintType"])
	if err != nil {
		return nil, &pdf.MalformedFileError{
			Err: fmt.Errorf("missing or invalid PaintType"),
		}
	}
	if paintType != 1 && paintType != 2 {
		return nil, pdf.Errorf("invalid PaintType: %d (must be 1 or 2)", paintType)
	}

	// extract required TilingType
	tilingType, err := x.GetInteger(path, dict["TilingType"])
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
	xStep, err := x.GetNumber(path, dict["XStep"])
	if err != nil {
		return nil, &pdf.MalformedFileError{
			Err: fmt.Errorf("missing or invalid XStep"),
		}
	}
	if xStep == 0 {
		return nil, pdf.Errorf("XStep cannot be zero")
	}

	// extract required YStep
	yStep, err := x.GetNumber(path, dict["YStep"])
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
		BBox:       *bbox,
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
		res, err := pdf.ExtractorGet(x, path, resObj, Resources)
		if err != nil {
			return nil, err
		}
		if res != nil {
			pat.Res = res
		}
	}

	// store a reader factory closure so each iteration re-opens the PDF stream
	stmType := content.PatternColored
	if !pat.Color {
		stmType = content.PatternUncolored
	}
	stm := stream // capture for closure
	getter := x.R
	version := pdf.GetVersion(x.R)
	res := pat.Res
	pat.Content = content.NewScanner(
		func() (io.ReadCloser, error) {
			return pdf.DecodeStream(getter, stm, 0)
		},
		version, stmType, res,
	)

	return pat, nil
}
