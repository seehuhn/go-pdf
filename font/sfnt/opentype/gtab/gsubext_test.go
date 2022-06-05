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

package gtab_test

import (
	"flag"
	"fmt"
	"os"
	"testing"
	"time"

	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/debug"
	"seehuhn.de/go/pdf/font/sfnt"
	"seehuhn.de/go/pdf/font/sfnt/opentype/classdef"
	"seehuhn.de/go/pdf/font/sfnt/opentype/gdef"
	"seehuhn.de/go/pdf/font/sfnt/opentype/gtab"
	"seehuhn.de/go/pdf/font/sfnt/opentype/gtab/builder"
	"seehuhn.de/go/pdf/locale"
)

func TestGsub(t *testing.T) {
	fontInfo := debug.MakeSimpleFont()

	gdef := &gdef.Table{
		GlyphClass: classdef.Table{
			fontInfo.CMap.Lookup('B'): gdef.GlyphClassBase,
			fontInfo.CMap.Lookup('K'): gdef.GlyphClassLigature,
			fontInfo.CMap.Lookup('L'): gdef.GlyphClassLigature,
			fontInfo.CMap.Lookup('M'): gdef.GlyphClassMark,
			fontInfo.CMap.Lookup('N'): gdef.GlyphClassMark,
		},
	}

	a, b := fontInfo.CMap.CodeRange()
	rev := make(map[font.GlyphID]rune)
	for r := a; r <= b; r++ {
		gid := fontInfo.CMap.Lookup(r)
		if gid != 0 {
			rev[gid] = r
		}
	}

	for testIdx, test := range gsubTestCases {
		t.Run(fmt.Sprintf("%02d", testIdx+1), func(t *testing.T) {
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
				exportFont(fontInfo, testIdx+1, test.in)
			}

			seq := make([]font.Glyph, len(test.in))
			for i, r := range test.in {
				seq[i].Gid = fontInfo.CMap.Lookup(r)
				seq[i].Text = []rune{r}
			}
			lookups := gsub.FindLookups(locale.EnUS, nil)
			for _, lookupIndex := range lookups {
				seq = gsub.LookupList.ApplyLookup(seq, lookupIndex, gdef)
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
			// fmt.Printf("test%04d.otf %s -> %s\n", testIdx+1, test.in, test.out)
			if out != test.out {
				t.Errorf("expected output %q, got %q", test.out, out)
			} else if text != expectedText {
				t.Errorf("expected text %q, got %q", expectedText, text)
			}
		})
	}
}

