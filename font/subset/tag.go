// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2023  Jochen Voss <voss@seehuhn.de>
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

// Package subset supports embedding a subset of a font: the tagged names which
// tell one subset from another, and the glyph mapping which describes it.
package subset

import (
	"regexp"
	"unicode/utf8"

	"seehuhn.de/go/postscript/type1"

	"seehuhn.de/go/sfnt/glyph"
)

const subsetModulus = 26 * 26 * 26 * 26 * 26 * 26

// Tag constructs a 6-letter tag (range AAAAAA to ZZZZZZ) to describe a subset
// of glyphs of a font.  This is used for the /BaseFont entry in PDF Font
// dictionaries and the /FontName entry in FontDescriptor dictionaries.
//
// If origGid contains all glyphs in the correct order, the empty string is
// returned.
func Tag(origGid []glyph.ID, origNumGlyphs int) string {
	if len(origGid) == origNumGlyphs {
		// If all glyphs are included in order, we don't need a subset
		needTag := false
		for i, gid := range origGid {
			if glyph.ID(i) != gid {
				needTag = true
				break
			}
		}
		if !needTag {
			return ""
		}
	}

	// mix all the information into a single uint32
	X := uint32(origNumGlyphs)
	for _, gid := range origGid {
		// 11 is the largest integer smaller than `1<<32 / subsetModulus` which
		// is relatively prime to 26.
		X = (X*11 + uint32(gid)) % subsetModulus
	}

	// convert to a string of six capital letters
	var buf [6]byte
	for i := range buf {
		buf[i] = 'A' + byte(X%26)
		X /= 26
	}
	return string(buf[:])
}

// IsValidTag reports whether s is a subset tag, that is a string of exactly
// six upper-case letters.
func IsValidTag(s string) bool {
	if len(s) != 6 {
		return false
	}

	for _, char := range s {
		if char < 'A' || char > 'Z' {
			return false
		}
	}

	return true
}

// tagRegexp matches the PostScript name of a font subset, splitting it into
// the tag and the name of the font the subset was made from.
var tagRegexp = regexp.MustCompile(`^([A-Z]{6})\+(.*)$`)

// Retag returns the tag for a font subset which is about to be embedded.
//
// computed is the tag for the glyphs being embedded, as returned by [Tag],
// and inherited is the tag the font program at hand already carried.  A
// program taken out of a PDF file is usually a subset already, so keeping
// every one of its glyphs does not make the result the complete font: it is
// the same subset as before and keeps its tag rather than claiming to be the
// whole typeface.
func Retag(computed, inherited string) string {
	if computed != "" {
		return computed
	}
	return inherited
}

// Split separates a PostScript name into its subset tag and the name of the
// font the subset was made from.  A name with no tag is returned with an empty
// tag.
//
// A font program taken out of a PDF file carries the tagged name the file gave
// it.  The tag describes that particular subset, not the font, so a caller
// which embeds the program again must start from the untagged name: a new tag
// prefixed to a tagged name would name no font at all.
//
// The name is returned as it stands.  Making room for a new tag is [ClipName],
// which needs to know how much the font at hand can carry.
func Split(name string) (tag, base string) {
	if m := tagRegexp.FindStringSubmatch(name); m != nil {
		return m[1], m[2]
	}
	return "", name
}

// Join builds the PostScript name of a font subset from the tag and the name
// of the font the subset was made from.  An empty tag means the font is not a
// subset, and name is returned unchanged.
//
// Join is the inverse of [Split] for any name [Split] returns.
func Join(tag, name string) string {
	if tag == "" {
		return name
	}
	return tag + "+" + name
}

const (
	// MaxNameLen is the greatest number of bytes a PostScript font name may
	// occupy in a PDF file.  A font file cannot carry a longer name, so
	// neither can the dictionaries describing it.
	MaxNameLen = type1.MaxFontNameLen

	// TagLen is the length a subset tag adds to a name: six letters and the
	// "+" separating them from the name of the font.
	TagLen = 7

	// MaxBaseNameLen is the greatest number of bytes the name of a font may
	// occupy if a subset of that font is still to be nameable.  This is the
	// most any font can carry; a font whose name must go into a "name" table
	// carries less, and [sfnt.Font.MaxFontNameLen] gives its limit.
	MaxBaseNameLen = MaxNameLen - TagLen
)

