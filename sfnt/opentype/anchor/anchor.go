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

package anchor

import (
	"fmt"

	"seehuhn.de/go/pdf/sfnt/fonterror"
	"seehuhn.de/go/pdf/sfnt/funit"
	"seehuhn.de/go/pdf/sfnt/parser"
)

// Table is an OpenType "Anchor Table".
// https://docs.microsoft.com/en-us/typography/opentype/spec/gpos#anchor-tables
type Table struct {
	X, Y funit.Int16
}

// Read reads an anchor table from the given parser.
func Read(p *parser.Parser, pos int64) (Table, error) {
	err := p.SeekPos(pos)
	if err != nil {
		return Table{}, err
	}

	buf, err := p.ReadBytes(6)
	if err != nil {
		return Table{}, err
	}

	format := uint16(buf[0])<<8 | uint16(buf[1])
	x := funit.Int16(buf[2])<<8 | funit.Int16(buf[3])
	y := funit.Int16(buf[4])<<8 | funit.Int16(buf[5])

	if format == 0 || format > 3 {
		return Table{}, &fonterror.InvalidFontError{
			SubSystem: "sfnt/opentype/anchor",
			Reason:    fmt.Sprintf("invalid anchor table format %d", format),
		}
	}

	// We ignore the hinting information in formats 2 and 3
	return Table{X: x, Y: y}, nil
}

// IsEmpty returns true if the Anchor Table has not been initialised.
func (rec Table) IsEmpty() bool {
	return rec.X == 0 && rec.Y == 0
}

// Append appends the binary representation of the Anchor Table to buf.
func (rec Table) Append(buf []byte) []byte {
	return append(buf,
		0, 1, // anchorFormat
		byte(rec.X>>8), byte(rec.X),
		byte(rec.Y>>8), byte(rec.Y),
	)
}
