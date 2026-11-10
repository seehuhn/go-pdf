// Copyright 2024 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package fonttypes

import (
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cff"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/internal/debug/makefont"
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

func (f cffEmbedder) font() font.Layouter {
	var info *sfnt.Font
	switch f.tp {
	case 0:
		info = makefont.OpenType()
	case 1:
		info = makefont.OpenTypeCID()
	case 2:
		info = makefont.OpenTypeCID2()
	}

	if f.composite {
		return font.Must(cff.NewComposite(info, &cff.OptionsComposite{CMap: cmap.IdentityH}))
	}
	return font.Must(cff.NewSimple(info, nil))
}
