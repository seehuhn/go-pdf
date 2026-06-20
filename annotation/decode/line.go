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

func decodeLine(c pdf.Cursor, dict pdf.Dict) (*annotation.Line, error) {
	line := &annotation.Line{}

	// Extract common annotation fields
	if err := decodeCommon(c, &line.Common, dict); err != nil {
		return nil, err
	}

	// Extract markup annotation fields
	if err := decodeMarkup(c, dict, &line.Markup); err != nil {
		return nil, err
	}

	// Extract line-specific fields
	// L (required)
	if l, err := c.Array(dict["L"]); err == nil && len(l) == 4 {
		for i, coord := range l {
			if num, err := c.Number(coord); err == nil {
				line.Coords[i] = num
			}
		}
	}

	// BS (optional)
	if bs, err := pdf.DecodeOptional(c, dict["BS"], annotation.ExtractBorderStyle); err != nil {
		return nil, err
	} else {
		line.BorderStyle = bs
		if bs != nil {
			// per PDF spec, Border is ignored when BS is present
			line.Common.Border = nil
		}
	}

	// LE (optional; PDF 1.4) - default is [None, None]
	line.LineEndingStyle = [2]annotation.LineEndingStyle{annotation.LineEndingStyleNone, annotation.LineEndingStyleNone}
	if le, err := pdf.Optional(c.Array(dict["LE"])); err != nil {
		return nil, err
	} else if len(le) >= 1 {
		if name, err := c.Name(le[0]); err == nil {
			line.LineEndingStyle[0] = annotation.LineEndingStyle(name)
		}
		if len(le) >= 2 {
			if name, err := c.Name(le[1]); err == nil {
				line.LineEndingStyle[1] = annotation.LineEndingStyle(name)
			}
		} else {
			// if only one element, copy first element to second
			line.LineEndingStyle[1] = line.LineEndingStyle[0]
		}
	}

	// IC (optional; PDF 1.4)
	if ic, err := pdf.Optional(colorenc.Extract(c, dict["IC"])); err != nil {
		return nil, err
	} else {
		line.FillColor = ic
	}

	// Cap (optional)
	if cap, err := c.Boolean(dict["Cap"]); err == nil {
		line.Caption = bool(cap)
	}

	if line.Caption {
		// CP (optional)
		if cp, err := c.Name(dict["CP"]); err == nil && cp == "Top" {
			line.CaptionAbove = true
		}

		// CO (optional)
		if co, err := pdf.Optional(c.FloatArray(dict["CO"])); err != nil {
			return nil, err
		} else if len(co) == 2 {
			line.CaptionOffset = co
		}
	}

	// LL (optional)
	if ll, err := pdf.Optional(c.Number(dict["LL"])); err != nil {
		return nil, err
	} else {
		line.LL = ll
	}

	// LLE (optional)
	if lle, err := pdf.Optional(c.Number(dict["LLE"])); err != nil {
		return nil, err
	} else {
		line.LLE = max(lle, 0)
	}

	// LLO (optional)
	if llo, err := pdf.Optional(c.Number(dict["LLO"])); err != nil {
		return nil, err
	} else {
		line.LLO = max(llo, 0)
	}

	// Measure (optional)
	if m, err := pdf.DecodeOptional(c, dict["Measure"], measure.Extract); err != nil {
		return nil, err
	} else {
		line.Measure = m
	}

	return line, nil
}
