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

package cff

import (
	"math"
	"reflect"
	"testing"
)

func TestDictDecodeFloat(t *testing.T) {
	cases := []struct {
		in  []byte
		out float64
	}{
		{[]byte{0xe2, 0xa2, 0x5f}, -2.25},
		{[]byte{0x0a, 0x14, 0x05, 0x41, 0xc3, 0xff}, 0.140541e-3},
	}
	for _, test := range cases {
		buf, x, err := decodeFloat(test.in)
		if err != nil {
			t.Error(err)
			continue
		}
		if len(buf) != 0 {
			t.Error("not all input used")
		}
		if math.Abs(x-test.out) > 1e-6 {
			t.Errorf("wrong result: %g - %g = %g", x, test.out, x-test.out)
		}
	}
}

func TestDictDecodeInt(t *testing.T) {
	cases := []struct {
		x   int32
		enc []byte
	}{
		{0, []byte{0x8b}},
		{100, []byte{0xef}},
		{-100, []byte{0x27}},
		{1000, []byte{0xfa, 0x7c}},
		{-1000, []byte{0xfe, 0x7c}},
		{10000, []byte{0x1c, 0x27, 0x10}},
		{-10000, []byte{0x1c, 0xd8, 0xf0}},
		{100000, []byte{0x1d, 0x00, 0x01, 0x86, 0xa0}},
		{-100000, []byte{0x1d, 0xff, 0xfe, 0x79, 0x60}},
	}
	var buf []byte
	for _, test := range cases {
		buf = append(buf[:0], test.enc...)
		buf = append(buf, byte(opDebug>>8), byte(opDebug&0xFF))

		d, err := decodeDict(buf, nil)
		if err != nil {
			t.Error(err)
			continue
		}
		if len(d) != 1 {
			t.Error("wrong DICT length")
			continue
		}

		args, ok := d[opDebug]
		if !ok {
			t.Error("wrong DICT op")
			continue
		}
		if len(args) != 1 {
			t.Error("wrong DICT args length")
			continue
		}

		x := args[0].(int32)
		if x != test.x {
			t.Errorf("wrong value: %d != %d", x, test.x)
		}
	}
}

func TestDictEncodeInt(t *testing.T) {
	var op dictOp = 7
	for i := int32(-32769); i <= 32769; i += 3 {
		d := cffDict{
			op: []interface{}{i, i + 1, i + 2},
		}
		blob := d.encode(nil)
		d2, err := decodeDict(blob, nil)
		if err != nil {
			t.Fatal(err)
		}

		if len(d2) != 1 {
			t.Fatal("wrong length")
		}
		args, ok := d2[op]
		if !ok {
			t.Fatal("wrong op")
		}
		if len(d[op]) != len(args) {
			t.Errorf("wrong args count: %d != %d",
				len(d[op]), len(args))
		}
		for i, x := range args {
			if x.(int32) != d[op][i].(int32) {
				t.Fatalf("wrong value: %d != %d",
					x.(int32), d[op][i].(int32))
			}
		}
	}
}

func TestEncodeFloat(t *testing.T) {
	cases := []float64{
		0,
		1,
		-1,
		2,
		-2,
		999999,
		-999999,
		3.1415926535,
		1.234e56,
		1.234e-56,
		-1.234e56,
		-1.234e-56,
	}
	for _, x := range cases {
		d := cffDict{
			opDebug: []interface{}{x},
		}
		blob := d.encode(nil)
		d2, err := decodeDict(blob, nil)
		if err != nil {
			t.Fatal(err)
		}

		if len(d2) != 1 {
			t.Fatalf("wrong length %d", len(d2))
		}
		args, ok := d2[opDebug]
		if !ok {
			t.Fatal("wrong op")
		}
		if len(args) != 1 {
			t.Errorf("wrong args count: %d != %d",
				len(args), len(d[0]))
		}
		out := args[0].(float64)
		if math.Abs(out-x) > 1e-6 || math.Abs(out-x) > 1e-6*(math.Abs(out)+math.Abs(x)) {
			t.Errorf("%g != %g", out, x)
		}
	}
}

func FuzzDict(f *testing.F) {
	f.Fuzz(func(t *testing.T, data []byte) {
		ss := &cffStrings{}

		d1, err := decodeDict(data, ss)
		if err != nil {
			return
		}

		data2 := d1.encode(ss)
		if len(ss.data) != 0 {
			t.Errorf("%d strings appeared out of thin air", len(ss.data))
		}
		if len(data2) > len(data) {
			t.Errorf("inefficient encoding")
		}

		d2, err := decodeDict(data2, ss)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(d1, d2) {
			t.Errorf("%#v != %#v", d1, d2)
		}
	})
}
