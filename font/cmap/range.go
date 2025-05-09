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

package cmap

import (
	"bytes"
	"iter"
)

func rangeIsValid(first, last []byte) bool {
	if len(first) != len(last) || len(first) == 0 {
		return false
	}
	for i := 0; i < len(first); i++ {
		if first[i] > last[i] {
			return false
		}
	}
	return true
}

func codesInRange(first []byte, last []byte) iter.Seq2[int, []byte] {
	if !rangeIsValid(first, last) {
		// If there are no valid codes in the range, return an empty iterator.
		return func(yield func(int, []byte) bool) {}
	}

	return func(yield func(int, []byte) bool) {
		idx := 0
		buf := bytes.Clone(first)
		for {
			if !yield(idx, buf) {
				return
			}

			pos := len(first) - 1
			for pos >= 0 {
				if buf[pos] < last[pos] {
					buf[pos]++
					break
				}
				buf[pos] = first[pos]
				pos--
			}
			if pos < 0 {
				break
			}
			idx++
		}
	}
}
