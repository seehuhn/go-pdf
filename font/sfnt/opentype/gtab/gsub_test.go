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

	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/parser"
)

func FuzzGsub1_1(f *testing.F) {
	l := &Gsub1_1{
		Cov:   map[font.GlyphID]int{3: 0},
		Delta: 26,
	}
	meta := &LookupMetaInfo{LookupType: 1}
	f.Add(l.Encode(meta))

	f.Fuzz(func(t *testing.T, data []byte) {
		p := parser.New("test", bytes.NewReader(data))
		format, err := p.ReadUInt16()
		if err != nil || format != 1 {
			return
		}

		l1, err := readGsub1_1(p, 0)
		if err != nil {
			return
		}

		data2 := l1.Encode(meta)
		if len(data2) != l1.EncodeLen(meta) {
			t.Errorf("encodeLen mismatch: %d != %d", len(data2), l1.EncodeLen(meta))
		}

		p = parser.New("test", bytes.NewReader(data2))
		format, err = p.ReadUInt16()
		if err != nil {
			t.Fatal(err)
		} else if format != 1 {
			t.Fatalf("unexpected format: %d", format)
		}
		l2, err := readGsub1_1(p, 0)
		if err != nil {
			t.Fatal(err)
		}

		if !reflect.DeepEqual(l1, l2) {
			t.Error("different")
		}
	})
}

func FuzzGsub1_2(f *testing.F) {
	l := &Gsub1_2{
		Cov:                map[font.GlyphID]int{3: 0, 2: 1},
		SubstituteGlyphIDs: []font.GlyphID{6, 7},
	}
	meta := &LookupMetaInfo{LookupType: 1}
	f.Add(l.Encode(meta))

	f.Fuzz(func(t *testing.T, data []byte) {
		p := parser.New("test", bytes.NewReader(data))
		format, err := p.ReadUInt16()
		if err != nil || format != 2 {
			return
		}

		l1, err := readGsub1_2(p, 0)
		if err != nil {
			return
		}

		data2 := l1.Encode(meta)
		if len(data2) != l1.EncodeLen(meta) {
			t.Errorf("encodeLen mismatch: %d != %d", len(data2), l1.EncodeLen(meta))
		}

		p = parser.New("test", bytes.NewReader(data2))
		format, err = p.ReadUInt16()
		if err != nil {
			t.Fatal(err)
		} else if format != 2 {
			t.Fatalf("unexpected format: %d", format)
		}
		l2, err := readGsub1_2(p, 0)
		if err != nil {
			t.Fatal(err)
		}

		if !reflect.DeepEqual(l1, l2) {
			t.Error("different")
		}
	})
}
