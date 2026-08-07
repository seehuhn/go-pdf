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

// Package fontname settles what a PDF file calls a font.
//
// A font dictionary must name the font it describes, but a font program need
// not name itself: a "name" table is optional, and a program taken out of a
// PDF file often has none.  Naming such a font is a PDF matter rather than a
// font matter, so it happens here rather than in the font packages, which
// report only what a font file says.
package fontname

import (
	"strings"
	"unicode"

	"seehuhn.de/go/pdf/font/subset"
	"seehuhn.de/go/postscript/type1"
	"seehuhn.de/go/sfnt"
	"seehuhn.de/go/sfnt/cff"
)

// placeholder names a font which gives nothing to name it by.  A PDF file must
// name every font it describes, so some name has to be chosen; this one says
// as little as the font does.
const placeholder = "Font"

// FontName returns the PostScript name a PDF file uses for a font.
//
// Where the font program names itself, that name is used.  Otherwise a name is
// derived from the family and subfamily names, which is what the font is known
// by in the systems it is installed on, and where the font gives neither, a
// placeholder is used.
//
// maxLen is the greatest number of bytes the name of the font may occupy, not
// counting a subset tag.  A tag which psName already carries is kept, since it
// names the subset the caller started from.
//
// canCarry reports whether the font at hand can store a given name.  The two
// places a name is kept differ in what they hold: a CFF Name INDEX takes any
// PostScript font name, whereas a "name" table takes only a subset of ASCII, so
// a name which reaches one need not reach the other.  A name the font refuses
// counts as one the font never gave, and a name is derived below instead.
//
// The result is never empty and is always a name the font can carry.
func FontName(psName, familyName, subfamily string, maxLen int, canCarry func(string) error) string {
	tag, base := subset.Split(psName)

	// The name is repaired rather than trusted: it may come from a place with
	// rules of its own, such as the /BaseFont entry of a PDF file or a CFF Name
	// INDEX, which holds whatever bytes it was given.  Clipping comes first,
	// since repairing drops a name which is still too long, and here a
	// shortened name is better than none.
	base = type1.RepairFontName(subset.ClipName(base, maxLen))

	// Removing the characters this font cannot hold would leave a fragment
	// standing for a different font, so the name is given up whole and one is
	// derived from the family instead.  A derived name is ASCII, which every
	// font can carry.
	if canCarry(base) != nil {
		base = ""
	}

	if base == "" {
		base = subset.ClipName(derive(familyName, subfamily), maxLen)
	}
	if base == "" {
		base = placeholder
	}
	return subset.Join(tag, base)
}

// ForSFNT returns the PostScript name a PDF file uses for a TrueType or
// OpenType font.
func ForSFNT(f *sfnt.Font) string {
	return FontName(f.FontName, f.FamilyName, f.Subfamily,
		f.MaxFontNameLen()-subset.TagLen, f.CheckFontName)
}

// ForCFF returns the PostScript name a PDF file uses for a bare CFF font.
// Such a font keeps its name in the CFF Name INDEX, which takes any PostScript
// font name, so the widest limit applies.
func ForCFF(f *cff.Font) string {
	return FontName(f.FontName, f.FamilyName, "", subset.MaxBaseNameLen,
		type1.CheckFontName)
}

// ForType1 returns the PostScript name a PDF file uses for a Type 1 font.
// Such a font keeps its name in its /FontName entry, which takes any
// PostScript font name, so the widest limit applies.
func ForType1(f *type1.Font) string {
	return FontName(f.FontName, f.FamilyName, "", subset.MaxBaseNameLen,
		type1.CheckFontName)
}

// derive builds a PostScript name from the family and subfamily names of a
// font which does not name itself.  The result is empty where no usable name
// can be built.
//
// A derived name is this library's own invention rather than something the
// font stated, so it is kept to ASCII: every font program can carry such a
// name, whereas only some can carry more, and nothing is lost by not relying
// on the difference.  A family name which needs more than ASCII yields no name
// at all, since dropping its other characters would leave a fragment standing
// for a different font.
func derive(familyName, subfamily string) string {
	family := asciiName(familyName)
	if family == "" {
		return ""
	}
	if style := asciiName(subfamily); style != "" {
		return family + "-" + style
	}
	return family
}

// asciiName reduces s to the characters a PostScript name may hold in a "name"
// table: the printable ASCII characters, less the space and the ten PostScript
// delimiters.  It returns the empty string if s holds any character outside
// ASCII.
func asciiName(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r > unicode.MaxASCII {
			return ""
		}
		if r > ' ' && r < unicode.MaxASCII && !strings.ContainsRune("()<>[]{}/%", r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}
