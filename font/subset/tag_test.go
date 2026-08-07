// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2021  Jochen Voss <voss@seehuhn.de>
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

package subset

import (
	"strings"
	"testing"
	"unicode/utf8"

	"seehuhn.de/go/postscript/type1"
)

func TestIsValidTag(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"Valid tag", "ABCDEF", true},
		{"Valid tag with different letters", "XYZPQR", true},
		{"Empty string", "", false},
		{"Too short", "ABCDE", false},
		{"Too long", "ABCDEFG", false},
		{"Lowercase letters", "abcdef", false},
		{"Mixed case", "ABCDEf", false},
		{"Numbers", "A1CDEF", false},
		{"Special characters", "A@CDEF", false},
		{"Unicode characters", "АBCDEF", false}, // first letter cyrillic
		{"Spaces", "ABC EF", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsValidTag(tt.input); got != tt.want {
				t.Errorf("IsValidTag(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

// TagFontInfo must leave the font information it is given alone: font data is
// commonly shared between instances, so renaming it in place would rename the
// font in every document which uses it.
func TestTagFontInfoCopies(t *testing.T) {
	info := &type1.FontInfo{FontName: "Go-Regular"}

	subsetInfo := TagFontInfo(info, "AAAAAA", "Go-Regular")

	if info.FontName != "Go-Regular" {
		t.Errorf("the original was renamed to %q", info.FontName)
	}
	if subsetInfo == info {
		t.Error("the subset shares its font information with the original")
	}
	if got, want := subsetInfo.FontName, "AAAAAA+Go-Regular"; got != want {
		t.Errorf("the subset is named %q, want %q", got, want)
	}
}

// A font which is not a subset is still named by the dictionary describing it.
// The program need not have been named for the font it is embedded as: the CFF
// table inside an OpenType font carries a name of its own, which the wrapper
// the dictionary describes may not share.
func TestTagFontInfoWithoutTag(t *testing.T) {
	info := &type1.FontInfo{FontName: "Go-Italic"}

	renamed := TagFontInfo(info, "", "Go-Regular")

	if got, want := renamed.FontName, "Go-Regular"; got != want {
		t.Errorf("the program is named %q, want %q", got, want)
	}
	if info.FontName != "Go-Italic" {
		t.Errorf("the original was renamed to %q", info.FontName)
	}
}

func TestSplit(t *testing.T) {
	for _, tc := range []struct{ in, tag, base string }{
		{"AAAAAA+Go-Regular", "AAAAAA", "Go-Regular"},
		{"Go-Regular", "", "Go-Regular"},
		{"", "", ""},
		{"aaaaaa+Go-Regular", "", "aaaaaa+Go-Regular"}, // tags are uppercase
		{"AAAAA+Go-Regular", "", "AAAAA+Go-Regular"},   // tags are six letters
	} {
		tag, base := Split(tc.in)
		if tag != tc.tag || base != tc.base {
			t.Errorf("Split(%q) = %q, %q; want %q, %q", tc.in, tag, base, tc.tag, tc.base)
		}
	}
}

// A font program taken out of a PDF file is usually a subset already, so
// embedding every one of its glyphs leaves it the same subset it was.
func TestRetag(t *testing.T) {
	if got := Retag("NEWTAG", "OLDTAG"); got != "NEWTAG" {
		t.Errorf("a fresh subset used the tag %q, want %q", got, "NEWTAG")
	}
	if got := Retag("", "OLDTAG"); got != "OLDTAG" {
		t.Errorf("an unchanged subset used the tag %q, want %q", got, "OLDTAG")
	}
	if got := Retag("", ""); got != "" {
		t.Errorf("a complete font was given the tag %q", got)
	}
}

// A subset is named by prefixing a tag to the name of the font it was made
// from, and a font file cannot carry a name longer than MaxNameLen.  A name
// which leaves no room for the tag is therefore clipped where it is settled,
// so that the dictionaries and the font program still agree.
func TestClipName(t *testing.T) {
	short := strings.Repeat("x", MaxBaseNameLen)
	if got := ClipName(short, MaxBaseNameLen); got != short {
		t.Errorf("a name of %d bytes was clipped to %d", len(short), len(got))
	}

	long := strings.Repeat("x", MaxNameLen)
	got := ClipName(long, MaxBaseNameLen)
	if len(Join("AAAAAA", got)) > MaxNameLen {
		t.Errorf("a subset of the clipped name is %d bytes long, want at most %d",
			len(Join("AAAAAA", got)), MaxNameLen)
	}
	if !strings.HasPrefix(long, got) {
		t.Errorf("the clipped name %q is not a prefix of the original", got)
	}
}

// Split only separates the tag from the name.  How much of the name a font can
// carry depends on where the font keeps it, which Split cannot know, so making
// room for a tag is left to the caller.
func TestSplitLeavesTheNameAlone(t *testing.T) {
	long := strings.Repeat("x", 500)
	for _, tc := range []struct{ in, tag, base string }{
		{long, "", long},
		{"AAAAAA+" + long, "AAAAAA", long},
		{"Go-Regular", "", "Go-Regular"},
		{"AAAAAA+Go-Regular", "AAAAAA", "Go-Regular"},
	} {
		tag, base := Split(tc.in)
		if tag != tc.tag || base != tc.base {
			t.Errorf("Split(%d bytes) = %q, %q, want %q, %q",
				len(tc.in), tag, base, tc.tag, tc.base)
		}
	}
}

// A PDF file may name a font in a way a font program cannot, non-ASCII names
// being the common case.  Such a name is repaired on reading, so that a tool
// which rewrites the file can embed a program under it.
func TestCleanName(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"Go-Regular", "Go-Regular"},
		{"Times New Roman", "TimesNewRoman"},
		{"Foo(Bar)", "FooBar"},
		// a name written in UTF-8 survives: it is what CJK fonts are commonly
		// named by, and a font program can carry it
		{"宋体", "宋体"},
		{strings.Repeat("x", 500), strings.Repeat("x", MaxBaseNameLen)},
	} {
		if got := CleanName(tc.in); got != tc.want {
			t.Errorf("CleanName(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// The whole point of CleanName is that the result can be written to a font
// file, so it is checked against the rule the font file layer applies rather
// than against a copy of it here.  A subset of the font must be nameable too,
// which is what the tag stands for.
func FuzzCleanName(f *testing.F) {
	f.Add("Go-Regular")
	f.Add("AAAAAA+Go-Regular")
	f.Add("Times New Roman")
	f.Add("Foo(Bar)")
	f.Add("宋体")
	f.Add(strings.Repeat("x", 500))

	f.Fuzz(func(t *testing.T, name string) {
		clean := CleanName(name)
		if err := type1.CheckFontName(Join("AAAAAA", clean)); err != nil {
			t.Errorf("a subset of CleanName(%q) = %q cannot be named: %v",
				name, clean, err)
		}
	})
}

// A font name may be written in UTF-8, so clipping must cut between characters:
// a name ending in half a character names nothing, and the font file would
// carry that half character into the world.
func TestClipNameKeepsWholeCharacters(t *testing.T) {
	// each "宋" is three bytes, so the limit falls inside a character for at
	// least one of these lengths
	for n := range 4 {
		name := strings.Repeat("x", n) + strings.Repeat("宋", MaxBaseNameLen)

		got := ClipName(name, MaxBaseNameLen)

		if len(got) > MaxBaseNameLen {
			t.Errorf("n=%d: the clipped name is %d bytes, want at most %d",
				n, len(got), MaxBaseNameLen)
		}
		if !utf8.ValidString(got) {
			t.Errorf("n=%d: clipping left a partial character: %q", n, got)
		}
		if !strings.HasPrefix(name, got) {
			t.Errorf("n=%d: the clipped name is not a prefix of the original", n)
		}
	}
}

// A name which is not UTF-8 has no character to preserve at the cut, so the
// plain cut stands rather than bytes being dropped to no purpose.
func TestClipNameKeepsNonUTF8(t *testing.T) {
	name := strings.Repeat("\xff", MaxBaseNameLen+10)

	got := ClipName(name, MaxBaseNameLen)

	if want := strings.Repeat("\xff", MaxBaseNameLen); got != want {
		t.Errorf("the clipped name is %d bytes, want %d", len(got), len(want))
	}
}

// A budget of a few bytes leaves less room than a character needs, and one
// below zero leaves room for nothing at all.  Both are answered with a name,
// short or empty, rather than with a panic.
func TestClipNameTinyBudget(t *testing.T) {
	for _, name := range []string{"", "x", "xx", "宋体", "\xff\xff", "x宋"} {
		for maxLen := -2; maxLen <= 5; maxLen++ {
			got := ClipName(name, maxLen)

			if len(got) > max(maxLen, 0) {
				t.Errorf("ClipName(%q, %d) = %q, which is %d bytes",
					name, maxLen, got, len(got))
			}
			if !strings.HasPrefix(name, got) {
				t.Errorf("ClipName(%q, %d) = %q, which the name does not start with",
					name, maxLen, got)
			}
			if utf8.ValidString(name) && !utf8.ValidString(got) {
				t.Errorf("ClipName(%q, %d) = %q, which is a partial character",
					name, maxLen, got)
			}
		}
	}
}
