// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2021  Jochen Voss <voss@seehuhn.de>
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

package table

import "testing"

func TestChecksum(t *testing.T) {
	cases := []struct {
		Body     []byte
		Expected uint32
	}{
		{[]byte{0, 1, 2, 3}, 0x00010203},
		{[]byte{0, 1, 2, 3, 4, 5, 6, 7}, 0x0406080a},
		{[]byte{1}, 0x01000000},
		{[]byte{1, 2, 3}, 0x01020300},
		{[]byte{1, 0, 0, 0, 1}, 0x02000000},
		{[]byte{255, 255, 255, 255, 0, 0, 0, 1}, 0},
	}

	for i, test := range cases {
		computed := checksum(test.Body)
		if computed != test.Expected {
			t.Errorf("test %d failed: %08x != %08x",
				i+1, computed, test.Expected)
		}
	}
}
