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

package pdf

import (
	"strings"
	"testing"
	"unicode/utf8"

	"golang.org/x/text/language"
)

func TestTextString_Get(t *testing.T) {
	tests := []struct {
		name    string
		input   Object
		want    TextString
		wantErr bool
	}{
		{
			name:  "PDFDocEncoded string",
			input: String("Hello, World!"),
			want:  Text("Hello, World!"),
		},
		{
			name:  "UTF-16BE string",
			input: String("\xFE\xFF\x00H\x00e\x00l\x00l\x00o"),
			want:  Text("Hello"),
		},
		{
			name:  "UTF-8 string",
			input: String("\xEF\xBB\xBFHello"),
			want:  Text("Hello"),
		},
		{
			name:  "Empty string",
			input: String(""),
		},
		{
			name:  "String with special characters",
			input: String("Line 1\nLine 2\tTabbed"),
			want:  Text("Line 1\nLine 2\tTabbed"),
		},
		{
			name:    "Invalid object type",
			input:   Integer(42),
			wantErr: true,
		},
		{
			name:  "Nil object",
			input: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewCursor(mockGetter).TextString(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("TextString() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("TextString() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTextString_AsPDF(t *testing.T) {
	tests := []struct {
		name string
		in   TextString
		opt  OutputOptions
		want Native
	}{
		{
			name: "ASCII string",
			in:   Text("hello"),
			opt:  0,
			want: String("hello"),
		},
		{
			name: "Non-PDFDocEncoding char",
			in:   Text("Ā"),
			opt:  0,
			want: String("\xfe\xff\x01\x00"),
		},
		{
			name: "Chinese characters",
			in:   Text("中文"),
			opt:  0,
			want: String("\xfe\xff\x4e\x2d\x65\x87"),
		},
		{
			name: "ASCII with UTF-8", // still encoded as PDFDocEncoding, though
			in:   Text("hello"),
			opt:  OptTextStringUtf8,
			want: String("hello"),
		},
		{
			name: "Non-PDFDocEncoding with UTF-8",
			in:   Text("Ā"),
			opt:  OptTextStringUtf8,
			want: String("\xef\xbb\xbfĀ"),
		},
		{
			name: "Chinese with UTF-8",
			in:   Text("中文"),
			opt:  OptTextStringUtf8,
			want: String("\xef\xbb\xbf中文"),
		},
		{
			name: "Empty string",
			in:   Text(""),
			opt:  0,
			want: String(""),
		},
		{
			name: "String with control characters",
			in:   Text("\x00\x01\x02"),
			opt:  0,
			want: String("\xfe\xff\x00\x00\x00\x01\x00\x02"),
		},
		{
			name: "language tag",
			in:   Text(language.German, "Hallo"),
			opt:  0,
			want: String("\xfe\xff\x00\x1b\x00d\x00e\x00\x1b\x00H\x00a\x00l\x00l\x00o"),
		},
		{
			name: "language tag with UTF-8",
			in:   Text(language.AmericanEnglish, "Hi"),
			opt:  OptTextStringUtf8,
			want: String("\xef\xbb\xbf\x1benUS\x1bHi"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.in.AsPDF(tt.opt)
			if !Equal(got, tt.want) {
				t.Errorf("AsPDF(%q, %v) = %v, want %v", tt.in, tt.opt, got, tt.want)
			}
		})
	}
}

func TestTextString_RoundTrip(t *testing.T) {
	tests := []struct {
		name string
		text TextString
	}{
		{"Empty string", Text("")},
		{"ASCII string", Text("hello")},
		{"String with umlauts", Text("ein Bär")},
		{"Romanian string", Text("o țesătură")},
		{"Chinese string", Text("中文")},
		{"Japanese string", Text("日本語")},
		{"Control characters", Text("\x00\x09\n\x0c\r")},
		{"Mixed ASCII and non-ASCII", Text("Hello, 世界!")},
		{"Emoji", Text("😀🌍🚀")},
		{"utf8 marker", Text(PDFDocDecode(utf8Marker))},
		{"utf16 marker", Text(PDFDocDecode(utf16Marker))},
		{"tagged", Text(language.German, "Hallo")},
		{"tagged with region", Text(language.BritishEnglish, "colour")},
		{"leading untagged run", Text("In German: ", language.German, "Hubschrauber")},
		{"several runs", Text(language.English, "a ", language.German, "b ", language.French, "c")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, opt := range []OutputOptions{0, OptTextStringUtf8} {
				enc := tt.text.AsPDF(opt).(String)
				out := enc.AsTextString()
				if out != tt.text {
					t.Errorf("round-trip failed for %q with option %v:\nencoded: % x\ndecoded: %q",
						tt.text, opt, enc, out)
				}
			}
		})
	}
}

func TestTextString_Decode(t *testing.T) {
	tests := []struct {
		name string
		in   String
		want TextString
	}{
		{
			name: "language escape",
			in:   String("\xfe\xff\x00\x1b\x00e\x00n\x00\x1b\x00H"),
			want: Text(language.English, "H"),
		},
		{
			name: "language and region",
			in:   String("\xef\xbb\xbf\x1bjaJP\x1b日本"),
			want: Text(language.Make("ja-JP"), "日本"),
		},
		{
			name: "case is normalised",
			in:   String("\xef\xbb\xbf\x1bDEde\x1bx"),
			want: Text(language.Make("de-DE"), "x"),
		},
		{
			name: "unknown language dropped",
			in:   String("\xef\xbb\xbf\x1bqq\x1bx"),
			want: Text("x"),
		},
		{
			name: "junk between escapes dropped",
			in:   String("\xef\xbb\xbfa\x1bjunk\x1bb"),
			want: Text("ab"),
		},
		{
			name: "unterminated escape drops to end of string",
			in:   String("\xef\xbb\xbfa\x1bb"),
			want: Text("a"),
		},
		{
			name: "escape with no text after it",
			in:   String("\xef\xbb\xbfa\x1bde\x1b"),
			want: Text("a"),
		},
		{
			name: "repeated language merges",
			in:   String("\xef\xbb\xbf\x1bde\x1ba\x1bde\x1bb"),
			want: Text(language.German, "ab"),
		},
		{
			name: "consecutive escapes, last wins",
			in:   String("\xef\xbb\xbf\x1bde\x1b\x1bfr\x1bx"),
			want: Text(language.French, "x"),
		},
		{
			name: "deprecated code kept as written",
			in:   String("\xef\xbb\xbf\x1btl\x1bx"),
			want: Text(language.Raw.Make("tl"), "x"),
		},
		{
			name: "code with alias kept as written",
			in:   String("\xef\xbb\xbf\x1bmo\x1bx"),
			want: Text(language.Raw.Make("mo"), "x"),
		},
		{
			name: "unknown region dropped, language kept",
			in:   String("\xef\xbb\xbf\x1benAB\x1bx"),
			want: Text(language.English, "x"),
		},
		{
			name: "invalid UTF-8 repaired",
			in:   String("\xef\xbb\xbfab\xff\xfecd"),
			want: Text("ab\uFFFDcd"),
		},
		{
			name: "unpaired surrogate repaired",
			in:   String("\xfe\xff\xd8\x00\x00a"),
			want: Text("\uFFFDa"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.in.AsTextString()
			if got != tt.want {
				t.Errorf("AsTextString(% x) = %q, want %q", tt.in, got, tt.want)
			}
			if !utf8.ValidString(got.s) {
				t.Errorf("decoded text is not valid UTF-8: %q", got.s)
			}
		})
	}
}

func TestText(t *testing.T) {
	de, en := language.German, language.English
	tests := []struct {
		name string
		a, b TextString
		same bool
	}{
		{"same language merges", Text(de, "a", de, "b"), Text(de, "ab"), true},
		{"consecutive strings concatenate", Text("a", "b"), Text("ab"), true},
		{"last of consecutive tags wins", Text(de, en, "x"), Text(en, "x"), true},
		{"tag differs from untagged", Text(en, "a"), Text("a"), false},
		{"tags differ", Text(en, "a"), Text(de, "a"), false},
		{"trailing tag without text", Text(de, ""), Text(), true},
		{"leading Und is no tag", Text(language.Und, "a"), Text("a"), true},
		{"script without language is no tag", Text(language.MustParse("und-Latn"), "a"), Text("a"), true},
		{"ESC removed", Text(de, "a\x1bb"), Text(de, "ab"), true},
		{"invalid UTF-8 repaired", Text("a\xffb"), Text("a\uFFFDb"), true},
		{"script dropped", Text(language.Make("de-Latn-CH"), "x"), Text(language.Make("de-CH"), "x"), true},
		{"zero value is empty", TextString{}, Text(""), true},
		{"splice plain", Text(Text("a"), "b"), Text("ab"), true},
		{"splice continues language", Text(de, "a", Text("b")), Text(de, "ab"), true},
		{"splice keeps tags", Text("a", Text(de, "b"), "c"), Text("a", de, "bc"), true},
		{"splice into language", Text(de, Text("a", en, "b")), Text(de, "a", en, "b"), true},
		{"splice same language is no-op marker", Text(de, "a", Text(de, "b")), Text(de, "ab"), true},
		{"splice zero value", Text(TextString{}, de, "a"), Text(de, "a"), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if (tt.a == tt.b) != tt.same {
				t.Errorf("%q == %q is %v, want %v", tt.a, tt.b, tt.a == tt.b, tt.same)
			}
		})
	}
}

func TestTextString_IsZero(t *testing.T) {
	if !Text().IsZero() || !(TextString{}).IsZero() {
		t.Error("empty string is not zero")
	}
	if Text("a").IsZero() || Text(language.German, "a").IsZero() {
		t.Error("non-empty string is zero")
	}
}

func TestTextString_String(t *testing.T) {
	s := Text("In German: ", language.German, "Hubschrauber", language.English, ".")
	if got, want := s.String(), "In German: Hubschrauber."; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
	if strings.IndexByte(s.s, textEsc) < 0 {
		t.Error("internal form has no language markers")
	}
}

func TestText_Panics(t *testing.T) {
	tests := []struct {
		name  string
		parts []any
	}{
		{"wrong type", []any{Name("x")}},
		{"no two-letter code", []any{language.Make("gsw"), "x"}},
		{"canonicalised code has no two-letter form", []any{language.Make("tl"), "x"}},
		{"Und after tag", []any{language.German, "a", language.Und, "b"}},
		{"Und after spliced tag", []any{Text(language.German, "a"), language.Und, "b"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Error("no panic")
				}
			}()
			Text(tt.parts...)
		})
	}
}

func FuzzTextString(f *testing.F) {
	f.Add([]byte("hello"))
	f.Add([]byte("\xfe\xff\x00\x1b\x00e\x00n\x00\x1b\x00H"))
	f.Add([]byte("\xef\xbb\xbfa\x1bde\x1bb\x1bjunk\x1bc\x1b"))
	f.Add([]byte("\xef\xbb\xbfab\xff\xfecd"))
	f.Add([]byte("\xef\xbb\xbf\x1btl\x1bx\x1bmo\x1by\x1benAB\x1bz"))
	f.Fuzz(func(t *testing.T, data []byte) {
		s1 := String(data).AsTextString()
		if !utf8.ValidString(s1.s) {
			t.Fatalf("decoded text is not valid UTF-8: %q", s1.s)
		}
		if strings.IndexByte(s1.String(), textEsc) >= 0 {
			t.Fatalf("plain text contains ESC: %q", s1.String())
		}
		for _, opt := range []OutputOptions{0, OptTextStringUtf8} {
			s2 := s1.AsPDF(opt).(String).AsTextString()
			if s2 != s1 {
				t.Errorf("round trip with option %v: %q -> %q", opt, s1, s2)
			}
		}
	})
}
