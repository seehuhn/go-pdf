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
)

func decodeSquare(c pdf.Cursor, dict pdf.Dict) (*annotation.Square, error) {
	square := &annotation.Square{}

	if err := decodeCommon(c, &square.Common, dict); err != nil {
		return nil, err
	}

	// Extract markup annotation fields
	if err := decodeMarkup(c, dict, &square.Markup); err != nil {
		return nil, err
	}

	// Extract square-specific fields
	// BS (optional)
	if bs, err := pdf.DecodeOptional(c, dict["BS"], annotation.ExtractBorderStyle); err != nil {
		return nil, err
	} else {
		square.BorderStyle = bs
		if bs != nil {
			// per PDF spec, Border is ignored when BS is present
			square.Common.Border = nil
		}
	}

	// BE (optional): a border effect is meaningful only together with a
	// border style, so drop it when BS is absent (the writer requires BS)
	if be, err := pdf.DecodeOptional(c, dict["BE"], annotation.ExtractBorderEffect); err != nil {
		return nil, err
	} else if square.BorderStyle != nil {
		square.BorderEffect = be
	}

	// IC (optional)
	if ic, err := pdf.Optional(colorenc.Extract(c, dict["IC"])); err != nil {
		return nil, err
	} else {
		square.FillColor = ic
	}

	// RD (optional)
	if rd, err := c.FloatArray(dict["RD"]); err == nil && len(rd) == 4 {
		for i := range rd {
			rd[i] = max(rd[i], 0)
		}
		if rd[0]+rd[2] < square.Rect.Dx() && rd[1]+rd[3] < square.Rect.Dy() {
			square.Margin = rd
		}
	}

	return square, nil
}
