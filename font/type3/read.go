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

package type3

import (
	"math"
	"regexp"
	"strconv"

	"seehuhn.de/go/postscript/funit"
)

func setGlyphGeometry(g *Glyph, data []byte) {
	m := type3StartRegexp.FindSubmatch(data)
	if len(m) != 9 {
		return
	}
	if m[1] != nil {
		g.WidthX, _ = strconv.ParseFloat(string(m[1]), 64)
	} else if m[3] != nil {
		var xx [6]float64
		for i := range xx {
			xx[i], _ = strconv.ParseFloat(string(m[3+i]), 64)
		}
		g.WidthX = xx[0]
		g.BBox = funit.Rect16{
			LLx: funit.Int16(math.Round(xx[2])),
			LLy: funit.Int16(math.Round(xx[3])),
			URx: funit.Int16(math.Round(xx[4])),
			URy: funit.Int16(math.Round(xx[5])),
		}
	}
}

var (
	spc = `[\t\n\f\r ]+`
	num = `([+-]?[0-9.]+)` + spc
	d0  = num + num + "d0"
	d1  = num + num + num + num + num + num + "d1"

	type3StartRegexp = regexp.MustCompile(`^[\t\n\f\r ]*(?:` + d0 + "|" + d1 + ")" + spc)
)
