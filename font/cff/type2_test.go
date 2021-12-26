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

package cff

import (
	"testing"
)

func TestRoll(t *testing.T) {
	in := []int32{1, 2, 3, 4, 5, 6, 7, 8}
	out := []int32{1, 2, 4, 5, 6, 3, 7, 8}

	min := make([]stackSlot, len(in))
	for i, v := range in {
		min[i].val = v
	}
	mout := make([]stackSlot, len(in))
	for i, v := range out {
		mout[i].val = v
	}

	roll(min[2:6], 3)
	for i, x := range min {
		if mout[i] != x {
			t.Error(min, mout)
			break
		}
	}
}
