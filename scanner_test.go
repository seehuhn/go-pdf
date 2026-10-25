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

	"github.com/google/go-cmp/cmp"
)

func testScanner(contents string) *scanner {
	buf := bytes.NewReader([]byte(contents))
	s := newScanner(buf, func(o Object) (Integer, error) {
		if o == nil {
			return 0, nil
		}
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

// erroringReader returns a fixed error on every Read and counts the calls.
type erroringReader struct {
	calls int
	err   error
}

func (r *erroringReader) Read(p []byte) (int, error) {
	r.calls++
	return 0, r.err
}

func TestRefillStickyError(t *testing.T) {
	sentinel := errors.New("read failed")
	r := &erroringReader{err: sentinel}
	s := newScanner(r, nil, nil)

	if err := s.refill(); err != sentinel {
		t.Fatalf("first refill: want sentinel, got %v", err)
	}
	callsAfterFirst := r.calls

	if err := s.refill(); err != sentinel {
		t.Fatalf("second refill: want sentinel, got %v", err)
	}
	if r.calls != callsAfterFirst {
		t.Errorf("expected no extra Read calls after error latched, got %d more",
			r.calls-callsAfterFirst)
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

// withSizeBound temporarily replaces a package-level size bound for the
// duration of a test, so size-limit checks can be exercised without
// allocating the production limit.
func withSizeBound(t *testing.T, p *int, val int) {
	t.Helper()
	orig := *p
	*p = val
	t.Cleanup(func() { *p = orig })
}

// TestReadIndirectObjectTruncated checks that an indirect object cut short by
// the end of the input is reported as a MalformedFileError, never as a bare
// io.EOF.  Callers rely on this to treat an unreadable object as null (PDF
// 2.0, 7.3.10) instead of aborting.
func TestReadIndirectObjectTruncated(t *testing.T) {
	const full = "3 0 obj\n<</A [1 (x\\061) <41> /N 2 0 R] /Length 3>>\nstream\nabc\nendstream\nendobj"
	for i := range len(full) {
		_, _, err := testScanner(full[:i]).ReadIndirectObject()
		if !IsMalformed(err) {
			t.Errorf("%q: got error %v, want MalformedFileError", full[:i], err)
		}
	}

	obj, _, err := testScanner(full).ReadIndirectObject()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := obj.(*Stream); !ok {
		t.Errorf("got %T, want *Stream", obj)
	}
}

// TestReadIndirectObjectEndobj checks the handling of a missing "endobj"
// keyword.  Other data after the value is tolerated, whatever its length.  If
// the input ends within a prefix of the keyword, the object may have been
// truncated.
func TestReadIndirectObjectEndobj(t *testing.T) {
	cases := []struct {
		name      string
		tail      string
		malformed bool
	}{
		{"keyword", "endobj", false},
		{"keyword and more", "endobj\n1 0 obj", false},
		{"eof", "", true},
		{"white space then eof", "\n", true},
		{"prefix e", "e", true},
		{"prefix endob", "endob", true},
		{"prefix of stream", "stre", true},
		{"comment then eof", "%%EOF", true},
		{"short junk", "trail", false},
		{"one junk byte", "x", false},
		{"six junk bytes", "trailer", false},
		{"long junk", "xref\n0 1\n", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := testScanner("4 0 obj\n<</A 1>>\n" + c.tail)
			obj, ref, err := s.ReadIndirectObject()
			if c.malformed {
				if !IsMalformed(err) {
					t.Fatalf("got error %v, want MalformedFileError", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if ref != NewReference(4, 0) {
				t.Errorf("wrong reference %v", ref)
			}
			if _, ok := obj.(Dict); !ok {
				t.Errorf("got %T, want Dict", obj)
			}
		})
	}
}

// TestReadIndirectObjectEndobjReadError checks that a read error while
// looking for the "endobj" keyword is propagated.
func TestReadIndirectObjectEndobjReadError(t *testing.T) {
	errTest := errors.New("test read error")
	src := io.MultiReader(strings.NewReader("4 0 obj\n<</A 1>>\n"), errReader{errTest})
	s := newScanner(src, func(o Object) (Integer, error) { return 0, nil }, nil)
	_, _, err := s.ReadIndirectObject()
	if !IsReadError(err) {
		t.Errorf("got error %v, want a non-malformed read error", err)
	}
}

type errReader struct{ err error }

func (r errReader) Read([]byte) (int, error) { return 0, r.err }

// TestReadObjectTruncatedString checks that unterminated strings are
// reported as malformed, so that ReadObject never returns a bare io.EOF.
func TestReadObjectTruncatedString(t *testing.T) {
	for _, body := range []string{"(", "(abc", "(a\\", "(a(b)", "<", "<41", "<41 4"} {
		_, err := testScanner(body).ReadObject()
		if !IsMalformed(err) {
			t.Errorf("%q: got error %v, want MalformedFileError", body, err)
		}
	}
}

func TestReadStringBomb(t *testing.T) {
	withSizeBound(t, &maxStringBytes, 100)
	body := "(" + strings.Repeat("a", maxStringBytes+1) + ")"
	s := testScanner(body)
	if _, err := s.ReadObject(); err == nil {
		t.Fatal("expected error, got nil")
	} else if !IsMalformed(err) {
		t.Errorf("expected *MalformedFileError, got %T: %v", err, err)
	}
}

func TestReadHexStringBomb(t *testing.T) {
	withSizeBound(t, &maxStringBytes, 100)
	body := "<" + strings.Repeat("00", maxStringBytes+1) + ">"
	s := testScanner(body)
	if _, err := s.ReadObject(); err == nil {
		t.Fatal("expected error, got nil")
	} else if !IsMalformed(err) {
		t.Errorf("expected *MalformedFileError, got %T: %v", err, err)
	}
}

func TestReadNameBomb(t *testing.T) {
	withSizeBound(t, &maxNameBytes, 100)
	body := "/" + strings.Repeat("a", maxNameBytes+1) + " "
	s := testScanner(body)
	if _, err := s.ReadObject(); err == nil {
		t.Fatal("expected error, got nil")
	} else if !IsMalformed(err) {
		t.Errorf("expected *MalformedFileError, got %T: %v", err, err)
	}
}

func TestReadIntegerBomb(t *testing.T) {
	withSizeBound(t, &maxNameBytes, 100)
	body := strings.Repeat("9", maxNameBytes+50) + " "
	s := testScanner(body)
	if _, err := s.ReadInteger(); err == nil {
		t.Fatal("expected error, got nil")
	} else if !IsMalformed(err) {
		t.Errorf("expected *MalformedFileError, got %T: %v", err, err)
	}
}

func TestReadNumberBomb(t *testing.T) {
	withSizeBound(t, &maxNameBytes, 100)
	body := strings.Repeat("9", maxNameBytes+50) + ".0 /Next "
	s := testScanner(body)

	// the token is consumed in full, but has no PDF value
	obj, err := s.ReadObject()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if obj != nil {
		t.Errorf("expected null, got %s", AsString(obj))
	}

	// the scanner is still positioned at a token boundary
	if err := s.SkipWhiteSpace(); err != nil {
		t.Fatal(err)
	}
	next, err := s.ReadObject()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if next != Name("Next") {
		t.Errorf("expected /Next, got %s", AsString(next))
	}
}

func TestReadArrayBomb(t *testing.T) {
	withSizeBound(t, &maxArrayLen, 100)
	body := "[" + strings.Repeat("1 ", maxArrayLen+1) + "]"
	s := testScanner(body)
	if _, err := s.ReadObject(); err == nil {
		t.Fatal("expected error, got nil")
	} else if !IsMalformed(err) {
		t.Errorf("expected *MalformedFileError, got %T: %v", err, err)
	}
}

func TestReadDictBomb(t *testing.T) {
	withSizeBound(t, &maxDictLen, 100)
	var b strings.Builder
	b.WriteString("<<")
	for i := 0; i < maxDictLen+1; i++ {
		fmt.Fprintf(&b, "/k%d 1 ", i)
	}
	b.WriteString(">>")
	s := testScanner(b.String())
	if _, err := s.ReadObject(); err == nil {
		t.Fatal("expected error, got nil")
	} else if !IsMalformed(err) {
		t.Errorf("expected *MalformedFileError, got %T: %v", err, err)
	}
}

// TestReadDictNullValues checks that dictionary entries whose value is
// null, or a token which reads as null, are dropped: PDF 7.3.7 treats
// them as absent.  Array elements keep their nulls, since arrays are
// positional.
func TestReadDictNullValues(t *testing.T) {
	tests := []struct {
		in   string
		want Object
	}{
		{"<< /A null /B 1 >>", Dict{"B": Integer(1)}},
		{"<< /A + /B 1 >>", Dict{"B": Integer(1)}},
		{"<< /A . >>", Dict{}},
		{"<< /A -1 0 R /B 1 >>", Dict{"B": Integer(1)}},
		{"<< /A [null + 1] >>", Dict{"A": Array{nil, nil, Integer(1)}}},
		{"<< /A << /B null >> >>", Dict{"A": Dict{}}},
	}
	for _, test := range tests {
		s := testScanner(test.in)
		got, err := s.ReadObject()
		if err != nil {
			t.Errorf("%q: unexpected error: %v", test.in, err)
			continue
		}
		if diff := cmp.Diff(test.want, got); diff != "" {
			t.Errorf("%q: wrong value (-want +got):\n%s", test.in, diff)
		}
	}
}

// TestReadStreamDataLengthRecovery exercises the recovery path
// taken by ReadStreamData when the stream dictionary is missing
// /Length.  The regex-based search requires an EOL before the
// "endstream" keyword (PDF 7.3.8.2), so a substring "endstream"
// embedded in the stream content (without a preceding EOL) must
// not cut the stream short.
func TestReadStreamDataLengthRecovery(t *testing.T) {
	cases := []struct {
		name    string
		content string
	}{
		{
			"trailing LF only",
			"some bytes",
		},
		{
			"trailing CRLF",
			"some bytes",
		},
		{
			"substring endstream mid-content",
			"abc endstream def", // no EOL before "endstream"
		},
		{
			"empty content",
			"",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			eol := "\n"
			if tc.name == "trailing CRLF" {
				eol = "\r\n"
			}
			body := "<< /Type /XObject >>stream\n" +
				tc.content + eol + "endstream\n"
			s := testScanner(body)
			obj, err := s.ReadObject()
			if err != nil {
				t.Fatalf("ReadObject: %v", err)
			}
			stream, ok := obj.(*Stream)
			if !ok {
				t.Fatalf("got %T, want *Stream", obj)
			}
			if int(stream.length) != len(tc.content) {
				t.Errorf("length = %d, want %d", stream.length, len(tc.content))
			}
		})
	}
}

// TestReadStreamDataBrokenLength checks that a present but wrong /Length is
// recovered by scanning for the endstream keyword, giving the real extent.
func TestReadStreamDataBrokenLength(t *testing.T) {
	const content = "some bytes"
	// /Length 2 is wrong; the real extent ends at the endstream keyword
	body := "<< /Length 2 >>stream\n" + content + "\nendstream\n"

	s := testScanner(body)
	obj, err := s.ReadObject()
	if err != nil {
		t.Fatalf("ReadObject: %v", err)
	}
	stream, ok := obj.(*Stream)
	if !ok {
		t.Fatalf("got %T, want *Stream", obj)
	}
	if int(stream.length) != len(content) {
		t.Errorf("length = %d, want %d", stream.length, len(content))
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
	// tokens which are consumed in full but have no PDF value read as null
	{".", nil, true},
	{".+5", nil, true},

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
	{"/#00", Name("#00"), true},
	{"/A#00B", Name("A#00B"), true},
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
	// /Length is consumed while reading and does not become part of the
	// stream's dictionary
	if !Equal(stm.Dict, Dict{}) {
		t.Errorf("wrong dict: %v", stm.Dict)
	}
	if got := stm.Length(); got != 5 {
		t.Errorf("wrong stream length: got %d, want 5", got)
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

func aesStringScanner(t *testing.T, contents string, authenticated bool) *scanner {
	t.Helper()
	id := []byte("0123456789ABCDEF")
	sec, err := createStdSecHandler(id, "secret", "supersecret", PermPrint, 128, 4, false)
	if err != nil {
		t.Fatal(err)
	}
	if !authenticated {
		sec.key = nil
	}
	s := testScanner(contents)
	s.enc = &encryptInfo{
		strF: &cryptFilter{Cipher: cipherAES, Length: 128},
		sec:  sec,
	}
	s.encRef = NewReference(1, 0)
	return s
}

func TestDictStringCorruptKeepsRaw(t *testing.T) {
	// 5 bytes is not a valid AES ciphertext length
	for _, in := range []string{"(abcde)", "<6162636465>"} {
		s := aesStringScanner(t, in, true)
		obj, err := s.ReadObject()
		if err != nil {
			t.Fatal(err)
		}
		if got, ok := obj.(String); !ok || string(got) != "abcde" {
			t.Errorf("%s: got %v, want raw bytes", in, obj)
		}
	}
}

func TestDictStringAuthErrorPropagates(t *testing.T) {
	for _, in := range []string{"(abcde)", "<6162636465>"} {
		s := aesStringScanner(t, in, false)
		_, err := s.ReadObject()
		if _, ok := errors.AsType[*AuthenticationError](err); !ok {
			t.Errorf("%s: got %v, want authentication error", in, err)
		}
	}
}

// TestReadStreamDataEmbeddedFile checks that streams with /Type /EmbeddedFile
// are marked as embedded files.
func TestReadStreamDataEmbeddedFile(t *testing.T) {
	for _, tc := range []struct {
		dict string
		want bool
	}{
		{"<< /Type /EmbeddedFile /Length 3 >>", true},
		{"<< /Length 3 >>", false},
		{"<< /Type /XObject /Length 3 >>", false},
	} {
		s := testScanner(tc.dict + "stream\nabc\nendstream\n")
		obj, err := s.ReadObject()
		if err != nil {
			t.Fatal(err)
		}
		stm, ok := obj.(*Stream)
		if !ok {
			t.Fatalf("got %T, want *Stream", obj)
		}
		if stm.embeddedFile != tc.want {
			t.Errorf("%s: embeddedFile = %t, want %t", tc.dict, stm.embeddedFile, tc.want)
		}
	}
}

func TestSigContentsNotDecrypted(t *testing.T) {
	id := []byte("0123456789ABCDEF")
	sec, err := createStdSecHandler(id, "secret", "supersecret", PermPrint, 128, 4, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, cipher := range []cipherType{cipherRC4, cipherAES} {
		testSigContentsNotDecrypted(t, sec, cipher)
	}
}

func testSigContentsNotDecrypted(t *testing.T, sec *stdSecHandler, cipher cipherType) {
	ref := NewReference(5, 0)
	cf := &cryptFilter{Cipher: cipher, Length: 128}
	enc := &encryptInfo{sec: sec, strF: cf, stmF: cf, efF: cf}

	plain := []byte("signature bytes!")
	cipherText, err := enc.EncryptBytes(ref, plain)
	if err != nil {
		t.Fatal(err)
	}
	plainHex := fmt.Sprintf("<%x>", plain)
	cipherHex := fmt.Sprintf("<%x>", cipherText)
	esc := strings.NewReplacer("\\", "\\\\", "(", "\\(", ")", "\\)", "\r", "\\r")
	literal := "(" + esc.Replace(string(cipherText)) + ")"

	cases := []struct {
		name string
		src  string
		want Object // expected /Contents in the outer dictionary
	}{
		{"before type", "<</Contents " + plainHex + "/Type/Sig>>", String(plain)},
		{"after type", "<</Type/Sig/Contents " + plainHex + ">>", String(plain)},
		{"timestamp", "<</Type/DocTimeStamp/Contents " + plainHex + ">>", String(plain)},
		{"byte range only", "<</ByteRange[0 1 2 3]/Contents " + plainHex + ">>", String(plain)},
		{"other type", "<</Type/Other/Contents " + cipherHex + ">>", String(plain)},
		{"no type", "<</Contents " + cipherHex + ">>", String(plain)},
		{"literal string", "<</Type/Sig/Contents " + literal + ">>", String(plain)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := newScanner(bytes.NewReader([]byte(c.src)), nil, enc)
			s.encRef = ref
			dict, err := s.ReadDict()
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(c.want, dict["Contents"]); diff != "" {
				t.Errorf("wrong /Contents (-want +got):\n%s", diff)
			}
		})
	}

	t.Run("nested", func(t *testing.T) {
		src := "<</V<</Type/Sig/Contents " + plainHex + "/Reason " + cipherHex + ">>/Contents " + cipherHex + ">>"
		s := newScanner(bytes.NewReader([]byte(src)), nil, enc)
		s.encRef = ref
		dict, err := s.ReadDict()
		if err != nil {
			t.Fatal(err)
		}
		if diff := cmp.Diff(String(plain), dict["Contents"]); diff != "" {
			t.Errorf("outer /Contents (-want +got):\n%s", diff)
		}
		v, ok := dict["V"].(Dict)
		if !ok {
			t.Fatalf("missing /V")
		}
		if diff := cmp.Diff(String(plain), v["Contents"]); diff != "" {
			t.Errorf("inner /Contents (-want +got):\n%s", diff)
		}
		if diff := cmp.Diff(String(plain), v["Reason"]); diff != "" {
			t.Errorf("inner /Reason (-want +got):\n%s", diff)
		}
	})
}

func TestSigContentsDuplicateKey(t *testing.T) {
	id := []byte("0123456789ABCDEF")
	ref := NewReference(5, 0)
	sec, err := createStdSecHandler(id, "secret", "supersecret", PermPrint, 128, 4, false)
	if err != nil {
		t.Fatal(err)
	}
	cf := &cryptFilter{Cipher: cipherRC4, Length: 128}
	enc := &encryptInfo{sec: sec, strF: cf, stmF: cf, efF: cf}
	plain := []byte("abc")
	cipherText, err := enc.EncryptBytes(ref, plain)
	if err != nil {
		t.Fatal(err)
	}
	src := fmt.Sprintf("<</Type/Sig/Contents <6161> /Contents <%x>>>", cipherText)
	s := newScanner(bytes.NewReader([]byte(src)), nil, enc)
	s.encRef = ref
	dict, err := s.ReadDict()
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(String(cipherText), dict["Contents"]); diff != "" {
		t.Errorf("wrong /Contents (-want +got):\n%s", diff)
	}
}

// TestReadStreamDataUnencryptedCrypt checks that a named /Crypt filter in an
// unencrypted file is dropped on read, while Identity is kept.
func TestReadStreamDataUnencryptedCrypt(t *testing.T) {
	for _, tc := range []struct {
		dict      string
		wantF     Object
		wantParms Object
	}{
		{"<< /Length 3 /Filter /Crypt /DecodeParms << /Name /StdCF >> >>", nil, nil},
		{"<< /Length 3 /Filter /Crypt /DecodeParms << /Name 1 >> >>", nil, nil},
		{"<< /Length 3 /Filter /Crypt /DecodeParms << /Name /Identity >> >>",
			Name("Crypt"), Dict{"Name": Name("Identity")}},
		{"<< /Length 3 /Filter /Crypt >>", Name("Crypt"), nil},
		{"<< /Length 3 /Filter [/Crypt /ASCIIHexDecode] /DecodeParms [<< /Name /StdCF >> null] >>",
			Name("ASCIIHexDecode"), nil},
	} {
		s := testScanner(tc.dict + "stream\nabc\nendstream\n")
		obj, err := s.ReadObject()
		if err != nil {
			t.Fatal(err)
		}
		stm, ok := obj.(*Stream)
		if !ok {
			t.Fatalf("got %T, want *Stream", obj)
		}
		if diff := cmp.Diff(tc.wantF, stm.Dict["Filter"]); diff != "" {
			t.Errorf("%s: wrong /Filter (-want +got):\n%s", tc.dict, diff)
		}
		if diff := cmp.Diff(tc.wantParms, stm.Dict["DecodeParms"]); diff != "" {
			t.Errorf("%s: wrong /DecodeParms (-want +got):\n%s", tc.dict, diff)
		}
	}
}

// TestReadIndirectObjectNoEndobj checks that an object whose "endobj" keyword
// is missing is still read, as long as more data follows.
func TestReadIndirectObjectNoEndobj(t *testing.T) {
	cases := []struct {
		body string
		want Native
	}{
		{"<</A 1>>\n4 0 obj\n<<>>\nendobj\n", Dict{"A": Integer(1)}},
		{"[1 2]\nxref\n0 1\n", Array{Integer(1), Integer(2)}},
		{"(abc)\n4 0 obj\n", String("abc")},
		{"5\n4 0 obj\n<<>>\nendobj\n", Integer(5)},
		{"5 6 endobj\n", Integer(5)},
		{"7 0 R\nxref\n0 1\n", NewReference(7, 0)},
		{"/N\n4 0 obj\n", Name("N")},
	}
	for _, c := range cases {
		obj, ref, err := testScanner("3 0 obj\n" + c.body).ReadIndirectObject()
		if err != nil {
			t.Errorf("%q: %v", c.body, err)
			continue
		}
		if ref != NewReference(3, 0) {
			t.Errorf("%q: got reference %s", c.body, ref)
		}
		if diff := cmp.Diff(c.want, obj); diff != "" {
			t.Errorf("%q: wrong object (-want +got):\n%s", c.body, diff)
		}
	}

	const stm = "3 0 obj\n<</Length 5>>\nstream\nhello\nendstream\n4 0 obj\n"
	obj, _, err := testScanner(stm).ReadIndirectObject()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := obj.(*Stream); !ok {
		t.Errorf("got %T, want *Stream", obj)
	}
}

// TestReadIndirectObjectInvalidRef checks that an object whose value is a
// reference to an object which cannot exist reads as null (PDF 7.3.10).
func TestReadIndirectObjectInvalidRef(t *testing.T) {
	for _, body := range []string{"99999999 0 R", "5 70000 R", "-1 0 R", "0 0 R", "0 5 R"} {
		obj, ref, err := testScanner("3 0 obj\n" + body + "\nendobj").ReadIndirectObject()
		if err != nil {
			t.Errorf("%q: %v", body, err)
			continue
		}
		if obj != nil {
			t.Errorf("%q: got %v, want nil", body, obj)
		}
		if ref != NewReference(3, 0) {
			t.Errorf("%q: got reference %s", body, ref)
		}
	}
}

// TestReadIndirectObjectNumberZero checks that an object definition with
// object number 0 is rejected as malformed (PDF 7.3.10).
func TestReadIndirectObjectNumberZero(t *testing.T) {
	for _, in := range []string{"0 0 obj\n1\nendobj", "0 65535 obj\n1\nendobj"} {
		_, _, err := testScanner(in).ReadIndirectObject()
		if !IsMalformed(err) {
			t.Errorf("%q: got %v, want MalformedFileError", in, err)
		}
	}
}

// TestReadObjectRefZero checks that references to object number 0 inside
// arrays and dictionaries read as null.
func TestReadObjectRefZero(t *testing.T) {
	cases := []struct {
		in   string
		want Native
	}{
		{"[0 0 R 2 0 R]", Array{nil, NewReference(2, 0)}},
		{"<</A 0 5 R/B 2 0 R>>", Dict{"B": NewReference(2, 0)}},
	}
	for _, c := range cases {
		obj, err := testScanner(c.in).ReadObject()
		if err != nil {
			t.Errorf("%q: %v", c.in, err)
			continue
		}
		if diff := cmp.Diff(c.want, obj); diff != "" {
			t.Errorf("%q: wrong object (-want +got):\n%s", c.in, diff)
		}
	}
}
