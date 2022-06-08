package markarray

import (
	"seehuhn.de/go/pdf/font/parser"
	"seehuhn.de/go/pdf/font/sfnt/opentype/anchor"
)

// Table is a Mark Array Table.
// The table defines the class and the anchor point for a set of mark glyphs.
// https://docs.microsoft.com/en-us/typography/opentype/spec/gpos#mark-array-table
type Table []Record // ordered by mark coverage index

// Record is a mark record in a Mark Array Table.
// Each mark record defines the class of the mark and an offset to the Anchor
// table that contains data for a single mark.
type Record struct {
	Class uint16
	anchor.Table
}

// Read reads a Mark Array Table from the given parser.
// If there are more than numMarks entries in the table, the remaining entries
// are ignored.
func Read(p *parser.Parser, pos int64, numMarks int) (Table, error) {
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

	res := make(Table, markCount)
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
