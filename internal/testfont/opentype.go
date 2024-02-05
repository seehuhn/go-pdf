// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2023  Jochen Voss <voss@seehuhn.de>
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

package testfont

import (
	"seehuhn.de/go/sfnt"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/opentype"
)

type openTypeEmbedder int

// OpenType fonts
var (
	// OpenTypeGlyf is an OpenType font with glyph outlines.
	OpenTypeGlyf font.Embedder = openTypeEmbedder(0)

	// OpenTypeCFF is an OpenType font with CFF outlines and no CIDFont
	// operators.
	OpenTypeCFF font.Embedder = openTypeEmbedder(1)

	// OpenTypeCFFCID is an OpenType font with CFF outlines and CIDFont
	// operators.
	OpenTypeCFFCID font.Embedder = openTypeEmbedder(2)

	// OpenTypeCFFCID2 is an OpenType font with CFF outlines, CIDFont
	// operators, and multiple private dictionaries.
	OpenTypeCFFCID2 font.Embedder = openTypeEmbedder(3)
)

func (f openTypeEmbedder) Embed(w pdf.Putter, opt *font.Options) (font.Layouter, error) {
	var info *sfnt.Font
	switch f {
	case 0:
		info = MakeGlyfFont()
	case 1:
		info = MakeCFFFont()
	case 2:
		info = MakeCFFCIDFont()
	case 3:
		info = MakeCFFCIDFont2()
	}

	F, err := opentype.New(info)
	if err != nil {
		return nil, err
	}
	return F.Embed(w, opt)
}
