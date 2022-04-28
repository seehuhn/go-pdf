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
	"testing"

	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/debug"
	"seehuhn.de/go/pdf/font/sfnt/opentype/classdef"
	"seehuhn.de/go/pdf/font/sfnt/opentype/coverage"
	"seehuhn.de/go/pdf/font/sfnt/opentype/gdef"
	"seehuhn.de/go/pdf/font/sfnt/opentype/gtab"
	"seehuhn.de/go/pdf/font/sfntcff"
	"seehuhn.de/go/pdf/locale"
)

func TestGsub(t *testing.T) {
	fontInfo := debug.MakeSimpleFont()
	gidA := fontInfo.CMap.Lookup('A')
	gidB := fontInfo.CMap.Lookup('B')
	gidC := fontInfo.CMap.Lookup('C')
	gidM := fontInfo.CMap.Lookup('M') // marked as a mark character, ignored
	gidN := fontInfo.CMap.Lookup('N')
	gidX := fontInfo.CMap.Lookup('X')

	type testCase struct {
		lookupType uint16
		subtable   gtab.Subtable
		in, out    string
		text       string // text content, if different from in
	}
	cases := []testCase{
		{ // test GSUB 1.1
			lookupType: 1,
			subtable: &gtab.Gsub1_1{
				Cov:   coverage.Table{gidA: 0, gidM: 1},
				Delta: 1,
			},
			in:  "AAMBA",
			out: "BBMBB",
		},
		{ // test GSUB 1.2
			lookupType: 1,
			subtable: &gtab.Gsub1_2{
				Cov:                coverage.Table{gidA: 0, gidB: 1, gidM: 2},
				SubstituteGlyphIDs: []font.GlyphID{gidB, gidA, gidB},
			},
			in:  "ABCMA",
			out: "BACMB",
		},
		{ // test GSUB 2.1
			lookupType: 2,
			subtable: &gtab.Gsub2_1{
				Cov: coverage.Table{gidA: 0, gidM: 1},
				Repl: [][]font.GlyphID{
					{gidA, gidB, gidA},
					{gidA},
				},
			},
			in:  "ABMA",
			out: "ABABMABA",
		},
		{ // test GSUB 3.1
			lookupType: 3,
			subtable: &gtab.Gsub3_1{
				Cov: coverage.Table{gidA: 0, gidM: 1},
				Alt: [][]font.GlyphID{
					{gidB, gidC},
					{gidN},
				},
			},
			in:  "ABMA",
			out: "BBMB",
		},
		{ // simple test for GSUB 4.1
			lookupType: 4,
			subtable: &gtab.Gsub4_1{
				Cov: coverage.Table{gidA: 0},
				Repl: [][]gtab.Ligature{
					{
						{
							In:  []font.GlyphID{gidA},
							Out: gidA,
						},
					},
				},
			},
			in:  "AA",
			out: "A",
		},
		{ // test GSUB 4.1
			lookupType: 4,
			subtable: &gtab.Gsub4_1{
				Cov: coverage.Table{gidA: 0, gidM: 1},
				Repl: [][]gtab.Ligature{
					{
						{In: []font.GlyphID{gidA, gidA}, Out: gidC},
						{In: []font.GlyphID{gidA}, Out: gidB},
					},
					{
						{
							In:  []font.GlyphID{gidA},
							Out: gidN,
						},
					},
				},
			},
			in:   "AAAMAMAMAAA",
			out:  "CMCMMB",
			text: "AAAMAAAMMAA",
		},
		{ // test GSUB 5.1
			lookupType: 5,
			subtable: &gtab.SeqContext1{
				Cov: coverage.Table{gidA: 0, gidM: 1},
				Rules: [][]*gtab.SeqRule{
					{
						{
							Input: []font.GlyphID{gidA},
							Actions: []gtab.SeqLookup{
								{SequenceIndex: 0, LookupListIndex: 1},
							},
						},
					},
					{
						{
							Input: []font.GlyphID{gidA},
							Actions: []gtab.SeqLookup{
								{SequenceIndex: 0, LookupListIndex: 1},
							},
						},
					},
				},
			},
			in:  "AAMAAA",
			out: "BAMBAA",
		},
		{ // test lookup flags for nested lookups, using GSUB 5.1
			lookupType: 5,
			subtable: &gtab.SeqContext1{
				Cov: coverage.Table{gidA: 0},
				Rules: [][]*gtab.SeqRule{
					{
						{
							Input: []font.GlyphID{gidA, gidB},
							Actions: []gtab.SeqLookup{
								{SequenceIndex: 0, LookupListIndex: 5},
							},
						},
					},
				},
			},
			in:  "AB",
			out: "AB",
		},
		{ // test infinite loop using GSUB5.1
			lookupType: 5,
			subtable: &gtab.SeqContext1{
				Cov: coverage.Table{gidA: 0},
				Rules: [][]*gtab.SeqRule{
					{
						{
							Input: []font.GlyphID{},
							Actions: []gtab.SeqLookup{
								{SequenceIndex: 0, LookupListIndex: 3}, // A->AA
								{SequenceIndex: 0, LookupListIndex: 0}, // repeat
							},
						},
					},
				},
			},
			in:  "AB",
			out: "AB", // MS Word: "B", harfbuzz&Mac: "A{65}B"
		},
		{ // test finite recursion, using GSUB 5.1
			lookupType: 5,
			subtable: &gtab.SeqContext1{
				Cov: coverage.Table{gidA: 0},
				Rules: [][]*gtab.SeqRule{
					{
						{
							Input: []font.GlyphID{gidB},
							Actions: []gtab.SeqLookup{
								{SequenceIndex: 0, LookupListIndex: 3}, // AB -> AAB
								{SequenceIndex: 0, LookupListIndex: 0}, // recurse
							},
						},
						{
							Input: []font.GlyphID{gidA, gidB},
							Actions: []gtab.SeqLookup{
								{SequenceIndex: 0, LookupListIndex: 3}, // AAB -> AAAB
								{SequenceIndex: 0, LookupListIndex: 0}, // recurse
							},
						},
						{
							Input: []font.GlyphID{gidA, gidA, gidB},
							Actions: []gtab.SeqLookup{
								{SequenceIndex: 0, LookupListIndex: 3}, // AAAB -> AAAAB
								{SequenceIndex: 0, LookupListIndex: 0}, // recurse
							},
						},
					},
				},
			},
			in:  "AB",
			out: "AAAAB",
		},
		{ // test next position when lookup flags are used for nested lookups, using GSUB 5.1
			lookupType: 5,
			subtable: &gtab.SeqContext1{
				Cov: coverage.Table{gidA: 0},
				Rules: [][]*gtab.SeqRule{
					{
						{
							Input: []font.GlyphID{gidB, gidA},
							Actions: []gtab.SeqLookup{
								{SequenceIndex: 1, LookupListIndex: 6}, // B(A*)B -> B\1X
							},
						},
						{
							Actions: []gtab.SeqLookup{
								{SequenceIndex: 0, LookupListIndex: 2}, // A -> X
							},
						},
					},
				},
			},
			in:  "ABAAAABA",
			out: "ABAAAAAAX",
		},
		{ // test GSUB 5.2
			lookupType: 5,
			subtable: &gtab.SeqContext2{
				Cov:     coverage.Table{gidA: 0, gidB: 1, gidM: 2},
				Classes: classdef.Table{gidA: 1, gidB: 1},
				Rules: [][]*gtab.ClassSequenceRule{
					{ // class 0 (not used)
						{
							Input: []uint16{},
							Actions: []gtab.SeqLookup{
								{SequenceIndex: 1, LookupListIndex: 1},
							},
						},
					},
					{ // class 1
						{
							Input: []uint16{1, 1},
							Actions: []gtab.SeqLookup{
								{SequenceIndex: 1, LookupListIndex: 2},
							},
						},
					},
				},
			},
			in:  "AAAAMAABBMBMAAA",
			out: "AXAAMXABXMBMAXA",
		},
		{ // test GSUB 5.2
			lookupType: 5,
			subtable: &gtab.SeqContext2{
				Cov:     coverage.Table{gidA: 0},
				Classes: classdef.Table{gidA: 1},
				Rules: [][]*gtab.ClassSequenceRule{
					{},
					{
						{
							Input: []uint16{1, 1},
							Actions: []gtab.SeqLookup{
								{SequenceIndex: 0, LookupListIndex: 4},
								{SequenceIndex: 0, LookupListIndex: 4},
							},
						},
					},
				},
			},
			in:   "AMAMA",
			out:  "AMM",
			text: "AAAMM",
		},
		{ // test GSUB 5.3
			lookupType: 5,
			subtable: &gtab.SeqContext3{
				InputCov: []coverage.Table{
					{gidA: 0, gidB: 1},
					{gidB: 0, gidC: 1},
					{gidA: 0, gidC: 1},
				},
				Actions: []gtab.SeqLookup{
					{SequenceIndex: 0, LookupListIndex: 5},
					{SequenceIndex: 2, LookupListIndex: 5},
				},
			},
			in:  "CBBAABC",
			out: "CXBAABX",
		},
		{ // test GSUB 6.1
			lookupType: 6,
			subtable: &gtab.ChainedSeqContext1{
				Cov: coverage.Table{gidA: 0, gidB: 1},
				Rules: [][]*gtab.ChainedSeqRule{
					{
						{
							Lookahead: []font.GlyphID{gidA},
							Actions: []gtab.SeqLookup{
								{SequenceIndex: 0, LookupListIndex: 2},
							},
						},
					},
					{
						{
							Backtrack: []font.GlyphID{gidB},
							Actions: []gtab.SeqLookup{
								{SequenceIndex: 0, LookupListIndex: 2},
							},
						},
					},
				},
			},
			in:  "AAAAABBBBB",
			out: "XXXXABXBXB",
		},
		{
			lookupType: 6,
			subtable: &gtab.ChainedSeqContext1{
				Cov: coverage.Table{gidA: 0},
				Rules: [][]*gtab.ChainedSeqRule{
					{
						{
							Lookahead: []font.GlyphID{gidB},
							Actions: []gtab.SeqLookup{
								{SequenceIndex: 0, LookupListIndex: 3}, // A B -> AA B
								{SequenceIndex: 0, LookupListIndex: 0}, // recurse
							},
						},
						{
							Lookahead: []font.GlyphID{gidA, gidB},
							Actions: []gtab.SeqLookup{
								{SequenceIndex: 0, LookupListIndex: 3}, // AA B -> AAA B
								{SequenceIndex: 0, LookupListIndex: 0}, // recurse
							},
						},
						{
							Lookahead: []font.GlyphID{gidA, gidA, gidB},
							Actions: []gtab.SeqLookup{
								{SequenceIndex: 0, LookupListIndex: 3}, // AAA B -> AAAA B
								{SequenceIndex: 0, LookupListIndex: 0}, // recurse
							},
						},
					},
				},
			},
			in:  "AB",
			out: "AAAAB",
		},
	}

	gdef := &gdef.Table{
		GlyphClass: classdef.Table{
			gidA: gdef.GlyphClassBase,
			gidM: gdef.GlyphClassMark,
		},
	}
	lookups := []*gtab.LookupTable{
		{ // lookup index 0
			Meta: &gtab.LookupMetaInfo{
				LookupType: 0, // placeholder for test.lookupType
				LookupFlag: gtab.LookupIgnoreMarks,
			},
			Subtables: []gtab.Subtable{
				nil, // placeholder for test.Subtable
			},
		},
		{ // lookup index 1: A->B, B->C, C->D, M->N, N->O
			Meta: &gtab.LookupMetaInfo{LookupType: 1},
			Subtables: []gtab.Subtable{
				&gtab.Gsub1_1{
					Cov:   coverage.Table{gidA: 0, gidB: 1, gidC: 2, gidM: 3, gidN: 4},
					Delta: 1,
				},
			},
		},
		{ // lookup index 2: A->X, B->X, C->X, M->X, N->X
			Meta: &gtab.LookupMetaInfo{LookupType: 1},
			Subtables: []gtab.Subtable{
				&gtab.Gsub1_2{
					Cov:                coverage.Table{gidA: 0, gidB: 1, gidC: 2, gidM: 3, gidN: 4},
					SubstituteGlyphIDs: []font.GlyphID{gidX, gidX, gidX, gidX, gidX},
				},
			},
		},
		{ // lookup index 3: A -> AA, B -> AA, C -> ABAAC
			Meta: &gtab.LookupMetaInfo{LookupType: 2},
			Subtables: []gtab.Subtable{
				&gtab.Gsub2_1{
					Cov: coverage.Table{gidA: 0, gidB: 1, gidC: 2},
					Repl: [][]font.GlyphID{
						{gidA, gidA},
						{gidA, gidA},
						{gidA, gidB, gidA, gidA, gidC},
					},
				},
			},
		},
		{ // lookup index 4: A(M*)A -> A\1
			Meta: &gtab.LookupMetaInfo{
				LookupType: 4,
				LookupFlag: gtab.LookupIgnoreMarks,
			},
			Subtables: []gtab.Subtable{
				&gtab.Gsub4_1{
					Cov: coverage.Table{gidA: 0},
					Repl: [][]gtab.Ligature{
						{
							{
								In:  []font.GlyphID{gidA},
								Out: gidA,
							},
						},
					},
				},
			},
		},
		{ // lookup index 5: B->X, C->X, M->X, N->X (ignore base glyph A)
			Meta: &gtab.LookupMetaInfo{
				LookupType: 1,
				LookupFlag: gtab.LookupIgnoreBaseGlyphs,
			},
			Subtables: []gtab.Subtable{
				&gtab.Gsub1_2{
					Cov:                coverage.Table{gidB: 0, gidC: 1, gidM: 2, gidN: 3},
					SubstituteGlyphIDs: []font.GlyphID{gidX, gidX, gidX, gidX},
				},
			},
		},
		{ // lookup index 6: B(A*)B -> B\1AA
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
	}
	gsub := &gtab.Info{
		ScriptList: map[gtab.ScriptLang]*gtab.Features{
			{}: {}, // Required: 0
		},
		FeatureList: []*gtab.Feature{
			{Tag: "test", Lookups: []gtab.LookupIndex{0}},
		},
		LookupList: lookups,
	}

	a, b := fontInfo.CMap.CodeRange()
	rev := make(map[font.GlyphID]rune)
	for r := a; r <= b; r++ {
		gid := fontInfo.CMap.Lookup(r)
		if gid != 0 {
			rev[gid] = r
		}
	}

	for testIdx, test := range cases {
		t.Run(fmt.Sprintf("%d", testIdx+1), func(t *testing.T) {

			lookups[0].Meta.LookupType = test.lookupType
			lookups[0].Subtables[0] = test.subtable

			seq := make([]font.Glyph, len(test.in))
			for i, r := range test.in {
				seq[i].Gid = fontInfo.CMap.Lookup(r)
				seq[i].Text = []rune{r}
			}
			lookups := gsub.FindLookups(locale.EnUS, nil)
			for _, lookupIndex := range lookups {
				seq = gsub.ApplyLookup(seq, lookupIndex, gdef)
			}

			var textRunes []rune
			var outRunes []rune
			for _, g := range seq {
				textRunes = append(textRunes, g.Text...)
				outRunes = append(outRunes, rev[g.Gid])
			}
			text := string(textRunes)
			out := string(outRunes)

			expectedText := test.text
			if expectedText == "" {
				expectedText = test.in
			}
			fmt.Printf("test%04d.otf %s -> %s\n", testIdx+1, test.in, test.out)
			if out != test.out {
				t.Errorf("expected output %q, got %q", test.out, out)
			} else if text != expectedText {
				t.Errorf("expected text %q, got %q", expectedText, text)
			}

			if *exportFonts {
				fontInfo.Gdef = gdef
				fontInfo.Gsub = gsub
				err := exportFont(fontInfo, testIdx+1, test.in+" -> "+test.out)
				if err != nil {
					t.Error(err)
				}
			}
		})
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

func exportFont(fontInfo *sfntcff.Info, idx int, desc string) error {
	if !*exportFonts {
		return nil
	}

	fontInfo.FamilyName = fmt.Sprintf("Test%04d", idx)
	fontInfo.Description = desc

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
