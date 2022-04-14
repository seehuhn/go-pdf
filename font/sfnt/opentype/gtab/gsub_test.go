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
	"bytes"
	"reflect"
	"testing"

	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/parser"
)

func doFuzz(t *testing.T, lookupType, lookupFormat uint16,
	readFn func(p *parser.Parser, subtablePos int64) (Subtable, error),
	data1 []byte) {

	// t.Helper()

	p := parser.New("test", bytes.NewReader(data1))
	format, err := p.ReadUInt16()
	if err != nil || format != lookupFormat {
		return
	}

	l1, err := readFn(p, 0)
	if err != nil {
		return
	}

	data2 := l1.Encode()
	if len(data2) != l1.EncodeLen() {
		t.Errorf("encodeLen mismatch: %d != %d", len(data2), l1.EncodeLen())
	}

	p = parser.New("test", bytes.NewReader(data2))
	format, err = p.ReadUInt16()
	if err != nil {
		t.Fatal(err)
	} else if format != lookupFormat {
		t.Fatalf("unexpected format: %d.%d", lookupType, format)
	}
	l2, err := readFn(p, 0)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(l1, l2) {
		t.Error("different")
	}
}

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
