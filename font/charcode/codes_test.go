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

package charcode

import (
	"bytes"
	"testing"

	"seehuhn.de/go/pdf"
)

func TestFirstCode(t *testing.T) {
	cs := CodeSpaceRange{
		{Low: []byte{1}, High: []byte{8}},
		{Low: []byte{10, 1, 1, 1, 1}, High: []byte{10, 1, 1, 9, 9}},
		{Low: []byte{10, 1, 1, 10, 10}, High: []byte{10, 1, 1, 20, 20}},
	}

	type testCase struct {
		in  pdf.String
		out pdf.String
		k   int
	}
	testCases := []testCase{
		{in: nil, out: nil, k: 0},
		{in: pdf.String{}, out: nil, k: 0},
		{in: pdf.String{0}, out: nil, k: 1},
		{in: pdf.String{1}, out: pdf.String{1}, k: 1},
		{in: pdf.String{2}, out: pdf.String{2}, k: 1},
		{in: pdf.String{9}, out: nil, k: 1},
		{in: pdf.String{10, 1, 1, 1, 1, 255}, out: pdf.String{10, 1, 1, 1, 1}, k: 5},
		{in: pdf.String{10, 1, 1, 2, 2}, out: pdf.String{10, 1, 1, 2, 2}, k: 5},
		{in: pdf.String{10, 1, 1, 5}, out: nil, k: 1},
		{in: pdf.String{10, 1, 1, 1, 10}, out: nil, k: 1},
		{in: pdf.String{10, 1, 1}, out: nil, k: 1},
	}
	for _, tc := range testCases {
		out, k := cs.FirstCode(tc.in)
		if k != tc.k {
			t.Errorf("%v: expected %d bytes, got %d", tc.in, tc.k, k)
		}
		if !bytes.Equal(out, tc.out) || (out == nil) != (tc.out == nil) {
			t.Errorf("%v: expected %v, got %v", tc.in, tc.out, out)
		}
	}
}

func TestCustom(t *testing.T) {
	cs := CodeSpaceRange{
		{Low: []byte{0x0, 0x0}, High: []byte{0xff, 0xf0}},
	}

	if len(cs) != 1 {
		t.Errorf("expected 1 custom range, got %d", len(cs))
	}
	if cs[0].numCodes() != (0xf0-0x00+1)*(0xff-0x00+1) {
		t.Errorf("expected %d codes, got %d", (0xf0-0x00+1)*(0xff-0x00+1), cs[0].numCodes())
	}
}

func TestCustom2(t *testing.T) {
	cs := CodeSpaceRange{
		{Low: []byte{0x10, 0x80}, High: []byte{0x30, 0xA0}},
	}

	seen := make(map[CharCode]bool)
	var buf []byte
	for c1 := byte(0x10); c1 <= 0x30; c1++ {
		for c2 := byte(0x80); c2 <= 0xA0; c2++ {
			buf = append(buf[:0], c1, c2)
			code, k := cs.Decode(buf)
			if k != 2 {
				t.Errorf("expected 2 bytes, got %d", k)
			}
			if seen[code] {
				t.Errorf("code %d seen twice", code)
			}
			seen[code] = true

			buf = cs.Append(buf[:0], code)
			if !bytes.Equal(buf, []byte{c1, c2}) {
				t.Fatalf("expected %v, got %v", []byte{c1, c2}, buf)
			}
		}
	}
}

func TestCustom3(t *testing.T) {
	cs := CodeSpaceRange{ // from the EUC-H cmap
		{Low: []byte{0x00}, High: []byte{0x80}},
		{Low: []byte{0x8E, 0xA0}, High: []byte{0x8E, 0xDF}},
		{Low: []byte{0xA1, 0xA1}, High: []byte{0xFE, 0xFE}},
	}

	testCases := [][]byte{
		{0x00},
		{0x41},
		{0x80},
		{0x8E, 0xA0},
		{0x8E, 0xDF},
		{0xA1, 0xA1},
		{0xC1, 0xD2},
		{0xFE, 0xFE},
	}
	for _, in := range testCases {
		code, k := cs.Decode(in)
		if k != len(in) {
			t.Errorf("expected %d bytes, got %d", len(in), k)
		} else if code < 0 {
			t.Errorf("expected positive code, got %d", code)
		}
		out := cs.Append(nil, code)
		if !bytes.Equal(in, out) {
			t.Fatalf("%d: expected <%x>, got <%x>", code, in, out)
		}
	}

	for code := CharCode(0); code < 1000; code++ {
		buf := cs.Append(nil, code)
		code2, k := cs.Decode(buf)
		if k != len(buf) {
			t.Errorf("expected %d bytes, got %d", len(buf), k)
		}
		if code2 != code {
			t.Errorf("expected code %d, got %d", code, code2)
		}
	}
}
