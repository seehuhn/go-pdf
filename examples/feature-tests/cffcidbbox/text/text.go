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
	"fmt"
	"math"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
)

func Show(w *graphics.Writer, args ...interface{}) {
	var leading float64
	var leadingSet bool

	w.TextBegin()
	for _, arg := range args {
		switch v := arg.(type) {
		case M:
			w.TextFirstLine(v.X, v.Y)
		case F:
			w.TextSetFont(v.Font, v.Size)
			leading = 0
			if l, ok := v.Font.(font.Layouter); ok {
				leading = l.GetGeometry().BaseLineDistance * v.Size
			}
			if leading <= 0 {
				leading = math.Round(v.Size*15) / 10
			}
			leadingSet = false
		case string:
			w.TextShow(v)
		case pdf.String:
			w.TextShowRaw(v)
		case color.Color:
			w.SetFillColor(v)
		case nl:
			if leadingSet {
				w.TextNextLine()
			} else {
				w.TextSecondLine(0, -leading)
				leadingSet = true
			}
		default:
			panic(fmt.Sprintf("unexpected argument type %T", v))
		}
	}
	w.TextEnd()
}

type nl struct{}

var NewLine = nl{}

type M struct {
	X, Y float64
}

type F struct {
	Font font.Font
	Size float64
}
