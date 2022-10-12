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

	"github.com/google/go-cmp/cmp"
	"golang.org/x/text/language"
	"seehuhn.de/go/pdf/sfnt/parser"
)

func TestGetLookups(t *testing.T) {
	gtabInfo := Info{
		ScriptList: map[language.Tag]*Features{
			language.MustParse("und-Latn"): {
				Required: 0,
				Optional: []FeatureIndex{
					1, 2, 3,
				},
			},
		},
		FeatureList: []*Feature{
			{Tag: "dflt", Lookups: []LookupIndex{0, 3}},
			{Tag: "kern", Lookups: []LookupIndex{1, 3}},
			{Tag: "mkmk", Lookups: []LookupIndex{4, 3, 2}},
			{Tag: "test", Lookups: []LookupIndex{4, 5}},
		},
		LookupList: LookupList{
			nil, nil, nil, nil, nil, nil,
		},
	}

	cases := []struct {
		tags     []string
		expected []LookupIndex
	}{
		{nil, []LookupIndex{0, 3}},
		{[]string{"kern"}, []LookupIndex{0, 1, 3}},
		{[]string{"kern", "test"}, []LookupIndex{0, 1, 3, 4, 5}},
	}

	for _, test := range cases {
		includeFeature := map[string]bool{}
		for _, tag := range test.tags {
			includeFeature[tag] = true
		}
		ll := gtabInfo.FindLookups(language.BritishEnglish, includeFeature)
		if len(ll) != len(test.expected) {
			t.Errorf("GetLookups(%v) = %v, expected %v", test.tags, ll, test.expected)
		}
	}
}

func FuzzGtab(f *testing.F) {
	info := &Info{}
	f.Add(info.Encode(999))

	info.ScriptList = ScriptListInfo{
		language.MustParse("und-Zzzz"): {
			Required: 0xFFFF,
			Optional: []FeatureIndex{1, 2, 3, 4},
		},
		language.MustParse("und-Latn"): {
			Required: 0,
			Optional: []FeatureIndex{2, 4, 5},
		},
		language.German: {
			Required: 6,
		},
	}
	info.FeatureList = FeatureListInfo{
		{Tag: "kern", Lookups: []LookupIndex{0, 1}},
		{Tag: "liga", Lookups: []LookupIndex{2, 3, 4}},
		{Tag: "frac", Lookups: []LookupIndex{1, 5}},
		{Tag: "locl", Lookups: []LookupIndex{2, 6}},
		{Tag: "onum", Lookups: []LookupIndex{3, 7}},
		{Tag: "sups", Lookups: []LookupIndex{9}},
		{Tag: "numr", Lookups: []LookupIndex{1, 9, 10}},
	}
	info.LookupList = LookupList{
		&LookupTable{
			Meta: &LookupMetaInfo{LookupType: 1},
			Subtables: Subtables{
				dummySubTable{0},
				dummySubTable{1},
				dummySubTable{2},
			},
		},
		&LookupTable{
			Meta: &LookupMetaInfo{LookupType: 2, LookupFlag: LookupUseMarkFilteringSet, MarkFilteringSet: 7},
			Subtables: Subtables{
				dummySubTable{3, 4},
				dummySubTable{5, 6},
			},
		},
		&LookupTable{
			Meta: &LookupMetaInfo{LookupType: 3},
			Subtables: Subtables{
				dummySubTable{7, 8, 9},
			},
		},
		&LookupTable{
			Meta: &LookupMetaInfo{LookupType: 1},
			Subtables: Subtables{
				dummySubTable{0},
				dummySubTable{1},
				dummySubTable{2},
			},
		},
		&LookupTable{
			Meta: &LookupMetaInfo{LookupType: 2, LookupFlag: LookupUseMarkFilteringSet, MarkFilteringSet: 7},
			Subtables: Subtables{
				dummySubTable{3, 4},
				dummySubTable{5, 6},
			},
		},
		&LookupTable{
			Meta: &LookupMetaInfo{LookupType: 3},
			Subtables: Subtables{
				dummySubTable{7, 8, 9},
			},
		},
		&LookupTable{
			Meta: &LookupMetaInfo{LookupType: 1},
			Subtables: Subtables{
				dummySubTable{0},
				dummySubTable{1},
				dummySubTable{2},
			},
		},
		&LookupTable{
			Meta: &LookupMetaInfo{LookupType: 2, LookupFlag: LookupUseMarkFilteringSet, MarkFilteringSet: 7},
			Subtables: Subtables{
				dummySubTable{3, 4},
				dummySubTable{5, 6},
			},
		},
		&LookupTable{
			Meta: &LookupMetaInfo{LookupType: 3},
			Subtables: Subtables{
				dummySubTable{7, 8, 9},
			},
		},
		&LookupTable{
			Meta: &LookupMetaInfo{LookupType: 5},
			Subtables: Subtables{
				dummySubTable{10, 11, 12, 13, 14},
			},
		},
	}
	f.Add(info.Encode(999))

	f.Fuzz(func(t *testing.T, data1 []byte) {
		info1, err := readGtab(bytes.NewReader(data1), "test", readDummySubtable)
		if err != nil {
			return
		}

		data2 := info1.Encode(999)

		info2, err := readGtab(bytes.NewReader(data2), "test", readDummySubtable)
		if err != nil {
			t.Fatal(err)
		}

		if !reflect.DeepEqual(info1, info2) {
			t.Error("different")
		}
	})
}

func doFuzz(t *testing.T, lookupType, lookupFormat uint16,
	readFn func(p *parser.Parser, subtablePos int64) (Subtable, error),
	data1 []byte) {

	// t.Helper()

	p := parser.New(bytes.NewReader(data1))
	format, err := p.ReadUint16()
	if err != nil || format != lookupFormat {
		return
	}

	l1, err := readFn(p, 0)
	if err != nil {
		return
	}

	data2 := l1.Encode()
	if len(data2) != l1.EncodeLen() {
		t.Errorf("encodeLen mismatch: %d != %d", len(data2), l1.EncodeLen())
	}

	p = parser.New(bytes.NewReader(data2))
	format, err = p.ReadUint16()
	if err != nil {
		t.Fatal(err)
	} else if format != lookupFormat {
		t.Fatalf("unexpected format: %d.%d", lookupType, format)
	}
	l2, err := readFn(p, 0)
	if err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff(l1, l2); diff != "" {
		t.Errorf("different (-old +new):\n%s", diff)
	}
}
