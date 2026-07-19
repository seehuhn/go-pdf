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

func decodeInk(c pdf.Cursor, dict pdf.Dict) (*annotation.Ink, error) {
	ink := &annotation.Ink{}

	if err := decodeCommon(c, &ink.Common, dict); err != nil {
		return nil, err
	}

	// Extract markup annotation fields
	if err := decodeMarkup(c, dict, &ink.Markup); err != nil {
		return nil, err
	}

	// InkList (required)
	inkList, err := c.Array(dict["InkList"])
	if err != nil {
		return nil, pdf.Wrap(err, "InkList")
	}
	if len(inkList) == 0 {
		return nil, pdf.Error("ink annotation requires InkList")
	}
	paths := make([][]vec.Vec2, len(inkList))
	for i, pathEntry := range inkList {
		coords, err := pdf.Optional(c.FloatArray(pathEntry))
		if err != nil {
			return nil, err
		}
		// pair consecutive floats; silently drop an odd trailing coordinate
		n := len(coords) / 2
		pts := make([]vec.Vec2, n)
		for j := range n {
			pts[j] = vec.Vec2{X: coords[2*j], Y: coords[2*j+1]}
		}
		paths[i] = pts
	}
	ink.InkList = paths

	// BS (optional)
	if bs, err := pdf.DecodeOptional(c, dict["BS"], annotation.ExtractBorderStyle); err != nil {
		return nil, err
	} else {
		ink.BorderStyle = bs
		if bs != nil {
			// per PDF spec, Border is ignored when BS is present
			ink.Common.Border = nil
		}
	}

	// Path (optional; PDF 2.0)
	if p, err := decodePath(c, dict["Path"]); err != nil {
		return nil, err
	} else {
		ink.Path = p
	}

	return ink, nil
}
