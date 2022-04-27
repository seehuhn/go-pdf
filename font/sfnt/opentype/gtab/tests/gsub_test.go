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
	gidM := fontInfo.CMap.Lookup('M')
	gidN := fontInfo.CMap.Lookup('N')

	type testCase struct {
		lookupType uint16
		subtable   gtab.Subtable
		in, out    string
	}
	cases := []testCase{
		{
			lookupType: 1,
			subtable: &gtab.Gsub1_1{
				Cov:   coverage.Table{gidA: 0, gidM: 1},
				Delta: 1,
			},
			in:  "AAMBA",
			out: "BBMBB",
		},
		{
			lookupType: 1,
			subtable: &gtab.Gsub1_2{
				Cov:                coverage.Table{gidA: 0, gidB: 1, gidM: 2},
				SubstituteGlyphIDs: []font.GlyphID{gidB, gidA, gidB},
			},
			in:  "ABCMA",
			out: "BACMB",
		},
		{
			lookupType: 2,
			subtable: &gtab.Gsub2_1{
				Cov: map[font.GlyphID]int{gidA: 0, gidM: 1},
				Repl: [][]font.GlyphID{
					{gidA, gidB, gidA},
					{gidA},
				},
			},
			in:  "ABMA",
			out: "ABABMABA",
		},
		{
			lookupType: 3,
			subtable: &gtab.Gsub3_1{
				Cov: map[font.GlyphID]int{gidA: 0, gidM: 1},
				Alt: [][]font.GlyphID{
					{gidB, gidC},
					{gidN},
				},
			},
			in:  "ABMA",
			out: "BBMB",
		},
		{
			lookupType: 4,
			subtable: &gtab.Gsub4_1{
				Cov: map[font.GlyphID]int{gidA: 0, gidM: 1},
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
			in:  "AAAMAMAMAAA",
			out: "CMCMMB",
		},
	}

	gdef := &gdef.Table{
		GlyphClass: classdef.Table{
			gidM: gdef.GlyphClassMark,
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
						LookupFlag: gtab.LookupIgnoreMarks,
					},
					Subtables: []gtab.Subtable{test.subtable},
				},
			},
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

		fmt.Println(testIdx+1, test.in, "->", test.out)
		if out != test.out {
			t.Errorf("expected output %q, got %q", test.out, out)
		} else if text != test.in {
			t.Errorf("expected text %q, got %q", test.in, text)
		}

		if *exportFonts {
			fontInfo.Gdef = gdef
			fontInfo.Gsub = gsub
			err := exportFont(fontInfo, testIdx+1)
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
