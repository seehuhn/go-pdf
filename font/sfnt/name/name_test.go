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
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"seehuhn.de/go/pdf/font/sfnt/cmap"
	"seehuhn.de/go/pdf/locale"
)

func FuzzNames(f *testing.F) {
	names, err := filepath.Glob("../../../demo/try-all-fonts/name/*.bin")
	if err != nil {
		f.Fatal(err)
	} else if len(names) < 2 {
		f.Fatal("need at least two fonts")
	}
	for _, fname := range names {
		body, err := os.ReadFile(fname)
		if err != nil {
			f.Fatal(err)
		}
		f.Add(body)
	}

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
