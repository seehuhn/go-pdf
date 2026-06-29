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

package font_test

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/gofont"
	"seehuhn.de/go/pdf/font/standard"
	"seehuhn.de/go/pdf/internal/fonttypes"
)

// registering an ASCII repertoire decreases the available code space and is
// idempotent on a second call.
func TestIncludeGlyphsRegisters(t *testing.T) {
	F := font.Must(gofont.Regular.NewSimple(nil))

	before := F.CodesRemaining()
	if err := font.IncludeGlyphs(F, font.GlyphsASCII); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	mid := F.CodesRemaining()
	if mid >= before {
		t.Errorf("no glyphs registered: %d -> %d", before, mid)
	}

	if err := font.IncludeGlyphs(F, font.GlyphsASCII); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if after := F.CodesRemaining(); after != mid {
		t.Errorf("not idempotent: %d -> %d", mid, after)
	}
}

// registering ASCII works for every font type and never overflows (95 < 256).
func TestIncludeGlyphsAllFonts(t *testing.T) {
	for _, sample := range fonttypes.All {
		t.Run(sample.Label, func(t *testing.T) {
			F := sample.MakeFont()

			err := font.IncludeGlyphs(F, font.GlyphsASCII)
			if err != nil {
				var ge *font.GlyphsError
				if !errors.As(err, &ge) {
					t.Fatalf("unexpected error type: %v", err)
				}
				if len(ge.Overflow) > 0 {
					t.Errorf("ASCII overflowed: %q", string(ge.Overflow))
				}
			}

			before := F.CodesRemaining()
			if err := font.IncludeGlyphs(F, font.GlyphsASCII); err != nil {
				var ge *font.GlyphsError
				if !errors.As(err, &ge) || len(ge.Overflow) > 0 {
					t.Fatalf("unexpected error: %v", err)
				}
			}
			if after := F.CodesRemaining(); after != before {
				t.Errorf("not idempotent: %d -> %d", before, after)
			}
		})
	}
}

// a simple font reports an overflow once its 256-code space is full.
func TestIncludeGlyphsOverflow(t *testing.T) {
	F := font.Must(gofont.Regular.NewSimple(nil))

	var sb strings.Builder
	sb.WriteString(font.GlyphsLatin1)
	for r := rune(0x0391); r <= 0x03C9; r++ { // Greek
		sb.WriteRune(r)
	}
	for r := rune(0x0410); r <= 0x044F; r++ { // Cyrillic
		sb.WriteRune(r)
	}

	err := font.IncludeGlyphs(F, sb.String())
	var ge *font.GlyphsError
	if !errors.As(err, &ge) {
		t.Fatalf("want *GlyphsError, got %v", err)
	}
	if len(ge.Overflow) == 0 {
		t.Errorf("no overflow reported")
	}
	if rem := F.CodesRemaining(); rem != 0 {
		t.Errorf("code space not full: %d remaining", rem)
	}
}

// a rune absent from the font is reported as missing, not overflow.
func TestIncludeGlyphsMissing(t *testing.T) {
	F := font.Must(gofont.Regular.NewSimple(nil))

	err := font.IncludeGlyphs(F, "中") // CJK, not in the Go font
	var ge *font.GlyphsError
	if !errors.As(err, &ge) {
		t.Fatalf("want *GlyphsError, got %v", err)
	}
	if !slices.Contains(ge.Missing, '中') {
		t.Errorf("missing rune not reported: %q", string(ge.Missing))
	}
	if len(ge.Overflow) > 0 {
		t.Errorf("unexpected overflow: %q", string(ge.Overflow))
	}
}

// the standard-14 Helvetica covers all of printable ASCII.
func TestIncludeGlyphsStandardASCII(t *testing.T) {
	F := font.Must(standard.Helvetica.New())
	if err := font.IncludeGlyphs(F, font.GlyphsASCII); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestGlyphSets(t *testing.T) {
	if font.GlyphsDigits != "0123456789" {
		t.Errorf("GlyphsDigits = %q", font.GlyphsDigits)
	}
	if n := len([]rune(font.GlyphsLetters)); n != 52 {
		t.Errorf("GlyphsLetters has %d runes, want 52", n)
	}

	asciiRunes := []rune(font.GlyphsASCII)
	if asciiRunes[0] != ' ' || asciiRunes[len(asciiRunes)-1] != '~' {
		t.Errorf("GlyphsASCII = %q...%q", asciiRunes[0], asciiRunes[len(asciiRunes)-1])
	}

	for _, r := range font.GlyphsLatin1 {
		if r < 0x20 {
			t.Errorf("GlyphsLatin1 contains control rune %U", r)
		}
	}
	for r := rune(0xA0); r <= 0xFF; r++ {
		if !strings.ContainsRune(font.GlyphsLatin1, r) {
			t.Errorf("GlyphsLatin1 missing %U", r)
		}
	}

	for _, r := range []rune{'€', '•', '—', '™'} {
		if !strings.ContainsRune(font.GlyphsWinAnsi, r) {
			t.Errorf("GlyphsWinAnsi missing %q", r)
		}
	}
	winRunes := []rune(font.GlyphsWinAnsi)
	seen := make(map[rune]bool)
	for _, r := range winRunes {
		if seen[r] {
			t.Errorf("GlyphsWinAnsi has duplicate rune %q", r)
		}
		seen[r] = true
	}
}
