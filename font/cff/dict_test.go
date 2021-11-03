package cff

import (
	"math"
	"testing"
)

func TestDecodeFloat(t *testing.T) {
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

func TestDecodeInt(t *testing.T) {
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
		buf = append(buf, 0)

		d, err := decodeDict(buf)
		if err != nil {
			t.Error(err)
			continue
		}
		if len(d) != 1 {
			t.Error("wrong DICT length")
			continue
		}

		e := d[0]
		if e.op != 0 {
			t.Error("wrong DICT op")
			continue
		}
		if len(e.args) != 1 {
			t.Error("wrong DICT args length")
			continue
		}

		x := e.args[0].(int32)
		if x != test.x {
			t.Errorf("wrong value: %d != %d", x, test.x)
		}
	}
}

func TestEncodeInt(t *testing.T) {
	var op uint16 = 7
	for i := int32(-32769); i <= 32769; i += 3 {
		d := cffDict{
			cffDictEntry{
				op:   op,
				args: []interface{}{i, i + 1, i + 2},
			},
		}
		blob := d.encode()
		d2, err := decodeDict(blob)
		if err != nil {
			t.Fatal(err)
		}

		if len(d2) != 1 {
			t.Fatal("wrong length")
		}
		if d2[0].op != d[0].op {
			t.Errorf("wrong op: %d != %d", d2[0].op, d[0].op)
		}
		if len(d[0].args) != len(d2[0].args) {
			t.Errorf("wrong args count: %d != %d",
				len(d2[0].args), len(d[0].args))
		}
		for i, x := range d[0].args {
			if x.(int32) != d2[0].args[i].(int32) {
				t.Fatalf("wrong value: %d != %d",
					x.(int32), d2[0].args[i].(int32))
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
			cffDictEntry{
				args: []interface{}{x},
			},
		}
		blob := d.encode()
		d2, err := decodeDict(blob)
		if err != nil {
			t.Fatal(err)
		}

		if len(d2) != 1 {
			t.Fatalf("wrong length %d", len(d2))
		}
		e := d2[0]
		if e.op != 0 {
			t.Errorf("wrong op: %d != %d", e.op, 0)
		}
		if len(e.args) != 1 {
			t.Errorf("wrong args count: %d != %d",
				len(e.args), len(d[0].args))
		}
		out := e.args[0].(float64)
		if math.Abs(out-x) > 1e-6 || math.Abs(out-x) > 1e-6*(math.Abs(out)+math.Abs(x)) {
			t.Errorf("%g != %g", out, x)
		}
	}
}
