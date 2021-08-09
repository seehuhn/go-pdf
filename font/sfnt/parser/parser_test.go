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

package parser

import (
	"testing"

	"seehuhn.de/go/pdf/font/sfnt"
	"seehuhn.de/go/pdf/locale"
)

func TestInterpreter(t *testing.T) {
	tt, err := sfnt.Open("../../truetype/ttf/SourceSerif4-Regular.ttf")
	// tt, err := sfnt.Open("../../truetype/ttf/FreeSerif.ttf")
	// tt, err := sfnt.Open("../../truetype/ttf/Roboto-Regular.ttf")
	// tt, err := sfnt.Open("/Applications/LibreOffice.app/Contents/Resources/fonts/truetype/DejaVuSerif.ttf")
	if err != nil {
		t.Fatal(err)
	}
	defer tt.Close()

	tableName := "GSUB"

	includeFeature := make(map[string]bool)
	if tableName == "GSUB" {
		includeFeature["ccmp"] = true
		includeFeature["liga"] = true
		includeFeature["clig"] = true
	} else { // tableName == "GPOS"
		includeFeature["kern"] = true
		includeFeature["mark"] = true
		includeFeature["mkmk"] = true
	}

	p := New(tt)
	g, err := newGTab(p, tableName, locale.EnGB, includeFeature)
	if err != nil {
		t.Fatal(err)
	}

	if tableName == "GSUB" {
		for _, idx := range g.lookupIndices {
			_, err := g.getGtabLookup(idx)
			if err != nil {
				t.Fatal(err)
			}
		}
	}
}
