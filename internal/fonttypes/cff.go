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
	"seehuhn.de/go/pdf/font/cff"
	"seehuhn.de/go/pdf/internal/makefont"
	"seehuhn.de/go/sfnt"
)

type cffEmbedder struct {
	tp        int
	composite bool
}

// CFF fonts
var (
	// CFF makes a simple CFF font without CIDFont operators.
	CFFSimple = cffEmbedder{0, false}.font

	// CFFCID makes a simple CFF font with CIDFont operators.
	CFFCIDSimple = cffEmbedder{1, false}.font

	// CFF makes a composite CFF font without CIDFont operators.
	CFFComposite = cffEmbedder{0, true}.font

	// CFFCID makes a composite CFF font with CIDFont operators.
	CFFCIDComposite = cffEmbedder{1, true}.font

	// CFFCID2 makes a composite CFF font with CIDFont operators and multiple private
	// dictionaries.
	CFFCID2Composite = cffEmbedder{2, true}.font
)

func (f cffEmbedder) font(*pdf.ResourceManager) font.Layouter {
	var info *sfnt.Font
	switch f.tp {
	case 0:
		info = makefont.OpenType()
	case 1:
		info = makefont.OpenTypeCID()
	case 2:
		info = makefont.OpenTypeCID2()
	}

	var opt *font.Options
	if f.composite {
		opt = &font.Options{Composite: true}
	}

	F, err := cff.New(info, opt)
	if err != nil {
		panic(err)
	}
	return F
}
