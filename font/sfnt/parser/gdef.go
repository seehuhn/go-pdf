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

import "seehuhn.de/go/pdf/font"

// GdefInfo represents the information from the "GDEF" table of a font.
type GdefInfo struct {
	GlyphClassDef   map[font.GlyphID]glyphClass
	MarkAttachClass ClassDef
}

// ReadGdefInfo reads the GDEF table of a font file.
func (p *Parser) ReadGdefInfo() (*GdefInfo, error) {
	err := p.OpenTable("GDEF")
	if err != nil {
		return nil, err
	}

	s := &State{}
	err = p.Exec(s,
		CmdRead16, TypeUInt, // majorVersion
		CmdAssertEq, 1,
		CmdRead16, TypeUInt, // minorVersion
		CmdStash, // glyphClassDefOffset
		CmdStash, // attachListOffset
		CmdStash, // ligCaretListOffset
		CmdStash, // markAttachClassDefOffset
		CmdExitIfLt, 2,
		CmdStash, // markGlyphSetsDefOffset
		CmdExitIfLt, 3,
		CmdStash, // itemVarStoreOffset
	)
	if err != nil {
		return nil, err
	}
	offsets := s.GetStash()
	glyphClassDefOffset := int64(offsets[0])
	markAttachClassDefOffset := int64(offsets[3])

	res := &GdefInfo{}
	if glyphClassDefOffset != 0 {
		classes, err := p.readClassDefTable(glyphClassDefOffset)
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
		res.MarkAttachClass, err = p.readClassDefTable(markAttachClassDefOffset)
		if err != nil {
			return nil, err
		}
	}

	// TODO(voss): read the remaining sub-tables

	return res, nil
}

type glyphClass uint8

const (
	glyphClassBase glyphClass = 1 << iota
	glyphClassLigature
	glyphClassMark
	glyphClassComponent
)
