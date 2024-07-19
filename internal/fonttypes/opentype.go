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

type openTypeEmbedder struct {
	tp        int
	composite bool
}

// OpenType fonts
var (
	// OpenTypeGlyf is an OpenType font with glyph outlines.
	OpenTypeGlyfSimple font.Font = openTypeEmbedder{tp: 0, composite: false}

	// OpenTypeCFF is an OpenType font with CFF outlines and no CIDFont
	// operators.
	OpenTypeCFFSimple font.Font = openTypeEmbedder{tp: 1, composite: false}

	// OpenTypeCFFCID is an OpenType font with CFF outlines and CIDFont
	// operators.
	OpenTypeCFFCIDSimple font.Font = openTypeEmbedder{tp: 2, composite: false}

	// OpenTypeGlyf is an OpenType font with glyph outlines.
	OpenTypeGlyfComposite font.Font = openTypeEmbedder{tp: 0, composite: true}

	// OpenTypeCFF is an OpenType font with CFF outlines and no CIDFont
	// operators.
	OpenTypeCFFComposite font.Font = openTypeEmbedder{tp: 1, composite: true}

	// OpenTypeCFFCID is an OpenType font with CFF outlines and CIDFont
	// operators.
	OpenTypeCFFCIDComposite font.Font = openTypeEmbedder{tp: 2, composite: true}

	// OpenTypeCFFCID2 is an OpenType font with CFF outlines, CIDFont
	// operators, and multiple private dictionaries.
	OpenTypeCFFCID2Composite font.Font = openTypeEmbedder{tp: 3, composite: true}
)

func (f openTypeEmbedder) Embed(w pdf.Putter) (font.Layouter, error) {
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

	var opt *font.Options
	if f.composite {
		opt = &font.Options{Composite: true}
	}

	F, err := opentype.New(info, opt)
	if err != nil {
		return nil, err
	}
	return F.Embed(w)
}
