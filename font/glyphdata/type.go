// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2025  Jochen Voss <voss@seehuhn.de>
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

package glyphdata

import (
	"fmt"

	"seehuhn.de/go/pdf"
)

// Type specifies the format of font data in a PDF font file stream.
type Type int

const (
	// Type1 indicates that glyph outlines are provided in Type 1 format.
	Type1 Type = iota + 1

	// TrueType indicates that glyph outlines are provided in TrueType "glyf"
	// format.
	TrueType

	// CFF indicates that glyph outlines are provided in CFF format,
	// using CIDFont operators.
	CFF

	// CFFSimple indicates that glyph outlines are provided in CFF format,
	// and glyph names are present in the font data.
	CFFSimple

	// OpenTypeCFF indicates that glyph outlines are provided in OpenType
	// format, with font data given within a "CFF" table.  The CFF font data
	// uses CIDFont operators.
	OpenTypeCFF

	// OpenTypeCFFSimple indicates that glyph outlines are provided in OpenType
	// format, with font data given within a "CFF" table.  The CFF font data
	// does not use CIDFont operators.
	OpenTypeCFFSimple

	// OpenTypeGlyf indicates that glyph outlines are provided as an OpenType
	// font with a "glyf" table.
	OpenTypeGlyf
)

// String returns a human-readable representation of the font type.
func (t Type) String() string {
	switch t {
	case CFF:
		return "CFF"
	case CFFSimple:
		return "CFF (simple)"
	case OpenTypeCFF:
		return "OpenType/CFF"
	case OpenTypeCFFSimple:
		return "OpenType/CFF (simple)"
	case OpenTypeGlyf:
		return "OpenType/glyf"
	case TrueType:
		return "TrueType"
	case Type1:
		return "Type1"
	default:
		return fmt.Sprintf("Type(%d)", t)
	}
}

// subtype returns the PDF subtype value for FontFile3 streams, or empty string for others.
func (t Type) subtype() pdf.Name {
	switch t {
	case CFFSimple:
		return "Type1C"
	case CFF:
		return "CIDFontType0C"
	case OpenTypeCFFSimple, OpenTypeCFF, OpenTypeGlyf:
		return "OpenType"
	default:
		return ""
	}
}
