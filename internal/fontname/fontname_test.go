// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2026  Jochen Voss <voss@seehuhn.de>
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

package fontname

import (
	"strings"
	"testing"

	"seehuhn.de/go/postscript/type1"
	"seehuhn.de/go/sfnt"
	"seehuhn.de/go/sfnt/glyf"

	"seehuhn.de/go/pdf/font/subset"
)

// nameTable holds the rules of a font which keeps its PostScript name in the
// "name" table.  This is the narrower of the two sets of rules a name may have
// to satisfy, and the one a font with "glyf" outlines imposes.
var nameTable = (&sfnt.Font{Outlines: &glyf.Outlines{}}).CheckFontName

// nameTableBudget is the number of bytes such a font leaves for a name once a
// subset tag is accounted for: the "name" table holds 63 characters.
const nameTableBudget = 63 - subset.TagLen

func TestFontName(t *testing.T) {
	const maxLen = subset.MaxBaseNameLen

	for _, tc := range []struct {
		label                      string
		psName, familyName, subfam string
		want                       string
	}{
		{
			label:  "the font's own name wins",
			psName: "Quire-Regular", familyName: "Quire Display", subfam: "Bold",
			want: "Quire-Regular",
		},
		{
			label:  "a subset tag is kept",
			psName: "ABCDEF+Quire-Regular", familyName: "Quire",
			want: "ABCDEF+Quire-Regular",
		},
		{
			label:      "derived from family and subfamily",
			familyName: "Times New Roman", subfam: "Bold",
			want: "TimesNewRoman-Bold",
		},
		{
			label:      "derived from the family alone",
			familyName: "Times New Roman",
			want:       "TimesNewRoman",
		},
		{
			label:      "delimiters are dropped from a derived name",
			familyName: "A(n)d[r]o{m}e/d<a> N%ebula", subfam: "Bold Italic",
			want: "AndromedaNebula-BoldItalic",
		},
		{
			// filtering the other characters out would leave a fragment
			// naming nothing, so there is no name to derive
			label:      "a family name outside ASCII yields the placeholder",
			familyName: "Grüße Sans", subfam: "Bold",
			want: placeholder,
		},
		{
			label:      "a subfamily outside ASCII is left out",
			familyName: "Quire", subfam: "粗体",
			want: "Quire",
		},
		{
			label: "a font which names nothing gets the placeholder",
			want:  placeholder,
		},
		{
			label:  "a subfamily alone is not a name",
			subfam: "Bold",
			want:   placeholder,
		},
	} {
		t.Run(tc.label, func(t *testing.T) {
			got := FontName(tc.psName, tc.familyName, tc.subfam, maxLen,
				type1.CheckFontName)
			if got != tc.want {
				t.Errorf("FontName(%q, %q, %q) = %q, want %q",
					tc.psName, tc.familyName, tc.subfam, got, tc.want)
			}
		})
	}
}

// A font may name itself in a way it cannot itself store: a "name" table takes
// only a subset of ASCII, whereas the name reaching this function may have come
// from a CFF Name INDEX or from the /BaseFont entry of a PDF file, where a CJK
// font is commonly named in UTF-8.  Such a name is given up whole rather than
// stripped down to the characters which fit, since the remainder would stand
// for a different font.
func TestFontNameTheFontCannotCarry(t *testing.T) {
	for _, tc := range []struct {
		label                      string
		psName, familyName, subfam string
		want                       string
	}{
		{
			label:  "the family name takes over",
			psName: "宋体-Regular", familyName: "Song Ti", subfam: "Regular",
			want: "SongTi-Regular",
		},
		{
			label:  "the subset tag is kept",
			psName: "ABCDEF+宋体", familyName: "Song Ti",
			want: "ABCDEF+SongTi",
		},
		{
			label:  "nothing else to go on",
			psName: "宋体", familyName: "宋体",
			want: placeholder,
		},
	} {
		t.Run(tc.label, func(t *testing.T) {
			got := FontName(tc.psName, tc.familyName, tc.subfam,
				nameTableBudget, nameTable)
			if got != tc.want {
				t.Errorf("FontName(%q, %q, %q) = %q, want %q",
					tc.psName, tc.familyName, tc.subfam, got, tc.want)
			}
			if err := nameTable(got); err != nil {
				t.Errorf("the font cannot carry %q: %v", got, err)
			}
		})
	}
}

// The result must always be a name a font program can carry, and must leave
// room for a subset tag: the tagged name goes into the font program as well as
// into the dictionaries describing it.
func TestFontNameFits(t *testing.T) {
	budgets := []struct {
		maxLen   int
		canCarry func(string) error
	}{
		{nameTableBudget, nameTable},
		{subset.MaxBaseNameLen, type1.CheckFontName},
	}

	for _, tc := range []struct{ psName, familyName, subfam string }{
		{strings.Repeat("x", 500), "", ""},
		{"ABCDEF+" + strings.Repeat("x", 500), "", ""},
		{"", strings.Repeat("Family Name ", 50), strings.Repeat("Bold ", 50)},
		{"", "", ""},
		{"", "宋体", "粗体"},
	} {
		for _, b := range budgets {
			got := FontName(tc.psName, tc.familyName, tc.subfam, b.maxLen, b.canCarry)
			checkFontName(t, got, b.maxLen, b.canCarry)
		}
	}
}

func FuzzFontName(f *testing.F) {
	f.Add("Quire-Regular", "Quire", "Regular")
	f.Add("", "Times New Roman", "Bold")
	f.Add("AAAAAA+Go-Regular", "", "")
	f.Add("", "宋体", "")
	f.Add("宋体-Regular", "Song Ti", "Regular")
	f.Add(strings.Repeat("x", 500), "", "")

	f.Fuzz(func(t *testing.T, psName, familyName, subfam string) {
		got := FontName(psName, familyName, subfam, nameTableBudget, nameTable)
		checkFontName(t, got, nameTableBudget, nameTable)
	})
}

// checkFontName verifies the promises FontName makes: the result names a font,
// the font can carry it, and it leaves room for a subset tag, since the tagged
// name goes into the font program as well as into the dictionaries describing
// it.
func checkFontName(t *testing.T, name string, maxLen int, canCarry func(string) error) {
	t.Helper()

	if name == "" {
		t.Fatal("the font was left unnamed")
	}
	// the caller strips whatever tag the name carries and prefixes the one
	// describing the subset it is about to write
	_, base := subset.Split(name)
	if len(base) > maxLen {
		t.Errorf("the untagged name of %q is %d bytes, want at most %d",
			name, len(base), maxLen)
	}
	if err := canCarry(subset.Join("AAAAAA", base)); err != nil {
		t.Errorf("the font cannot carry a subset named after %q: %v", name, err)
	}
}
