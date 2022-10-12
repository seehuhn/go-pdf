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

package gtab

import (
	"seehuhn.de/go/pdf/sfnt/glyph"
	"seehuhn.de/go/pdf/sfnt/parser"
)

// ReadGIDSlice reads a length followed by a sequence of GlyphID values.
func readGIDSlice(p *parser.Parser) ([]glyph.ID, error) {
	n, err := p.ReadUint16()
	if err != nil {
		return nil, err
	}
	res := make([]glyph.ID, n)
	for i := range res {
		val, err := p.ReadUint16()
		if err != nil {
			return nil, err
		}
		res[i] = glyph.ID(val)
	}
	return res, nil
}
