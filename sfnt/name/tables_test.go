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

package name

import (
	"testing"

	"golang.org/x/text/language"
)

func TestChoose(t *testing.T) {
	tt := Tables{}
	makeTable := func(lang string, numEntries int) {
		t := &Table{}
		for i := 0; i < numEntries; i++ {
			t.set(ID(i), "x")
		}
		t.set(1000, lang)
		tt[lang] = t
	}
	makeTable("en-GB", 3)
	makeTable("en-US", 2)

	table, conf := tt.Choose(language.AmericanEnglish)
	if got := table.get(1000); got != "en-US" || conf != language.Exact {
		t.Errorf("%q %d", got, conf)
	}

	table, conf = tt.Choose(language.BritishEnglish)
	if got := table.get(1000); got != "en-GB" || conf != language.Exact {
		t.Errorf("%q %d", got, conf)
	}

	table, conf = tt.Choose(language.MustParse("en-NZ"))
	if got := table.get(1000); got != "en-GB" || conf != language.High {
		t.Errorf("%q %d", got, conf)
	}

	table, conf = tt.Choose(language.German)
	if got := table.get(1000); got != "en-US" || conf != language.No {
		t.Errorf("%q %d", got, conf)
	}
}
