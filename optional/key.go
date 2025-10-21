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

package optional

import "seehuhn.de/go/pdf"

// Int represents an optional integer.
type Int struct {
	val uint64
}

func NewInt(v pdf.Integer) Int {
	var k Int
	k.Set(v)
	return k
}

func (k Int) Get() (pdf.Integer, bool) {
	if k.val == 0 {
		return 0, false
	}
	return pdf.Integer(k.val - 1), true
}

func (k *Int) Set(v pdf.Integer) {
	if v < 0 || uint64(v) >= 1<<64-1 {
		panic("key value out of range")
	}
	k.val = uint64(v) + 1
}

func (k *Int) Clear() {
	k.val = 0
}

// Equal compares two Keys for equality.
func (k Int) Equal(other Int) bool {
	return k.val == other.val
}
