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

package markarray

import (
	"seehuhn.de/go/pdf/font/parser"
	"seehuhn.de/go/pdf/font/sfnt/opentype/anchor"
)

// Record is a mark record in a Mark Array Table.
// Each mark record defines the class of the mark and its anchor point.
// https://docs.microsoft.com/en-us/typography/opentype/spec/gpos#mark-array-table
type Record struct {
	Class uint16
	anchor.Table
}

// Read reads a Mark Array Table from the given parser.
// If there are more than numMarks entries in the table, the remaining entries
// are ignored.
func Read(p *parser.Parser, pos int64, numMarks int) ([]Record, error) {
	err := p.SeekPos(pos)
	if err != nil {
		return nil, err
	}

	markCount, err := p.ReadUint16()
	if err != nil {
		return nil, err
	}
	if int(markCount) > numMarks {
		markCount = uint16(numMarks)
	}

	res := make([]Record, markCount)
	offsets := make([]uint16, markCount)
	for i := 0; i < int(markCount); i++ {
		res[i].Class, err = p.ReadUint16()
		if err != nil {
			return nil, err
		}

		offsets[i], err = p.ReadUint16()
		if err != nil {
			return nil, err
		}
	}

	for i, offs := range offsets {
		res[i].Table, err = anchor.Read(p, pos+int64(offs))
		if err != nil {
			return nil, err
		}
	}
	return res, nil
}
