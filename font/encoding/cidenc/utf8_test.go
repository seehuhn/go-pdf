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

package cidenc

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf/font/charcode"
)

func TestUTF8CS(t *testing.T) {
	var seen [4]bool
	cs := charcode.CodeSpaceRange{
		{Low: []byte{0}, High: []byte{0}},
		{Low: []byte{0, 0}, High: []byte{0, 0}},
		{Low: []byte{0, 0, 0}, High: []byte{0, 0, 0}},
		{Low: []byte{0, 0, 0, 0}, High: []byte{0, 0, 0, 0}},
	}
	for r := rune(0); r <= 0x10_FFFF; r++ {
		// avoid surrogates
		if 0xD800 <= r && r <= 0xDFFF {
			continue
		}

		b := []byte(string(r))
		k := len(b) - 1
		if !seen[k] {
			seen[k] = true
			for i := 0; i <= k; i++ {
				cs[k].Low[i] = b[i]
				cs[k].High[i] = b[i]
			}
		} else {
			for i := 0; i <= k; i++ {
				if b[i] < cs[k].Low[i] {
					cs[k].Low[i] = b[i]
				}
				if b[i] > cs[k].High[i] {
					cs[k].High[i] = b[i]
				}
			}
		}
	}
	if d := cmp.Diff(utf8cs, cs); d != "" {
		t.Error(d)
	}
}
