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

package debug

import (
	"fmt"
	"os"
	"testing"

	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/debug"
	"seehuhn.de/go/pdf/font/sfnt/opentype/classdef"
	"seehuhn.de/go/pdf/font/sfnt/opentype/coverage"
	"seehuhn.de/go/pdf/font/sfnt/opentype/gdef"
	"seehuhn.de/go/pdf/font/sfnt/opentype/gtab"
	"seehuhn.de/go/pdf/locale"
)

func TestSequenceContext1(t *testing.T) {
	fontInfo := debug.MakeSimpleFont()

	fontInfo.Gdef = &gdef.Table{
		GlyphClass: classdef.Table{
			2: gdef.GlyphClassLigature,
			4: gdef.GlyphClassLigature,
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
			{ // lookup 0
				Meta: &gtab.LookupMetaInfo{
					LookupType: 5,
					LookupFlag: gtab.LookupIgnoreLigatures,
				},
				Subtables: []gtab.Subtable{
					&gtab.SeqContext1{
						Cov: coverage.Table{1: 0},
						Rules: [][]gtab.SequenceRule{
							{
								{
									In: []font.GlyphID{3, 5},
									Actions: []gtab.SeqLookup{
										{SequenceIndex: 1, LookupListIndex: 1},
										{SequenceIndex: 1, LookupListIndex: 3},
										// {SequenceIndex: 1, LookupListIndex: 3},
										// {SequenceIndex: 2, LookupListIndex: 3},
									},
								},
							},
						},
					},
				},
			},
			{ // lookup 1
				Meta: &gtab.LookupMetaInfo{
					LookupType: 4,
				},
				Subtables: []gtab.Subtable{
					&gtab.Gsub4_1{
						Cov: coverage.Table{3: 0},
						Repl: [][]gtab.Ligature{
							{
								{
									In:  []font.GlyphID{4},
									Out: 9,
								},
							},
						},
					},
				},
			},
			{ // lookup 2
				Meta: &gtab.LookupMetaInfo{
					LookupType: 2,
				},
				Subtables: []gtab.Subtable{
					&gtab.Gsub2_1{
						Cov:  coverage.Table{3: 0},
						Repl: [][]font.GlyphID{{3, 4}},
					},
				},
			},
			{ // lookup 3
				Meta: &gtab.LookupMetaInfo{
					LookupType: 1,
				},
				Subtables: []gtab.Subtable{
					&gtab.Gsub1_1{
						Cov:   coverage.Table{1: 0, 2: 1, 3: 2, 4: 3, 5: 4, 9: 5},
						Delta: 20,
					},
				},
			},
		},
	}

	fd, err := os.Create("test.otf")
	if err != nil {
		t.Fatal(err)
	}
	_, err = fontInfo.Write(fd)
	if err != nil {
		t.Fatal(err)
	}
	err = fd.Close()
	if err != nil {
		t.Error(err)
	}
}

func Test9735(t *testing.T) {
	fontInfo := debug.MakeSimpleFont()
	fontInfo.FamilyName = "Test9735"

	fontInfo.Gdef = &gdef.Table{
		GlyphClass: classdef.Table{
			2: gdef.GlyphClassLigature,
			4: gdef.GlyphClassLigature,
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
			{ // lookup 0
				Meta: &gtab.LookupMetaInfo{
					LookupType: 5,
					LookupFlag: gtab.LookupIgnoreLigatures,
				},
				Subtables: []gtab.Subtable{
					&gtab.SeqContext1{
						Cov: coverage.Table{1: 0},
						Rules: [][]gtab.SequenceRule{
							{
								{
									In: []font.GlyphID{3, 5},
									Actions: []gtab.SeqLookup{
										{SequenceIndex: 1, LookupListIndex: 1},
										{SequenceIndex: 2, LookupListIndex: 2},
									},
								},
							},
						},
					},
				},
			},
			{ // lookup 1
				Meta: &gtab.LookupMetaInfo{
					LookupType: 4,
				},
				Subtables: []gtab.Subtable{
					&gtab.Gsub4_1{
						Cov: coverage.Table{3: 0},
						Repl: [][]gtab.Ligature{
							{
								{
									In:  []font.GlyphID{4},
									Out: 9,
								},
							},
						},
					},
				},
			},
			{ // lookup 2
				Meta: &gtab.LookupMetaInfo{
					LookupType: 1,
				},
				Subtables: []gtab.Subtable{
					&gtab.Gsub1_1{
						Cov:   coverage.Table{3: 0, 5: 1, 9: 2},
						Delta: 10,
					},
				},
			},
		},
	}

	fd, err := os.Create("test9735.otf")
	if err != nil {
		t.Fatal(err)
	}
	_, err = fontInfo.Write(fd)
	if err != nil {
		t.Fatal(err)
	}
	err = fd.Close()
	if err != nil {
		t.Error(err)
	}
}

