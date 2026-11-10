// Copyright 2023 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package makefont

import (
	"seehuhn.de/go/postscript/afm"
	"seehuhn.de/go/postscript/type1"
)

// Type1 returns a Type1 font.
func Type1() *type1.Font {
	info := TrueType()
	psFont, err := toType1(info)
	if err != nil {
		panic(err)
	}
	return psFont
}

// AFM returns the font metrics for the font returned by [Type1].
func AFM() *afm.Metrics {
	info := TrueType()
	metrics, err := toAFM(info)
	if err != nil {
		panic(err)
	}
	return metrics
}
