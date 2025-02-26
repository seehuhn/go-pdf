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
	"seehuhn.de/go/pdf/font"
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

	var opt *truetype.Options
	if t.composite {
		opt = &truetype.Options{
			Composite: true,
		}
	}

	F, err := truetype.New(info, opt)
	if err != nil {
		panic(err)
	}
	return F
}
