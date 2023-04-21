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
			s := newScanner(r, nil, nil)
			obj, err := s.ReadObject()
			if err != nil {
				t.Fatal(err)
			}

			rect, err := asRectangle(nil, obj.(Array))

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

			s := newScanner(buf, nil, nil)
			obj, err := s.ReadObject()
			if err != nil {
				t.Fatal(err)
			}

			rect, err := asRectangle(nil, obj.(Array))
			if err != nil {
				t.Errorf("Decode(%q) returned error %v", test.String(), err)
			}

			if !rect.NearlyEqual(test, .5e-2) {
				t.Errorf("Decode(%q) = %v, want %v", test.String(), rect, test)
			}
		})
	}
}
