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

package gtab

import (
	"testing"

	"seehuhn.de/go/pdf/font"
)

func FuzzGsub1_1(f *testing.F) {
	l := &Gsub1_1{
		Cov:   map[font.GlyphID]int{3: 0},
		Delta: 26,
	}
	f.Add(l.Encode())

	f.Fuzz(func(t *testing.T, data []byte) {
		doFuzz(t, 1, 1, readGsub1_1, data)
	})
}

func FuzzGsub1_2(f *testing.F) {
	l := &Gsub1_2{
		Cov:                map[font.GlyphID]int{2: 0, 3: 1},
		SubstituteGlyphIDs: []font.GlyphID{6, 7},
	}
	f.Add(l.Encode())

	f.Fuzz(func(t *testing.T, data []byte) {
		doFuzz(t, 1, 2, readGsub1_2, data)
	})
}

func FuzzGsub2_1(f *testing.F) {
	l := &Gsub2_1{
		Cov: map[font.GlyphID]int{2: 0, 3: 1},
		Repl: [][]font.GlyphID{
			{4, 5},
			{1, 2, 3},
		},
	}
	f.Add(l.Encode())

	f.Fuzz(func(t *testing.T, data []byte) {
		doFuzz(t, 2, 1, readGsub2_1, data)
	})
}

func FuzzGsub3_1(f *testing.F) {
	l := &Gsub3_1{
		Cov: map[font.GlyphID]int{1: 0, 2: 1},
		Alt: [][]font.GlyphID{
			{3, 4},
			{5, 6, 7},
		},
	}
	f.Add(l.Encode())

	f.Fuzz(func(t *testing.T, data []byte) {
		doFuzz(t, 3, 1, readGsub3_1, data)
	})
}

func FuzzGsub4_1(f *testing.F) {
	l := &Gsub4_1{
		Cov: map[font.GlyphID]int{1: 0, 2: 1},
		Repl: [][]Ligature{
			{
				{In: []font.GlyphID{1, 2, 3}, Out: 10},
				{In: []font.GlyphID{1, 2}, Out: 11},
				{In: []font.GlyphID{1}, Out: 12},
			},
			{
				{In: []font.GlyphID{1, 2}, Out: 13},
				{In: []font.GlyphID{1}, Out: 14},
			},
		},
	}
	f.Add(l.Encode())

	f.Fuzz(func(t *testing.T, data []byte) {
		doFuzz(t, 4, 1, readGsub4_1, data)
	})
}
