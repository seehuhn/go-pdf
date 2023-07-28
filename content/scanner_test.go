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

package content

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf"
)

func TestComment(t *testing.T) {
	type testCase struct {
		in  string
		out pdf.Object
		err error
	}
	cases := []testCase{
		{"% This is a comment\n1", pdf.Integer(1), nil},
		{"%\n", nil, io.EOF},
		{"%", nil, io.EOF},
	}
	for i, c := range cases {
		s := NewScanner(bytes.NewReader([]byte(c.in)))
		obj, err := s.Next()
		if err != c.err {
			t.Errorf("%d: Expected error %v, got %v", i, c.err, err)
			continue
		}
		if d := cmp.Diff(c.out, obj); d != "" {
			t.Errorf("%d: Diff: %s", i, d)
		}
	}
}

func TestString(t *testing.T) {
	type testCase struct {
		in  string
		out string
	}
	cases := []testCase{
		{"(This is a string)", "This is a string"},
		{"()", ""},
		{"(a (and b))", "a (and b)"},
		{"(a\nb)", "a\nb"},
		{"(a\\nb)", "a\nb"},
		{"(a\rb)", "a\rb"},
		{"(a\\rb)", "a\rb"},
		{"(a\\\rb)", "ab"},
		{"(a\\\nb)", "ab"},
		{"(a\\\r\nb)", "ab"},   // CR LF is one line ending
		{"(a\\\n\rb)", "a\rb"}, // LF CR is two line endings
		{"(\0053)", "\0053"},
		{"<414243>", "ABC"},
		{"< 4 1 4 2 4 3 >", "ABC"},
		{"<534950>", "SIP"},
		{"<53495>", "SIP"},
	}

	for i, c := range cases {
		s := NewScanner(bytes.NewReader([]byte(c.in)))
		obj, err := s.Next()
		if err != nil {
			t.Error(err)
			continue
		}
		outString, ok := obj.(pdf.String)
		if !ok {
			t.Errorf("Expected String, got %T", obj)
			continue
		}
		if string(outString) != c.out {
			t.Errorf("%d: Expected %q, got %q", i, c.out, outString)
		}
	}
}

func TestName(t *testing.T) {
	type testCase struct {
		in  string
		out pdf.Name
	}
	cases := []testCase{
		{"/abc", "abc"},
		{"/Name1", "Name1"},
		{"/ASomewhatLongerName", "ASomewhatLongerName"},
		{"/A;Name_With-Various***Characters?", "A;Name_With-Various***Characters?"},
		{"/1.2", "1.2"},
		{"/$$", "$$"},
		{"/@pattern", "@pattern"},
		{"/.notdef", ".notdef"},
		{"/lime#20green", "lime green"},
		{"/paired#28#29parentheses", "paired()parentheses"},
		{"/The_Key_of_F#23_Minor", "The_Key_of_F#_Minor"},
		{"/A#42", "AB"},
	}

	for i, c := range cases {
		s := NewScanner(bytes.NewReader([]byte(c.in)))
		obj, err := s.Next()
		if err != nil {
			t.Error(err)
			continue
		}
		outName, ok := obj.(pdf.Name)
		if !ok {
			t.Errorf("Expected Name, got %T", obj)
			continue
		}
		if outName != c.out {
			t.Errorf("%d: Expected %q, got %q", i, c.out, outName)
		}
	}
}

func TestScanner(t *testing.T) {
	for _, c := range testCases {
		s := NewScanner(bytes.NewReader([]byte(c.in)))
		obj, err := s.Next()
		if err != nil && c.ok {
			t.Errorf("%q: Unexpected error: %s", c.in, err)
			continue
		}
		if !c.ok && err == nil {
			t.Errorf("%q: Expected error, got %T", c.in, obj)
			continue
		}
		if d := cmp.Diff(c.val, obj); d != "" {
			t.Errorf("%q: Diff: %s", c.in, d)
		}
	}
}

func FuzzScanner(f *testing.F) {
	for _, test := range testCases {
		f.Add(test.in)
	}

	f.Fuzz(func(t *testing.T, in string) {
		r1 := strings.NewReader(in)

		s := NewScanner(r1)
		obj1, err := s.Next()
		if err != nil {
			return
		}

		buf := &bytes.Buffer{}
		err = writeObject(buf, obj1)
		if err != nil {
			t.Fatal(err)
		}
		out1 := buf.String()

		r2 := strings.NewReader(out1)
		s = NewScanner(r2)
		obj2, err := s.Next()
		if err != nil {
			fmt.Printf("%q -> %v -> %q\n", in, obj1, out1)
			t.Fatal(err)
		}

		buf.Reset()
		err = writeObject(buf, obj2)
		if err != nil {
			t.Fatal(err)
		}
		out2 := buf.String()

		if out1 != out2 {
			fmt.Printf("%q -> %v -> %q -> %v -> %q\n",
				in, obj1, out1, obj2, out2)
			t.Error("results differ")
		}
	})
}

