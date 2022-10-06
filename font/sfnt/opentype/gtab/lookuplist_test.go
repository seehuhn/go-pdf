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

func readDummySubtable(p *parser.Parser, pos int64, info *LookupMetaInfo) (Subtable, error) {
	if info.LookupType > 32 {
		return nil, errors.New("invalid type for dummy lookup")
	}
	err := p.SeekPos(pos)
	if err != nil {
		return nil, err
	}
	res := make(dummySubTable, info.LookupType)
	_, err = p.Read(res)
	if err != nil {
		return nil, err
	}
	return res, nil
}

type dummySubTable []byte

func (st dummySubTable) Apply(_ keepGlyphFn, glyphs []font.Glyph, a, b int) *Match {
	return nil
}

func (st dummySubTable) EncodeLen() int {
	return len(st)
}

func (st dummySubTable) Encode() []byte {
	return []byte(st)
}

func FuzzLookupList(f *testing.F) {
	l := LookupList{
		&LookupTable{
			Meta: &LookupMetaInfo{},
			Subtables: Subtables{
				dummySubTable{},
			},
		},
	}
	f.Add(l.encode(999))

	l = LookupList{
		&LookupTable{
			Meta: &LookupMetaInfo{
				LookupType: 4,
				LookupFlag: LookupUseMarkFilteringSet,
			},
			Subtables: Subtables{
				dummySubTable{1, 2, 3, 4},
			},
		},
	}
	f.Add(l.encode(999))

	l = LookupList{
		&LookupTable{
			Meta: &LookupMetaInfo{
				LookupType: 1,
			},
			Subtables: Subtables{
				dummySubTable{0},
				dummySubTable{1},
				dummySubTable{2},
			},
		},
		&LookupTable{
			Meta: &LookupMetaInfo{
				LookupType:       2,
				LookupFlag:       LookupUseMarkFilteringSet,
				MarkFilteringSet: 7,
			},
			Subtables: Subtables{
				dummySubTable{3, 4},
				dummySubTable{5, 6},
			},
		},
		&LookupTable{
			Meta: &LookupMetaInfo{
				LookupType: 3,
			},
			Subtables: Subtables{
				dummySubTable{7, 8, 9},
			},
		},
	}
	f.Add(l.encode(999))

	f.Fuzz(func(t *testing.T, data1 []byte) {
		p := parser.New("lookupList test", bytes.NewReader(data1))
		l1, err := readLookupList(p, 0, readDummySubtable)
		if err != nil {
			return
		}

		data2 := l1.encode(999)

		p = parser.New("lookupList test", bytes.NewReader(data2))
		l2, err := readLookupList(p, 0, readDummySubtable)
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
