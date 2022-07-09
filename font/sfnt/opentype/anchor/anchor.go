package anchor

import (
	"fmt"

	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/funit"
	"seehuhn.de/go/pdf/font/parser"
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
		return Table{}, &font.InvalidFontError{
			SubSystem: "sfnt/opentype/anchor",
			Reason:    fmt.Sprintf("invalid anchor table format %d", format),
		}
	}

	// We ignore the hinting information in formats 2 and 3
	return Table{X: x, Y: y}, nil
}

func (rec Table) IsEmpty() bool {
	return rec.X == 0 && rec.Y == 0
}
