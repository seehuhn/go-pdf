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
