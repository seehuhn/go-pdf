// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2022  Jochen Voss <voss@seehuhn.de>
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

package glyf

import "seehuhn.de/go/pdf/font"

// Glyph represents a single glyph in a TrueType font.
type Glyph struct {
	numCont int16
	xMin    int16
	yMin    int16
	xMax    int16
	yMax    int16

	tail []byte
}

func decodeGlyph(data []byte) (*Glyph, error) {
	if len(data) == 0 {
		return nil, nil
	} else if len(data) < 10 {
		return nil, &font.InvalidFontError{
			SubSystem: "sfnt/glyf",
			Reason:    "incomplete glyph header",
		}
	}

	numCont := int16(data[0])<<8 | int16(data[1])
	if numCont < 0 {
		numCont = -1
	}
	g := &Glyph{
		numCont: numCont,
		xMin:    int16(data[2])<<8 | int16(data[3]),
		yMin:    int16(data[4])<<8 | int16(data[5]),
		xMax:    int16(data[6])<<8 | int16(data[7]),
		yMax:    int16(data[8])<<8 | int16(data[9]),

		tail: data[10:],
	}

	return g, nil
}

func (g *Glyph) encode() []byte {
	if g == nil {
		return nil
	}

	data := make([]byte, 10+len(g.tail))
	data[0] = byte(g.numCont >> 8)
	data[1] = byte(g.numCont)
	data[2] = byte(g.xMin >> 8)
	data[3] = byte(g.xMin)
	data[4] = byte(g.yMin >> 8)
	data[5] = byte(g.yMin)
	data[6] = byte(g.xMax >> 8)
	data[7] = byte(g.xMax)
	data[8] = byte(g.yMax >> 8)
	data[9] = byte(g.yMax)
	copy(data[10:], g.tail)
	return data
}
