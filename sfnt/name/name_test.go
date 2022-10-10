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

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf/sfnt/cmap"
)

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

func FuzzNames(f *testing.F) {
	info := &Info{
		Mac: Tables{
			"en": {
				Copyright:   "Copyright (c) 2022 Jochen Voss <voss@seehuhn.de>",
				Description: "This is a test.",
			},
			"de": {
				Copyright:   "Copyright (c) 2022 Jochen Voss <voss@seehuhn.de>",
				Description: "Dies ist ein Test.",
			},
		},
		Windows: Tables{
			"en-US": {
				Copyright:   "Copyright (c) 2022 Jochen Voss <voss@seehuhn.de>",
				Description: "This is a test.",
			},
			"de-DE": {
				Copyright:   "Copyright (c) 2022 Jochen Voss <voss@seehuhn.de>",
				Description: "Dies ist ein Test.",
			},
		},
	}
	f.Add(info.Encode(1))

	f.Fuzz(func(t *testing.T, in []byte) {
		n1, err := Decode(in)
		if err != nil {
			return
		}

		ss := make(cmap.Table)
		ss[cmap.Key{PlatformID: 3, EncodingID: 1}] = nil

		buf := n1.Encode(1)
		n2, err := Decode(buf)
		if err != nil {
			t.Fatal(err)
		}

		if diff := cmp.Diff(n1, n2); diff != "" {
			t.Errorf("different (-old +new):\n%s", diff)
		}
	})
}
