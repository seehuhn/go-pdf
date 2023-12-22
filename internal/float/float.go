// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2022  Jochen Voss <voss@seehuhn.de>
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

package float

import (
	"regexp"
	"strconv"
	"strings"
)

func Format(x float64, precision int) string {
	out := strconv.FormatFloat(x, 'f', precision, 64)
	if m := tailRegexp.FindStringSubmatchIndex(out); m != nil {
		if m[2] > 0 {
			out = out[:m[2]]
		} else if m[4] > 0 {
			out = out[:m[4]]
		}
	}
	if strings.HasPrefix(out, "0.") {
		out = out[1:]
	}
	return out
}

func Round(x float64, digits int) float64 {
	s := Format(x, digits)
	y, err := strconv.ParseFloat(s, 64)
	if err != nil {
		panic(err)
	}
	return y
}

var (
	tailRegexp = regexp.MustCompile(`(?:\..*[1-9](0+)|(\.0+))$`)
)
