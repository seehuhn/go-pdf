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

// GdefInfo represents the information from the "GDEF" table of a font.
type GdefInfo struct {
	GlyphClassDef ClassDef
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
	glyphClassDefOffset := offsets[0]

	res := &GdefInfo{}
	if glyphClassDefOffset != 0 {
		res.GlyphClassDef, err = p.readClassDefTable(int64(glyphClassDefOffset))
		if err != nil {
			return nil, err
		}
	}

	// TODO(voss): read the remaining sub-tables

	return res, nil
}
