package pdflib

import (
	"bytes"
	"errors"
	"io"
	"io/ioutil"
	"reflect"
	"strings"
	"testing"
)

func testScanner(contents string) *scanner {
	buf := bytes.NewReader([]byte(contents))
	return newScanner(buf, nil)
}

func TestRefill(t *testing.T) {
	n := scannerBufSize + 2
	buf := make([]byte, n)
	s := newScanner(bytes.NewReader(buf), nil)

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

		{`()`, String(""), true, nil},
		{"(test string)", String("test string"), true, nil},
		{`(hello)`, String("hello"), true, nil},
		{`(he(ll)o)`, String("he(ll)o"), true, nil},
		{`(he\)ll\(o)`, String("he)ll(o"), true, nil},
		{"(hello\n)", String("hello\n"), true, nil},
		{"(hello\r)", String("hello\n"), true, nil},
		{"(hello\r\n)", String("hello\n"), true, nil},
		{"(hello\n\r)", String("hello\n\n"), true, nil},
		{"(hell\\\no)", String("hello"), true, nil},
		{`(h\145llo)`, String("hello"), true, nil},
		{`(\0612)`, String("12"), true, nil},

		{"<>", String(""), true, nil},
		{"<68656c6c6f>", String("hello"), true, nil},
		{"<68656C6C6F>", String("hello"), true, nil},
		{"<68 65 6C 6C 6F>", String("hello"), true, nil},
		{"<68656C70>", String("help"), true, nil},
		{"<68656C7>", String("help"), true, nil},

		{"[1 2 3]", Array{Integer(1), Integer(2), Integer(3)}, true, nil},
		{"[1 2 3 R 4]", Array{Integer(1), &Reference{2, 3}, Integer(4)}, true, nil},

		{"<< /key 12 /val /23 >>", Dict{
			Name("key"): Integer(12),
			Name("val"): Name("23"),
		}, true, nil},
		{"<< /key1 1 /key2 2 2 R /key3 3 >>", Dict{
			Name("key1"): Integer(1),
			Name("key2"): &Reference{2, 2},
			Name("key3"): Integer(3),
		}, true, nil},

		{"<< /Length 5 >>\nstream\nhello\nendstream", &Stream{
			Dict: Dict{
				Name("Length"): Integer(5),
			},
			R: strings.NewReader("hello"),
		}, true, nil},

		{"fals", nil, false, nil},
		{"abc", nil, false, nil},
	}

	for _, test := range cases {
		for _, suffix := range []string{"", " 1\n"} {
			body := test.in + suffix
			file := testScanner(body)

			val, err := file.ReadObject()
			if s2, ok := test.val.(*Stream); ok {
				s1, ok := val.(*Stream)
				if !ok {
					t.Errorf("%q: wront type: expected *Stream, got %T",
						body, val)
					continue
				}
				if !reflect.DeepEqual(s1.Dict, s2.Dict) {
					t.Errorf("%q: wrong value: expected %#v, got %#v",
						body, s2.Dict, s1.Dict)
					continue
				}
				data1, err := ioutil.ReadAll(s1.R)
				if err != nil {
					t.Errorf("%q: %s", body, err)
				}

				// rewind the reader for the second suffix
				s2r := s2.R.(io.Seeker)
				_, _ = s2r.Seek(0, io.SeekStart)

				data2, err := ioutil.ReadAll(s2.R)
				if err != nil {
					t.Errorf("%q: %s", body, err)
				}
				if !bytes.Equal(data1, data2) {
					t.Errorf("%q: wrong data in stream, expected %x, got %x",
						body, data2, data1)
				}
			} else if !reflect.DeepEqual(val, test.val) {
				t.Errorf("%q: wrong value: expected %#v, got %#v",
					body, test.val, val)
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
