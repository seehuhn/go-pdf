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

package annotation

import "seehuhn.de/go/pdf"

// StrikeOut represents a strikeout annotation that appears as struck-out text.
// When opened, it displays a popup window containing the text of the associated note.
type StrikeOut struct {
	Common
	Markup

	// QuadPoints (required) is an array of 8×n numbers specifying the coordinates
	// of n quadrilaterals in default user space. Each quadrilateral encompasses
	// a word or group of contiguous words in the text underlying the annotation.
	// The coordinates for each quadrilateral are given in the order:
	// x1 y1 x2 y2 x3 y3 x4 y4
	// specifying the quadrilateral's four vertices in counterclockwise order.
	QuadPoints []float64
}

var _ pdf.Annotation = (*StrikeOut)(nil)

// AnnotationType returns "StrikeOut".
// This implements the [pdf.Annotation] interface.
func (s *StrikeOut) AnnotationType() pdf.Name {
	return "StrikeOut"
}

func extractStrikeOut(r pdf.Getter, dict pdf.Dict) (*StrikeOut, error) {
	strikeOut := &StrikeOut{}

	// Extract common annotation fields
	if err := extractCommon(r, dict, &strikeOut.Common); err != nil {
		return nil, err
	}

	// Extract markup annotation fields
	if err := extractMarkup(r, dict, &strikeOut.Markup); err != nil {
		return nil, err
	}

	// Extract strikeout-specific fields
	// QuadPoints (required)
	if quadPoints, err := pdf.GetArray(r, dict["QuadPoints"]); err == nil && len(quadPoints) > 0 {
		coords := make([]float64, len(quadPoints))
		for i, point := range quadPoints {
			if num, err := pdf.GetNumber(r, point); err == nil {
				coords[i] = float64(num)
			}
		}
		strikeOut.QuadPoints = coords
	}

	return strikeOut, nil
}

func (s *StrikeOut) Embed(rm *pdf.ResourceManager) (pdf.Native, pdf.Unused, error) {
	var zero pdf.Unused

	if err := pdf.CheckVersion(rm.Out, "strikeout annotation", pdf.V1_3); err != nil {
		return nil, zero, err
	}

	dict := pdf.Dict{
		"Type":    pdf.Name("Annot"),
		"Subtype": pdf.Name("StrikeOut"),
	}

	// Add common annotation fields
	if err := s.Common.fillDict(rm, dict); err != nil {
		return nil, zero, err
	}

	// Add markup annotation fields
	if err := s.Markup.fillDict(rm, dict); err != nil {
		return nil, zero, err
	}

	// Add strikeout-specific fields
	// QuadPoints (required)
	if len(s.QuadPoints) > 0 {
		quadArray := make(pdf.Array, len(s.QuadPoints))
		for i, point := range s.QuadPoints {
			quadArray[i] = pdf.Number(point)
		}
		dict["QuadPoints"] = quadArray
	}

	return dict, zero, nil
}
