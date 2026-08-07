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

package extract

import (
	"strings"
	"testing"

	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/subset"
)

func TestResolveFontName(t *testing.T) {
	for _, tc := range []struct {
		label    string
		psName   string
		tag      string
		fontName string
		embedded bool

		wantPSName string
		wantTag    string
	}{
		{
			label:      "BaseFont and FontName agree",
			psName:     "Toaster",
			tag:        "AAAAAA",
			fontName:   "AAAAAA+Toaster",
			embedded:   true,
			wantPSName: "Toaster",
			wantTag:    "AAAAAA",
		},
		{
			label:      "BaseFont missing, FontName carries a tag",
			fontName:   "AAAAAA+Toaster",
			embedded:   true,
			wantPSName: "Toaster",
			wantTag:    "AAAAAA",
		},
		{
			label:      "BaseFont missing, FontName carries no tag",
			fontName:   "Toaster",
			embedded:   true,
			wantPSName: "Toaster",
		},
		{
			// The names in the file disagree.  BaseFont is the entry which
			// names the font, so the descriptor only fills in what it left out.
			label:      "BaseFont wins over FontName",
			psName:     "Toaster",
			tag:        "AAAAAA",
			fontName:   "BBBBBB+Trickster",
			embedded:   true,
			wantPSName: "Toaster",
			wantTag:    "AAAAAA",
		},
		{
			// A font must have a name, so one is invented where the file gives
			// none.
			label:      "no name anywhere",
			embedded:   true,
			wantPSName: "Font",
		},
		{
			label:      "invalid tag",
			psName:     "Toaster",
			tag:        "nope",
			fontName:   "Toaster",
			embedded:   true,
			wantPSName: "Toaster",
		},
		{
			// An external font is the whole typeface, so it must not claim to
			// be a subset: the tag would make the name match no font on the
			// system, and this library would refuse to write it back out.
			label:      "external font drops the tag",
			psName:     "Toaster",
			tag:        "AAAAAA",
			fontName:   "AAAAAA+Toaster",
			wantPSName: "Toaster",
		},
		{
			// The tag the descriptor supplies is dropped as well, which is why
			// the two are reconciled before the tag is cleared.
			label:      "external font drops a tag from FontName",
			fontName:   "AAAAAA+Toaster",
			wantPSName: "Toaster",
		},
	} {
		t.Run(tc.label, func(t *testing.T) {
			desc := &font.Descriptor{FontName: tc.fontName}

			psName, tag := resolveFontName(desc, tc.psName, tc.tag, tc.embedded)

			if psName != tc.wantPSName {
				t.Errorf("PostScript name %q, want %q", psName, tc.wantPSName)
			}
			if tag != tc.wantTag {
				t.Errorf("subset tag %q, want %q", tag, tc.wantTag)
			}
			want := tc.wantPSName
			if tc.wantTag != "" {
				want = tc.wantTag + "+" + tc.wantPSName
			}
			if desc.FontName != want {
				t.Errorf("the descriptor names the font %q, want %q", desc.FontName, want)
			}
		})
	}
}

// A file may name a font in a way a PostScript name cannot be written, a
// non-ASCII name being the common case, or in a name too long to leave room
// for a subset tag.  A tool which rewrites the file embeds a font program
// under the name settled here, so the name is repaired on reading.
func TestResolveFontNameRepairsUnwritableName(t *testing.T) {
	for _, tc := range []struct {
		label      string
		baseFont   string
		wantPSName string
	}{
		{"non-ASCII name is kept", "宋体", "宋体"},
		{"name with a space", "Times New Roman", "TimesNewRoman"},
		{"name with delimiters", "Foo(Bar)", "FooBar"},
		{
			"name too long for a tag",
			strings.Repeat("x", 500),
			strings.Repeat("x", subset.MaxBaseNameLen),
		},
	} {
		t.Run(tc.label, func(t *testing.T) {
			desc := &font.Descriptor{FontName: tc.baseFont}

			psName, tag := resolveFontName(desc, tc.baseFont, "", true)

			if psName != tc.wantPSName {
				t.Errorf("PostScript name %q, want %q", psName, tc.wantPSName)
			}
			if len(subset.Join("AAAAAA", psName)) > subset.MaxNameLen {
				t.Errorf("a subset of %q cannot be named", psName)
			}
			if got := subset.CleanName(psName); got != psName {
				t.Errorf("the name %q cannot be written to a font file", psName)
			}
			if want := subset.Join(tag, psName); desc.FontName != want {
				t.Errorf("the descriptor names the font %q, want %q", desc.FontName, want)
			}
		})
	}
}
