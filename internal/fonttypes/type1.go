// Copyright 2024 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package fonttypes

import (
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/type1"
	"seehuhn.de/go/pdf/internal/debug/makefont"
	"seehuhn.de/go/postscript/afm"
)

// Type1WithMetrics makes a Type 1 PDF font with metrics.
var Type1WithMetrics = type1embedder{true}.font

// Type1WithoutMetrics makes a Type 1 PDF font without the optional metrics.
var Type1WithoutMetrics = type1embedder{false}.font

type type1embedder struct{ metrics bool }

func (t type1embedder) font() font.Layouter {
	info := makefont.Type1()

	var afm *afm.Metrics
	if t.metrics {
		afm = makefont.AFM()
	}

	return font.Must(type1.New(info, afm))
}
