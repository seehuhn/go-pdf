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

package pdf

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"
)

func TestFormat(t *testing.T) {
	cases := []struct {
		in  Object
		out string
	}{
		{nil, "null"},
		{String("a"), "(a)"},
		{String("a (test version)"), "(a (test version))"},
		{String("a (test version"), "(a \\(test version)"},
		{String(""), "()"},
		{String("\000"), "<00>"},
		{Array{Integer(1), nil, Integer(3)}, "[1 null 3]"},
	}
	for _, test := range cases {
		out := format(test.in)
		if out != test.out {
			t.Errorf("string wrongly formatted, expected %q but got %q",
				test.out, out)
		}
	}
}

func TestParseString(t *testing.T) {
	type testCase struct {
		in  string
		out String
	}
	cases := []testCase{
		{`()`, String(nil)},
		{"(test string)", String("test string")},
		{`(hello)`, String("hello")},
		{`(he(ll)o)`, String("he(ll)o")},
		{`(he\)ll\(o)`, String("he)ll(o")},
		{"(hello\n)", String("hello\n")},
		{"(hello\r)", String("hello\r")},
		{"(hello\r\n)", String("hello\r\n")},
		{"(hello\n\r)", String("hello\n\r")},
		{"(hell\\\no)", String("hello")},
		{"(hell\\\ro)", String("hello")},
		{"(hell\\\r\no)", String("hello")},
		{`(h\145llo)`, String("hello")},
		{`(\0612)`, String("12")},
		{"<>", String(nil)},
		{"<68656c6c6f>", String("hello")},
		{"<68656C6C6F>", String("hello")},
		{"<68 65 6C 6C 6F>", String("hello")},
		{"<68656C70>", String("help")},
		{"<68656C7>", String("help")},
	}
	for i, test := range cases {
		out, err := ParseString([]byte(test.in))
		if err != nil {
			t.Errorf("%d %q: %s", i, test.in, err)
		} else if !bytes.Equal(out, test.out) {
			t.Errorf("wrong string: %q != %q", out, test.out)
		}
	}
}

func FuzzString(f *testing.F) {
	f.Add([]byte(""))
	f.Add([]byte("ABC"))
	f.Add([]byte{0, 1, 2})
	f.Add([]byte{0xFF, 0x00})
	f.Fuzz(func(t *testing.T, data []byte) {
		s1 := String(data)
		enc := format(s1)
		s2, err := ParseString([]byte(enc))
		if err != nil {
			t.Error(err)
		} else if !bytes.Equal(s1, s2) {
			t.Errorf("wrong string: %q != %q", s1, s2)
		}
	})
}

func TestTextString(t *testing.T) {
	cases := []string{
		"",
		"hello",
		"\000\011\n\f\r",
		"ein Bär",
		"o țesătură",
		"中文",
		"日本語",
	}
	for _, test := range cases {
		enc := TextString(test)
		out := enc.AsTextString()
		if out != test {
			t.Errorf("wrong text: %q != %q", out, test)
		}
	}
}

func TestDateString(t *testing.T) {
	PST := time.FixedZone("PST", -8*60*60)
	cases := []time.Time{
		time.Date(1998, 12, 23, 19, 52, 0, 0, PST),
		time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2020, 12, 24, 16, 30, 12, 0, time.FixedZone("", 90*60)),
	}
	for _, test := range cases {
		enc := Date(test)
		out, err := enc.AsDate()
		if err != nil {
			t.Error(err)
		} else if !test.Equal(out) {
			fmt.Println(test, string(enc), out)
			t.Errorf("wrong time: %s != %s", out, test)
		}
	}
}

func TestDecodeDate(t *testing.T) {
	cases := []string{
		"D:19981223195200-08'00'",
		"D:20000101000000Z",
		"D:20201224163012+01'30'",
		"D:20010809191510 ", // trailing space, seen in some PDF files
	}
	for i, test := range cases {
		enc := TextString(test)
		_, err := enc.AsDate()
		if err != nil {
			t.Errorf("%d %q %s\n", i, test, err)
		}
	}
}

func TestStream(t *testing.T) {
	dataIn := "\nbinary stream data\000123\n   "
	rIn := strings.NewReader(dataIn)
	stream := &Stream{
		Dict: map[Name]Object{
			"Length": Integer(len(dataIn)),
		},
		R: rIn,
	}
	rOut, err := (*Reader)(nil).DecodeStream(stream, 0)
	if err != nil {
		t.Fatal(err)
	}
	dataOut, err := io.ReadAll(rOut)
	if err != nil {
		t.Fatal(err)
	}
	if string(dataOut) != dataIn {
		t.Errorf("wrong result:\n  %q\n  %q", dataIn, dataOut)
	}
}

func format(x Object) string {
	buf := &bytes.Buffer{}
	if x == nil {
		buf.WriteString("null")
	} else {
		_ = x.PDF(buf)
	}
	return buf.String()
}
