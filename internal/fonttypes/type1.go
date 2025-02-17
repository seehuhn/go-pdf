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

	F, err := type1.New(info, afm, nil)
	if err != nil {
		panic(err)
	}
	return F
}
