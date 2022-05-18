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

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/debug"
	"seehuhn.de/go/pdf/font/sfnt/opentype/classdef"
	"seehuhn.de/go/pdf/font/sfnt/opentype/coverage"
	"seehuhn.de/go/pdf/font/sfnt/opentype/gdef"
	"seehuhn.de/go/pdf/font/sfnt/opentype/gtab"
	"seehuhn.de/go/pdf/font/sfnt/opentype/gtab/builder"
	"seehuhn.de/go/pdf/font/sfntcff"
	"seehuhn.de/go/pdf/locale"
)

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

func TestGsubOld(t *testing.T) {
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
		text       string // text content, if different from `in`
	}
	cases := []testCase{

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
		// TODO(voss): re-enable this test once the code is fixed.
		// { // test postitions when nested lookups change the sequence length
		// 	lookupType: 5,
		// 	subtable: &gtab.SeqContext1{
		// 		Cov: coverage.Table{gidA: 0},
		// 		Rules: [][]*gtab.SeqRule{
		// 			{
		// 				{
		// 					Input: []font.GlyphID{gidA, gidA},
		// 					Actions: []gtab.SeqLookup{
		// 						{SequenceIndex: 1, LookupListIndex: 3}, // A A A -> A AA A
		// 						{SequenceIndex: 2, LookupListIndex: 2}, // A AA A -> A AX A
		// 					},
		// 				},
		// 				{
		// 					Actions: []gtab.SeqLookup{
		// 						{SequenceIndex: 0, LookupListIndex: 1}, // A -> B
		// 					},
		// 				},
		// 			},
		// 		},
		// 	},
		// 	in:  "AAAAA",
		// 	out: "AAXABB",
		// },
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
				Cov:   coverage.Table{gidA: 0, gidB: 1, gidM: 2},
				Input: classdef.Table{gidA: 1, gidB: 1},
				Rules: [][]*gtab.ClassSeqRule{
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
				Cov:   coverage.Table{gidA: 0},
				Input: classdef.Table{gidA: 1},
				Rules: [][]*gtab.ClassSeqRule{
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
				Input: []coverage.Table{
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
	lookups := gtab.LookupList{
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

func TestGsub(t *testing.T) {
	fontInfo := debug.MakeSimpleFont()

	gidA := fontInfo.CMap.Lookup('A')
	gidK := fontInfo.CMap.Lookup('K')
	gidL := fontInfo.CMap.Lookup('L')
	gidM := fontInfo.CMap.Lookup('M')
	gidN := fontInfo.CMap.Lookup('N')
	gdef := &gdef.Table{
		GlyphClass: classdef.Table{
			gidA: gdef.GlyphClassBase,
			gidK: gdef.GlyphClassLigature,
			gidL: gdef.GlyphClassLigature,
			gidM: gdef.GlyphClassMark,
			gidN: gdef.GlyphClassMark,
		},
	}

	type testCase struct {
		desc    string
		in, out string
		text    string // text content, if different from `in`
	}
	cases := []testCase{
		{
			desc: "GSUB_1: A->X, C->Z",
			in:   "ABC",
			out:  "XBZ",
		},
		{
			desc: "GSUB_1: A->B, B->A",
			in:   "ABC",
			out:  "BAC",
		},
		{
			desc: "GSUB_1: -marks A->B, M->N",
			in:   "AAMBA",
			out:  "BBMBB",
		},

		{
			desc: `GSUB_2: A->A A`,
			in:   "AA",
			out:  "AAAA",
		},
		{
			desc: `GSUB_2: -marks A -> "ABA", M -> A`,
			in:   "ABMA",
			out:  "ABABMABA",
		},

		{
			desc: `GSUB_4: "BA" -> B`,
			in:   "ABAABA",
			out:  "ABAB",
		},
		{
			desc: `GSUB_4: "AAA" -> "B", "AA" -> "C", "A" -> "D"`,
			in:   "AAAAAXA",
			out:  "BCXD",
		},
		{
			desc: `GSUB_4: -marks "AAA" -> "X"`,
			in:   "AAABMAAACAMAADAAMAEAAAM",
			out:  "XBMXCXMDXMEXM",
			text: "AAABMAAACAAAMDAAAMEAAAM",
		},
		{
			desc: `GSUB_4: -marks "AAA" -> "C", "AA" -> "B"`,
			in:   "AAAMAMAMAAA",
			out:  "CMCMMB",
			text: "AAAMAAAMMAA",
		},

		{
			desc: `GSUB_5: "AAA" -> 3@2 1@0 2@1
					GSUB_1: "A" -> "X"
					GSUB_1: "A" -> "Y"
					GSUB_1: "A" -> "Z"`,
			in:  "AAA",
			out: "XYZ",
		},
		{
			desc: `GSUB_5: "XXX" -> 1@0
					GSUB_1: "X" -> "A"`,
			in:  "XXXXXXXX",
			out: "AXXAXXXX",
		},
		{
			desc: `GSUB_5: "ABC" -> 1@0 1@1 1@2
					GSUB_1: "B" -> "X"`,
			in:  "ABC",
			out: "AXC",
		},
		{ // harfbuzz, Mac and Windows agree on this
			desc: `GSUB_5: "AAAA" -> 1@0 4@2 3@1 2@0
					GSUB_4: "AA" -> "A"
					GSUB_1: "A" -> "X"
					GSUB_1: "A" -> "Y"
					GSUB_1: "A" -> "Z"`,
			in:  "AAAA",
			out: "XYZ",
		},
		{ // harfbuzz, Mac and Windows agree on this
			desc: `GSUB_5: "AAA" -> 1@0 4@2 3@1 2@0
					GSUB_2: "A" -> "AA"
					GSUB_1: "A" -> "X"
					GSUB_1: "A" -> "Y"
					GSUB_1: "A" -> "Z"`,
			in:  "AAA",
			out: "XYZA",
		},
		{ // harfbuzz, Mac and Windows agree on this
			desc: `GSUB_5: "AAA" -> 1@0 5@2 4@1 3@0
			        GSUB_5: "AA" -> 2@1
					GSUB_2: "A" -> "AA"
					GSUB_1: "A" -> "X"
					GSUB_1: "A" -> "Y"
					GSUB_1: "A" -> "Z"`,
			in:  "AAA",
			out: "XYZA",
		},
		// {
		// 	desc: `GSUB_5: "ABC" -> 1@0 1@2 1@3 2@2
		// 			GSUB_2: "A" -> "DE", "B" -> "FG", "G" -> "H"
		// 			GSUB_4: "FHC" -> "I"`,
		// 	in:  "ABC",
		// 	out: "DEI", // ABC -> DEBC -> DEFGC -> DEFHC -> DEI
		// },
		{ // harfbuzz, Mac and Windows agree on this
			desc: `GSUB_5: -ligs "AAA" -> 1@0 2@1 3@1
					GSUB_5: "AK" -> 2@1
					GSUB_1: "K" -> "L"
					GSUB_1: "A" -> "X", "K" -> "X", "L" -> "X"`,
			in:  "AKAKA",
			out: "ALXKA",
		},
		{ // harfbuzz, Mac and Windows agree on this
			desc: `GSUB_5: -ligs "AAA" -> 1@0 5@2 4@1 3@0
					GSUB_5: "AL" -> 2@1
					GSUB_1: "L" -> "A"
					GSUB_1: "A" -> "X"
					GSUB_1: "A" -> "Y"
					GSUB_1: "A" -> "Z"`,
			in:  "ALAA",
			out: "XAYZ",
		},
		{ // harfbuzz: XYAZA, Mac: XAYZA, Windows: XAAYZ
			desc: `GSUB_5: -ligs "AAA" -> 1@0 5@2 4@1 3@0
					GSUB_5: "AL" -> 2@1
					GSUB_2: "L" -> "AA"
					GSUB_1: "A" -> "X"
					GSUB_1: "A" -> "Y"
					GSUB_1: "A" -> "Z"`,
			in:  "ALAA",
			out: "XAAYZ", // TODO(voss): what shall we do?
		},
		{ // harfbuzz, Mac: XLYZ, Windows: XLYZA
			desc: `GSUB_5: -ligs "AAAA" -> 1@0 5@2 4@1 3@0
					GSUB_5: "AL" -> 2@1
					GSUB_4: "LA" -> "L"
					GSUB_1: "A" -> "X"
					GSUB_1: "A" -> "Y"
					GSUB_1: "A" -> "Z"`,
			in:  "ALAAA",
			out: "XLYZ", // TODO(voss): what shall we do?
		},
		{ // harfbuzz, Mac: AKA, Windows: ABAA
			desc: `GSUB_5: -ligs "AAA" -> 1@0
					GSUB_5: "AL" -> 2@1 3@1
					GSUB_4: "LA" -> "K"
					GSUB_1: "L" -> "B"`,
			in:  "ALAA",
			out: "???",
		},
		// {
		// 	desc: `GSUB_5: -marks "AAA" -> 3@2 1@0 2@1
		// 			GSUB_1: "A" -> "X"
		// 			GSUB_1: "A" -> "Y"
		// 			GSUB_1: "A" -> "Z"`,
		// 	in:  "MAMAMAM",
		// 	out: "MXMYMZM",
		// },
		// {
		// 	desc: `GSUB_5: -marks "AAA" -> 1@0 1@1 1@2
		// 			GSUB_4: "AM" -> "X"`,
		// 	in:  "MAMAMAM",
		// 	out: "MXXAM",
		// },
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
			lookupList, err := builder.Parse(fontInfo, test.desc)
			if err != nil {
				t.Fatal(err)
			}

			gsub := &gtab.Info{
				ScriptList: map[gtab.ScriptLang]*gtab.Features{
					{}: {Required: 0},
				},
				FeatureList: []*gtab.Feature{
					{Tag: "test", Lookups: []gtab.LookupIndex{0}},
				},
				LookupList: lookupList,
			}

			if *exportFonts {
				fontInfo.Gdef = gdef
				fontInfo.Gsub = gsub
				err := exportFont(fontInfo, testIdx+1, test.in+" -> "+test.out)
				if err != nil {
					t.Error(err)
				}
			}

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
		})
	}
}

func Test1000(t *testing.T) {
	fontInfo := debug.MakeSimpleFont()

	gidA := fontInfo.CMap.Lookup('A')
	gidB := fontInfo.CMap.Lookup('B')
	gidX := fontInfo.CMap.Lookup('X')
	gidY := fontInfo.CMap.Lookup('Y')

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
								{ // ABABA
									Input: []font.GlyphID{gidB, gidA, gidB, gidA},
									Actions: []gtab.SeqLookup{
										{SequenceIndex: 1, LookupListIndex: 1}, // B(A*)B -> B\1Y
									},
								},
								{ // A
									Actions: []gtab.SeqLookup{
										{SequenceIndex: 0, LookupListIndex: 2}, // A -> X
									},
								},
							},
						},
					},
				},
			},
			{ // lookup 1: B(A*)B -> B\1AA
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
			{ // lookup 2: A -> X
				Meta: &gtab.LookupMetaInfo{LookupType: 1},
				Subtables: []gtab.Subtable{
					&gtab.Gsub1_2{
						Cov:                coverage.Table{gidA: 0},
						SubstituteGlyphIDs: []font.GlyphID{gidX},
					},
				},
			},
			{ // lookup 3: B -> Y
				Meta: &gtab.LookupMetaInfo{LookupType: 1},
				Subtables: []gtab.Subtable{
					&gtab.Gsub1_2{
						Cov:                coverage.Table{gidB: 0},
						SubstituteGlyphIDs: []font.GlyphID{gidY},
					},
				},
			},
		},
	}

	gg := []font.Glyph{
		{Gid: gidA}, // 0+-
		{Gid: gidB}, // 1|
		{Gid: gidA}, // 2| lookup 0 match
		{Gid: gidB}, // 3|
		{Gid: gidA}, // 4+-
		{Gid: gidA},
		{Gid: gidA},
		{Gid: gidA},
		{Gid: gidB},
	}
	gsub := fontInfo.Gsub
	for _, lookupIndex := range gsub.FindLookups(locale.EnUS, nil) {
		gg = gsub.ApplyLookup(gg, lookupIndex, fontInfo.Gdef)
	}

	got := unpack(gg)
	expected := []font.GlyphID{gidA, gidB, gidA, gidX, gidX, gidB}

	if diff := cmp.Diff(expected, got); diff != "" {
		// TODO(voss): re-enable this test once the code is fixed.
		// t.Errorf("unexpected glyphs (-want +got):\n%s", diff)
	}

	exportFont(fontInfo, 1000, "")
}

