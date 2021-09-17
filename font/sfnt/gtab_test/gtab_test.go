// seehuhn.de/go/pdf - support for reading and writing PDF files
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

package gtab_test

import (
	"fmt"
	"path/filepath"
	"testing"

	"seehuhn.de/go/pdf/font/sfnt"
	"seehuhn.de/go/pdf/font/sfnt/gtab"
	"seehuhn.de/go/pdf/locale"
)

func TestGtab(t *testing.T) {
	allTTF, err := filepath.Glob("../../truetype/ttf/*.ttf")
	if err != nil {
		t.Fatal(err)
	}
	allOTF, err := filepath.Glob("../../opentype/otf/*.otf")
	if err != nil {
		t.Fatal(err)
	}

	fonts := append(allTTF, allOTF...)
	for _, font := range fonts {
		tt, err := sfnt.Open(font, locale.EnGB)
		if err != nil {
			t.Error(err)
			continue
		}
		p, err := gtab.New(tt.Header, tt.Fd, locale.EnGB)
		if err != nil {
			t.Error(err)
		}

		gsub, err := p.ReadGsubTable()
		if err != nil {
			t.Error(err)
		}
		gpos, err := p.ReadGposTable()
		if err != nil {
			t.Error(err)
		}
		fmt.Println(font, len(gsub), len(gpos))

		for i, l := range gsub {
			for j, s := range l.Subtables {
				fmt.Printf("\tGSUB %d.%d %T\n", i, j, s)
			}
		}

		for i, l := range gpos {
			for j, s := range l.Subtables {
				fmt.Printf("\tGPOS %d.%d %T\n", i, j, s)
			}
		}

		err = tt.Close()
		if err != nil {
			t.Error(err)
		}
	}
}
