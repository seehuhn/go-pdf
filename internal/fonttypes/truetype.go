// Copyright 2024 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package fonttypes

import (
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/truetype"
	"seehuhn.de/go/pdf/internal/debug/makefont"
)

type trueTypeEmbedder struct {
	composite bool
}

// TrueType makes a TrueType font.
var (
	TrueTypeSimple    = trueTypeEmbedder{composite: false}.font
	TrueTypeComposite = trueTypeEmbedder{composite: true}.font
)

func (t trueTypeEmbedder) font() font.Layouter {
	info := makefont.TrueType()

	if t.composite {
		return font.Must(truetype.NewComposite(info, &truetype.OptionsComposite{CMap: cmap.IdentityH}))
	}
	return font.Must(truetype.NewSimple(info, nil))
}
