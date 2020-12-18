package pdflib

import (
	"bytes"
	"fmt"
	"os"
	"reflect"
	"testing"
)

func newReader(contents string) *Reader {
	buf := bytes.NewReader([]byte(contents))
	return &Reader{
		size: buf.Size(),
		r:    buf,
	}
}

func TestExpectWord(t *testing.T) {
	cases := []struct {
		in  string
		err error
	}{
		{"", errMalformed},
		{"tes", errMalformed},
		{"test", nil},
		{"teste", errMalformed},
	}

	for _, test := range cases {
		for _, suffix := range []string{"", " tast\n"} {
			file := newReader(test.in + suffix)

			pos, err := file.expectWord(0, "test")
			target := int64(len(test.in))
			if test.err == errMalformed {
				target = 0
			}
			if pos != target {
				t.Errorf("wrong position: expected %d, got %d", len(test.in), pos)
			}
			if err != test.err {
				if test.err == nil {
					t.Errorf("unexpected error: %s", err.Error())
				} else {
					t.Errorf("missing error: %s", test.err.Error())
				}
			}
		}
	}
}

func TestExpectBool(t *testing.T) {
	cases := []struct {
		in  string
		val bool
		err error
	}{
		{"true", true, nil},
		{"false", false, nil},
		{"truee", false, errMalformed},
		{"fals", false, errMalformed},
		{"", false, errMalformed},
	}

	for _, test := range cases {
		for _, suffix := range []string{"", " 1\n"} {
			file := newReader(test.in + suffix)

			pos, val, err := file.expectBool(0)
			target := int64(len(test.in))
			if test.err == errMalformed {
				target = 0
			}
			if pos != target {
				t.Errorf("wrong position: expected %d, got %d", len(test.in), pos)
			}
			if val != Bool(test.val) {
				t.Errorf("wrong value: expected %t, got %t", test.val, val)
			}
			if err != test.err {
				if test.err == nil {
					t.Errorf("unexpected error: %s", err.Error())
				} else {
					t.Errorf("missing error: %s", test.err.Error())
				}
			}
		}
	}
}

func TestExpectInt(t *testing.T) {
	cases := []struct {
		in  string
		val int64
		err error
	}{
		{"0", 0, nil},
		{"123", 123, nil},
		{"-4567", -4567, nil},
		{"", 0, errMalformed},
		{"+0", 0, nil},
		{"-0", 0, nil},
		{"+1", 1, nil},
		{"-1", -1, nil},
		{"999999999999999999", 999999999999999999, nil},
		{"-999999999999999999", -999999999999999999, nil},
	}

	for _, test := range cases {
		for _, suffix := range []string{"", " 1\n"} {
			file := newReader(test.in + suffix)

			pos, val, err := file.expectInteger(0)
			if pos != int64(len(test.in)) {
				t.Errorf("wrong position: expected %d, got %d", len(test.in), pos)
			}
			if val != test.val {
				t.Errorf("wrong value: expected %d, got %d", test.val, val)
			}
			if err != test.err {
				if test.err == nil {
					t.Errorf("unexpected error: %s", err.Error())
				} else {
					t.Errorf("missing error: %s", test.err.Error())
				}
			}
		}
	}
}

func TestExpectName(t *testing.T) {
	cases := []struct {
		in  string
		out string
		err error
	}{
		{"", "", errMalformed},
		{"abc", "", errMalformed},
		{"/a", "a", nil},
		{"/1234567890123456789012345678901", "1234567890123456789012345678901", nil},
		{"/12345678901234567890123456789012", "12345678901234567890123456789012", nil},
		{"/123456789012345678901234567890123", "123456789012345678901234567890123", nil},
		{"/A;Name_With-Various***Characters?", "A;Name_With-Various***Characters?", nil},
		{"/1.2", "1.2", nil},
		{"/A#42", "AB", nil},
		{"/F#23#20minor", "F# minor", nil},
		{"/ß", "ß", nil},
		{"/", "", nil},
	}

	for _, test := range cases {
		for _, suffix := range []string{"", "(", " "} {
			file := newReader(test.in + suffix)

			pos, val, err := file.expectName(0)
			newPos := int64(len(test.in))
			if test.err == errMalformed {
				newPos = 0
			}
			if pos != newPos {
				t.Errorf("wrong position: expected %d, got %d", len(test.in), pos)
			}
			if val != Name(test.out) {
				t.Errorf("wrong value: expected %s, got %s", test.out, val)
			}
			if err != test.err {
				if test.err == nil {
					t.Errorf("unexpected error: %s", err.Error())
				} else {
					t.Errorf("missing error: %s", test.err.Error())
				}
			}
		}
	}
}

