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

package head

import (
	"testing"
	"time"
)

func TestTimeEncoding(t *testing.T) {
	for _, z := range []int64{0, 1, 2, 10, 100, 1000, 10000, 100000, 1000000,
		10000000, 100000000, 1000000000} {
		for _, s := range []int64{-1, 1} {
			x := z * s
			if encodeTime(decodeTime(x)) != x {
				t.Errorf("encodeTime(%d) != %d", x, x)
			}

			if x != 0 && x+1 != 0 {
				t1 := decodeTime(x).Unix()
				t2 := decodeTime(x + 1).Unix()
				if t1+1 != t2 {
					t.Errorf("decodeTime(%d+1) != decodeTime(%d)+1", x, x)
				}
			}

		}
	}
}

func TestEpoch(t *testing.T) {
	epoch := time.Date(1904, time.January, 1, 0, 0, 0, 0, time.UTC)
	if zeroTime != epoch.Unix() {
		t.Errorf("zeroTime != %d", epoch.Unix())
	}

	if encodeTime(epoch) != 0 {
		t.Errorf("encodeTime(%s) != 0", epoch)
	}

	if !decodeTime(0).IsZero() {
		t.Error("decodeTime(0) != zero")
	}
}
