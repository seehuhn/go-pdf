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

package gpos

import (
	"fmt"

	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/parser"
	"seehuhn.de/go/pdf/font/sfnt/opentype/gtab"
)

func ReadGposSubtable(p *parser.Parser, pos int64, meta *gtab.LookupMetaInfo) (gtab.Subtable, error) {
	format, err := p.ReadUInt16()
	if err != nil {
		return nil, err
	}

	switch 10*meta.LookupType + format {
	default:
		msg := fmt.Sprintf("GPOS %d.%d\n", meta.LookupType, format)
		fmt.Print(msg)
		return notImplementedSubtable(format), nil
	}
}

type notImplementedSubtable uint16

func (st notImplementedSubtable) Apply(meta *gtab.LookupMetaInfo, _ []font.Glyph, _ int) ([]font.Glyph, int) {
	msg := fmt.Sprintf("GPOS lookup type %d, format %d not implemented",
		meta.LookupType, st)
	panic(msg)
}

func (st notImplementedSubtable) EncodeLen(meta *gtab.LookupMetaInfo) int {
	msg := fmt.Sprintf("GPOS lookup type %d, format %d not implemented",
		meta.LookupType, st)
	panic(msg)
}

func (st notImplementedSubtable) Encode(meta *gtab.LookupMetaInfo) []byte {
	msg := fmt.Sprintf("GPOS lookup type %d, format %d not implemented",
		meta.LookupType, st)
	panic(msg)
}
