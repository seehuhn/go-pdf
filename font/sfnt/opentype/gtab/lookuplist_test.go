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
	"errors"
	"fmt"
	"reflect"
	"testing"

	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/parser"
)

type dummySubTable []byte

func (st dummySubTable) Apply(meta *LookupMetaInfo, glyphs []font.Glyph, pos int) ([]font.Glyph, int) {
	return glyphs, -1
}

func (st dummySubTable) EncodeLen(meta *LookupMetaInfo) int {
	return len(st)
}

func (st dummySubTable) Encode(meta *LookupMetaInfo) []byte {
	if len(st) != int(meta.LookupType) {
		panic("wrong size for dummy lookup")
	}
	return []byte(st)
}

func dummyReader(p *parser.Parser, pos int64, info *LookupMetaInfo) (Subtable, error) {
	err := p.SeekPos(pos)
	if err != nil {
		return nil, err
	}
	if info.LookupType > 32 {
		return nil, errors.New("invalid size for dummy lookup")
	}
	res := make(dummySubTable, int(info.LookupType))
	_, err = p.Read(res)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func FuzzLookupList(f *testing.F) {
	l := LookupList{
		&LookupTable{
			Meta: &LookupMetaInfo{},
			Subtables: []Subtable{
				dummySubTable{},
			},
		},
	}
	f.Add(l.encode())

	l = LookupList{
		&LookupTable{
			Meta: &LookupMetaInfo{
				LookupType: 4,
				LookupFlag: 0x0010,
			},
			Subtables: []Subtable{
				dummySubTable{1, 2, 3, 4},
			},
		},
	}
	f.Add(l.encode())

	l = LookupList{
		&LookupTable{
			Meta: &LookupMetaInfo{
				LookupType: 1,
			},
			Subtables: []Subtable{
				dummySubTable{0},
				dummySubTable{1},
				dummySubTable{2},
			},
		},
		&LookupTable{
			Meta: &LookupMetaInfo{
				LookupType:       2,
				LookupFlag:       0x0010,
				MarkFilteringSet: 7,
			},
			Subtables: []Subtable{
				dummySubTable{3, 4},
				dummySubTable{5, 6},
			},
		},
		&LookupTable{
			Meta: &LookupMetaInfo{
				LookupType: 3,
			},
			Subtables: []Subtable{
				dummySubTable{7, 8, 9},
			},
		},
	}
	f.Add(l.encode())

	f.Fuzz(func(t *testing.T, data1 []byte) {
		p := parser.New("lookupList test", bytes.NewReader(data1))
		l1, err := readLookupList(p, 0, dummyReader)
		if err != nil {
			return
		}

		data2 := l1.encode()
		p = parser.New("lookupList test", bytes.NewReader(data2))
		l2, err := readLookupList(p, 0, dummyReader)
		if err != nil {
			t.Fatal(err)
		}

		if !reflect.DeepEqual(l1, l2) {
			fmt.Printf("A % x\n", data1)
			fmt.Printf("B % x\n", data2)
			fmt.Println(l1)
			fmt.Println(l2)
			t.Fatal("different")
		}
	})
}
