package gdef

import (
	"fmt"

	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/parser"
	"seehuhn.de/go/pdf/font/sfnt/opentype/classdef"
)

// Table contains the parsed GDEF table.
// https://docs.microsoft.com/en-us/typography/opentype/spec/GDEF
type Table struct {
	GlyphClass      classdef.Table
	MarkAttachClass classdef.Table
}

// Read reads the GDEF table.
func Read(r parser.ReadSeekSizer) (*Table, error) {
	p := parser.New("GDEF", r)
	buf, err := p.ReadBytes(12)
	if err != nil {
		return nil, err
	}
	majorVersion := uint16(buf[0])<<8 + uint16(buf[1])
	minorVersion := uint16(buf[2])<<8 + uint16(buf[3])
	if majorVersion != 1 || (minorVersion != 0 && minorVersion != 2 && minorVersion != 3) {
		return nil, &font.NotSupportedError{
			SubSystem: "sfnt/opentype/gdef",
			Feature:   fmt.Sprintf("GDEF table version %d.%d", majorVersion, minorVersion),
		}
	}
	glyphClassDefOffset := uint16(buf[4])<<8 + uint16(buf[5])
	attachListOffset := uint16(buf[6])<<8 + uint16(buf[7])
	ligCaretListOffset := uint16(buf[8])<<8 + uint16(buf[9])
	markAttachClassDefOffset := uint16(buf[10])<<8 + uint16(buf[11])
	var markGlyphSetsDefOffset uint16
	if minorVersion >= 2 {
		markGlyphSetsDefOffset, err = p.ReadUInt16()
		if err != nil {
			return nil, err
		}
	}
	var itemVarStoreOffset uint32
	if minorVersion >= 3 {
		itemVarStoreOffset, err = p.ReadUInt32()
		if err != nil {
			return nil, err
		}
	}

	table := &Table{}

	if glyphClassDefOffset != 0 {
		table.GlyphClass, err = classdef.ReadTable(p, int64(glyphClassDefOffset))
		if err != nil {
			return nil, err
		}
	}

	_ = attachListOffset   // TODO(voss): implement
	_ = ligCaretListOffset // TODO(voss): implement

	if markAttachClassDefOffset != 0 {
		table.MarkAttachClass, err = classdef.ReadTable(p, int64(markAttachClassDefOffset))
		if err != nil {
			return nil, err
		}
	}

	_ = markGlyphSetsDefOffset // TODO(voss): implement
	_ = itemVarStoreOffset     // TODO(voss): implement

	return table, nil
}

// Possible values for the GlyphClass field.
const (
	GlyphClassBase      = 1
	GlyphClassLigature  = 2
	GlyphClassMark      = 3
	GlyphClassComponent = 4
)
