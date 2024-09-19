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

package pdfenc

import (
	"slices"
	"testing"

	"golang.org/x/exp/maps"
)

func TestEncoding(t *testing.T) {
	encodings := []Encoding{
		Symbol,
		ZapfDingbats,
	}
	for _, enc := range encodings {
		var names1 []string
		for _, name := range enc.Encoding {
			if name != ".notdef" {
				names1 = append(names1, name)
			}
		}
		slices.Sort(names1)

		names2 := maps.Keys(enc.Has)
		slices.Sort(names2)

		if !slices.Equal(names1, names2) {
			t.Error("inconsistent name lists")
		}
	}
}
