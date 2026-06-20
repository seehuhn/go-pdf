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

package decode

import (
	"seehuhn.de/go/geom/vec"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
)

func decodeTextMarkup(c pdf.Cursor, dict pdf.Dict, subtype pdf.Name) (*annotation.TextMarkup, error) {
	textMarkup := &annotation.TextMarkup{}

	textMarkup.Type = annotation.TextMarkupType(subtype)

	// Extract common annotation fields
	if err := decodeCommon(c, &textMarkup.Common, dict); err != nil {
		return nil, err
	}

	// Extract markup annotation fields
	if err := decodeMarkup(c, dict, &textMarkup.Markup); err != nil {
		return nil, err
	}

	// Extract text markup-specific fields
	// QuadPoints (required)
	quadPoints, err := c.FloatArray(dict["QuadPoints"])
	if err != nil {
		return nil, pdf.Wrap(err, "QuadPoints")
	}
	if len(quadPoints) < 8 {
		return nil, pdf.Error("QuadPoints is required for text markup annotations and must contain at least one quadrilateral (8 values)")
	}

	// process floats in groups of 8, each group becomes 4 Vec2 points
	numCompleteQuads := len(quadPoints) / 8
	points := make([]vec.Vec2, numCompleteQuads*4)
	for quad := range numCompleteQuads {
		for corner := range 4 {
			idx := quad*8 + corner*2
			points[quad*4+corner] = vec.Vec2{X: quadPoints[idx], Y: quadPoints[idx+1]}
		}
	}
	textMarkup.QuadPoints = points

	return textMarkup, nil
}
