// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2023  Jochen Voss <voss@seehuhn.de>
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

package tounicode

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf/font/charcode"
)

func TestFromMapping(t *testing.T) {
	m := map[charcode.CharCode][]rune{
		'A': []rune("A"), // single
		'B': []rune("X"), // single
		'C': []rune("C"), // range ...
		'D': []rune("D"),
		'E': []rune("E"),
		'F': []rune("F"),
		'G': []rune("G"),
	}
	info := &Info{
		CS: charcode.Simple,
	}
	info.SetMapping(m)
	if len(info.Singles) != 2 {
		t.Fatalf("expected 2 singles, got %d:\n%v", len(info.Singles), info)
	}
	if info.Singles[0].Code != 'A' {
		t.Errorf("expected 'A', got %d", info.Singles[0].Code)
	}
	if info.Singles[1].Code != 'B' {
		t.Errorf("expected 'B', got %d", info.Singles[1].Code)
	}
	if len(info.Ranges) != 1 {
		t.Fatalf("expected 1 range, got %d", len(info.Ranges))
	}
	if info.Ranges[0].First != 'C' {
		t.Errorf("expected 'C', got %d", info.Ranges[0].First)
	}
}

func TestFromMapping2(t *testing.T) {
	m := map[charcode.CharCode][]rune{
		'A': []rune("A"), // range ...
		'C': []rune("C"),
		'E': []rune("E"),
	}
	info := &Info{
		CS: charcode.Simple,
	}
	info.SetMapping(m)
	if len(info.Singles) != 0 {
		t.Fatalf("expected 0 singles, got %d", len(info.Singles))
	}
	if len(info.Ranges) != 1 {
		t.Fatalf("expected 1 range, got %d", len(info.Ranges))
	}
	r := info.Ranges[0]
	rExpected := RangeEntry{
		First:  'A',
		Last:   'E',
		Values: [][]rune{[]rune("A")},
	}
	if d := cmp.Diff(r, rExpected); d != "" {
		t.Errorf("unexpected range: %s", d)
	}
}
