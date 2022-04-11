package gtab

import (
	"bytes"
	"reflect"
	"testing"

	"seehuhn.de/go/pdf/locale"
)

func TestGetLookups(t *testing.T) {
	gtabInfo := Info{
		ScriptList: map[ScriptLang]*Features{
			{Script: locale.ScriptLatin}: {
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
		LookupList: []*LookupTable{
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
		ll := gtabInfo.getLookups(locale.EnGB, includeFeature)
		if len(ll) != len(test.expected) {
			t.Errorf("GetLookups(%v) = %v, expected %v", test.tags, ll, test.expected)
		}
	}
}

func FuzzGtab(f *testing.F) {
	info := &Info{}
	f.Add(info.Encode())

	info.ScriptList = ScriptListInfo{
		{Script: locale.ScriptUndefined, Lang: locale.LangUndefined}: {
			Required: 0xFFFF,
			Optional: []FeatureIndex{1, 2, 3, 4},
		},
		{Script: locale.ScriptLatin, Lang: locale.LangUndefined}: {
			Required: 0,
			Optional: []FeatureIndex{2, 4, 5},
		},
		{Script: locale.ScriptLatin, Lang: locale.LangGerman}: {
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
			Subtables: []Subtable{
				dummySubTable{0},
				dummySubTable{1},
				dummySubTable{2},
			},
		},
		&LookupTable{
			Meta: &LookupMetaInfo{LookupType: 2, LookupFlag: 0x0010, MarkFilteringSet: 7},
			Subtables: []Subtable{
				dummySubTable{3, 4},
				dummySubTable{5, 6},
			},
		},
		&LookupTable{
			Meta: &LookupMetaInfo{LookupType: 3},
			Subtables: []Subtable{
				dummySubTable{7, 8, 9},
			},
		},
		&LookupTable{
			Meta: &LookupMetaInfo{LookupType: 1},
			Subtables: []Subtable{
				dummySubTable{0},
				dummySubTable{1},
				dummySubTable{2},
			},
		},
		&LookupTable{
			Meta: &LookupMetaInfo{LookupType: 2, LookupFlag: 0x0010, MarkFilteringSet: 7},
			Subtables: []Subtable{
				dummySubTable{3, 4},
				dummySubTable{5, 6},
			},
		},
		&LookupTable{
			Meta: &LookupMetaInfo{LookupType: 3},
			Subtables: []Subtable{
				dummySubTable{7, 8, 9},
			},
		},
		&LookupTable{
			Meta: &LookupMetaInfo{LookupType: 1},
			Subtables: []Subtable{
				dummySubTable{0},
				dummySubTable{1},
				dummySubTable{2},
			},
		},
		&LookupTable{
			Meta: &LookupMetaInfo{LookupType: 2, LookupFlag: 0x0010, MarkFilteringSet: 7},
			Subtables: []Subtable{
				dummySubTable{3, 4},
				dummySubTable{5, 6},
			},
		},
		&LookupTable{
			Meta: &LookupMetaInfo{LookupType: 3},
			Subtables: []Subtable{
				dummySubTable{7, 8, 9},
			},
		},
		&LookupTable{
			Meta: &LookupMetaInfo{LookupType: 5},
			Subtables: []Subtable{
				dummySubTable{10, 11, 12, 13, 14},
			},
		},
	}
	f.Add(info.Encode())

	f.Fuzz(func(t *testing.T, data1 []byte) {
		info1, err := doRead("test", bytes.NewReader(data1), dummyReader)
		if err != nil {
			return
		}

		data2 := info1.Encode()

		info2, err := doRead("test", bytes.NewReader(data2), dummyReader)
		if err != nil {
			t.Fatal(err)
		}

		if !reflect.DeepEqual(info1, info2) {
			t.Error("different")
		}
	})
}