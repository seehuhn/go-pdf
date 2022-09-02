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
	"fmt"
	"testing"

	"golang.org/x/text/language"
	"golang.org/x/text/language/display"
)

func TestLanguageTags(t *testing.T) {
	en := display.English.Tags()
	seen := make(map[string]bool)
	for _, list := range []map[uint16]string{appleBCP, msBCP} {
		for _, lang := range list {
			if seen[lang] {
				t.Errorf("duplicate language code %q", lang)
			}
			tag := language.MustParse(lang)
			region, _ := tag.Region()
			script, _ := tag.Script()
			if script.String() == "Zzzz" {
				t.Error(lang)
			}
			fmt.Println(en.Name(tag), script, region)
		}
	}
}

func TestUTF16(t *testing.T) {
	cases := []string{
		"",
		"hello",
		"♠♡♢♣",
	}
	for _, c := range cases {
		buf := utf16Encode(c)
		d := utf16Decode(buf)
		if d != c {
			t.Errorf("%q -> % x -> %q", c, buf, d)
		}
	}
}
