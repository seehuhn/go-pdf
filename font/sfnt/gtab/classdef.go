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

package gtab

import (
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/sfnt/parser"
)

func (p *GTab) readClassDefTable(pos int64) (ClassDef, error) {
	res, ok := p.classDefCache[pos]
	if ok {
		return res, nil
	}

	s := &parser.State{
		A: pos,
	}
	err := p.Exec(s,
		parser.CmdSeek,
		parser.CmdRead16, parser.TypeUInt, // format
	)
	if err != nil {
		return nil, err
	}
	format := int(s.A)

	res = make(ClassDef)
	switch format {
	case 1:
		err = p.Exec(s,
			parser.CmdStash,                   // startGlyphID
			parser.CmdRead16, parser.TypeUInt, // glyphCount
			parser.CmdLoop,
			parser.CmdStash, // classValueArray[i]
			parser.CmdEndLoop,
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
			parser.CmdRead16, parser.TypeUInt, // classRangeCount
			parser.CmdLoop,
			parser.CmdStash, // classRangeRecords[i].startGlyphID
			parser.CmdStash, // classRangeRecords[i].endGlyphID
			parser.CmdStash, // classRangeRecords[i].class
			parser.CmdEndLoop,
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
				return nil, p.Error("corrupt classDef record")
			}
			if class != 0 {
				for gid := start; gid <= end; gid++ {
					res[font.GlyphID(gid)] = class
				}
			}
			stash = stash[3:]
		}
	default:
		return nil, p.Error("unsupported classDef format %d", format)
	}

	if p.classDefCache != nil {
		p.classDefCache[pos] = res
	}

	return res, nil
}

// ClassDef represents the contents of a Class Definition Table
// https://docs.microsoft.com/en-us/typography/opentype/spec/chapter2#class-definition-table
// TODO(voss): use uint16 instead of int?
type ClassDef map[font.GlyphID]int
