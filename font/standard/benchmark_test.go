// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2026  Jochen Voss <voss@seehuhn.de>
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

package standard

import "testing"

// BenchmarkNewAll measures the time to load all 14 standard fonts, once each
// and in the order of [All].
//
// Nothing is cached between calls, so each iteration parses all 14 font
// programs afresh: every font program is run through a PostScript
// interpreter, its private section is eexec-decrypted, every glyph's
// charstring is decoded, and the glyph metrics are read from the AFM file.
func BenchmarkNewAll(b *testing.B) {
	for b.Loop() {
		for _, F := range All {
			if _, err := F.New(); err != nil {
				b.Fatal(err)
			}
		}
	}
}
