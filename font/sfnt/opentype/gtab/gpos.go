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
	"fmt"

	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/parser"
)

// readGposSubtable reads a GPOS subtable.
// This function can be used as the SubtableReader argument to Read().
func readGposSubtable(p *parser.Parser, pos int64, meta *LookupMetaInfo) (Subtable, error) {
	err := p.SeekPos(pos)
	if err != nil {
		return nil, err
	}

	format, err := p.ReadUInt16()
	if err != nil {
		return nil, err
	}

	switch 10*meta.LookupType + format {
	default:
		msg := fmt.Sprintf("GPOS %d.%d\n", meta.LookupType, format)
		fmt.Print(msg)
		return notImplementedGposSubtable(format), nil
	}
}

type notImplementedGposSubtable uint16

func (st notImplementedGposSubtable) Apply(meta *LookupMetaInfo, _ []font.Glyph, _ int) ([]font.Glyph, int) {
	msg := fmt.Sprintf("GPOS lookup type %d, format %d not implemented",
		meta.LookupType, st)
	panic(msg)
}

func (st notImplementedGposSubtable) EncodeLen(meta *LookupMetaInfo) int {
	msg := fmt.Sprintf("GPOS lookup type %d, format %d not implemented",
		meta.LookupType, st)
	panic(msg)
}

func (st notImplementedGposSubtable) Encode(meta *LookupMetaInfo) []byte {
	msg := fmt.Sprintf("GPOS lookup type %d, format %d not implemented",
		meta.LookupType, st)
	panic(msg)
}
