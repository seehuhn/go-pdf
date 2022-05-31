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
	"fmt"
	"strings"
	"testing"

	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/debug"
	"seehuhn.de/go/pdf/font/sfnt/opentype/classdef"
	"seehuhn.de/go/pdf/font/sfnt/opentype/gdef"
	"seehuhn.de/go/pdf/font/sfnt/opentype/gtab"
	"seehuhn.de/go/pdf/font/sfnt/opentype/gtab/builder"
	"seehuhn.de/go/pdf/locale"
)

func TestGpos(t *testing.T) {
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

	for testIdx, test := range gposTestCases {
		t.Run(fmt.Sprintf("%02d", testIdx+501), func(t *testing.T) {
			fmt.Printf("test%04d.otf %s\n", testIdx+501, test.in)

			desc := test.desc
			if strings.Contains(desc, "Δ") {
				ax := 0
				bx := 0
				pos := 0
				for _, r := range test.in {
					gid := fontInfo.CMap.Lookup(r)
					if r == '>' {
						ax = pos
					} else if r == '<' {
						bx = pos
					}
					pos += int(fontInfo.FGlyphWidth(gid))
				}
				delta := fmt.Sprintf("x%+d", ax-bx)
				desc = strings.Replace(desc, "Δ", delta, 1)
			}

			lookupList, err := builder.Parse(fontInfo, desc)
			if err != nil {
				t.Fatal(err)
			}

			gpos := &gtab.Info{
				ScriptList: map[gtab.ScriptLang]*gtab.Features{
					{}: {Required: 0},
				},
				FeatureList: []*gtab.Feature{
					{Tag: "kern", Lookups: []gtab.LookupIndex{0}},
				},
				LookupList: lookupList,
			}

			if *exportFonts {
				fontInfo.Gdef = gdef
				fontInfo.Gpos = gpos
				exportFont(fontInfo, testIdx+501, test.in)
			}

			seq := make([]font.Glyph, len(test.in))
			for i, r := range test.in {
				gid := fontInfo.CMap.Lookup(r)
				seq[i].Gid = gid
				seq[i].Text = []rune{r}
				seq[i].Advance = int32(fontInfo.FGlyphWidth(gid))
			}
			lookups := gpos.FindLookups(locale.EnUS, nil)
			for _, lookupIndex := range lookups {
				seq = gpos.LookupList.ApplyLookup(seq, lookupIndex, gdef)
			}

			for i, g := range seq {
				fmt.Println(i, g)
			}
		})
	}
}

type gposTestCase struct {
	desc string
	in   string
}

var gposTestCases = []gposTestCase{
	{ // test0501.odf
		desc: "GPOS1: B -> y+500",
		in:   "ABC",
	},
	{
		desc: `GPOS1: "<" -> Δ`,
		in:   ">ABC<",
	},
	{
		desc: "GPOS1: M -> y+500",
		in:   "AMA",
	},
	{
		desc: "GPOS1: -marks M -> y+500",
		in:   "AMA",
	},
}
