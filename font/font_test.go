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

package font_test

import (
	"testing"
	"unicode"

	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/gofont"
	"seehuhn.de/go/pdf/font/type1"
)

func TestGetGID(t *testing.T) {
	t1, err := gofont.Type1(gofont.GoRegular)
	if err != nil {
		t.Fatal(err)
	}
	F, err := type1.New(t1)
	if err != nil {
		t.Fatal(err)
	}

	runeIsUsed := make(map[rune]bool)
	for r := range F.CMap {
		runeIsUsed[r] = true
	}
	var unusedRune rune
	for r := rune(33); ; r++ {
		if unicode.IsGraphic(r) && !runeIsUsed[r] {
			unusedRune = r
			break
		}
	}

	gid, _ := font.GetGID(F, unusedRune)
	if gid != 0 {
		t.Errorf("GetGID(%q) = %d, expected 0", unusedRune, gid)
	}
}

func TestGetGID2(t *testing.T) {
	F, err := gofont.Type3(gofont.GoRegular)
	if err != nil {
		t.Fatal(err)
	}

	runeIsUsed := make(map[rune]bool)
	for r := range F.CMap {
		runeIsUsed[r] = true
	}
	var unusedRune rune
	for r := rune(33); ; r++ {
		if unicode.IsGraphic(r) && !runeIsUsed[r] {
			unusedRune = r
			break
		}
	}

	gid, _ := font.GetGID(F, unusedRune)
	if gid != 0 {
		t.Errorf("GetGID(%q) = %d, expected 0", unusedRune, gid)
	}
}
