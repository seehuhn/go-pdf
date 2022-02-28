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
	"reflect"
	"testing"

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
			{locale.LangGerman, locale.CountryDEU}: {
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
		apple := map[locale.Language]uint16{}
		for code, loc := range appleCodes {
			apple[loc.Language] = code
		}
		ms := map[Loc]uint16{}
		for code, loc := range microsoftCodes {
			ms[loc] = code
		}
		for key := range n1.Tables {
			languageID, ok := apple[key.Language]
			if ok {
				ss = append(ss, cmap.SubtableData{
					PlatformID: 1, // Macintosh
					EncodingID: 0, // Roman
					Language:   languageID,
				})
			}
			languageID, ok = ms[key]
			if ok {
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

		equal := true
		keys := map[Loc]bool{}
		for key := range n1.Tables {
			keys[key] = true
		}
		for key := range n2.Tables {
			keys[key] = true
		}
		for key := range keys {
			if !reflect.DeepEqual(n1.Tables[key], n2.Tables[key]) {
				t1 := n1.Tables[key]
				t2 := n2.Tables[key]
				fmt.Println(key)
				if t1 == nil {
					fmt.Println("missing")
				} else {
					fmt.Println(t1)
				}
				if t2 == nil {
					fmt.Println("missing")
				} else {
					fmt.Println(t2)
				}

				equal = false
				break
			}
		}

		if !equal {
			// fmt.Printf("A % x\n", in)
			// fmt.Printf("B % x\n", buf)
			t.Fatal("not equal")
		}
	})
}
