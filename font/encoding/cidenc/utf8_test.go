// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

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
	if d := cmp.Diff(charcode.UTF8, cs); d != "" {
		t.Error(d)
	}
}
