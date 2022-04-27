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

package tests

import (
	"flag"
	"fmt"
	"os"
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/debug"
	"seehuhn.de/go/pdf/font/sfnt/opentype/classdef"
	"seehuhn.de/go/pdf/font/sfnt/opentype/coverage"
	"seehuhn.de/go/pdf/font/sfnt/opentype/gdef"
	"seehuhn.de/go/pdf/font/sfnt/opentype/gtab"
	"seehuhn.de/go/pdf/font/sfntcff"
	"seehuhn.de/go/pdf/locale"
)

func TestGsub1_1(t *testing.T) {
	fontInfo := debug.MakeSimpleFont()
	gidA := fontInfo.CMap.Lookup('A')
	gidB := fontInfo.CMap.Lookup('B')
	gidM := fontInfo.CMap.Lookup('M')
	fontInfo.Gdef = &gdef.Table{
		GlyphClass: classdef.Table{
			gidM: gdef.GlyphClassMark,
		},
	}
	fontInfo.Gsub = &gtab.Info{
		ScriptList: map[gtab.ScriptLang]*gtab.Features{
			{}: {}, // Required: 0
		},
		FeatureList: []*gtab.Feature{
			{Tag: "test", Lookups: []gtab.LookupIndex{0}},
		},
		LookupList: []*gtab.LookupTable{
			{
				Meta: &gtab.LookupMetaInfo{
					LookupType: 1,
					LookupFlag: gtab.LookupIgnoreMarks,
				},
				Subtables: []gtab.Subtable{
					&gtab.Gsub1_1{
						Cov:   coverage.Table{gidA: 0, gidM: 1},
						Delta: 1,
					},
				},
			},
		},
	}

	gsub := fontInfo.Gsub
	lookups := gsub.FindLookups(locale.EnUS, nil)

	seq := []font.Glyph{{Gid: gidA}, {Gid: gidA}, {Gid: gidM}, {Gid: gidB}, {Gid: gidA}}
	want := []font.Glyph{{Gid: gidB}, {Gid: gidB}, {Gid: gidM}, {Gid: gidB}, {Gid: gidB}}
	for _, lookupIndex := range lookups {
		seq = gsub.ApplyLookup(seq, lookupIndex, fontInfo.Gdef)
	}
	if diff := cmp.Diff(want, seq); diff != "" {
		t.Errorf("unexpected result (-want +got):\n%s", diff)
	}
	exportFont(fontInfo, 1)
}

func TestGsub(t *testing.T) {
	gdef := &gdef.Table{
		GlyphClass: classdef.Table{
			4: gdef.GlyphClassLigature,
		},
	}

	type testCase struct {
		lookupType uint16
		subtable   gtab.Subtable
	}
	cases := []testCase{
		{1, &gtab.Gsub1_1{
			Cov:   map[font.GlyphID]int{3: 0},
			Delta: 26,
		}},
		{1, &gtab.Gsub1_2{
			Cov:                map[font.GlyphID]int{3: 0, 6: 1},
			SubstituteGlyphIDs: []font.GlyphID{29, 26},
		}},
		{2, &gtab.Gsub2_1{
			Cov: map[font.GlyphID]int{3: 0, 4: 1},
			Repl: [][]font.GlyphID{
				{29, 4},
				{26},
			},
		}},
		{3, &gtab.Gsub3_1{
			Cov: map[font.GlyphID]int{3: 0},
			Alt: [][]font.GlyphID{
				{29, 21, 22},
			},
		}},
		{4, &gtab.Gsub4_1{
			Cov: map[font.GlyphID]int{3: 0, 4: 1},
			Repl: [][]gtab.Ligature{
				{
					{In: []font.GlyphID{4, 5}, Out: 29}, // excluded by gdef
					{In: []font.GlyphID{5}, Out: 29},    // used in the test
				},
				{
					{In: []font.GlyphID{3, 2}, Out: 27},
					{In: []font.GlyphID{1}, Out: 27},
					{In: []font.GlyphID{}, Out: 26},
				},
			},
		}},
	}
	for testIdx, test := range cases {
		gsub := &gtab.Info{
			ScriptList: map[gtab.ScriptLang]*gtab.Features{
				{}: {}, // Required: 0
			},
			FeatureList: []*gtab.Feature{
				{Tag: "test", Lookups: []gtab.LookupIndex{0}},
			},
			LookupList: []*gtab.LookupTable{
				{
					Meta: &gtab.LookupMetaInfo{
						LookupType: test.lookupType,
						LookupFlag: gtab.LookupIgnoreLigatures,
					},
					Subtables: []gtab.Subtable{test.subtable},
				},
			},
		}

		lookups := gsub.FindLookups(locale.EnUS, nil)

		in := []font.Glyph{
			{Gid: 1},
			{Gid: 2},
			{Gid: 3},
			{Gid: 4},
			{Gid: 5},
		}
		expected := []font.GlyphID{1, 2, 29}

		gg := in
		for _, lookupIndex := range lookups {
			gg = gsub.ApplyLookup(gg, lookupIndex, gdef)
		}

		fmt.Println(testIdx, unpack(gg))
		if out := unpack(gg); !reflect.DeepEqual(out[:3], expected) {
			t.Errorf("expected %v, got %v", expected, out)
		}

		if *exportFonts {
			fontInfo := debug.MakeSimpleFont()
			fontInfo.Gsub = gsub
			fontInfo.Gdef = gdef
			err := exportFont(fontInfo, 700+testIdx)
			if err != nil {
				t.Error(err)
			}
		}
	}
}

func unpack(seq []font.Glyph) []font.GlyphID {
	res := make([]font.GlyphID, len(seq))
	for i, g := range seq {
		res[i] = g.Gid
	}
	return res
}

var exportFonts = flag.Bool("export-fonts", false, "export fonts used in tests")

func exportFont(fontInfo *sfntcff.Info, idx int) error {
	if !*exportFonts {
		return nil
	}

	fontInfo.FamilyName = fmt.Sprintf("Test%04d", idx)

	fname := fmt.Sprintf("test%04d.otf", idx)
	fd, err := os.Create(fname)
	if err != nil {
		return err
	}
	_, err = fontInfo.Write(fd)
	if err != nil {
		return err
	}
	err = fd.Close()
	if err != nil {
		return err
	}

	return nil
}