func writeObject(w io.Writer, obj pdf.Object) error {
	if obj == nil {
		_, err := w.Write([]byte("null"))
		return err
	}
	return obj.PDF(w)
}

var testCases = []struct {
	in  string
	val pdf.Object
	ok  bool
}{
	{"", nil, false},
	{"null", nil, true},

	{"true", pdf.Boolean(true), true},
	{"false", pdf.Boolean(false), true},

	{"0", pdf.Integer(0), true},
	{"+0", pdf.Integer(0), true},
	{"-0", pdf.Integer(0), true},
	{"1", pdf.Integer(1), true},
	{"+1", pdf.Integer(1), true},
	{"-1", pdf.Integer(-1), true},
	{"12", pdf.Integer(12), true},
	{"+12", pdf.Integer(12), true},
	{"-12", pdf.Integer(-12), true},
	{"123", pdf.Integer(123), true},
	{"-4567", pdf.Integer(-4567), true},
	{"999999999999999999", pdf.Integer(999999999999999999), true},
	{"-999999999999999999", pdf.Integer(-999999999999999999), true},

	{".5", pdf.Real(.5), true},
	{"+.5", pdf.Real(.5), true},
	{"-.5", pdf.Real(-.5), true},
	{"0.5", pdf.Real(.5), true},
	{"+0.5", pdf.Real(.5), true},
	{"-0.5", pdf.Real(-.5), true},

	{"/a", pdf.Name("a"), true},
	{"/1234567890123456789012345678901", pdf.Name("1234567890123456789012345678901"), true},
	{"/12345678901234567890123456789012", pdf.Name("12345678901234567890123456789012"), true},
	{"/123456789012345678901234567890123", pdf.Name("123456789012345678901234567890123"), true},
	{"/A;Name_With-Various***Characters?", pdf.Name("A;Name_With-Various***Characters?"), true},
	{"/1.2", pdf.Name("1.2"), true},
	{"/A#42", pdf.Name("AB"), true},
	{"/F#23#20minor", pdf.Name("F# minor"), true},
	{"/1#2E5", pdf.Name("1.5"), true},
	{"/ß", pdf.Name("ß"), true},
	{"/", pdf.Name(""), true},

	{`()`, pdf.String(nil), true},
	{"(test string)", pdf.String("test string"), true},
	{`(hello)`, pdf.String("hello"), true},
	{`(he(ll)o)`, pdf.String("he(ll)o"), true},
	{`(he\)ll\(o)`, pdf.String("he)ll(o"), true},
	{"(hello\n)", pdf.String("hello\n"), true},
	{"(hello\r)", pdf.String("hello\r"), true},
	{"(hello\r\n)", pdf.String("hello\r\n"), true},
	{"(hello\n\r)", pdf.String("hello\n\r"), true},
	{"(hell\\\no)", pdf.String("hello"), true},
	{"(hell\\\ro)", pdf.String("hello"), true},
	{"(hell\\\r\no)", pdf.String("hello"), true},
	{`(h\145llo)`, pdf.String("hello"), true},
	{`(\0612)`, pdf.String("12"), true},

	{"<>", pdf.String(nil), true},
	{"<68656c6c6f>", pdf.String("hello"), true},
	{"<68656C6C6F>", pdf.String("hello"), true},
	{"<68 65 6C 6C 6F>", pdf.String("hello"), true},
	{"<68656C70>", pdf.String("help"), true},
	{"<68656C7>", pdf.String("help"), true},

	{"[1 2 3]", pdf.Array{pdf.Integer(1), pdf.Integer(2), pdf.Integer(3)}, true},
	{"[1 2 << /three 3 >>]", pdf.Array{
		pdf.Integer(1),
		pdf.Integer(2),
		pdf.Dict{"three": pdf.Integer(3)},
	}, true},

	{"<< /key 12 /key2 /23 /key3 [1 2 3] /key4 << /a 1 >> >>", pdf.Dict{
		"key":  pdf.Integer(12),
		"key2": pdf.Name("23"),
		"key3": pdf.Array{pdf.Integer(1), pdf.Integer(2), pdf.Integer(3)},
		"key4": pdf.Dict{"a": pdf.Integer(1)},
	}, true},
	{"<< /key1 1 /key2 [1 2 3] /key3 3 >>", pdf.Dict{
		"key1": pdf.Integer(1),
		"key2": pdf.Array{pdf.Integer(1), pdf.Integer(2), pdf.Integer(3)},
		"key3": pdf.Integer(3),
	}, true},

	{"q", Operator("q"), true},
	{"T*", Operator("T*"), true},
	{"NULL", Operator("NULL"), true},
	{"TRUE", Operator("TRUE"), true},
	{"FALSE", Operator("FALSE"), true},
	{"A;Name_With-Various***Characters?", Operator("A;Name_With-Various***Characters?"), true},
	{"ß", Operator("ß"), true},
}
