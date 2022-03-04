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

	"github.com/go-test/deep"
	"seehuhn.de/go/pdf/font/sfnt/cmap"
	"seehuhn.de/go/pdf/locale"
)

func FuzzNames(f *testing.F) {
	info := &Info{
		Tables: map[Loc]*Table{
			{locale.LangEnglish, locale.CountryUSA}: {
				Copyright:   "Copyright (c) 2022 Jochen Voss <voss@seehuhn.de>",
				Description: "This is a test.",
			},
			{locale.LangEnglish, locale.CountryUndefined}: {
				Copyright:   "Copyright (c) 2022 Jochen Voss <voss@seehuhn.de>",
				Description: "This is a test.",
			},
			{locale.LangGerman, locale.CountryDEU}: {
				Copyright:   "Copyright (c) 2022 Jochen Voss <voss@seehuhn.de>",
				Description: "Dies ist ein Test.",
			},
			{locale.LangGerman, locale.CountryUndefined}: {
				Copyright:   "Copyright (c) 2022 Jochen Voss <voss@seehuhn.de>",
				Description: "Dies ist ein Test.",
			},
		},
	}
	ss := cmap.Subtables{
		{
			PlatformID: 0,
			EncodingID: 4,
			Language:   0,
		},
		{
			PlatformID: 1,
			EncodingID: 0,
			Language:   0,
		},
		{
			PlatformID: 1,
			EncodingID: 0,
			Language:   2,
		},
		{
			PlatformID: 3,
			EncodingID: 1,
			Language:   0,
		},
		{
			PlatformID: 3,
			EncodingID: 1,
			Language:   0x0407,
		},
	}
	f.Add(info.Encode(ss))

	f.Fuzz(func(t *testing.T, in []byte) {
		n1, err := Decode(in)
		if err != nil {
			return
		}

		var ss cmap.Subtables
		for key := range n1.Tables {
			languageID, ok := appleCode(key.Language)
			if ok {
				ss = append(ss, cmap.SubtableData{
					PlatformID: 1, // Macintosh
					EncodingID: 0, // Roman
					Language:   languageID,
				})
			}
			languageID = msCode(key)
			if languageID != 0xFFFF {
				ss = append(ss, cmap.SubtableData{
					PlatformID: 3, // Windows
					EncodingID: 1, // Unicode BMP
					Language:   languageID,
				})
			}
		}

		buf := n1.Encode(ss)
		n2, err := Decode(buf)
		if err != nil {
			t.Fatal(err)
		}

		for _, diff := range deep.Equal(n1, n2) {
			t.Error(diff)
		}
	})
}
