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
	"seehuhn.de/go/pdf/font/sfnt/table"
)

// ReadGdefTable reads the GDEF table of a font file.
func (g *GTab) ReadGdefTable() (*GdefInfo, error) {
	if g.gdef != nil {
		return g.gdef, nil
	}

	tableName := "GDEF"
	info := g.toc.Find(tableName)
	if info == nil {
		return nil, &table.ErrNoTable{Name: tableName}
	}
	err := g.SetRegion(tableName, int64(info.Offset), int64(info.Length))
	if err != nil {
		return nil, err
	}

	s := &parser.State{}
	err = g.Exec(s,
		parser.CmdRead16, parser.TypeUInt, // majorVersion
		parser.CmdAssertEq, 1,
		parser.CmdRead16, parser.TypeUInt, // minorVersion
		parser.CmdStash, // glyphClassDefOffset
		parser.CmdStash, // attachListOffset
		parser.CmdStash, // ligCaretListOffset
		parser.CmdStash, // markAttachClassDefOffset
		parser.CmdExitIfLt, 2,
		parser.CmdStash, // markGlyphSetsDefOffset
		parser.CmdExitIfLt, 3,
		parser.CmdStash, // itemVarStoreOffset
	)
	if err != nil {
		return nil, err
	}
	offsets := s.GetStash()
	glyphClassDefOffset := int64(offsets[0])
	markAttachClassDefOffset := int64(offsets[3])

	res := &GdefInfo{}
	if glyphClassDefOffset != 0 {
		classes, err := g.readClassDefTable(glyphClassDefOffset)
		if err != nil {
			return nil, err
		}
		res.GlyphClassDef = make(map[font.GlyphID]glyphClass)
		for gid, class := range classes {
			// https://docs.microsoft.com/en-us/typography/opentype/spec/gdef#glyph-class-definition-table
			switch class {
			case 1:
				res.GlyphClassDef[gid] = glyphClassBase
			case 2:
				res.GlyphClassDef[gid] = glyphClassLigature
			case 3:
				res.GlyphClassDef[gid] = glyphClassMark
			case 4:
				res.GlyphClassDef[gid] = glyphClassComponent
			}
		}
	}

	if markAttachClassDefOffset != 0 {
		res.MarkAttachClass, err = g.readClassDefTable(markAttachClassDefOffset)
		if err != nil {
			return nil, err
		}
	}

	// TODO(voss): read the remaining sub-tables

	return res, nil
}

// GdefInfo represents the information from the "GDEF" table of a font.
type GdefInfo struct {
	GlyphClassDef   map[font.GlyphID]glyphClass
	MarkAttachClass ClassDef
}

type glyphClass uint8

const (
	glyphClassBase glyphClass = 1 << iota
	glyphClassLigature
	glyphClassMark
	glyphClassComponent
)
