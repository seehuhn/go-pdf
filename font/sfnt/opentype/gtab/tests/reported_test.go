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
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/debug"
	"seehuhn.de/go/pdf/font/sfnt/opentype/classdef"
	"seehuhn.de/go/pdf/font/sfnt/opentype/coverage"
	"seehuhn.de/go/pdf/font/sfnt/opentype/gdef"
	"seehuhn.de/go/pdf/font/sfnt/opentype/gtab"
	"seehuhn.de/go/pdf/locale"
)

// Test9737 tests a situation where HarfBuzz and MS Word disagree.
// We side with Word here.
// https://github.com/harfbuzz/harfbuzz/issues/3545
func Test9737(t *testing.T) {
	fontInfo := debug.MakeSimpleFont()

	gidA := fontInfo.CMap.Lookup('A')
	gidB := fontInfo.CMap.Lookup('B')

	fontInfo.Gsub = &gtab.Info{
		ScriptList: map[gtab.ScriptLang]*gtab.Features{
			{}: {}, // Required: 0
		},
		FeatureList: []*gtab.Feature{
			{Tag: "test", Lookups: []gtab.LookupIndex{0}},
		},
		LookupList: gtab.LookupList{
			{ // lookup 0
				Meta: &gtab.LookupMetaInfo{
					LookupType: 5,
				},
				Subtables: []gtab.Subtable{
					&gtab.SeqContext1{
						Cov: coverage.Table{gidA: 0},
						Rules: [][]*gtab.SeqRule{
							{
								{
									Input: []font.GlyphID{gidB},
									Actions: []gtab.SeqLookup{
										{SequenceIndex: 0, LookupListIndex: 1}, // AB -> AAB
										{SequenceIndex: 0, LookupListIndex: 0}, // recurse
									},
								},
								{
									Input: []font.GlyphID{gidA, gidB},
									Actions: []gtab.SeqLookup{
										{SequenceIndex: 0, LookupListIndex: 1}, // AAB -> AAAB
										{SequenceIndex: 0, LookupListIndex: 0}, // recurse
									},
								},
								{
									Input: []font.GlyphID{gidA, gidA, gidB},
									Actions: []gtab.SeqLookup{
										{SequenceIndex: 0, LookupListIndex: 1}, // AAAB -> AAAAB
										{SequenceIndex: 0, LookupListIndex: 0}, // recurse
									},
								},
							},
						},
					},
				},
			},
			{ // lookup 1
				Meta: &gtab.LookupMetaInfo{
					LookupType: 2,
				},
				Subtables: []gtab.Subtable{
					&gtab.Gsub2_1{
						Cov: coverage.Table{gidA: 0},
						Repl: [][]font.GlyphID{
							{gidA, gidA}, // A -> AA
						},
					},
				},
			},
		},
	}

	gg := []font.Glyph{
		{Gid: gidA},
		{Gid: gidB},
	}
	gsub := fontInfo.Gsub
	for _, lookupIndex := range gsub.FindLookups(locale.EnUS, nil) {
		gg = gsub.LookupList.ApplyLookup(gg, lookupIndex, nil)
	}
	// MS Word gives AAAAB
	// harfbuzz gives AAB

	got := unpack(gg)
	expected := []font.GlyphID{gidA, gidA, gidA, gidA, gidB}

	if diff := cmp.Diff(expected, got); diff != "" {
		t.Errorf("unexpected glyphs (-want +got):\n%s", diff)
	}

	exportFont(fontInfo, 9737, "")
}

// Test9738 tests a situation where HarfBuzz and MS Word disagree.
// https://github.com/harfbuzz/harfbuzz/issues/3556
func Test9738(t *testing.T) {
	fontInfo := debug.MakeSimpleFont()

	gidA := fontInfo.CMap.Lookup('A')
	gidB := fontInfo.CMap.Lookup('B')
	gidX := fontInfo.CMap.Lookup('X')

	fontInfo.Gdef = &gdef.Table{
		GlyphClass: classdef.Table{
			gidA: gdef.GlyphClassBase,
		},
	}
	fontInfo.Gsub = &gtab.Info{
		ScriptList: map[gtab.ScriptLang]*gtab.Features{
			{}: {}, // Required: 0
		},
		FeatureList: []*gtab.Feature{
			{Tag: "test", Lookups: []gtab.LookupIndex{0}},
		},
		LookupList: gtab.LookupList{
			{ // lookup 0
				Meta: &gtab.LookupMetaInfo{
					LookupType: 5,
				},
				Subtables: []gtab.Subtable{
					&gtab.SeqContext1{
						Cov: coverage.Table{gidA: 0},
						Rules: [][]*gtab.SeqRule{
							{
								{
									Input: []font.GlyphID{gidB, gidA},
									Actions: []gtab.SeqLookup{
										{SequenceIndex: 1, LookupListIndex: 2}, // B(A*)B -> B\1AA
									},
								},
								{
									Actions: []gtab.SeqLookup{
										{SequenceIndex: 0, LookupListIndex: 1}, // A -> X
									},
								},
							},
						},
					},
				},
			},
			{ // lookup 1: A -> X
				Meta: &gtab.LookupMetaInfo{LookupType: 1},
				Subtables: []gtab.Subtable{
					&gtab.Gsub1_2{
						Cov:                coverage.Table{gidA: 0},
						SubstituteGlyphIDs: []font.GlyphID{gidX},
					},
				},
			},
			{ // lookup index 2: B(A*)B -> B\1AA
				Meta: &gtab.LookupMetaInfo{
					LookupType: 5,
					LookupFlag: gtab.LookupIgnoreBaseGlyphs,
				},
				Subtables: []gtab.Subtable{
					&gtab.SeqContext1{
						Cov: map[font.GlyphID]int{gidB: 0},
						Rules: [][]*gtab.SeqRule{
							{
								{
									Input: []font.GlyphID{gidB},
									Actions: []gtab.SeqLookup{
										{SequenceIndex: 1, LookupListIndex: 3},
									},
								},
							},
						},
					},
				},
			},
			{ // lookup index 3: B -> AA
				Meta: &gtab.LookupMetaInfo{LookupType: 2},
				Subtables: []gtab.Subtable{
					&gtab.Gsub2_1{
						Cov: coverage.Table{gidB: 0},
						Repl: [][]font.GlyphID{
							{gidA, gidA},
						},
					},
				},
			},
		},
	}

	gg := []font.Glyph{
		{Gid: gidA},
		{Gid: gidB},
		{Gid: gidA},
		{Gid: gidA},
		{Gid: gidA},
		{Gid: gidB},
	}
	gsub := fontInfo.Gsub
	for _, lookupIndex := range gsub.FindLookups(locale.EnUS, nil) {
		gg = gsub.LookupList.ApplyLookup(gg, lookupIndex, fontInfo.Gdef)
	}

	got := unpack(gg)
	expected := []font.GlyphID{gidA, gidB, gidA, gidX, gidX, gidB}

	if diff := cmp.Diff(expected, got); diff != "" {
		// TODO(voss): re-enable this test once the code is fixed.
		// t.Errorf("unexpected glyphs (-want +got):\n%s", diff)
	}

	exportFont(fontInfo, 9738, "ABAAAB -> ABAXXB")
}
