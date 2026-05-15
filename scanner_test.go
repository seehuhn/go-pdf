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
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
)

func testScanner(contents string) *scanner {
	buf := bytes.NewReader([]byte(contents))
	s := newScanner(buf, func(o Object) (Integer, error) {
		return o.(Integer), nil
	}, nil)
	s.fileReader = buf
	return s
}

func TestRefill(t *testing.T) {
	n := scannerBufSize + 2
	buf := make([]byte, n)
	s := newScanner(bytes.NewReader(buf), nil, nil)

	for _, inc := range []int{0, 1, scannerBufSize, 1} {
		s.pos += inc
		err := s.refill()
		total := int(s.filePos) + s.pos
		expectUsed := min(scannerBufSize, n-total)
		if err != nil || s.pos != 0 || s.used != expectUsed {
			errStr := "nil"
			if err != nil {
				errStr = err.Error()
			}
			t.Errorf("%d: s.pos = %d, s.used = %d, %s",
				total, s.pos, s.used, errStr)
		}
	}
}

func TestReadObject(t *testing.T) {
	for _, test := range testCases {
		for _, suffix := range []string{">>", " 1\n"} {
			body := test.in + suffix
			s := testScanner(body)

			val, err := s.ReadObject()
			if !Equal(val, test.val) {
				t.Errorf("%q: wrong value: expected %q, got %q",
					body, AsString(test.val), AsString(val))
			}

			switch {
			case test.ok && err != nil:
				t.Errorf("%q: unexpected error %q", body, err)
			case !test.ok && err == nil:
				t.Errorf("%q: missing error", body)
			case !test.ok:
				_, ok := err.(*MalformedFileError)
				if !ok {
					t.Errorf("%q: wrong error %q", body, err)
				}
			}
		}
	}
}

// TestReadObjectNestedArrayBomb checks that a deeply nested array is
// rejected as malformed rather than recursing without bound.
func TestReadObjectNestedArrayBomb(t *testing.T) {
	const depth = maxScannerNestDepth + 1
	body := strings.Repeat("[", depth) + strings.Repeat("]", depth)
	s := testScanner(body)

	_, err := s.ReadObject()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !IsMalformed(err) {
		t.Errorf("expected *MalformedFileError, got %T: %v", err, err)
	}
}

// TestReadObjectNestedDictBomb is the dict variant of
// TestReadObjectNestedArrayBomb.
func TestReadObjectNestedDictBomb(t *testing.T) {
	const depth = maxScannerNestDepth + 1
	body := strings.Repeat("<</A ", depth) + "null" + strings.Repeat(">>", depth)
	s := testScanner(body)

	_, err := s.ReadObject()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !IsMalformed(err) {
		t.Errorf("expected *MalformedFileError, got %T: %v", err, err)
	}
}

func TestSkipWhiteSpace(t *testing.T) {
	cases := []string{
		"",
		" ",
		"               ",
		"                ",
		"                 ",
		"\r",
		"\n",
		"% comment\r\n",
		" % comment\r\n % comment\r\n % comment\r\n   ",
	}

	for _, test := range cases {
		for _, suffix := range []string{">>", "x y\n"} {
			body := test + suffix
			s := testScanner(body)

			err := s.SkipWhiteSpace()
			if err != nil {
				t.Errorf("%q: unexpected error: %s", body, err)
			}
			total := int(s.filePos) + s.pos
			if total != len(test) {
				t.Errorf("%q: wrong position %d", body, total)
			}
		}
	}
}

func TestReadHeaderVersion(t *testing.T) {
	s := newScanner(strings.NewReader("%PDF-1.7\n1 0 obj\n"), nil, nil)
	version, err := s.ReadHeaderVersion()
	if err != nil {
		t.Errorf("unexpected error %q", err)
	}
	if version != V1_7 {
		t.Errorf("wrong version: expected %d, got %d", V1_7, version)
	}

	for _, in := range []string{"", "%PEF-1.7\n", "%PDF-0.1\n"} {
		s = newScanner(strings.NewReader(in), nil, nil)
		_, err = s.ReadHeaderVersion()
		if err == nil {
			t.Errorf("%q: missing error", in)
		}
	}

	for _, in := range []string{"%PDF-1.9\n", "%PDF-1.50\n"} {
		s = newScanner(strings.NewReader(in), nil, nil)
		_, err = s.ReadHeaderVersion()
		if !errors.Is(err, errVersion) {
			t.Errorf("%q: wrong error %q", in, err)
		}
	}
}

func TestFindHeaderOffset(t *testing.T) {
	t.Run("no preamble", func(t *testing.T) {
		data := []byte("%PDF-1.7\nrest of file")
		off, err := findHeaderOffset(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			t.Fatal(err)
		}
		if off != 0 {
			t.Errorf("expected offset 0, got %d", off)
		}
	})

	t.Run("short preamble", func(t *testing.T) {
		data := []byte("\x00\x01\x02\x03%PDF-1.5\nrest of file")
		off, err := findHeaderOffset(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			t.Fatal(err)
		}
		if off != 4 {
			t.Errorf("expected offset 4, got %d", off)
		}
	})

	t.Run("no signature", func(t *testing.T) {
		data := []byte("this is not a PDF file")
		_, err := findHeaderOffset(bytes.NewReader(data), int64(len(data)))
		if err == nil {
			t.Error("expected error for missing signature")
		}
	})
}

