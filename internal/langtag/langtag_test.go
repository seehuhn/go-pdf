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

package langtag

import (
	"testing"

	"golang.org/x/text/language"
)

func TestParse(t *testing.T) {
	cases := []struct {
		in   string
		want language.Tag
	}{
		{"en-US", language.AmericanEnglish},
		{"de", language.German},
		{"EN-us", language.AmericanEnglish},
		{"und-DE", language.MustParse("und-DE")},
		{"", language.Und},

		// malformed
		{"en-US-foobarbazqux", language.Und},
		{"en--US", language.Und},
		{"en-US-", language.Und},
		{"123-de", language.Und},
		{"zz-ZZ-qqqqqqqqq", language.Und},

		// unknown subtags after the primary language
		{"en-XYZ", language.English},
		{"de-1234", language.German},
		{"en-abcde", language.English},
		{"en-US-abcde-abcde", language.AmericanEnglish},

		// private-use and special tags survive
		{"x-foo", language.MustParse("x-foo")},
		{"en-x-foo", language.MustParse("en-x-foo")},
		{"qaa-DE", language.MustParse("qaa-DE")},
		{"zxx", language.MustParse("zxx")},

		// unknown primary language
		{"qq", language.Und},
		{"qq-en", language.Und},
		{"qq-DE", language.Und},
		{"xx-Latn", language.Und},
	}
	for _, tc := range cases {
		if got := Parse(tc.in); got != tc.want {
			t.Errorf("%q: got %v, want %v", tc.in, got, tc.want)
		}
	}
}
