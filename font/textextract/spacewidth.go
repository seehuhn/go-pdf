// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package textextract

import (
	"slices"

	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/dict"
)

// SpaceWidth estimates the width of a space character for a font,
// in TJ units (thousandths of text space).
// This is used to distinguish real word spaces from small kerning adjustments
// in TJ arrays.
func SpaceWidth(f font.Instance) float64 {
	type fontWithDict interface {
		font.Instance
		GetDict() dict.Dict
	}

	fe, ok := f.(fontWithDict)
	if !ok {
		return 280
	}

	d := fe.GetDict()
	if d == nil {
		return 280
	}

	return spaceWidthHeuristic(d)
}

type affine struct {
	intercept, slope float64
}

var commonCharacters = map[string]affine{
	" ":      {0, 1},
	"\u00A0": {0, 1}, // no-break space
	")":      {-43.01937, 1.0268},
	"/":      {-10.99708, 0.9623335},
	"•":      {-24.2725, 0.9956384},
	"−":      {-439.6255, 1.238626},
	"∗":      {91.30598, 0.7265824},
	"1":      {-130.7855, 0.9746186},
	"a":      {-131.2164, 0.9740258},
	"A":      {72.40703, 0.4928694},
	"e":      {-136.5258, 0.9895894},
	"E":      {-28.76257, 0.6957778},
	"i":      {51.62929, 0.8973944},
	"ε":      {-56.25771, 0.9947787},
	"Ω":      {-132.9966, 1.002173},
	"中":      {-356.8609, 1.215483},
}

func spaceWidthHeuristic(d dict.Dict) float64 {
	guesses := []float64{280}
	for text, coef := range commonCharacters {
		if w, ok := d.GlyphWidth(text); ok && w > 0 {
			guesses = append(guesses, coef.intercept+coef.slope*w*1000)
		}
	}
	slices.Sort(guesses)

	// calculate the median
	var guess float64
	n := len(guesses)
	if n%2 == 0 {
		guess = (guesses[n/2-1] + guesses[n/2]) / 2
	} else {
		guess = guesses[n/2]
	}

	// adjustment to remove empirical bias
	guess = 1.366239*guess - 139.183703

	// clamp to approximate [0.01, 0.99] quantile range
	if guess < 200 {
		guess = 200
	} else if guess > 1000 {
		guess = 1000
	}

	return guess
}
