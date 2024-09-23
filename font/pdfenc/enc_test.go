// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2024  Jochen Voss <voss@seehuhn.de>
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

package pdfenc

import (
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/exp/maps"
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
		names1 := maps.Keys(seen)
		slices.Sort(names1)

		names2 := maps.Keys(enc.Has)
		slices.Sort(names2)

		if d := cmp.Diff(names1, names2); d != "" {
			t.Errorf("%d: inconsistent name lists:\n%s", i, d)
		}
	}
}
