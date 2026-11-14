// Copyright 2024 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package makefont

import (
	"bytes"
	"math"

	"golang.org/x/image/font/gofont/goregular"

	"seehuhn.de/go/postscript/funit"
	"seehuhn.de/go/sfnt"
	"seehuhn.de/go/sfnt/glyph"
	"seehuhn.de/go/sfnt/parser"
)

// TrueType returns a font with glyf outlines.  Vertical metrics are added
// as described for [addVertical].
func TrueType() *sfnt.Font {
	r := bytes.NewReader(goregular.TTF)
	info, err := sfnt.Read(r, parser.NewBudget(int64(len(goregular.TTF))))
	if err != nil {
		panic(err)
	}
	addVertical(info)
	return info
}

// addVertical gives a font synthetic vertical metrics, replacing any it has.
//
// Glyphs which reach above the x-height have an advance height of one em,
// with the vertical origin at the typographic ascender.  The remaining
// glyphs, including blank ones, have an advance height of 3/4 em, with the
// vertical origin placed so that the x-height band is centred in the
// advance.
func addVertical(info *sfnt.Font) {
	n := info.NumGlyphs()
	upem := float64(info.UnitsPerEm)
	xHeight := float64(info.XHeight)

	tallHeight := funit.Uint16(info.UnitsPerEm)
	tallOrigin := info.Ascent
	shortHeight := funit.Uint16(math.Round(0.75 * upem))
	shortOrigin := funit.Int16(math.Round(xHeight + (0.75*upem-xHeight)/2))

	v := &sfnt.VerticalMetrics{
		Ascent:        funit.Int16(info.UnitsPerEm / 2),
		Descent:       -funit.Int16(info.UnitsPerEm / 2),
		CaretSlopeRun: 1,
		Heights:       make([]funit.Uint16, n),
		OriginY:       make([]funit.Int16, n),
	}
	for i := range n {
		// allow for the overshoot of round letters
		if info.GlyphBBox(glyph.ID(i)).URy > 1.05*xHeight {
			v.Heights[i] = tallHeight
			v.OriginY[i] = tallOrigin
		} else {
			v.Heights[i] = shortHeight
			v.OriginY[i] = shortOrigin
		}
	}
	info.Vertical = v
}

// OpenType returns a font with CFF outlines, not using CIDFont operators.
func OpenType() *sfnt.Font {
	info := TrueType()
	info, err := toCFF(info)
	if err != nil {
		panic(err)
	}
	return info
}

// OpenTypeCID returns a font with CFF outlines and CIDFont operators.
func OpenTypeCID() *sfnt.Font {
	info := OpenType()
	info, err := toCFFCID(info)
	if err != nil {
		panic(err)
	}
	return info
}

// OpenTypeCID2 returns a font with CFF outlines, CIDFont operators, and
// multiple private dictionaries.
func OpenTypeCID2() *sfnt.Font {
	info := OpenType()
	info, err := toCFFCID2(info)
	if err != nil {
		panic(err)
	}
	return info
}
