// seehuhn.de/go/pdf - support for reading and writing PDF files
// Copyright (C) 2021  Jochen Voss <voss@seehuhn.de>
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

package parser

import (
	"seehuhn.de/go/pdf/font"
)

func (p *Parser) readClassDefTable(pos int64) (ClassDef, error) {
	res, ok := p.classDefCache[pos]
	if ok {
		return res, nil
	}

	s := &State{
		A: pos,
	}
	err := p.Exec(s,
		CmdSeek,
		CmdRead16, TypeUInt, // format
	)
	if err != nil {
		return nil, err
	}
	format := int(s.A)

	res = make(ClassDef)
	switch format {
	case 1:
		err = p.Exec(s,
			CmdStash,            // startGlyphID
			CmdRead16, TypeUInt, // glyphCount
			CmdLoop,
			CmdStash, // classValueArray[i]
			CmdEndLoop,
		)
		if err != nil {
			return nil, err
		}
		stash := s.GetStash()
		startGlyphID := font.GlyphID(stash[0])
		for gidOffs, class := range stash[1:] {
			if class == 0 {
				continue
			}
			res[startGlyphID+font.GlyphID(gidOffs)] = int(class)
		}
	case 2:
		err = p.Exec(s,
			CmdRead16, TypeUInt, // classRangeCount
			CmdLoop,
			CmdStash, // classRangeRecords[i].startGlyphID
			CmdStash, // classRangeRecords[i].endGlyphID
			CmdStash, // classRangeRecords[i].class
			CmdEndLoop,
		)
		if err != nil {
			return nil, err
		}
		stash := s.GetStash()
		for len(stash) > 0 {
			start := int(stash[0])
			end := int(stash[1])
			class := int(stash[2])
			if end < start {
				return nil, p.error("corrupt classDef record")
			}
			if class != 0 {
				for gid := start; gid <= end; gid++ {
					res[font.GlyphID(gid)] = class
				}
			}
			stash = stash[3:]
		}
	default:
		return nil, p.error("unsupported classDef format %d", format)
	}

	p.classDefCache[pos] = res

	return res, nil
}

// ClassDef represents the contents of a Class Definition Table
// https://docs.microsoft.com/en-us/typography/opentype/spec/chapter2#class-definition-table
// TODO(voss): using uint16 instead of int?
type ClassDef map[font.GlyphID]int
