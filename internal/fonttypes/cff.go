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
	// CFF is a simple CFF font without CIDFont operators.
	CFFSimple font.Font = cffEmbedder{0, false}

	// CFFCID is a simple CFF font with CIDFont operators.
	CFFCIDSimple font.Font = cffEmbedder{1, false}

	// CFF is a composite CFF font without CIDFont operators.
	CFFComposite font.Font = cffEmbedder{0, true}

	// CFFCID is a composite CFF font with CIDFont operators.
	CFFCIDComposite font.Font = cffEmbedder{1, true}

	// CFFCID2 is a composite CFF font with CIDFont operators and multiple private
	// dictionaries.
	CFFCID2Composite font.Font = cffEmbedder{2, true}
)

func (f cffEmbedder) Embed(w pdf.Putter) (font.Layouter, error) {
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
		return nil, err
	}
	return F.Embed(w)
}
