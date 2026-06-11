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
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/annotation/colorenc"
	"seehuhn.de/go/pdf/measure"
)

func decodePolygon(x *pdf.Extractor, path *pdf.CycleCheck, dict pdf.Dict) (*annotation.Polygon, error) {
	polygon := &annotation.Polygon{}

	// Extract common annotation fields
	if err := decodeCommon(x, path, &polygon.Common, dict); err != nil {
		return nil, err
	}

	// Extract markup annotation fields
	if err := decodeMarkup(x, path, dict, &polygon.Markup); err != nil {
		return nil, err
	}

	// Extract polygon-specific fields
	// Vertices (required)
	vertices, err := pdf.GetFloatArray(x.R, dict["Vertices"])
	if err != nil {
		return nil, pdf.Wrap(err, "Vertices")
	}
	if len(vertices) < 4 {
		return nil, pdf.Error("polygon annotation requires Vertices")
	}
	polygon.Vertices = vertices[:len(vertices)&^1]

	// BS (optional)
	if bs, err := pdf.ExtractorGetOptional(x, path, dict["BS"], annotation.ExtractBorderStyle); err != nil {
		return nil, err
	} else {
		polygon.BorderStyle = bs
		if bs != nil {
			// per PDF spec, Border is ignored when BS is present
			polygon.Common.Border = nil
		}
	}

	// IC (optional)
	if ic, err := pdf.Optional(colorenc.Extract(x.R, dict["IC"])); err != nil {
		return nil, err
	} else {
		polygon.FillColor = ic
	}

	// BE (optional)
	if be, err := pdf.ExtractorGetOptional(x, path, dict["BE"], annotation.ExtractBorderEffect); err != nil {
		return nil, err
	} else {
		polygon.BorderEffect = be
	}

	// Measure (optional)
	if m, err := pdf.ExtractorGetOptional(x, path, dict["Measure"], measure.Extract); err != nil {
		return nil, err
	} else {
		polygon.Measure = m
	}

	// Path (optional; PDF 2.0)
	if p, err := decodePath(x, path, dict["Path"]); err != nil {
		return nil, err
	} else {
		polygon.Path = p
	}

	return polygon, nil
}
