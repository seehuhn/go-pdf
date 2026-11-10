// Copyright 2023 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package pdf

import "testing"

func TestVersion(t *testing.T) {
	cases := []struct {
		in        string
		out       Version
		ok        bool
		supported bool
	}{
		{"1.0", V1_0, true, true},
		{"1.1", V1_1, true, true},
		{"1.2", V1_2, true, true},
		{"1.3", V1_3, true, true},
		{"1.4", V1_4, true, true},
		{"1.5", V1_5, true, true},
		{"1.6", V1_6, true, true},
		{"1.7", V1_7, true, true},
		{"2.0", V2_0, true, true},
		{"1.8", 108, true, false},
		{"2.3", 203, true, false},
		{"3.0", 300, true, false},
		{"", 0, false, false},
		{"0.9", 0, false, false},
		{"1.50", 0, false, false},
		{"10.0", 0, false, false},
		{"2.", 0, false, false},
	}
	for _, test := range cases {
		v, err := ParseVersion(test.in)
		if (err == nil) != test.ok {
			t.Errorf("%q: unexpected err = %s", test.in, err)
			continue
		}
		if v != test.out {
			t.Errorf("%q: wrong version %d != %d", test.in, int(v), int(test.out))
			continue
		}
		if !test.ok {
			continue
		}
		if got := v.String(); got != test.in {
			t.Errorf("%q: wrong String() %q", test.in, got)
		}
		if got := v.IsSupported(); got != test.supported {
			t.Errorf("%q: IsSupported() = %v, want %v", test.in, got, test.supported)
		}
		s, err := v.ToString()
		if test.supported != (err == nil) {
			t.Errorf("%q: unexpected ToString error %v", test.in, err)
			continue
		}
		if test.supported && s != test.in {
			t.Errorf("wrong version %q != %q", s, test.in)
		}
	}
}

// TestVersionOrder checks that the integer order of Version values agrees
// with the order of PDF versions, including versions beyond MaxVersion.
func TestVersionOrder(t *testing.T) {
	inOrder := []Version{0, V1_0, V1_1, V1_7, 108, V2_0, 203}
	for i := 1; i < len(inOrder); i++ {
		if inOrder[i-1] >= inOrder[i] {
			t.Errorf("%v and %v are out of order", inOrder[i-1], inOrder[i])
		}
	}
}
