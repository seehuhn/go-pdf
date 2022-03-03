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

package mac

import "testing"

func TestEncoding(t *testing.T) {
	for i := 0; i < 256; i++ {
		s := Decode([]byte{byte(i)})
		cc := Encode(s)
		if len(cc) != 1 || cc[0] != byte(i) {
			t.Errorf("%d: %q -> %q", i, s, cc)
		}
	}
}