func TestExpectWhiteSpaceMaybe(t *testing.T) {
	cases := []string{
		"",
		" ",
		"               ",   // one shorter than blocksize
		"                ",  // blocksize
		"                 ", // one longer than blocksize
		"\r",
		"\n",
		"% comment\r\n",
		" % comment\r\n % comment\r\n % comment\r\n   ",
	}

	for _, test := range cases {
		for _, suffix := range []string{"", "x y\n"} {
			file := newReader(test + suffix)

			pos, err := file.expectWhiteSpaceMaybe(0)
			if err != nil {
				t.Errorf("unexpected error: %s", err.Error())
			}
			if pos != int64(len(test)) {
				t.Errorf("wrong position: expected %d, got %d", len(test), pos)
			}
		}
	}
}

func TestExpectNumericOrReference(t *testing.T) {
	cases := []struct {
		in  string
		pos int64
		val Object
		err error
	}{
		{"", 0, nil, errMalformed},
		{"12", 2, Integer(12), nil},
		{"+12", 3, Integer(12), nil},
		{"-12", 3, Integer(-12), nil},
		{".5", 2, Real(.5), nil},
		{"+.5", 3, Real(.5), nil},
		{"-.5", 3, Real(-.5), nil},
		{".+5", 0, nil, errMalformed},
		{"1 .+5 R", 1, Integer(1), nil},
		{"1 2 R", 5, &Reference{1, 2}, nil},
	}

	for _, test := range cases {
		for _, suffix := range []string{"", " 0\n", " 0 S\n", " R"} {
			file := newReader(test.in + suffix)

			pos, val, err := file.expectNumericOrReference(0)
			if pos != test.pos {
				t.Errorf("wrong position: expected %d, got %d", len(test.in), pos)
			}
			if !reflect.DeepEqual(val, test.val) {
				t.Errorf("wrong value: expected %#v, got %#v", test.val, val)
			}
			if err != test.err {
				if err != nil {
					t.Errorf("unexpected error: %s", err.Error())
				} else {
					t.Errorf("missing error: %s", err)
				}
			}
		}
	}
}

func TestExpectQuotedString(t *testing.T) {
	cases := []struct {
		in  string
		out string
	}{
		{`()`, ""},
		{`(hello)`, "hello"},
		{`(he(ll)o)`, "he(ll)o"},
		{"(hello\n)", "hello\n"},
		{"(hello\r)", "hello\n"},
		{"(hello\r\n)", "hello\n"},
		{"(hello\n\r)", "hello\n\n"},
		{"(hell\\\no)", "hello"},
		{`(h\145llo)`, "hello"},
		{`(\0612)`, "12"},
	}

	for _, test := range cases {
		file := newReader(test.in + " 1")

		pos, val, err := file.expectQuotedString(0)
		if pos != int64(len(test.in)) {
			t.Errorf("wrong position: expected %d, got %d", len(test.in), pos)
		}
		if val != String(test.out) {
			t.Errorf("wrong value: expected %q, got %q", test.out, val)
		}
		if err != nil {
			t.Errorf("unexpected error: %s", err.Error())
		}
	}
}

func TestExpectHexString(t *testing.T) {
	cases := []struct {
		in  string
		out string
	}{
		{"<>", ""},
		{"<68656c6c6f>", "hello"},
		{"<68656C6C6F>", "hello"},
		{"<68 65 6C 6C 6F>", "hello"},
		{"<68656C70>", "help"},
		{"<68656C7>", "help"},
	}

	for _, test := range cases {
		file := newReader(test.in + " 1")

		pos, val, err := file.expectHexString(0)
		if pos != int64(len(test.in)) {
			t.Errorf("wrong position: expected %d, got %d", len(test.in), pos)
		}
		if val != String(test.out) {
			t.Errorf("wrong value: expected %q, got %q", test.out, val)
		}
		if err != nil {
			t.Errorf("unexpected error: %s", err.Error())
		}
	}
}

func TestFile(t *testing.T) {
	// fd, err := os.Open("PDF32000_2008.pdf")
	fd, err := os.Open("example.pdf")
	if err != nil {
		t.Fatal(err)
	}
	defer fd.Close()

	fi, err := fd.Stat()
	if err != nil {
		t.Fatal(err)
	}

	file, err := NewReader(fd, fi.Size())
	if err != nil {
		t.Fatal(err)
	}

	fmt.Println(string(file.Trailer.PDF()))

	t.Error("fish")
}
