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

// Package classdef reads and writes OpenType "Class Definition Tables".
// https://docs.microsoft.com/en-us/typography/opentype/spec/chapter2#classDefTbl
package classdef

import (
	"fmt"
	"sort"

	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/parser"
)

// Table contains the information from an OpenType "Class Definition Table".
// All glyphs not assigned to a class fall into Class 0.
type Table map[font.GlyphID]uint16

// NumClasses returns the number of classes in the table.
// The count includes the zero class.
func (info Table) NumClasses() int {
	maxClass := uint16(0)
	for _, class := range info {
		if class > maxClass {
			maxClass = class
		}
	}
	return int(maxClass) + 1
}

// Glyphs returns the glyphs for each non-zero class in the Table.
// The first entry of the returned slice, corresponding to class 0,
// is always nil.
func (info Table) Glyphs() [][]font.GlyphID {
	numClasses := info.NumClasses()
	glyphs := make([][]font.GlyphID, numClasses)
	for gid, cls := range info {
		if cls == 0 {
			continue
		}
		glyphs[cls] = append(glyphs[cls], gid)
	}
	for i := 1; i < numClasses; i++ {
		sort.Slice(glyphs[i],
			func(k, l int) bool { return glyphs[i][k] < glyphs[i][l] })
	}
	return glyphs
}

// Read reads and decodes an OpenType "Class Definition Table".
func Read(p *parser.Parser, pos int64) (Table, error) {
	err := p.SeekPos(pos)
	if err != nil {
		return nil, err
	}

	version, err := p.ReadUint16()
	if err != nil {
		return nil, err
	}
	switch version {
	case 1:
		data, err := p.ReadBytes(4)
		if err != nil {
			return nil, err
		}
		startGlyphID := font.GlyphID(data[0])<<8 | font.GlyphID(data[1])
		glyphCount := int(data[2])<<8 | int(data[3])
		if int(startGlyphID)+glyphCount-1 > 0xFFFF {
			return nil, &font.InvalidFontError{
				SubSystem: "opentype/classdef",
				Reason:    "glyph count too large in class definition table",
			}
		}

		res := make(Table, glyphCount)
		for i := 0; i < glyphCount; i++ {
			classValue, err := p.ReadUint16()
			if err != nil {
				return nil, err
			}
			if classValue != 0 {
				res[startGlyphID+font.GlyphID(i)] = classValue
			}
		}
		return res, nil

	case 2:
		classRangeCount, err := p.ReadUint16()
		if err != nil {
			return nil, err
		}

		res := Table{}
		var prevEnd font.GlyphID
		for i := 0; i < int(classRangeCount); i++ {
			data, err := p.ReadBytes(6)
			if err != nil {
				return nil, err
			}
			startGlyphID := font.GlyphID(data[0])<<8 | font.GlyphID(data[1])
			endGlyphID := font.GlyphID(data[2])<<8 | font.GlyphID(data[3])
			classValue := uint16(data[4])<<8 | uint16(data[5])

			if i > 0 && startGlyphID <= prevEnd {
				return nil, &font.InvalidFontError{
					SubSystem: "opentype/classdef",
					Reason:    "overlapping ranges in class definition table",
				}
			}
			prevEnd = endGlyphID

			if classValue != 0 {
				for j := int(startGlyphID); j <= int(endGlyphID); j++ {
					res[font.GlyphID(j)] = classValue
				}
			}
		}
		return res, nil

	default:
		return nil, &font.NotSupportedError{
			SubSystem: "opentype/classdef",
			Feature:   fmt.Sprintf("class definition table version %d", version),
		}
	}
}

type encInfo struct {
	minGid, maxGid font.GlyphID
	format1Size    int
	format2Size    int
}

func (info Table) getEncInfo() *encInfo {
	minGid := font.GlyphID(0xFFFF)
	maxGid := font.GlyphID(0)
	for key := range info {
		if key < minGid {
			minGid = key
		}
		if key > maxGid {
			maxGid = key
		}
	}

	format1Size := 6 + 2*(int(maxGid)-int(minGid)+1)

	segCount := 0
	segStart := -1
	var segClass uint16
	for i := int(minGid); i <= int(maxGid) && 4+6*segCount < format1Size; i++ {
		class := info[font.GlyphID(i)]

		if segStart >= 0 && class != segClass {
			segCount++
			segStart = -1
		}
		if segStart == -1 {
			if class != 0 {
				segStart = i
				segClass = class
			}
		}
	}
	if segStart >= 0 {
		segCount++
	}

	format2Size := 4 + 6*segCount

	return &encInfo{
		minGid:      minGid,
		maxGid:      maxGid,
		format1Size: format1Size,
		format2Size: format2Size,
	}
}

// AppendLen returns the size of the binary table representation.
func (info Table) AppendLen() int {
	if len(info) == 0 {
		return 4
	}
	encInfo := info.getEncInfo()
	if encInfo.format1Size < encInfo.format2Size {
		return encInfo.format1Size
	}
	return encInfo.format2Size
}

// Append appends the binary table representation to the given buffer.
func (info Table) Append(buf []byte) []byte {
	if len(info) == 0 {
		return append(buf, 0, 2, 0, 0)
	}

	encInfo := info.getEncInfo()

	if encInfo.format1Size <= encInfo.format2Size {
		count := encInfo.maxGid - encInfo.minGid + 1
		buf = append(buf,
			0, 1,
			byte(encInfo.minGid>>8), byte(encInfo.minGid),
			byte(count>>8), byte(count))
		for i := 0; i < int(count); i++ {
			class := info[encInfo.minGid+font.GlyphID(i)]
			buf = append(buf, byte(class>>8), byte(class))
		}
		return buf
	}

	segCount := (encInfo.format2Size - 4) / 6
	buf = append(buf, 0, 2, byte(segCount>>8), byte(segCount))
	segStart := -1
	var segClass uint16
	for i := int(encInfo.minGid); i <= int(encInfo.maxGid); i++ {
		class := info[font.GlyphID(i)]

		if segStart >= 0 && class != segClass {
			buf = append(buf,
				byte(segStart>>8), byte(segStart),
				byte((i-1)>>8), byte(i-1),
				byte(segClass>>8), byte(segClass))
			segStart = -1
		}
		if segStart == -1 {
			if class != 0 {
				segStart = i
				segClass = class
			}
		}
	}
	if segStart >= 0 {
		buf = append(buf,
			byte(segStart>>8), byte(segStart),
			byte(encInfo.maxGid>>8), byte(encInfo.maxGid),
			byte(segClass>>8), byte(segClass))
	}

	return buf
}
