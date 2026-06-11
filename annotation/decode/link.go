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
	"seehuhn.de/go/pdf/action"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/destination"
)

func decodeLink(x *pdf.Extractor, path *pdf.CycleCheck, dict pdf.Dict) (*annotation.Link, error) {
	link := &annotation.Link{}

	// Extract common annotation fields
	if err := decodeCommon(x, path, &link.Common, dict); err != nil {
		return nil, err
	}

	// Extract link-specific fields
	if dict["A"] != nil {
		act, err := pdf.ExtractorGetOptional(x, path, dict["A"], action.Decode)
		if err != nil {
			return nil, err
		}
		link.Action = act
	}
	if dict["Dest"] != nil && link.Action == nil {
		dest, err := pdf.ExtractorGetOptional(x, path, dict["Dest"], destination.Decode)
		if err != nil {
			return nil, err
		}
		link.Destination = dest
	}

	link.Highlight = decodeHighlight(x, path, dict["H"])

	if pa, ok := dict["PA"].(pdf.Reference); ok {
		link.Backup = pa
	}

	if quadPoints, err := pdf.GetFloatArray(x.R, dict["QuadPoints"]); err == nil && len(quadPoints) >= 8 {
		// process floats in groups of 8, each group becomes 4 Vec2 points
		numCompleteQuads := len(quadPoints) / 8
		points := make([]vec.Vec2, numCompleteQuads*4)
		for quad := range numCompleteQuads {
			for corner := range 4 {
				idx := quad*8 + corner*2
				points[quad*4+corner] = vec.Vec2{X: quadPoints[idx], Y: quadPoints[idx+1]}
			}
		}
		link.QuadPoints = points
	}

	if bs, err := pdf.ExtractorGetOptional(x, path, dict["BS"], annotation.ExtractBorderStyle); err != nil {
		return nil, err
	} else {
		link.BorderStyle = bs
		if bs != nil {
			// per PDF spec, Border is ignored when BS is present
			link.Common.Border = nil
		}
	}

	return link, nil
}
