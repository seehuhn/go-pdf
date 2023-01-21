package pdf

import (
	"bytes"
	"strings"
	"testing"
)

func TestRectangle1(t *testing.T) {
	type testCase struct {
		in  string
		out *Rectangle
	}
	cases := []testCase{
		{"[0 0 0 0]", &Rectangle{0, 0, 0, 0}},
		{"[1 2 3 4]", &Rectangle{1, 2, 3, 4}},
		{"[1.0 2.0 3.0 4.0]", &Rectangle{1, 2, 3, 4}},
		{"[1.1 2.2 3.3 4.4]", &Rectangle{1.1, 2.2, 3.3, 4.4}},
		{"[1.11 2.22 3.33 4.44]", &Rectangle{1.11, 2.22, 3.33, 4.44}},
		{"[1 2.222 3.333 4.4444]", &Rectangle{1, 2.222, 3.333, 4.4444}},
	}
	for _, test := range cases {
		t.Run(test.in, func(t *testing.T) {
			r := strings.NewReader(test.in)
			s := newScanner(r, 0, nil, nil)
			obj, err := s.ReadObject()
			if err != nil {
				t.Fatal(err)
			}

			rect, err := obj.(Array).AsRectangle()

			if err != nil {
				t.Errorf("Decode(%q) returned error %v", test.in, err)
			}
			if !rect.NearlyEqual(test.out, 1e-6) {
				t.Errorf("Decode(%q) = %v, want %v", test.in, rect, *test.out)
			}
		})
	}
}

func TestRectangle2(t *testing.T) {
	cases := []*Rectangle{
		{0, 0, 0, 0},
		{1, 2, 3, 4},
		{0.5, 1.5, 2.5, 3.5},
		{0.5005, 1.5005, 2.5005, 3.5005},
		{1.0 / 3.0, 1.5, 2.5, 3.5},
	}
	for _, test := range cases {
		t.Run(test.String(), func(t *testing.T) {
			buf := &bytes.Buffer{}
			err := test.PDF(buf)
			if err != nil {
				t.Fatal(err)
			}

			s := newScanner(buf, 0, nil, nil)
			obj, err := s.ReadObject()
			if err != nil {
				t.Fatal(err)
			}

			rect, err := obj.(Array).AsRectangle()
			if err != nil {
				t.Errorf("Decode(%q) returned error %v", test.String(), err)
			}

			if !rect.NearlyEqual(test, .5e-2) {
				t.Errorf("Decode(%q) = %v, want %v", test.String(), rect, test)
			}
		})
	}
}