func Test9736(t *testing.T) {
	fontInfo := debug.MakeCompleteFont()
	fontInfo.FamilyName = "Test9736"

	gidF := fontInfo.CMap.Lookup('F')
	gidI := fontInfo.CMap.Lookup('I')
	gidX := fontInfo.CMap.Lookup('ﬁ')
	gidComma := fontInfo.CMap.Lookup(',')

	fontInfo.Gdef = &gdef.Table{
		GlyphClass: classdef.Table{
			gidComma: gdef.GlyphClassMark,
			gidF:     gdef.GlyphClassBase,
			gidI:     gdef.GlyphClassBase,
			gidX:     gdef.GlyphClassLigature,
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
			{ // lookup 0
				Meta: &gtab.LookupMetaInfo{
					LookupType: 5,
					LookupFlag: gtab.LookupIgnoreMarks,
				},
				Subtables: []gtab.Subtable{
					&gtab.SeqContext1{
						Cov: coverage.Table{gidF: 0},
						Rules: [][]gtab.SequenceRule{
							{
								{
									In: []font.GlyphID{gidI, gidF},
									Actions: []gtab.SeqLookup{
										{SequenceIndex: 0, LookupListIndex: 1},
										{SequenceIndex: 1, LookupListIndex: 2},
									},
								}, // F,I,F -> ﬁ,I
							},
						},
					},
				},
			},
			{ // lookup 1
				Meta: &gtab.LookupMetaInfo{
					LookupType: 4,
				},
				Subtables: []gtab.Subtable{
					&gtab.Gsub4_1{
						Cov: coverage.Table{gidF: 0},
						Repl: [][]gtab.Ligature{
							{
								{
									In:  []font.GlyphID{gidComma, gidI},
									Out: gidX,
								},
							},
						},
					},
				},
			},
			{ // lookup 2
				Meta: &gtab.LookupMetaInfo{
					LookupType: 1,
				},
				Subtables: []gtab.Subtable{
					&gtab.Gsub1_1{
						Cov:   coverage.Table{gidF: 0},
						Delta: gidI - gidF,
					},
				},
			},
		},
	}

	fd, err := os.Create("test9736.otf")
	if err != nil {
		t.Fatal(err)
	}
	_, err = fontInfo.Write(fd)
	if err != nil {
		t.Fatal(err)
	}
	err = fd.Close()
	if err != nil {
		t.Error(err)
	}
}

func Test9737(t *testing.T) {
	fontInfo := debug.MakeCompleteFont()
	fontInfo.FamilyName = "Test9737"

	gidA := fontInfo.CMap.Lookup('A')
	gidB := fontInfo.CMap.Lookup('B')

	fontInfo.Gsub = &gtab.Info{
		ScriptList: map[gtab.ScriptLang]*gtab.Features{
			{}: {}, // Required: 0
		},
		FeatureList: []*gtab.Feature{
			{Tag: "test", Lookups: []gtab.LookupIndex{0}},
		},
		LookupList: []*gtab.LookupTable{
			{ // lookup 0
				Meta: &gtab.LookupMetaInfo{
					LookupType: 5,
				},
				Subtables: []gtab.Subtable{
					&gtab.SeqContext1{
						Cov: coverage.Table{gidA: 0},
						Rules: [][]gtab.SequenceRule{
							{
								{
									In: []font.GlyphID{gidB},
									Actions: []gtab.SeqLookup{
										{SequenceIndex: 0, LookupListIndex: 1},
										{SequenceIndex: 0, LookupListIndex: 0},
									},
								},
								{
									In: []font.GlyphID{gidA, gidB},
									Actions: []gtab.SeqLookup{
										{SequenceIndex: 0, LookupListIndex: 1},
										{SequenceIndex: 0, LookupListIndex: 0},
									},
								},
								{
									In: []font.GlyphID{gidA, gidA, gidB},
									Actions: []gtab.SeqLookup{
										{SequenceIndex: 0, LookupListIndex: 1},
										{SequenceIndex: 0, LookupListIndex: 0},
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
	}
	gsub := fontInfo.Gsub
	for _, lookupIndex := range gsub.FindLookups(locale.EnUS, nil) {
		gg = gsub.ApplyLookup(gg, lookupIndex, nil)
	}
	fmt.Println(gg)
	// harfbuzz gives AAB
	// MS Word gives AAAAB

	fd, err := os.Create("test9737.otf")
	if err != nil {
		t.Fatal(err)
	}
	_, err = fontInfo.Write(fd)
	if err != nil {
		t.Fatal(err)
	}
	err = fd.Close()
	if err != nil {
		t.Error(err)
	}
}