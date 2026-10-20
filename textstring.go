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
	"bytes"
	"fmt"
	"strings"
	"unicode/utf16"

	"golang.org/x/text/language"
)

// PDF 2.0 sections: 7.9.2.2

// TextString is a PDF text string: Unicode text in which runs of characters
// may be tagged with a language.  The zero value is the empty string.
//
// Values are comparable; two values are equal when they denote the same text
// with the same language tags.
//
// Use [Text] to construct a TextString and [TextString.String] to get back the
// plain text.
type TextString struct {
	// s holds the text in normal form: valid UTF-8 with no U+001B except
	// as part of a language marker "\x1b" + code + "\x1b", where code is a
	// two-letter language optionally followed by a two-letter region.  A
	// marker never names the language already in effect, and every
	// marker is followed by at least one character of text.
	s string
}

const textEsc = '\x1b'

// Text constructs a TextString from its arguments.  Each argument is a
// string, a TextString, or a [language.Tag]; a tag applies to all text that
// follows it, up to the next tag.  Any other argument type causes a panic.
// Text before the first tag is in the prevailing language of the document.
//
// Invalid UTF-8 in a string is replaced by U+FFFD, and U+001B characters are
// removed.  A TextString argument is spliced in: its text before the first
// tag takes the language in effect at that point, and its own tags apply
// from there on.
//
// Only the base language and, if given, a two-letter region of a tag are kept,
// since PDF text strings cannot hold more.  A tag whose base language has no
// two-letter ISO 639-1 code causes a panic.  Note that [language.Make] and
// [language.Parse] replace deprecated codes by their current forms (for
// example "tl" by "fil", which has no two-letter form); [language.Raw] keeps a
// tag as written.
//
// Use of [language.Und] or other tags without a base language after a language
// has already been set causes a panic.
func Text(parts ...any) TextString {
	var b textBuilder
	tagged := false
	for _, part := range parts {
		switch p := part.(type) {
		case string:
			p = strings.ToValidUTF8(p, "�")
			p = strings.ReplaceAll(p, "\x1b", "")
			b.write(p)
		case TextString:
			rest := p.s
			for {
				i := strings.IndexByte(rest, textEsc)
				if i < 0 {
					b.write(rest)
					break
				}
				b.write(rest[:i])
				rest = rest[i+1:]
				j := strings.IndexByte(rest, textEsc)
				b.pending = rest[:j]
				rest = rest[j+1:]
				tagged = true
			}
		case language.Tag:
			code := langCode(p)
			if code == "" {
				if tagged {
					panic("pdf.Text: language.Und after a language tag")
				}
			} else {
				tagged = true
			}
			b.pending = code
		default:
			panic(fmt.Sprintf("pdf.Text: unexpected argument of type %T", part))
		}
	}
	return b.result()
}

// textBuilder assembles the normal form of a TextString.
type textBuilder struct {
	buf     strings.Builder
	cur     string // language code in effect
	pending string // code requested for the next text, "" for none
}

// write appends text in the pending language.
func (b *textBuilder) write(text string) {
	if text == "" {
		return
	}
	if b.pending != b.cur {
		b.buf.WriteByte(textEsc)
		b.buf.WriteString(b.pending)
		b.buf.WriteByte(textEsc)
		b.cur = b.pending
	}
	b.buf.WriteString(text)
}

func (b *textBuilder) result() TextString {
	return TextString{s: b.buf.String()}
}

// langCode returns the marker code for tag, or "" if tag has no base
// language.  It panics if the base language has no two-letter code.
func langCode(tag language.Tag) string {
	if base, _, _ := tag.Raw(); base.String() == "und" {
		return ""
	}
	code, ok := reduceTag(tag)
	if !ok {
		panic(fmt.Sprintf("pdf.Text: language %q has no two-letter code", tag))
	}
	return code
}

// reduceTag returns the marker code for tag: the two-letter base language,
// followed by the two-letter region if the tag has one.
func reduceTag(tag language.Tag) (string, bool) {
	base, _, region := tag.Raw()
	code := base.String()
	if len(code) != 2 {
		return "", false
	}
	if r := region.String(); region != (language.Region{}) && len(r) == 2 {
		code += r
	}
	return code, true
}