func FuzzScanner(f *testing.F) {
	for _, test := range testCases {
		f.Add(test.in)
	}
	for _, in := range []string{"0 ", "<0d>", "-0.", "//", "/#23", "<<>>0", "<</<</ 0 0>>"} {
		f.Add(in)
	}

	getInt := func(obj Object) (Integer, error) {
		switch x := obj.(type) {
		case Integer:
			return x, nil
		case Reference:
			// Allow the fuzzer to generate different indirect integer values,
			// both positive and negative.
			return Integer(x - 1000000), nil
		default:
			return 0, errors.New("not an integer")
		}
	}

	f.Fuzz(func(t *testing.T, in string) {
		r1 := strings.NewReader(in)

		s := newScanner(r1, getInt, nil)
		obj1, err := s.ReadObject()
		if err != nil {
			return
		}
		if _, isStream := obj1.(*Stream); isStream {
			// Skip streams, as they cannot be written using Format, below.
			return
		}

		buf := &bytes.Buffer{}
		err = Format(buf, 0, obj1)
		if err != nil {
			t.Fatal(err)
		}
		out1 := buf.String()

		r2 := strings.NewReader(out1)
		s = newScanner(r2, getInt, nil)
		obj2, err := s.ReadObject()
		if err != nil {
			fmt.Printf("%q -> %v -> %q\n", in, obj1, out1)
			t.Fatal(err)
		}

		buf.Reset()
		err = Format(buf, 0, obj2)
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

var testCases = []struct {
	in  string
	val Object
	ok  bool
}{
	{"", nil, false},
	{"null", nil, true},

	{"true", Boolean(true), true},
	{"false", Boolean(false), true},
	{"TRUE", nil, false},
	{"FALSE", nil, false},

	{"0", Integer(0), true},
	{"+0", Integer(0), true},
	{"-0", Integer(0), true},
	{"1", Integer(1), true},
	{"+1", Integer(1), true},
	{"-1", Integer(-1), true},
	{"12", Integer(12), true},
	{"+12", Integer(12), true},
	{"-12", Integer(-12), true},
	{"123", Integer(123), true},
	{"-4567", Integer(-4567), true},
	{"999999999999999999", Integer(999999999999999999), true},
	{"-999999999999999999", Integer(-999999999999999999), true},

	{".5", Real(.5), true},
	{"+.5", Real(.5), true},
	{"-.5", Real(-.5), true},
	{"0.5", Real(.5), true},
	{"+0.5", Real(.5), true},
	{"-0.5", Real(-.5), true},
	{".", nil, false},
	{".+5", nil, false},

	{"/a", Name("a"), true},
	{"/1234567890123456789012345678901", Name("1234567890123456789012345678901"), true},
	{"/12345678901234567890123456789012", Name("12345678901234567890123456789012"), true},
	{"/123456789012345678901234567890123", Name("123456789012345678901234567890123"), true},
	{"/A;Name_With-Various***Characters?", Name("A;Name_With-Various***Characters?"), true},
	{"/1.2", Name("1.2"), true},
	{"/A#42", Name("AB"), true},
	{"/F#23#20minor", Name("F# minor"), true},
	{"/1#2E5", Name("1.5"), true},
	{"/A#aF", Name("A\xaf"), true},
	// PDF 7.3.5: when "#" is not followed by two hex digits, treat "#"
	// as a literal character rather than corrupting the name.
	{"/A#Z9", Name("A#Z9"), true},
	{"/A#9Z", Name("A#9Z"), true},
	{"/A#", Name("A#"), true},
	{"/A#9", Name("A#9"), true},
	{"/##41", Name("#A"), true},
	{"/#00", Name("\x00"), true},
	{"/ß", Name("ß"), true},
	{"/", Name(""), true},

	{`()`, String(nil), true},
	{"(test string)", String("test string"), true},
	{`(hello)`, String("hello"), true},
	{`(he(ll)o)`, String("he(ll)o"), true},
	{`(he\)ll\(o)`, String("he)ll(o"), true},
	{"(hello\n)", String("hello\n"), true},
	// PDF 7.3.4.2: an unescaped end-of-line marker inside a literal
	// string is normalised to a single LF, regardless of whether it
	// is CR, LF, or CR-LF.
	{"(hello\r)", String("hello\n"), true},
	{"(hello\r\n)", String("hello\n"), true},
	{"(hello\n\r)", String("hello\n\n"), true},
	{"(a\rb\rc)", String("a\nb\nc"), true},
	{"(a\r\nb\r\nc)", String("a\nb\nc"), true},
	{"(hell\\\no)", String("hello"), true},
	{"(hell\\\ro)", String("hello"), true},
	{"(hell\\\r\no)", String("hello"), true},
	{`(h\145llo)`, String("hello"), true},
	{`(\0612)`, String("12"), true},
	// PDF 7.3.4.2: an octal escape ends as soon as a non-octal digit is
	// seen, even if fewer than three digits have been read.
	{`(a\17X)`, String("a\x0fX"), true},
	{`(a\1X)`, String("a\x01X"), true},
	{`(a\78)`, String("a\x078"), true},
	{`(a\1\2)`, String("a\x01\x02"), true},

	{"<>", String(nil), true},
	{"<68656c6c6f>", String("hello"), true},
	{"<68656C6C6F>", String("hello"), true},
	{"<68 65 6C 6C 6F>", String("hello"), true},
	{"<68656C70>", String("help"), true},
	{"<68656C7>", String("help"), true},

	{"[1 2 3]", Array{Integer(1), Integer(2), Integer(3)}, true},
	{"[1 2 3 R 4]", Array{Integer(1), NewReference(2, 3), Integer(4)}, true},

	{"<< /key 12 /val /23 >>", Dict{
		Name("key"): Integer(12),
		Name("val"): Name("23"),
	}, true},
	{"<< /key1 1 /key2 2 2 R /key3 3 >>", Dict{
		Name("key1"): Integer(1),
		Name("key2"): NewReference(2, 2),
		Name("key3"): Integer(3),
	}, true},

	{"fals", nil, false},
	{"abc", nil, false},
}

func TestStreamReader(t *testing.T) {
	in := "<< /Length 6 >>\nstream\nABCDEF\nendstream 1 2"
	sr := strings.NewReader(in)
	s := newScanner(sr, func(x Object) (Integer, error) { return x.(Integer), nil }, nil)
	s.fileReader = sr
	stmObj, err := s.ReadObject()
	if err != nil {
		t.Fatal(err)
	}
	stm, ok := stmObj.(*Stream)
	if !ok {
		t.Fatalf("expected stream, got %T", stmObj)
	}

	x1, err := s.ReadInteger()
	if err != nil {
		t.Error(err)
	} else if x1 != 1 {
		t.Errorf("expected 1, got %d", x1)
	}

	stmData, err := io.ReadAll(stm.NewReader())
	if err != nil {
		t.Error(err)
	}
	if string(stmData) != "ABCDEF" {
		t.Errorf("expected ABCDEF, got %q", stmData)
	}

	x2, err := s.ReadInteger()
	if err != nil {
		t.Error(err)
	} else if x2 != 2 {
		t.Errorf("expected 2, got %d", x2)
	}
}

func TestStreamFromScanner(t *testing.T) {
	in := "<< /Length 5 >>\nstream\nhello\nendstream"
	sr := strings.NewReader(in)
	s := newScanner(sr, func(x Object) (Integer, error) { return x.(Integer), nil }, nil)
	s.fileReader = sr
	stmObj, err := s.ReadObject()
	if err != nil {
		t.Fatal(err)
	}
	stm, ok := stmObj.(*Stream)
	if !ok {
		t.Fatalf("expected stream, got %T", stmObj)
	}
	if !Equal(stm.Dict, Dict{Name("Length"): Integer(5)}) {
		t.Errorf("wrong dict: %v", stm.Dict)
	}
	data, err := io.ReadAll(stm.NewReader())
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello" {
		t.Errorf("expected %q, got %q", "hello", data)
	}
}

func TestStreamConcurrentReaders(t *testing.T) {
	in := "<< /Length 6 >>\nstream\nABCDEF\nendstream"
	sr := strings.NewReader(in)
	s := newScanner(sr, func(x Object) (Integer, error) { return x.(Integer), nil }, nil)
	s.fileReader = sr
	stmObj, err := s.ReadObject()
	if err != nil {
		t.Fatal(err)
	}
	stm, ok := stmObj.(*Stream)
	if !ok {
		t.Fatalf("expected stream, got %T", stmObj)
	}

	// multiple independent readers return the same data
	r1 := stm.NewReader()
	r2 := stm.NewReader()

	data1, err := io.ReadAll(r1)
	if err != nil {
		t.Fatal(err)
	}
	data2, err := io.ReadAll(r2)
	if err != nil {
		t.Fatal(err)
	}
	if string(data1) != "ABCDEF" || string(data2) != "ABCDEF" {
		t.Errorf("expected ABCDEF from both readers, got %q and %q", data1, data2)
	}

	// partial read on one reader does not affect another
	r3 := stm.NewReader()
	buf := make([]byte, 3)
	_, err = r3.Read(buf)
	if err != nil {
		t.Fatal(err)
	}
	if string(buf) != "ABC" {
		t.Errorf("expected ABC, got %q", buf)
	}

	r4 := stm.NewReader()
	data4, err := io.ReadAll(r4)
	if err != nil {
		t.Fatal(err)
	}
	if string(data4) != "ABCDEF" {
		t.Errorf("expected ABCDEF, got %q", data4)
	}
}
