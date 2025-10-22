// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2025  Jochen Voss <voss@seehuhn.de>
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

package text

import (
	"slices"

	"seehuhn.de/go/postscript/cid"
	"seehuhn.de/go/postscript/type1/names"
	"seehuhn.de/go/sfnt/glyf"

	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/dict"
	"seehuhn.de/go/pdf/font/glyphdata/sfntglyphs"
)

type fontFromFile interface {
	font.Instance
	GetDict() dict.Dict
}

func getSpaceWidth(F font.Instance) float64 {
	Fe, ok := F.(fontFromFile)
	if !ok {
		return 280
	}

	d := Fe.GetDict()
	if d == nil {
		return 0
	}

	return spaceWidthHeuristic(d)
}

func getExtraMapping(F font.Instance) map[cid.CID]string {
	fontInfo := F.FontInfo()

	switch fontInfo := fontInfo.(type) {
	case *dict.FontInfoGlyfEmbedded:
		if fontInfo.FontFile == nil {
			return nil
		}

		info, err := sfntglyphs.FromStream(fontInfo.FontFile)
		if err != nil {
			return nil
		}
		outlines, ok := info.Outlines.(*glyf.Outlines)
		if !ok {
			return nil
		}

		m := make(map[cid.CID]string)

		if outlines.Names != nil {
			if fontInfo.CIDToGID != nil {
				for cidVal, gid := range fontInfo.CIDToGID {
					if int(gid) > len(outlines.Names) {
						continue
					}
					name := outlines.Names[gid]
					if name == "" {
						continue
					}

					text := names.ToUnicode(name, fontInfo.PostScriptName)
					m[cid.CID(cidVal)] = text
				}
			}
		}
		return m
	default:
		return nil
	}
}

type affine struct {
	intercept, slope float64
}

const noBreakSpace = "\u00A0" // no-break space

var commonCharacters = map[string]affine{
	" ":          {0, 1},
	noBreakSpace: {0, 1},
	")":          {-43.01937, 1.0268},
	"/":          {-10.99708, 0.9623335},
	"•":          {-24.2725, 0.9956384},
	"−":          {-439.6255, 1.238626},
	"∗":          {91.30598, 0.7265824},
	"1":          {-130.7855, 0.9746186},
	"a":          {-131.2164, 0.9740258},
	"A":          {72.40703, 0.4928694},
	"e":          {-136.5258, 0.9895894},
	"E":          {-28.76257, 0.6957778},
	"i":          {51.62929, 0.8973944},
	"ε":          {-56.25771, 0.9947787},
	"Ω":          {-132.9966, 1.002173},
	"中":          {-356.8609, 1.215483},
}

func spaceWidthHeuristic(dict dict.Dict) float64 {
	guesses := []float64{280}
	for _, info := range dict.Characters() {
		if coef, ok := commonCharacters[info.Text]; ok && info.Width > 0 {
			guesses = append(guesses, coef.intercept+coef.slope*info.Width*1000)
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