func Test1001(t *testing.T) {
	fontInfo := debug.MakeSimpleFont()

	gidA := fontInfo.CMap.Lookup('A')
	gidB := fontInfo.CMap.Lookup('B')
	gidX := fontInfo.CMap.Lookup('X')
	gidY := fontInfo.CMap.Lookup('Y')

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
								{ // ABABA
									Input: []font.GlyphID{gidB, gidA, gidB, gidA},
									Actions: []gtab.SeqLookup{
										{SequenceIndex: 3, LookupListIndex: 1}, // B(A*)B -> B\1Y
									},
								},
								{ // A
									Actions: []gtab.SeqLookup{
										{SequenceIndex: 0, LookupListIndex: 2}, // A -> X
									},
								},
							},
						},
					},
				},
			},
			{ // lookup 1: B(A*)B -> B\1AA
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
			{ // lookup 2: A -> X
				Meta: &gtab.LookupMetaInfo{LookupType: 1},
				Subtables: []gtab.Subtable{
					&gtab.Gsub1_2{
						Cov:                coverage.Table{gidA: 0},
						SubstituteGlyphIDs: []font.GlyphID{gidX},
					},
				},
			},
			{ // lookup 3: B -> Y
				Meta: &gtab.LookupMetaInfo{LookupType: 1},
				Subtables: []gtab.Subtable{
					&gtab.Gsub1_2{
						Cov:                coverage.Table{gidB: 0},
						SubstituteGlyphIDs: []font.GlyphID{gidY},
					},
				},
			},
		},
	}

	gg := []font.Glyph{
		{Gid: gidA}, // 0+-
		{Gid: gidB}, // 1|
		{Gid: gidA}, // 2| lookup 0 match
		{Gid: gidB}, // 3|
		{Gid: gidA}, // 4+-
		{Gid: gidA},
		{Gid: gidA},
		{Gid: gidA},
		{Gid: gidB},
	}
	gsub := fontInfo.Gsub
	for _, lookupIndex := range gsub.FindLookups(locale.EnUS, nil) {
		gg = gsub.ApplyLookup(gg, lookupIndex, fontInfo.Gdef)
	}

	got := unpack(gg)
	expected := []font.GlyphID{gidA, gidB, gidA, gidX, gidX, gidB}

	if diff := cmp.Diff(expected, got); diff != "" {
		// TODO(voss): re-enable this test once the code is fixed.
		// t.Errorf("unexpected glyphs (-want +got):\n%s", diff)
	}

	exportFont(fontInfo, 1001, "")
}

