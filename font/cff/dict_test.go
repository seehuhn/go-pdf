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
