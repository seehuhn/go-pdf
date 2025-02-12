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

import "fmt"

type Type int

const (
	// None indicates that no glyph data is embedded.
	None Type = iota

	CFF
	CFFSimple
	OpenTypeCFF

	// OpenTypeCFFSimple indicates that glyph outlines are provided in OpenType
	// format, with font data given within a "CFF" table.  The CFF font data
	// does not use CIDFont operators.
	//
	// This can be used for [seehuhn.de/go/pdf/font/dict.Type1] and
	// [seehuhn.de/go/pdf/font/dict.CIDFontType0] font dictionaries.
	//
	// Font data can be embedded using [seehuhn.de/go/pdf/font/glyphdata/opentypeglyphs.Embed],
	// and extracted using [seehuhn.de/go/pdf/font/glyphdata/opentypeglyphs.Extract].
	OpenTypeCFFSimple

	// OpenTypeGlyf indicates that glyph outlines as an OpenType font with a
	// "glyf" table.
	//
	// This can be used for [seehuhn.de/go/pdf/font/dict.TrueType] and
	// [seehuhn.de/go/pdf/font/dict.CIDFontType2] font dictionaries.
	//
	// Font data can be embedded using [seehuhn.de/go/pdf/font/glyphdata/opentypeglyphs.Embed],
	// and extracted using [seehuhn.de/go/pdf/font/glyphdata/opentypeglyphs.Extract].
	OpenTypeGlyf

	// TrueType indicates that glyph outlines are provided in TrueType "glyf"
	// format.
	//
	// This can be used for [seehuhn.de/go/pdf/font/dict.TrueType] and
	// [seehuhn.de/go/pdf/font/dict.CIDFontType2] font dictionaries.
	//
	// Font data can be embedded using [seehuhn.de/go/pdf/font/glyphdata/opentypeglyphs.Embed],
	// and extracted using [seehuhn.de/go/pdf/font/glyphdata/opentypeglyphs.Extract].
	TrueType

	// Type1 indicates that glyph outlines are provided in Type 1 format.
	//
	// This can be used for [seehuhn.de/go/pdf/font/dict.Type1] font dictionaries.
	//
	// Font data can be embedded using [seehuhn.de/go/pdf/font/glyphdata/type1glyphs.Embed],
	// and extracted using [seehuhn.de/go/pdf/font/glyphdata/type1glyphs.Extract].
	Type1
)

func (t Type) String() string {
	switch t {
	case None:
		return "None"
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
