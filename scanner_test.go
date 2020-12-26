package pdflib

import (
	"bytes"
	"errors"
	"io"
	"testing"
)

func testScanner(contents string) *scanner {
	buf := bytes.NewReader([]byte(contents))
	return newScanner(buf)
}

func TestRefill(t *testing.T) {
	n := scannerBufSize + 2
	buf := make([]byte, n)
	s := newScanner(bytes.NewReader(buf))

	for _, inc := range []int{0, 1, scannerBufSize, 1} {
		s.pos += inc
		err := s.refill()
		total := int(s.total) + s.pos
		expectUsed := scannerBufSize
		if expectUsed > n-total {
			expectUsed = n - total
		}
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
	cases := []struct {
		in  string
		val Object
		ok  bool
		err error
	}{
		{"", nil, false, io.EOF},
		{"null", nil, true, nil},

		{"true", Bool(true), true, nil},
		{"false", Bool(false), true, nil},
		{"TRUE", nil, false, nil},
		{"FALSE", nil, false, nil},

		{"0", Integer(0), true, nil},
		{"+0", Integer(0), true, nil},
		{"-0", Integer(0), true, nil},
		{"1", Integer(1), true, nil},
		{"+1", Integer(1), true, nil},
		{"-1", Integer(-1), true, nil},
		{"12", Integer(12), true, nil},
		{"+12", Integer(12), true, nil},
		{"-12", Integer(-12), true, nil},
		{"123", Integer(123), true, nil},
		{"-4567", Integer(-4567), true, nil},
		{"999999999999999999", Integer(999999999999999999), true, nil},
		{"-999999999999999999", Integer(-999999999999999999), true, nil},

		{".5", Real(.5), true, nil},
		{"+.5", Real(.5), true, nil},
		{"-.5", Real(-.5), true, nil},
		{"0.5", Real(.5), true, nil},
		{"+0.5", Real(.5), true, nil},
		{"-0.5", Real(-.5), true, nil},
		{".", nil, false, nil},
		{".+5", nil, false, nil},

		{"/a", Name("a"), true, nil},
		{"/1234567890123456789012345678901", Name("1234567890123456789012345678901"), true, nil},
		{"/12345678901234567890123456789012", Name("12345678901234567890123456789012"), true, nil},
		{"/123456789012345678901234567890123", Name("123456789012345678901234567890123"), true, nil},
		{"/A;Name_With-Various***Characters?", Name("A;Name_With-Various***Characters?"), true, nil},
		{"/1.2", Name("1.2"), true, nil},
		{"/A#42", Name("AB"), true, nil},
		{"/F#23#20minor", Name("F# minor"), true, nil},
		{"/ß", Name("ß"), true, nil},
		{"/", Name(""), true, nil},

		{"fals", nil, false, nil},
		{"abc", nil, false, nil},
	}

	for _, test := range cases {
		for _, suffix := range []string{"", " 1\n"} {
			body := test.in + suffix
			file := testScanner(body)

			val, err := file.ReadObject()
			if val != test.val {
				t.Errorf("%q: wrong value: expected %s, got %s",
					body, format(test.val), format(val))
			}

			if test.ok && err != nil {
				t.Errorf("%q: unexpected error %q", body, err)
			} else if !test.ok {
				var e2 *MalformedFileError
				if err == nil {
					t.Errorf("%q: missing error", body)
				} else if errors.As(err, &e2) {
					if test.err != nil && !errors.Is(err, test.err) {
						t.Errorf("%q: error does not wrap %q",
							body, test.err)
					}
				} else {
					t.Errorf("%q: wrong error %q", body, err)
				}
			}
		}
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
		for _, suffix := range []string{"", "x y\n"} {
			body := test + suffix
			s := testScanner(body)

			err := s.SkipWhiteSpace()
			if err != nil {
				t.Errorf("%q: unexpected error: %s", body, err)
			}
			total := int(s.total) + s.pos
			if total != len(test) {
				t.Errorf("%q: wrong position %d", body, total)
			}
		}
	}
}
