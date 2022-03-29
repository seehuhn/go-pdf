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

package gtab

import (
	"bytes"
	"reflect"
	"testing"

	"seehuhn.de/go/pdf/font/parser"
	"seehuhn.de/go/pdf/locale"
)

func FuzzScriptList(f *testing.F) {
	info := ScriptListInfo{}
	info[ScriptLang{script: locale.ScriptUndefined, lang: locale.LangUndefined}] = &Features{
		Optional: []FeatureIndex{0},
	}
	f.Add(info.encode())
	info = ScriptListInfo{}
	info[ScriptLang{script: locale.ScriptLatin, lang: locale.LangEnglish}] = &Features{
		Required: FeatureIndex(0xFFFF),
		Optional: []FeatureIndex{1, 2, 3, 4},
	}
	info[ScriptLang{script: locale.ScriptLatin, lang: locale.LangUndefined}] = &Features{
		Required: 7,
		Optional: []FeatureIndex{5, 6},
	}
	f.Add(info.encode())
	info = ScriptListInfo{}
	info[ScriptLang{script: locale.ScriptArabic, lang: locale.LangUndefined}] = &Features{
		Required: FeatureIndex(0),
		Optional: []FeatureIndex{1, 2},
	}
	f.Add(info.encode())
	info = ScriptListInfo{}
	info[ScriptLang{script: locale.ScriptUndefined, lang: locale.LangArabic}] = &Features{
		Required: FeatureIndex(0xFFFF),
		Optional: []FeatureIndex{1, 3, 5},
	}
	info[ScriptLang{script: locale.ScriptUndefined, lang: locale.LangAzerbaijani}] = &Features{
		Required: FeatureIndex(0xFFFF),
		Optional: []FeatureIndex{2, 4, 6},
	}
	f.Add(info.encode())

	f.Fuzz(func(t *testing.T, data []byte) {
		p := parser.New("scriptList test", bytes.NewReader(data))
		info, err := readScriptList(p, 0)
		if err != nil {
			return
		}

		data2 := info.encode()

		// if len(data2) > len(data) {
		// 	fmt.Printf("A % x\n", data)
		// 	fmt.Printf("B % x\n", data2)
		// 	t.Errorf("encode: %d > %d", len(data2), len(data))
		// }

		p = parser.New("scriptList test", bytes.NewReader(data2))
		info2, err := readScriptList(p, 0)
		if err != nil {
			t.Fatal(err)
		}

		if !reflect.DeepEqual(info, info2) {
			t.Error("different")
		}
	})
}
