// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package cmap

import (
	"bytes"
	"iter"

	"seehuhn.de/go/pdf/font/charcode"
)

// validCodeLength reports whether a character code of n bytes is supported,
// see [charcode.MaxCodeLength].  The CMap readers drop entries with longer codes,
// since these can never match a decoded code.
func validCodeLength(n int) bool {
	return n >= 1 && n <= charcode.MaxCodeLength
}

func rangeIsValid(first, last []byte) bool {
	if len(first) != len(last) || len(first) == 0 {
		return false
	}
	for i := range first {
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
