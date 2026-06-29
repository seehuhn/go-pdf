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

package font

import (
	"fmt"
	"strings"

	"seehuhn.de/go/postscript/type1/names"

	"seehuhn.de/go/pdf/font/pdfenc"
)

// Predefined character sets for use with [IncludeGlyphs].
const (
	// GlyphsDigits contains the decimal digits 0 to 9.
	GlyphsDigits = "0123456789"

	// GlyphsLetters contains the basic Latin letters, A to Z and a to z.
	GlyphsLetters = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
)

// Predefined character sets for use with [IncludeGlyphs].
var (
	// GlyphsASCII contains the printable ASCII characters, U+0020 to U+007E.
	GlyphsASCII string

	// GlyphsLatin1 contains the printable ISO 8859-1 (Latin-1) characters:
	// the ASCII range plus U+00A0 to U+00FF.
	GlyphsLatin1 string

	// GlyphsWinAnsi contains the printable characters of the PDF
	// WinAnsiEncoding (Windows-1252).
	GlyphsWinAnsi string
)

func init() {
	var ascii, latin1, win strings.Builder
	for r := rune(0x20); r <= 0x7E; r++ {
		ascii.WriteRune(r)
		latin1.WriteRune(r)
	}
	for r := rune(0xA0); r <= 0xFF; r++ {
		latin1.WriteRune(r)
	}
	GlyphsASCII = ascii.String()
	GlyphsLatin1 = latin1.String()

	// derive WinAnsi from the canonical encoding table, deduplicating the
	// runes (0xA0/0xAD alias space/hyphen)
	seen := make(map[rune]bool)
	for code := 0x20; code < 256; code++ {
		name := pdfenc.WinAnsi.Encoding[code]
		if name == "" || name == ".notdef" {
			continue
		}
		for _, r := range names.ToUnicode(name, "") {
			if !seen[r] {
				seen[r] = true
				win.WriteRune(r)
			}
		}
	}
	GlyphsWinAnsi = win.String()
}

// GlyphsError reports runes that [IncludeGlyphs] could not register.
type GlyphsError struct {
	// Missing lists runes for which the font has no glyph, or no glyph for
	// one of the rune's components.
	Missing []rune

	// Overflow lists runes whose glyph exists but did not fit the font's
	// character code space.
	Overflow []rune
}

func (e *GlyphsError) Error() string {
	return fmt.Sprintf("font: %d glyphs missing, %d over capacity",
		len(e.Missing), len(e.Overflow))
}

// IncludeGlyphs registers the glyphs for every rune in s as used by the font
// instance F.  This ensures the glyphs are included in the embedded font
// subset even if they are never drawn, for example so that a form field can
// display values entered after the document was written.
//
// Each rune is processed individually, so ligatures spanning multiple runes
// are not formed.
//
// If some glyphs could not be registered, the returned error is a
// [*GlyphsError] listing the affected runes; all other glyphs are still
// registered.  A rune may fail because the font has no glyph for it or one
// of its components, or because the font's character code space is full (only
// simple fonts, which are limited to 256 codes).
func IncludeGlyphs(F Layouter, s string) error {
	seq := &GlyphSeq{}
	var missing, overflow []rune
	for _, r := range s {
		seq.Reset()
		F.Layout(seq, 1, string(r))
		var anyGlyph, anyMissing, anyOverflow bool
		for _, g := range seq.Seq {
			if g.GID == 0 { // .notdef
				anyMissing = true
				continue
			}
			anyGlyph = true
			if _, ok := F.Encode(g.GID, g.Text); !ok {
				anyOverflow = true
			}
		}
		switch {
		case anyMissing || !anyGlyph:
			missing = append(missing, r)
		case anyOverflow:
			overflow = append(overflow, r)
		}
	}
	if len(missing) > 0 || len(overflow) > 0 {
		return &GlyphsError{Missing: missing, Overflow: overflow}
	}
	return nil
}
