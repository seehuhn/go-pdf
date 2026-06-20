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

func decodePolyline(c pdf.Cursor, dict pdf.Dict) (*annotation.PolyLine, error) {
	polyline := &annotation.PolyLine{}

	// Extract common annotation fields
	if err := decodeCommon(c, &polyline.Common, dict); err != nil {
		return nil, err
	}

	// Extract markup annotation fields
	if err := decodeMarkup(c, dict, &polyline.Markup); err != nil {
		return nil, err
	}

	// Extract polyline-specific fields
	// Vertices (required)
	vertices, err := c.FloatArray(dict["Vertices"])
	if err != nil {
		return nil, pdf.Wrap(err, "Vertices")
	}
	if len(vertices) < 4 {
		return nil, pdf.Error("polyline annotation requires Vertices")
	}
	polyline.Vertices = vertices[:len(vertices)&^1]

	// LE (optional) - default is [None, None]
	polyline.LineEndingStyle = [2]annotation.LineEndingStyle{annotation.LineEndingStyleNone, annotation.LineEndingStyleNone}
	if le, err := pdf.Optional(c.Array(dict["LE"])); err != nil {
		return nil, err
	} else if len(le) >= 1 {
		if name, err := c.Name(le[0]); err == nil {
			polyline.LineEndingStyle[0] = annotation.LineEndingStyle(name)
		}
		if len(le) >= 2 {
			if name, err := c.Name(le[1]); err == nil {
				polyline.LineEndingStyle[1] = annotation.LineEndingStyle(name)
			}
		} else {
			// if only one element, copy first element to second
			polyline.LineEndingStyle[1] = polyline.LineEndingStyle[0]
		}
	}

	// BS (optional)
	if bs, err := pdf.DecodeOptional(c, dict["BS"], annotation.ExtractBorderStyle); err != nil {
		return nil, err
	} else {
		polyline.BorderStyle = bs
		if bs != nil {
			// per PDF spec, Border is ignored when BS is present
			polyline.Common.Border = nil
		}
	}

	// IC (optional)
	if ic, err := pdf.Optional(colorenc.Extract(c, dict["IC"])); err != nil {
		return nil, err
	} else {
		polyline.FillColor = ic
	}

	// Measure (optional)
	if m, err := pdf.DecodeOptional(c, dict["Measure"], measure.Extract); err != nil {
		return nil, err
	} else {
		polyline.Measure = m
	}

	// Path (optional; PDF 2.0)
	if p, err := decodePath(c, dict["Path"]); err != nil {
		return nil, err
	} else {
		polyline.Path = p
	}

	return polyline, nil
}