func Test1002(t *testing.T) {
	fontInfo := debug.MakeSimpleFont()

	gidA := fontInfo.CMap.Lookup('A')
	gidB := fontInfo.CMap.Lookup('B')
	gidX := fontInfo.CMap.Lookup('X')
	gidY := fontInfo.CMap.Lookup('Y')

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
						Cov: coverage.Table{gidA: 0, gidB: 1},
						Rules: [][]*gtab.SeqRule{
							{
								{ // A
									Actions: []gtab.SeqLookup{
										{SequenceIndex: 0, LookupListIndex: 2}, // A -> X
									},
								},
							},
							{
								{ // BAAAA
									Input: []font.GlyphID{gidA, gidA, gidA, gidA},
									Actions: []gtab.SeqLookup{
										{SequenceIndex: 3, LookupListIndex: 1},
									},
								},
							},
						},
					},
				},
			},
			{ // lookup 1: AAA -> AYA
				Meta: &gtab.LookupMetaInfo{
					LookupType: 5,
				},
				Subtables: []gtab.Subtable{
					&gtab.SeqContext1{
						Cov: map[font.GlyphID]int{gidA: 0},
						Rules: [][]*gtab.SeqRule{
							{
								{
									Input: []font.GlyphID{gidA, gidA},
									Actions: []gtab.SeqLookup{
										{SequenceIndex: 1, LookupListIndex: 3},
									},
								},
							},
						},
					},
				},
			},
			{ // lookup 2: A -> X
				Meta: &gtab.LookupMetaInfo{LookupType: 1},
				Subtables: []gtab.Subtable{
					&gtab.Gsub1_2{
						Cov:                coverage.Table{gidA: 0},
						SubstituteGlyphIDs: []font.GlyphID{gidX},
					},
				},
			},
			{ // lookup 3: A -> Y
				Meta: &gtab.LookupMetaInfo{LookupType: 1},
				Subtables: []gtab.Subtable{
					&gtab.Gsub1_2{
						Cov:                coverage.Table{gidA: 0},
						SubstituteGlyphIDs: []font.GlyphID{gidY},
					},
				},
			},
		},
	}

	gg := []font.Glyph{
		{Gid: gidB}, // 0+-
		{Gid: gidA}, // 1|
		{Gid: gidA}, // 2| lookup 0 match
		{Gid: gidA}, // 3|
		{Gid: gidA}, // 4+-
		{Gid: gidA},
		{Gid: gidA},
	}
	gsub := fontInfo.Gsub
	for _, lookupIndex := range gsub.FindLookups(locale.EnUS, nil) {
		gg = gsub.ApplyLookup(gg, lookupIndex, fontInfo.Gdef)
	}

	got := unpack(gg)
	expected := []font.GlyphID{gidA, gidB, gidA, gidX, gidX, gidB}

	if diff := cmp.Diff(expected, got); diff != "" {
		// TODO(voss): re-enable this test once the code is fixed.
		// t.Errorf("unexpected glyphs (-want +got):\n%s", diff)
	}

	exportFont(fontInfo, 1002, "")
}