func FuzzGsub(f *testing.F) {
	for _, test := range gsubTestCases {
		f.Add(test.desc, test.in)
	}

	fontInfo := debug.MakeSimpleFont()
	gdefTable := &gdef.Table{
		GlyphClass: classdef.Table{
			fontInfo.CMap.Lookup('B'): gdef.GlyphClassBase,
			fontInfo.CMap.Lookup('K'): gdef.GlyphClassLigature,
			fontInfo.CMap.Lookup('L'): gdef.GlyphClassLigature,
			fontInfo.CMap.Lookup('M'): gdef.GlyphClassMark,
			fontInfo.CMap.Lookup('N'): gdef.GlyphClassMark,
		},
	}

	f.Fuzz(func(t *testing.T, desc string, in string) {
		lookupList, err := builder.Parse(fontInfo, desc)
		if err != nil {
			return
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

		seq := make([]font.Glyph, len(in))
		for i, r := range in {
			seq[i].Gid = fontInfo.CMap.Lookup(r)
			seq[i].Text = []rune{r}
		}
		lookups := gsub.FindLookups(locale.EnUS, nil)
		for _, lookupIndex := range lookups {
			seq = gsub.LookupList.ApplyLookup(seq, lookupIndex, gdefTable)
		}

		runeCountIn := len([]rune(in))
		runeCountOut := 0
		for _, g := range seq {
			runeCountOut += len([]rune(g.Text))
		}
		if runeCountOut != runeCountIn {
			fmt.Printf("desc = %q\n", desc)
			fmt.Printf("in = %q\n", in)
			for i, g := range seq {
				fmt.Printf("out[%d] = %d %q\n", i, g.Gid, string(g.Text))
			}
			t.Errorf("expected %d runes, got %d", runeCountIn, runeCountOut)
		}
	})
}

var exportFonts = flag.Bool("export-fonts", false, "export fonts used in tests")

func exportFont(fontInfo *sfnt.Info, idx int, in string) {
	if !*exportFonts {
		return
	}

	fontInfo.FamilyName = fmt.Sprintf("Test%04d", idx)
	now := time.Now()
	fontInfo.CreationTime = now
	fontInfo.ModificationTime = now
	fontInfo.SampleText = in

	fname := fmt.Sprintf("test%04d.otf", idx)
	fd, err := os.Create(fname)
	if err != nil {
		panic(err)
	}
	_, err = fontInfo.Write(fd)
	if err != nil {
		panic(err)
	}
	err = fd.Close()
	if err != nil {
		panic(err)
	}
}

type gsubTestCase struct {
	desc    string
	in, out string
	text    string // text content, if different from `in`
}

var gsubTestCases = []gsubTestCase{
	{ // test0001.odf
		desc: "GSUB1: A->X, C->Z",
		in:   "ABC",
		out:  "XBZ",
	},
	{
		desc: "GSUB1: A->B, B->A",
		in:   "ABC",
		out:  "BAC",
	},
	{
		desc: "GSUB1: -base A->B, B->A",
		in:   "ABC",
		out:  "BBC",
	},
	{
		desc: "GSUB1: -marks A->B, M->N",
		in:   "AAMBA",
		out:  "BBMBB",
	},

	{
		desc: `GSUB2: A->A A`,
		in:   "AA",
		out:  "AAAA",
	},
	{
		desc: `GSUB2: -marks A -> "ABA", M -> A`,
		in:   "ABMA",
		out:  "ABABMABA",
	},

	{
		desc: `GSUB3: A -> [B C D]`,
		in:   "AB",
		out:  "BB",
	},
	{
		desc: `GSUB3: -marks A -> [B C], M -> [B C]`,
		in:   "AM",
		out:  "BM",
	},
	{
		desc: `GSUB3: A -> []`,
		in:   "AB",
		out:  "AB",
	},

	{
		desc: `GSUB4: "BA" -> B`,
		in:   "ABAABA",
		out:  "ABAB",
	},
	{
		desc: `GSUB4: "AAA" -> "B", "AA" -> "C", "A" -> "D"`,
		in:   "AAAAAXA",
		out:  "BCXD",
	},
	{
		desc: `GSUB4: -marks "AAA" -> "X"`,
		in:   "AAABMAAACAMAADAAMAEAAAM",
		out:  "XBMXCXMDXMEXM",
		text: "AAABMAAACAAAMDAAAMEAAAM",
	},
	{
		desc: `GSUB4: -marks "AAA" -> "C", "AA" -> "B"`,
		in:   "AAAMAMAMAAA",
		out:  "CMCMMB",
		text: "AAAMAAAMMAA",
	},

	{
		desc: `GSUB5: "AAA" -> 3@2 1@0 2@1
				GSUB1: "A" -> "X"
				GSUB1: "A" -> "Y"
				GSUB1: "A" -> "Z"`,
		in:  "AAA",
		out: "XYZ",
	},
	{ // test0011.odf
		desc: `GSUB5: "XXX" -> 1@0
				GSUB1: "X" -> "A"`,
		in:  "XXXXXXXX",
		out: "AXXAXXXX",
	},
	{
		desc: `GSUB5: "ABC" -> 1@0 1@1 1@2
				GSUB1: "B" -> "X"`,
		in:  "ABC",
		out: "AXC",
	},
	{ // harfbuzz, Mac and Windows agree on this
		desc: `GSUB5: "AAAA" -> 1@0 4@2 3@1 2@0
				GSUB4: "AA" -> "A"
				GSUB1: "A" -> "X"
				GSUB1: "A" -> "Y"
				GSUB1: "A" -> "Z"`,
		in:  "AAAA",
		out: "XYZ",
	},
	{ // harfbuzz, Mac and Windows agree on this
		desc: `GSUB5: "AAA" -> 1@0 4@2 3@1 2@0
				GSUB2: "A" -> "AA"
				GSUB1: "A" -> "X"
				GSUB1: "A" -> "Y"
				GSUB1: "A" -> "Z"`,
		in:  "AAA",
		out: "XYZA",
	},
	{ // harfbuzz, Mac and Windows agree on this
		desc: `GSUB5: "AAA" -> 1@0 5@2 4@1 3@0
				GSUB5: "AA" -> 2@1
				GSUB2: "A" -> "AA"
				GSUB1: "A" -> "X"
				GSUB1: "A" -> "Y"
				GSUB1: "A" -> "Z"`,
		in:  "AAA",
		out: "XYZA",
	},

	//
	// ------------------------------------------------------------------
	// Testing glyph positions in recursive lookups, in particular when the
	// sequence length changes:
	//
	{ // harfbuzz, Mac and Windows agree on this
		desc: `GSUB5: "AA" -> 1@0 2@0
				GSUB1: "A" -> "B"
				GSUB1: "A" -> "X", "B" -> "X"`,
		in:  "AA",
		out: "XA",
	},
	{ // harfbuzz, Mac and Windows agree on this
		desc: `GSUB5: "AA" -> 1@0 2@0
				GSUB2: "A" -> "B"
				GSUB1: "A" -> "X", "B" -> "X"`,
		in:  "AA",
		out: "XA",
	},
	{ // harfbuzz, Mac and Windows agree on this
		desc: `GSUB5: "AA" -> 1@0 2@0
				GSUB4: "A" -> "B"
				GSUB1: "A" -> "X", "B" -> "X"`,
		in:  "AA",
		out: "XA",
	},
	//
	// The same, but with one more level of nesting.
	{
		desc: `GSUB5: "AA" -> 1@0 3@0
				GSUB5: "A" -> 2@0
				GSUB1: "A" -> "B"
				GSUB1: "A" -> "X", "B" -> "X"`,
		in:  "AA",
		out: "XA",
	},
	{
		desc: `GSUB5: "AA" -> 1@0 3@0
				GSUB5: "A" -> 2@0
				GSUB2: "A" -> "B"
				GSUB1: "A" -> "X", "B" -> "X"`,
		in:  "AA",
		out: "XA",
	},
	{
		desc: `GSUB5: "AA" -> 1@0 3@0
				GSUB5: "A" -> 2@0
				GSUB4: "A" -> "B"
				GSUB1: "A" -> "X", "B" -> "X"`,
		in:  "AA",
		out: "XA",
	},
	//
	// ... and with ligatures ignored
	{
		desc: `GSUB5: -ligs "AA" -> 1@0 3@0
				GSUB5: "A" -> 2@0
				GSUB1: "A" -> "B"
				GSUB1: "A" -> "X", "B" -> "X"`,
		in:  "ALA",
		out: "XLA",
	},
	{
		desc: `GSUB5: -ligs "AA" -> 1@0 3@0
				GSUB5: "AL" -> 2@0
				GSUB1: "A" -> "B"
				GSUB1: "A" -> "X", "B" -> "X"`,
		in:  "ALA",
		out: "XLA",
	},
	{ // harfbuzz, Mac and Windows agree on this
		desc: `GSUB5: -ligs "AA" -> 1@0 3@1
				GSUB5: "AL" -> 2@0
				GSUB2: "A" -> "BB"
				GSUB1: "A" -> "Y", "B" -> "Y"`,
		in:  "ALA",
		out: "BYLA",
	},

	// everything above currently passes ---------------------------------

	// Check under which circumstances new glyphs are added to the
	// input sequence.

	// ------------------------------------------------------------------
	// single glyphs:
	//   A: yes
	//   M: no

	// We have seen above that if a normal glyph is replaced, the
	// replacement IS added to the input sequence.

	// If a single ignored glyph is replaced, the replacement is NOT added
	// to the input sequence:
	{ // ALA -> ABA -> ABX
		// harfbuzz, Mac and Windows agree on this
		desc: `GSUB5: -ligs "AA" -> 1@0 3@1
				GSUB5: "ALA" -> 2@1
				GSUB4: "L" -> "B"
				GSUB1: "A" -> "X", "B" -> "Y"`,
		in:  "ALA",
		out: "ABX",
	},
	{ // ALA -> ABBA -> ...
		// harfbuzz: AYBA, Mac: ABYA, Windows: ABBY
		desc: `GSUB5: -ligs "AA" -> 1@0 3@1
				GSUB5: "AL" -> 2@1
				GSUB2: "L" -> "BB"
				GSUB1: "A" -> "Y", "B" -> "Y"`,
		in:  "ALA",
		out: "ABBY",
	},

	// ------------------------------------------------------------------
	// pairs:
	//   AA: yes
	//   MA: yes
	//   AM: mixed????
	//   MM: no

	// When a pair of normal glyphs is replaced, the replacement IS added.
	{ // AAA -> AB -> ...
		// harfbuzz, Mac and Windows agree on this
		desc: `GSUB5: -ligs "AAA" -> 1@0 3@1
				GSUB5: "AAA" -> 2@1
				GSUB4: "AA" -> "B"
				GSUB1: "A" -> "X", "B" -> "Y"`,
		in:  "AAA",
		out: "AY",
	},

	{ // ALA -> AB -> ...
		// harfbuzz: AB, Mac: AB, Windows: AY -> yes
		desc: `GSUB5: -ligs "AA" -> 1@0 3@1
				GSUB5: "ALA" -> 2@1
				GSUB4: "LA" -> "B"
				GSUB1: "A" -> "X", "B" -> "Y"`,
		in:  "ALA",
		out: "AY",
	},

	// normal+ignored
	{ // harfbuzz: , Mac: YAA, Windows: YAA -> yes
		desc: `GSUB5: -marks "AAA" -> 1@0 3@0
				GSUB5: "AM" -> 2@0
				GSUB4: "AM" -> "B"
				GSUB1: "A" -> "X", "B" -> "Y"`,
		in:  "AMAA",
		out: "YAA",
	},
	// { // harfbuzz: AXA, Mac: AXA, Windows: AAX -> no ????????????????????
	// 	desc: `GSUB5: -marks "AAA" -> 1@1 2@1
	// 			GSUB4: "AM" -> "A"
	// 			GSUB1: "A" -> "X"`,
	// 	in:  "AAMA",
	// 	out: "AAX",
	// },
	// { // harfbuzz: AXA, Mac: AXA, Windows: AAX -> no ????????????????????
	// 	desc: `GSUB5: -marks "ALA" -> 1@1 2@1
	// 			GSUB4: "LM" -> "A"
	// 			GSUB1: "A" -> "X"`,
	// 	in:  "ALMA",
	// 	out: "AAX",
	// },

	// When a pair of ignored glyphs is replaced, the replacement is NOT
	// added.
	{ // ALLA -> ABA -> ...
		// harfbuzz: ABA, Mac: ABX, Windows: ABX -> no
		desc: `GSUB5: -ligs "AA" -> 1@0 3@1
				GSUB5: "ALL" -> 2@1
				GSUB4: "LL" -> "B"
				GSUB1: "A" -> "X", "B" -> "Y"`,
		in:  "ALLA",
		out: "ABX",
	},

	// ------------------------------------------------------------------
	// triples:
	//   AAA: (I assume yes)
	//   AAM: yes
	//   AMA: yes
	//   AMM: yes
	//   MAA: yes
	//   MAM: no
	//   MMA: yes
	//   MMM: no

	{ // harfbuzz: YA, Mac: YA, Windows: YA -> included
		desc: `GSUB5: -marks "AAA" -> 1@0 3@0
				GSUB5: "AAM" -> 2@0
				GSUB4: "AAM" -> "B"
				GSUB1: "A" -> "X", "B" -> "Y"`,
		in:  "AAMA",
		out: "YA",
	},

	{ // harfbuzz: YA, Mac: YA, Windows: YA -> included
		desc: `GSUB5: -marks "AAA" -> 1@0 3@0
				GSUB5: "AMA" -> 2@0
				GSUB4: "AMA" -> "B"
				GSUB1: "A" -> "X", "B" -> "Y"`,
		in:  "AMAA",
		out: "YA",
	},
	{ // harfbuzz: ABA, Mac: AYA, Windows: AYA -> included
		desc: `GSUB5: -marks "AAA" -> 1@0 3@1
				GSUB5: "AAMA" -> 2@1
				GSUB4: "AMA" -> "B"
				GSUB1: "A" -> "X", "B" -> "Y"`,
		in:  "AAMAA",
		out: "AYA",
	},
	{ // harfbuzz: ABX, Mac: AYA, Windows: AYA -> included
		desc: `GSUB5: -marks "AAAA" -> 1@0 3@1
				GSUB5: "AAMA" -> 2@1
				GSUB4: "AMA" -> "B"
				GSUB1: "A" -> "X", "B" -> "Y"`,
		in:  "AAMAA",
		out: "AYA",
	},

	{ // harfbuzz: YAA, Mac: YAA, Windows: YAA -> included
		desc: `GSUB5: -marks "AAA" -> 1@0 3@0
				GSUB5: "AMM" -> 2@0
				GSUB4: "AMM" -> "B"
				GSUB1: "A" -> "X", "B" -> "Y"`,
		in:  "AMMAA",
		out: "YAA",
	},

	{ // harfbuzz: AB, Mac: AB, Windows: AY -> included
		desc: `GSUB5: -marks "AAA" -> 1@0 3@1
				GSUB5: "AMAA" -> 2@1
				GSUB4: "MAA" -> "B"
				GSUB1: "A" -> "X", "B" -> "Y"`,
		in:  "AMAA",
		out: "AY",
	},

	{ // harfbuzz: ABA, Mac: ABX, Windows: ABX -> not included
		desc: `GSUB5: -marks "AAA" -> 1@0 3@1
				GSUB5: "AMAM" -> 2@1
				GSUB4: "MAM" -> "B"
				GSUB1: "A" -> "X", "B" -> "Y"`,
		in:  "AMAMA",
		out: "ABX",
	},
	{ // ALALA -> ABA -> ...
		// harfbuzz: ABA, Mac: ABX, Windows: ABX -> not included
		desc: `GSUB5: -ligs "AAA" -> 1@0 3@1
				GSUB5: "ALALA" -> 2@1
				GSUB4: "LAL" -> "B"
				GSUB1: "A" -> "X", "B" -> "Y"`,
		in:  "ALALA",
		out: "ABX",
	},

	{ // harfbuzz: ABA, Mac: ABX, Windows: AYA -> included
		desc: `GSUB5: -marks "AAA" -> 1@0 3@1
				GSUB5: "AMMA" -> 2@1
				GSUB4: "MMA" -> "B"
				GSUB1: "A" -> "X", "B" -> "Y"`,
		in:  "AMMAA",
		out: "AYA",
	},

	{ // harfbuzz: ABAA, Mac: ABXA, Windows: ABXA -> not included
		desc: `GSUB5: -marks "AAA" -> 1@0 3@1
				GSUB5: "AMMM" -> 2@1
				GSUB4: "MMM" -> "B"
				GSUB1: "A" -> "X", "B" -> "Y"`,
		in:  "AMMMAA",
		out: "ABXA",
	},

	// XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX

	// sequences of length 4
	//   AAAA -> (yes, I guess)
	//   AAAM
	//   AAMA
	//   AAMM
	//   AMAA
	//   AMAM yes
	//   AMMA
	//   AMMM yes
	//   MAAA
	//   MAAM no
	//   MAMA yes
	//   MAMM no
	//   MMAA
	//   MMAM no
	//   MMMA
	//   MMMM -> (no, I guess)

	{ // harfbuzz: , Mac: YA, Windows: YA -> yes
		desc: `GSUB5: -marks "AAA" -> 1@0 3@0
				GSUB5: "AMAM" -> 2@0
				GSUB4: "AMAM" -> "B"
				GSUB1: "A" -> "X", "B" -> "Y"`,
		in:  "AMAMA",
		out: "YA",
	},

	{ // harfbuzz: , Mac: YAA, Windows: YAA -> yes
		desc: `GSUB5: -marks "AAA" -> 1@0 3@0
				GSUB5: "AMMM" -> 2@0
				GSUB4: "AMMM" -> "B"
				GSUB1: "A" -> "X", "B" -> "Y"`,
		in:  "AMMMAA",
		out: "YAA",
	},

	{ // harfbuzz: , Mac: ABX, Windows: ABX -> no
		desc: `GSUB5: -marks "AAAA" -> 1@0 3@1
				GSUB5: "AMAAM" -> 2@1
				GSUB4: "MAAM" -> "B"
				GSUB1: "A" -> "X", "B" -> "Y"`,
		in:  "AMAAMA",
		out: "ABX",
	},

	{ // harfbuzz: , Mac: AB, Windows: AY -> yes
		desc: `GSUB5: -marks "AAA" -> 1@0 3@1
				GSUB5: "AMAMA" -> 2@1
				GSUB4: "MAMA" -> "B"
				GSUB1: "A" -> "X", "B" -> "Y"`,
		in:  "AMAMA",
		out: "AY",
	},

	{ // harfbuzz: , Mac: ABX, Windows: ABX -> no
		desc: `GSUB5: -marks "AAA" -> 1@0 3@1
				GSUB5: "AMAMM" -> 2@1
				GSUB4: "MAMM" -> "B"
				GSUB1: "A" -> "X", "B" -> "Y"`,
		in:  "AMAMMA",
		out: "ABX",
	},

	{ // harfbuzz: , Mac: ABX, Windows: ABX -> no
		desc: `GSUB5: -marks "AAA" -> 1@0 3@1
				GSUB5: "AMMAM" -> 2@1
				GSUB4: "MMAM" -> "B"
				GSUB1: "A" -> "X", "B" -> "Y"`,
		in:  "AMMAMA",
		out: "ABX",
	},

	// XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX

	// The difference between the following two cases is mysterious to me.

	{ // harfbuzz: YAA, Mac: YAA, Windows: YAA -> yes
		desc: `GSUB5: -marks "AAA" -> 1@0 2@0
				GSUB4: "AM" -> "B"
				GSUB1: "A" -> "X", "B" -> "Y"`,
		in:  "AMAA",
		out: "YAA",
	},
	// { // harfbuzz: AYA, Mac: AYA, Windows: ABX -> no ????????????????????
	// 	desc: `GSUB5: -marks "AAA" -> 1@1 2@1
	// 			GSUB4: "AM" -> "B"
	// 			GSUB1: "A" -> "X", "B" -> "Y"`,
	// 	in:  "AAMA",
	// 	out: "ABX",
	// },

	// XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX

	// longer:
	//   MMMAAA -> yes
	//   MMAMAA -> yes

	{ // harfbuzz: ABA, Mac: ABX, Windows: AYA -> included
		desc: `GSUB5: -marks "AAAAA" -> 1@0 3@1
				GSUB5: "AMMMAAA" -> 2@1
				GSUB4: "MMMAAA" -> "B"
				GSUB1: "A" -> "X", "B" -> "Y"`,
		in:  "AMMMAAAA",
		out: "AYA",
	},

	{ // harfbuzz: ABA, Mac: ABX, Windows: AYA -> included
		desc: `GSUB5: -marks "AAAAA" -> 1@0 3@1
				GSUB5: "AMMAMAA" -> 2@1
				GSUB4: "MMAMAA" -> "B"
				GSUB1: "A" -> "X", "B" -> "Y"`,
		in:  "AMMAMAAA",
		out: "AYA",
	},

	// XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX

	// {
	// 	// ALMA -> AAA -> ...
	// 	// harfbuzz: AXA, Mac: AXY, Windows: AAY ????????????????????????
	// 	desc: `GSUB5: -ligs -marks "AA" -> 1@0 4@1
	// 			GSUB5: -marks "ALA" -> 2@1 3@1
	// 			GSUB4: "LM" -> "A"
	// 			GSUB1: "A" -> "X"
	// 			GSUB1: "A" -> "Y", "X" -> "Y"`,
	// 	in:  "ALMA",
	// 	out: "AAY",
	// },

	{ // harfbuzz: DEI, Mac: DEI, Windows: DEI
		desc: `GSUB5: "ABC" -> 1@0 1@2 1@3 2@2
				GSUB2: "A" -> "DE", "B" -> "FG", "G" -> "H"
				GSUB4: "FHC" -> "I"`,
		in:  "ABC",
		out: "DEI", // ABC -> DEBC -> DEFGC -> DEFHC -> DEI
	},
	{ // harfbuzz: AXAAKA, Mac: AAXAKA, Windows: AAAXKA
		desc: `GSUB5: -ligs "AAA" -> 1@0 2@1 3@1
				GSUB5: "AK" -> 2@1
				GSUB2: "K" -> "AA"
				GSUB1: "A" -> "X", "K" -> "X", "L" -> "X"`,
		in:  "AKAKA",
		out: "AAAXKA",
	},
	{ //  harfbuzz: AXLAKA, Mac: ALXAKA, Windows: ALLXKA
		desc: `GSUB5: -ligs "AAA" -> 1@0 2@1 3@1
				GSUB5: "AK" -> 2@1
				GSUB2: "K" -> "LL"
				GSUB1: "A" -> "X", "K" -> "X", "L" -> "X"`,
		in:  "AKAKA",
		out: "ALLXKA",
	},
	{ // harfbuzz, Mac and Windows agree on this
		desc: `GSUB5: -ligs "AAA" -> 1@0 5@2 4@1 3@0
				GSUB5: "AL" -> 2@1
				GSUB1: "L" -> "A"
				GSUB1: "A" -> "X"
				GSUB1: "A" -> "Y"
				GSUB1: "A" -> "Z"`,
		in:  "ALAA",
		out: "XAYZ",
	},
	{ // harfbuzz, Mac and Windows agree on this
		desc: `GSUB5: "AB" -> 1@0 0@0, "AAB" -> 1@0 0@0, "AAAB" -> 1@0 0@0
				GSUB2: "A" -> "AA"`,
		in:  "AB",
		out: "AAAAB",
	},
	{ // harfbuzz: XYAZA, Mac: XAYZA, Windows: XAAYZ
		desc: `GSUB5: -ligs "AAA" -> 1@0 5@2 4@1 3@0
				GSUB5: "AL" -> 2@1
				GSUB2: "L" -> "AA"
				GSUB1: "A" -> "X"
				GSUB1: "A" -> "Y"
				GSUB1: "A" -> "Z"`,
		in:  "ALAA",
		out: "XAAYZ",
	},
	{ // harfbuzz: XLYZ, Mac: XLYZ, Windows: XLYZA
		desc: `GSUB5: -ligs "AAAA" -> 1@0 5@2 4@1 3@0
				GSUB5: "AL" -> 2@1
				GSUB4: "LA" -> "L"
				GSUB1: "A" -> "X"
				GSUB1: "A" -> "Y"
				GSUB1: "A" -> "Z"`,
		in:  "ALAAA",
		out: "XLYZA",
	},
	{ // harfbuzz: AKA, Mac: AKA, Windows: ABAA
		desc: `GSUB5: -ligs "AAA" -> 1@0
				GSUB5: "AL" -> 2@1 3@1
				GSUB4: "LA" -> "K"
				GSUB1: "L" -> "B"`,
		in:  "ALAA",
		out: "ABAA",
	},
	{ // harfbuzz: LXLYLZL, Mac: LXLYLZL, Windows: LXLYLZL
		desc: `GSUB5: -ligs "AAA" -> 3@2 1@0 2@1
				GSUB1: "A" -> "X"
				GSUB1: "A" -> "Y"
				GSUB1: "A" -> "Z"`,
		in:  "LALALAL",
		out: "LXLYLZL",
	},

	// { // harfbuzz: LXALX, Mac: LXXX, Windows: LXXAL ???????????????????????
	// 	desc: `GSUB5: -ligs "AAA" -> 1@0 1@1 1@2
	// 			GSUB4: "AL" -> "X"`,
	// 	in:  "LALALAL",
	// 	out: "LXXAL",
	// },
	// { // Mac: LALALX, Windows: LALALAL ????????????????????????????????????
	// 	desc: `GSUB5: -ligs "AAA" -> 1@2
	// 			GSUB4: "AL" -> "X"`,
	// 	in:  "LALALAL",
	// 	out: "LALALAL",
	// },
	{ // Mac: LALALXB, Windows: LALALXB -> "AL" WAS added to input here
		desc: `GSUB5: -ligs "AAA" -> 1@2
				GSUB4: "AL" -> "X"`,
		in:  "LALALALB",
		out: "LALALXB",
	},
	{ // Mac: ABXD, Windows: ABXD -> "CL" was added to input here
		desc: `GSUB5: -ligs "ABC" -> 1@2
			GSUB4: C L -> X`,
		in:  "ABCLD",
		out: "ABXD",
	},

	{ // Mac: XCEAAAAXFBCAACX, Windows: XCEAAAAXFBCAACX
		desc: `GSUB5:
				"ACE" -> 1@0 ||
				class :AB: = [A B]
				class :CD: = [C D]
				class :EF: = [E F]
				/A B/ :AB: :CD: :EF: -> 1@1 ||
				class :AB: = [A B]
				/A/ :AB: :: :AB: -> 1@2
			GSUB1: A -> X, B -> X, C -> X, D -> X, E -> X, F -> X`,
		in:  "ACEAAAACFBCAACB",
		out: "XCEAAAAXFBCAACX",
	},

	{ // Mac: XBCFBCXA, Windows: XBCFBCXA
		desc: `GSUB5:
				[A-E] [A B] [C-X] -> 1@0 ||
				[B-E] [B-E] [A-C] -> 1@1
			GSUB1: A -> X, B -> X, C -> X, D -> X, E -> X, F -> X`,
		in:  "ABCFBCEA",
		out: "XBCFBCXA",
	},
	{ // Mac: X, Windows: X
		desc: `GSUB5: -ligs
				class :A: = [A]
				/A/ :A: -> 1@0
			GSUB4: A L -> X`,
		in:  "AL",
		out: "X",
	},

	// lookup rules with context

	{
		desc: `GSUB6: A B | C D | E F -> 1@0
			GSUB1: C -> X`,
		in:  "ABCDEF",
		out: "ABXDEF",
	},
	{
		desc: `GSUB6: A B | C D | E F -> 1@0
			GSUB1: C -> X`,
		in:  "ABCDE",
		out: "ABCDE",
	},
	{
		desc: `GSUB6: A B | C D | E F -> 1@0
			GSUB1: C -> X`,
		in:  "ABC",
		out: "ABC",
	},
	{
		desc: `GSUB6: A | A | A -> 1@0, X | A | A -> 1@0
			GSUB1: A -> X`,
		in:  "AAAAAA",
		out: "AXXXXA",
	},
	{
		desc: `GSUB6: -ligs A | B B | A -> 1@0 2@0
			GSUB4: -ligs B B -> X
			GSUB1: X -> Y`,
		in:   "ABBALBLBLA",
		out:  "AYALYLLA",
		text: "ABBALBBLLA",
	},
	{ // harfbuzz: AX, Mac: AX, Windows: ABB
		desc: `GSUB6: A | B | B -> 1@0
			GSUB4: B B -> X`,
		in:  "ABB",
		out: "ABB",
	},
	{ // harfbuzz, Mac and Windows agree on this
		desc: `GSUB6: B | C | D -> 1@0
			GSUB6: A B | C | D E -> 2@0
			GSUB4: C -> X`,
		in:  "ABCDE",
		out: "ABXDE",
	},
	{
		desc: `GSUB6: -ligs A | A A | A -> 1@0
			GSUB4: A L A -> X`,
		in:  "AALAA",
		out: "AXA",
	},
	{
		desc: `GSUB6: -ligs A | A | A -> 1@0
			GSUB4: A L A -> X`,
		in:  "AALA",
		out: "AALA",
	},
	{
		desc: `GSUB6: -ligs A | A | A -> 1@0
			GSUB4: A L -> X`,
		in:  "AALA",
		out: "AXA",
	},
}