// ClipName shortens a PostScript font name to at most maxLen bytes, so that a
// subset made from the font can still be named.
//
// A subset is named by prefixing a tag to the name of the font it was made
// from, and both the font program and the dictionaries describing it carry
// that tagged name.  A name which leaves no room for the tag would therefore
// name the font in a file it cannot be embedded in, so it is clipped here
// instead, where the untagged name is settled and every use of it still agrees.
// The prefix which survives still tells the font apart from most others; names
// long enough for this to happen do not occur in practice.
//
// Clipping a name the font itself gave is not the same as clipping one this
// library made up, since a shortened name stands for a different font.  It is
// done anyway because the alternative is a font which cannot be embedded at
// all, and because such names do not occur in practice.
//
// Whole characters are kept: a font name may be written in UTF-8, so cutting at
// an arbitrary byte could leave a partial character behind.
//
// A maxLen below zero leaves room for nothing and yields the empty string.
func ClipName(name string, maxLen int) string {
	maxLen = max(maxLen, 0)
	if len(name) <= maxLen {
		return name
	}
	clipped := name[:maxLen]

	// The cut may have landed inside a character.  A character is at most
	// utf8.UTFMax bytes long, so dropping fewer bytes than that must expose a
	// whole one if the name is UTF-8 at all.  Where it does not, the name is in
	// some other encoding and there is no character to preserve, so the plain
	// cut stands rather than bytes being dropped to no purpose.  Never more
	// bytes are dropped than there are, which for a short name is the tighter
	// of the two bounds.
	maxDrop := min(utf8.UTFMax-1, len(clipped))
	for drop := 0; drop <= maxDrop; drop++ {
		candidate := clipped[:len(clipped)-drop]
		if endsWithWholeChar(candidate) {
			return candidate
		}
	}
	return clipped
}

// endsWithWholeChar reports whether s ends with a complete UTF-8 character.
// [utf8.DecodeLastRuneInString] reports a partial one as RuneError with a size
// of one byte, which a correctly encoded U+FFFD does not match.
func endsWithWholeChar(s string) bool {
	r, size := utf8.DecodeLastRuneInString(s)
	return r != utf8.RuneError || size != 1
}

// CleanName turns a font name read from a PDF file into one which can be
// written back out, by removing the characters a PostScript name may not
// contain and clipping the result as by [ClipName].  The result is empty if
// nothing usable is left.
//
// Only the characters a font name cannot hold are removed: white space, the
// PostScript delimiters, and the characters which cannot be shown.  A
// /BaseFont entry written in UTF-8 survives, which is what a CJK font is
// commonly named by: the name is repaired only so far as a font program cannot
// carry it.
func CleanName(name string) string {
	// Clipping comes first: RepairFontName drops a name which is still too long,
	// which is right for a font file but not here, where a shortened name is
	// better than none.  Repairing can only make a name shorter, so the result
	// still fits.
	return type1.RepairFontName(ClipName(name, MaxBaseNameLen))
}

// TagFontInfo returns font information which names the font the way the
// dictionary describing it does.  The PostScript name of a Type 1 or CFF font
// is the FontName of its font program, so an embedded program must call itself
// by the name the /BaseFont and /FontName entries use, tag and all.
//
// The name is written even where tag is empty.  A program need not be named
// for the font it is embedded as: the CFF table inside an OpenType font does
// not see the name the wrapper carries, and it is the wrapper the dictionary
// describes.
//
// The result is a copy: font data is commonly shared between instances, so
// renaming the original would rename the font everywhere it is used.
func TagFontInfo(info *type1.FontInfo, tag, name string) *type1.FontInfo {
	renamed := *info
	renamed.FontName = Join(tag, name)
	return &renamed
}
