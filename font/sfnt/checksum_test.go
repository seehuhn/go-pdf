package sfnt

import "testing"

func TestChecksum(t *testing.T) {
	cases := []struct {
		Body     []byte
		Expected uint32
	}{
		{[]byte{0, 1, 2, 3}, 0x00010203},
		{[]byte{0, 1, 2, 3, 4, 5, 6, 7}, 0x0406080a},
		{[]byte{1}, 0x01000000},
		{[]byte{1, 2, 3}, 0x01020300},
		{[]byte{1, 0, 0, 0, 1}, 0x02000000},
		{[]byte{255, 255, 255, 255, 0, 0, 0, 1}, 0},
	}

	for i, test := range cases {
		computed := Checksum(test.Body)
		if computed != test.Expected {
			t.Errorf("test %d failed: %08x != %08x",
				i+1, computed, test.Expected)
		}
	}
}