// parseLangCode converts the content of an escape sequence read from a
// file into a marker code.  It returns "" if the content does not
// identify a language.  Codes are kept as written, apart from case; an
// unknown region is dropped while the language is kept.
func parseLangCode(content string) string {
	if len(content) != 2 && len(content) != 4 {
		return ""
	}
	for i := 0; i < len(content); i++ {
		c := content[i]
		if (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') {
			return ""
		}
	}
	base, err := language.ParseBase(content[:2])
	if err != nil {
		return ""
	}
	code := base.String()
	if len(code) != 2 {
		return ""
	}
	if len(content) == 4 {
		if region, err := language.ParseRegion(content[2:]); err == nil {
			if r := region.String(); len(r) == 2 {
				code += r
			}
		}
	}
	return code
}

// String returns the plain text, without language markers.
func (s TextString) String() string {
	if strings.IndexByte(s.s, textEsc) < 0 {
		return s.s
	}
	var buf strings.Builder
	rest := s.s
	for {
		i := strings.IndexByte(rest, textEsc)
		if i < 0 {
			buf.WriteString(rest)
			return buf.String()
		}
		buf.WriteString(rest[:i])
		rest = rest[i+1:]
		j := strings.IndexByte(rest, textEsc)
		rest = rest[j+1:]
	}
}

// IsZero reports whether s is the empty string.
func (s TextString) IsZero() bool {
	return s.s == ""
}

// Equal reports whether s and other denote the same text string.
func (s TextString) Equal(other TextString) bool {
	return s == other
}

// AsPDF implements the [Object] interface.
func (s TextString) AsPDF(opt OutputOptions) Native {
	// use PDFDocEncoding where possible, because it is smallest; it cannot
	// hold language markers
	if strings.IndexByte(s.s, textEsc) < 0 {
		if buf, ok := PDFDocEncode(s.s); ok && !bytes.HasPrefix(buf, utf16Marker) && !bytes.HasPrefix(buf, utf8Marker) {
			return buf
		}
	}

	// otherwise UTF-8 if supported
	if opt.HasAny(OptTextStringUtf8) {
		obj := make(String, 0, 3+len(s.s))
		obj = append(obj, utf8Marker...)
		obj = append(obj, s.s...)
		return obj
	}

	// otherwise UTF-16
	var buf = []uint16{0xFEFF}
	for _, r := range s.s {
		buf = utf16.AppendRune(buf, r)
	}
	out := make(String, 0, 2*len(buf))
	for _, x := range buf {
		out = append(out, byte(x>>8), byte(x))
	}
	return out
}

var utf16Marker = []byte{254, 255}
var utf8Marker = []byte{239, 187, 191}

// AsTextString decodes x as a PDF text string.
//
// Malformed input is repaired: invalid UTF-16 and UTF-8 become U+FFFD,
// and an escape sequence which does not identify a language is dropped.
func (x String) AsTextString() TextString {
	b := []byte(x)

	var s string
	if bytes.HasPrefix(b, utf16Marker) {
		buf := make([]uint16, 0, (len(b)-2)/2)
		for i := 2; i+1 < len(b); i += 2 {
			buf = append(buf, uint16(b[i])<<8|uint16(b[i+1]))
		}
		s = string(utf16.Decode(buf))
	} else if bytes.HasPrefix(b, utf8Marker) {
		s = strings.ToValidUTF8(string(b[3:]), "�")
	} else {
		// PDFDocEncoding has no escape sequences, and PDFDocDecode never
		// yields U+001B
		return TextString{s: PDFDocDecode(x)}
	}

	if strings.IndexByte(s, textEsc) < 0 {
		return TextString{s: s}
	}

	// Remove every escape sequence, i.e. the span from an ESC to the next
	// ESC, or to the end of the string.  A sequence naming a language
	// changes the language of the text that follows.
	var tb textBuilder
	rest := s
	for {
		i := strings.IndexByte(rest, textEsc)
		if i < 0 {
			tb.write(rest)
			return tb.result()
		}
		tb.write(rest[:i])
		rest = rest[i+1:]
		j := strings.IndexByte(rest, textEsc)
		if j < 0 {
			return tb.result()
		}
		if code := parseLangCode(rest[:j]); code != "" {
			tb.pending = code
		}
		rest = rest[j+1:]
	}
}

// TextString resolves any indirect reference and returns the object as a
// text string.
func (c Cursor) TextString(obj Object) (TextString, error) {
	s, err := c.String(obj)
	if err != nil {
		return TextString{}, err
	}
	return s.AsTextString(), nil
}
