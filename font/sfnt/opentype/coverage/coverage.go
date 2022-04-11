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

// Package coverage reads and writes OpenType "Coverage Tables"
// https://docs.microsoft.com/en-us/typography/opentype/spec/chapter2#coverage-table
package coverage

import (
	"fmt"

	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/parser"
)

// Table represents an OpenType "Coverage Table".
type Table map[font.GlyphID]int

// ReadTable reads a coverage table from the given parser.
func ReadTable(p *parser.Parser, pos int64) (Table, error) {
	err := p.SeekPos(pos)
	if err != nil {
		return nil, err
	}

	format, err := p.ReadUInt16()
	if err != nil {
		return nil, err
	}

	table := make(Table)

	switch format {
	case 1: // Coverage Format 1
		glyphCount, err := p.ReadUInt16()
		if err != nil {
			return nil, err
		}
		for i := 0; i < int(glyphCount); i++ {
			gid, err := p.ReadUInt16()
			if err != nil {
				return nil, err
			}
			if _, alreadySeen := table[font.GlyphID(gid)]; alreadySeen {
				return nil, &font.InvalidFontError{
					SubSystem: "sfnt/opentype/coverage",
					Reason:    "invalid coverage table (format 1)",
				}
			}
			table[font.GlyphID(gid)] = i
		}

	case 2: // Coverage Format 2
		rangeCount, err := p.ReadUInt16()
		if err != nil {
			return nil, err
		}
		pos := 0
		for i := 0; i < int(rangeCount); i++ {
			buf, err := p.ReadBytes(6)
			if err != nil {
				return nil, err
			}
			startGlyphID := int(buf[0])<<8 | int(buf[1])
			endGlyphID := int(buf[2])<<8 | int(buf[3])
			startCoverageIndex := int(buf[4])<<8 | int(buf[5])
			if startCoverageIndex != pos {
				return nil, &font.InvalidFontError{
					SubSystem: "sfnt/opentype/coverage",
					Reason:    "invalid coverage table (format 2)",
				}
			}
			for gid := startGlyphID; gid <= endGlyphID; gid++ {
				if _, alreadySeen := table[font.GlyphID(gid)]; alreadySeen {
					return nil, &font.InvalidFontError{
						SubSystem: "sfnt/opentype/coverage",
						Reason:    "invalid coverage table (format 2)",
					}
				}
				table[font.GlyphID(gid)] = pos
				pos++
			}
		}

	default:
		return nil, &font.NotSupportedError{
			SubSystem: "sfnt/opentype/coverage",
			Feature:   fmt.Sprintf("coverage format %d", format),
		}
	}

	return table, nil
}

func (table Table) encInfo() ([]font.GlyphID, int, int) {
	rev := make([]font.GlyphID, len(table))
	for gid, i := range table {
		rev[i] = gid
	}

	format1Length := 4 + 2*len(table)

	rangeCount := 0
	prev := 0xFFFF
	for _, gid := range rev {
		if int(gid) != prev+1 {
			rangeCount++
		}
		prev = int(gid)
	}
	format2Length := 4 + 6*rangeCount

	return rev, format1Length, format2Length
}

// EncodeLen returns the number of bytes in the binary representation of the
// coverage table.
func (table Table) EncodeLen() int {
	_, format1Length, format2Length := table.encInfo()
	if format1Length <= format2Length {
		return format1Length
	}
	return format2Length
}

// Encode returns the binary representation of the coverage table.
func (table Table) Encode() []byte {
	rev, format1Length, format2Length := table.encInfo()

	if format1Length <= format2Length {
		buf := make([]byte, format1Length)
		buf[0] = 0
		buf[1] = 1
		buf[2] = byte(len(rev) >> 8)
		buf[3] = byte(len(rev))
		for i, gid := range rev {
			buf[4+2*i] = byte(gid >> 8)
			buf[4+2*i+1] = byte(gid)
		}
		return buf
	}

	rangeCount := (format2Length - 4) / 6

	buf := make([]byte, 4, format2Length)
	buf[0] = 0
	buf[1] = 2
	buf[2] = byte(rangeCount >> 8)
	buf[3] = byte(rangeCount)
	var startGlyphID font.GlyphID
	var startCoverageIndex int
	prev := 0xFFFF
	for i, gid := range rev {
		if int(gid) != prev+1 {
			if i > 0 {
				buf = append(buf,
					byte(startGlyphID>>8), byte(startGlyphID),
					byte(prev>>8), byte(prev),
					byte(startCoverageIndex>>8), byte(startCoverageIndex))
			}
			startGlyphID = gid
			startCoverageIndex = i
		}
		prev = int(gid)
	}
	buf = append(buf,
		byte(startGlyphID>>8), byte(startGlyphID),
		byte(prev>>8), byte(prev),
		byte(startCoverageIndex>>8), byte(startCoverageIndex))
	return buf
}