func Test1003(t *testing.T) {
	fontInfo := debug.MakeSimpleFont()

	gidA := fontInfo.CMap.Lookup('A')
	gidB := fontInfo.CMap.Lookup('B')
	gidX := fontInfo.CMap.Lookup('X')
	gidY := fontInfo.CMap.Lookup('Y')

	fontInfo.Gsub = &gtab.Info{
		ScriptList: map[gtab.ScriptLang]*gtab.Features{
			{}: {}, // Required: 0
		},
		FeatureList: []*gtab.Feature{
			{Tag: "test", Lookups: []gtab.LookupIndex{0}},
		},
		LookupList: gtab.LookupList{
			{ // lookup 0
				// match "A"; apply 2@0
				// match "BAAAA"; apply 1@4
				Meta: &gtab.LookupMetaInfo{
					LookupType: 5,
				},
				Subtables: []gtab.Subtable{
					&gtab.SeqContext1{
						Cov: coverage.Table{gidA: 0, gidB: 1},
						Rules: [][]*gtab.SeqRule{
							{
								{ // A
									Actions: []gtab.SeqLookup{
										{SequenceIndex: 0, LookupListIndex: 2}, // A -> X
									},
								},
							},
							{
								{ // BAAAA
									Input: []font.GlyphID{gidA, gidA, gidA, gidA},
									Actions: []gtab.SeqLookup{
										{SequenceIndex: 4, LookupListIndex: 1},
									},
								},
							},
						},
					},
				},
			},
			{ // lookup 1: AA -> Y
				Meta: &gtab.LookupMetaInfo{
					LookupType: 4,
				},
				Subtables: []gtab.Subtable{
					&gtab.Gsub4_1{
						Cov: map[font.GlyphID]int{gidA: 0},
						Repl: [][]gtab.Ligature{
							{
								{
									In:  []font.GlyphID{gidA},
									Out: gidY,
								},
							},
						},
					},
				},
			},
			{ // lookup 2: A -> X
				Meta: &gtab.LookupMetaInfo{LookupType: 1},
				Subtables: []gtab.Subtable{
					&gtab.Gsub1_2{
						Cov:                coverage.Table{gidA: 0},
						SubstituteGlyphIDs: []font.GlyphID{gidX},
					},
				},
			},
		},
	}

	gg := []font.Glyph{
		{Gid: gidB}, // 0+-
		{Gid: gidA}, // 1|
		{Gid: gidA}, // 2| lookup 0 match
		{Gid: gidA}, // 3|
		{Gid: gidA}, // 4+-
		{Gid: gidA},
		{Gid: gidA},
	}
	gsub := fontInfo.Gsub
	for _, lookupIndex := range gsub.FindLookups(locale.EnUS, nil) {
		gg = gsub.ApplyLookup(gg, lookupIndex, fontInfo.Gdef)
	}

	got := unpack(gg)
	expected := []font.GlyphID{gidA, gidB, gidA, gidX, gidX, gidB}

	if diff := cmp.Diff(expected, got); diff != "" {
		// TODO(voss): re-enable this test once the code is fixed.
		// t.Errorf("unexpected glyphs (-want +got):\n%s", diff)
	}

	exportFont(fontInfo, 1003, "")
}
