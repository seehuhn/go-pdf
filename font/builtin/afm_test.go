// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2021  Jochen Voss <voss@seehuhn.de>
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

package builtin

import (
	"os"
	"testing"
)

func Test14Fonts(t *testing.T) {
	known, err := afmData.ReadDir("afm")
	if err != nil {
		t.Fatal(err)
	}
	if len(known) != len(FontNames) {
		t.Error("wrong number of afm files:", len(known))
	}

	for _, fontName := range FontNames {
		afm, err := Afm(fontName)
		if err != nil {
			t.Error(err)
			continue
		}

		if afm.FontName != fontName {
			t.Errorf("wrong font name: %q != %q", afm.FontName, fontName)
		}
	}
}

func TestError(t *testing.T) {
	_, err := Afm("unknown font")
	if !os.IsNotExist(err) {
		t.Errorf("wrong error: %s", err)
	}
}
