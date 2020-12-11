package pdflib

import (
	"bytes"
	"fmt"
	"os"
	"testing"
)

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
			buf := bytes.NewReader([]byte(test.in + suffix))
			file, err := newFile(buf)
			if err != nil {
				t.Fatal(err)
			}

			pos, val, err := file.expectInteger(0)
			if pos != int64(len(test.in)) {
				t.Errorf("wrong position: expected %d, got %d", len(test.in), pos)
			}
			if val != test.val {
				t.Errorf("wrong value: expected %d, got %d", test.val, val)
			}
			if err != test.err {
				t.Errorf("unexpected error: %s", err.Error())
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
			buf := bytes.NewReader([]byte(test.in + suffix))
			file, err := newFile(buf)
			if err != nil {
				t.Fatal(err)
			}

			pos, val, err := file.expectName(0)
			newPos := int64(len(test.in))
			if test.err == errMalformed {
				newPos = 0
			}
			if pos != newPos {
				t.Errorf("wrong position: expected %d, got %d", len(test.in), pos)
			}
			if val != test.out {
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

func TestSkipWhiteSpace(t *testing.T) {
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
			buf := bytes.NewReader([]byte(test + suffix))
			file, err := newFile(buf)
			if err != nil {
				t.Fatal(err)
			}

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

func TestFile(t *testing.T) {
	fd, err := os.Open("PDF32000_2008.pdf")
	if err != nil {
		t.Fatal(err)
	}
	defer fd.Close()

	file, err := NewFile(fd)
	if err != nil {
		t.Fatal(err)
	}

	pos, err := file.findXRef()
	if err != nil {
		t.Fatal(err)
	}
	pos, err = file.expectXRef(pos)
	if err != nil {
		t.Fatal(err)
	}
	pos, err = file.expectTrailer(pos)
	if err != nil {
		t.Fatal(err)
	}

	fmt.Println(file.Size - pos)

	t.Error("fish")
}
