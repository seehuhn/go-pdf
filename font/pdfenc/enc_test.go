// Copyright 2024 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package pdfenc

import (
	"maps"
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestEncoding(t *testing.T) {
	encodings := []Encoding{
		Standard,
		WinAnsi,
		MacRoman,
		MacRomanAlt,
		MacExpert,
		Symbol,
		ZapfDingbats,
		PDFDoc,
	}
	for i, enc := range encodings {
		seen := make(map[string]bool)
		for _, name := range enc.Encoding {
			if name == ".notdef" {
				continue
			}
			seen[name] = true
		}
		names1 := slices.Sorted(maps.Keys(seen))

		names2 := slices.Sorted(maps.Keys(enc.Has))

		if d := cmp.Diff(names1, names2); d != "" {
			t.Errorf("%d: inconsistent name lists:\n%s", i, d)
		}
	}
}
