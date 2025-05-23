// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2024  Jochen Voss <voss@seehuhn.de>
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

package fonttypes

import (
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/opentype"
	"seehuhn.de/go/pdf/internal/debug/makefont"
	"seehuhn.de/go/sfnt"
)

type openTypeEmbedder struct {
	tp        int
	composite bool
}

// OpenType fonts
var (
	// OpenTypeGlyf makes an OpenType font with glyph outlines.
	OpenTypeGlyfSimple = openTypeEmbedder{tp: 0, composite: false}.font

	// OpenTypeCFF makes an OpenType font with CFF outlines and no CIDFont
	// operators.
	OpenTypeCFFSimple = openTypeEmbedder{tp: 1, composite: false}.font

	// OpenTypeCFFCID makes an OpenType font with CFF outlines and CIDFont
	// operators.
	OpenTypeCFFCIDSimple = openTypeEmbedder{tp: 2, composite: false}.font

	// OpenTypeGlyf makes an OpenType font with glyph outlines.
	OpenTypeGlyfComposite = openTypeEmbedder{tp: 0, composite: true}.font

	// OpenTypeCFF makes an OpenType font with CFF outlines and no CIDFont
	// operators.
	OpenTypeCFFComposite = openTypeEmbedder{tp: 1, composite: true}.font

	// OpenTypeCFFCID makes an OpenType font with CFF outlines and CIDFont
	// operators.
	OpenTypeCFFCIDComposite = openTypeEmbedder{tp: 2, composite: true}.font

	// OpenTypeCFFCID2 makes an OpenType font with CFF outlines, CIDFont
	// operators, and multiple private dictionaries.
	OpenTypeCFFCID2Composite = openTypeEmbedder{tp: 3, composite: true}.font
)

func (f openTypeEmbedder) font() font.Layouter {
	var info *sfnt.Font
	switch f.tp {
	case 0:
		info = makefont.TrueType()
	case 1:
		info = makefont.OpenType()
	case 2:
		info = makefont.OpenTypeCID()
	case 3:
		info = makefont.OpenTypeCID2()
	}

	opt := &opentype.Options{Composite: f.composite}
	F, err := opentype.New(info, opt)
	if err != nil {
		panic(err)
	}
	return F
}
