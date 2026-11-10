// Copyright 2024 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package fonttypes

import (
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cmap"
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

	if f.composite {
		return font.Must(opentype.NewComposite(info, &opentype.OptionsComposite{CMap: cmap.IdentityH}))
	}
	return font.Must(opentype.NewSimple(info, nil))
}
