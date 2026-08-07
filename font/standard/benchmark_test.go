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

// BenchmarkNewAll measures the time to make an instance of each of the 14
// standard fonts, once each and in the order of [All].
//
// The fonts are read before the loop starts, so this measures what a caller
// pays once that work is shared: the encoding state of a clone and nothing
// else.  Use [BenchmarkReadAll] for the cost of the reading itself.
func BenchmarkNewAll(b *testing.B) {
	for _, F := range All {
		if _, err := F.New(); err != nil {
			b.Fatal(err)
		}
	}
	for b.Loop() {
		for _, F := range All {
			if _, err := F.New(); err != nil {
				b.Fatal(err)
			}
		}
	}
}

// BenchmarkReadAll measures the time to read all 14 standard fonts from the
// bundled font data, bypassing the sharing done by [Font.New].
func BenchmarkReadAll(b *testing.B) {
	for b.Loop() {
		for _, F := range All {
			if _, err := F.read(); err != nil {
				b.Fatal(err)
			}
		}
	}
}
