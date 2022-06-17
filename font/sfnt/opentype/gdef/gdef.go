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

package gdef

import (
	"fmt"

	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/parser"
	"seehuhn.de/go/pdf/font/sfnt/opentype/classdef"
	"seehuhn.de/go/pdf/font/sfnt/opentype/coverage"
)

// Table contains the parsed GDEF table.
// https://docs.microsoft.com/en-us/typography/opentype/spec/GDEF
type Table struct {
	GlyphClass classdef.Table // class definition table for glyph type
	// TODO(voss): attachment point list table
	// TODO(voss): ligature caret list table
	MarkAttachClass classdef.Table // class definition table for mark attachment type
	MarkGlyphSets   []coverage.Set // table of mark glyph set definitions
	// TODO(voss): Item Variation Store table
}

func (table *Table) IsMark(gid font.GlyphID) bool {
	if table == nil || table.GlyphClass == nil {
		return false
	}
	return table.GlyphClass[gid] == GlyphClassMark
}

// Read reads the GDEF table.
func Read(r parser.ReadSeekSizer) (*Table, error) {
	p := parser.New("GDEF", r)
	buf, err := p.ReadBytes(12)
	if err != nil {
		return nil, err
	}
	majorVersion := uint16(buf[0])<<8 | uint16(buf[1])
	minorVersion := uint16(buf[2])<<8 | uint16(buf[3])
	if majorVersion != 1 || (minorVersion != 0 && minorVersion != 2 && minorVersion != 3) {
		return nil, &font.NotSupportedError{
			SubSystem: "sfnt/opentype/gdef",
			Feature:   fmt.Sprintf("GDEF table version %d.%d", majorVersion, minorVersion),
		}
	}
	glyphClassDefOffset := uint16(buf[4])<<8 | uint16(buf[5])
	attachListOffset := uint16(buf[6])<<8 | uint16(buf[7])
	ligCaretListOffset := uint16(buf[8])<<8 | uint16(buf[9])
	markAttachClassDefOffset := uint16(buf[10])<<8 | uint16(buf[11])
	var markGlyphSetsDefOffset uint16
	if minorVersion >= 2 {
		markGlyphSetsDefOffset, err = p.ReadUint16()
		if err != nil {
			return nil, err
		}
	}
	var itemVarStoreOffset uint32
	if minorVersion >= 3 {
		itemVarStoreOffset, err = p.ReadUint32()
		if err != nil {
			return nil, err
		}
	}

	table := &Table{}

	if glyphClassDefOffset != 0 {
		table.GlyphClass, err = classdef.Read(p, int64(glyphClassDefOffset))
		if err != nil {
			return nil, err
		}
	}

	_ = attachListOffset   // TODO(voss): implement
	_ = ligCaretListOffset // TODO(voss): implement

	if markAttachClassDefOffset != 0 {
		table.MarkAttachClass, err = classdef.Read(p, int64(markAttachClassDefOffset))
		if err != nil {
			return nil, err
		}
	}

	if markGlyphSetsDefOffset != 0 {
		pos := int64(markGlyphSetsDefOffset)
		err = p.SeekPos(pos)
		if err != nil {
			return nil, err
		}
		buf, err := p.ReadBytes(4)
		if err != nil {
			return nil, err
		}
		format := uint16(buf[0])<<8 | uint16(buf[1])
		if format != 1 {
			return nil, &font.NotSupportedError{
				SubSystem: "sfnt/opentype/gdef",
				Feature:   fmt.Sprintf("mark glyph sets format %d", format),
			}
		}
		markGlyphSetCount := uint16(buf[2])<<8 | uint16(buf[3])
		coverageOffsets := make([]uint32, markGlyphSetCount)
		for i := range coverageOffsets {
			coverageOffsets[i], err = p.ReadUint32()
			if err != nil {
				return nil, err
			}
		}

		table.MarkGlyphSets = make([]coverage.Set, markGlyphSetCount)
		for i := range table.MarkGlyphSets {
			table.MarkGlyphSets[i], err = coverage.ReadSet(p, pos+int64(coverageOffsets[i]))
			if err != nil {
				return nil, err
			}
		}
	}

	_ = itemVarStoreOffset // TODO(voss): implement

	return table, nil
}

// Encode converts the GDEF table to its binary form.
func (table *Table) Encode() []byte {
	version := uint32(0x00010000)
	total := 12

	if table.MarkGlyphSets != nil {
		version = 0x00010002
		total = 14
	}

	var glyphClassDefOffset int
	if table.GlyphClass != nil {
		glyphClassDefOffset = total
		total += table.GlyphClass.AppendLen()
	}
	var markAttachClassDefOffset int
	if table.MarkAttachClass != nil {
		markAttachClassDefOffset = total
		total += table.MarkAttachClass.AppendLen()
	}
	var markGlyphSetsDefOffset int
	if table.MarkGlyphSets != nil {
		markGlyphSetsDefOffset = total
		total += 4 + 4*len(table.MarkGlyphSets)
		for _, set := range table.MarkGlyphSets {
			cov := set.ToTable()
			total += cov.EncodeLen()
		}
	}

	buf := make([]byte, 12, total)
	// We always write table version 1.0:
	buf[0] = byte(version >> 24)
	buf[1] = byte(version >> 16)
	buf[2] = byte(version >> 8)
	buf[3] = byte(version)
	buf[4] = byte(glyphClassDefOffset >> 8)
	buf[5] = byte(glyphClassDefOffset)
	buf[10] = byte(markAttachClassDefOffset >> 8)
	buf[11] = byte(markAttachClassDefOffset)
	if version >= 0x00010002 {
		buf = append(buf, byte(markGlyphSetsDefOffset>>8), byte(markGlyphSetsDefOffset))
	}
	if glyphClassDefOffset > 0 {
		buf = table.GlyphClass.Append(buf)
	}
	if markAttachClassDefOffset > 0 {
		buf = table.MarkAttachClass.Append(buf)
	}
	if markGlyphSetsDefOffset > 0 {
		markGlyphSetCount := len(table.MarkGlyphSets)
		buf = append(buf,
			0, 1, // format
			byte(markGlyphSetCount>>8), byte(markGlyphSetCount))
		offs := 4 + 4*markGlyphSetCount
		for _, set := range table.MarkGlyphSets {
			buf = append(buf,
				byte(offs>>24), byte(offs>>16), byte(offs>>8), byte(offs))
			cov := set.ToTable()
			offs += cov.EncodeLen()
		}
		for _, set := range table.MarkGlyphSets {
			cov := set.ToTable()
			buf = append(buf, cov.Encode()...)
		}
	}
	return buf
}

// Possible values for the GlyphClass field.
// https://docs.microsoft.com/en-us/typography/opentype/spec/GDEF#glyph-class-definition-table
const (
	GlyphClassBase      = 1
	GlyphClassLigature  = 2
	GlyphClassMark      = 3
	GlyphClassComponent = 4
)
