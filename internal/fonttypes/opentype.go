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
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/opentype"
	"seehuhn.de/go/pdf/internal/makefont"
	"seehuhn.de/go/sfnt"
)

type openTypeEmbedder int

// OpenType fonts
var (
	// OpenTypeGlyf is an OpenType font with glyph outlines.
	OpenTypeGlyf font.Font = openTypeEmbedder(0)

	// OpenTypeCFF is an OpenType font with CFF outlines and no CIDFont
	// operators.
	OpenTypeCFF font.Font = openTypeEmbedder(1)

	// OpenTypeCFFCID is an OpenType font with CFF outlines and CIDFont
	// operators.
	OpenTypeCFFCID font.Font = openTypeEmbedder(2)

	// OpenTypeCFFCID2 is an OpenType font with CFF outlines, CIDFont
	// operators, and multiple private dictionaries.
	OpenTypeCFFCID2 font.Font = openTypeEmbedder(3)
)

func (f openTypeEmbedder) Embed(w pdf.Putter, opt *font.Options) (font.Layouter, error) {
	var info *sfnt.Font
	switch f {
	case 0:
		info = makefont.TrueType()
	case 1:
		info = makefont.OpenType()
	case 2:
		info = makefont.OpenTypeCID()
	case 3:
		info = makefont.OpenTypeCID2()
	}

	F, err := opentype.New(info)
	if err != nil {
		return nil, err
	}
	return F.Embed(w, opt)
}
