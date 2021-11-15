package cff

import "testing"

func TestRoll(t *testing.T) {
	in := []int32{1, 2, 3, 4, 5, 6, 7, 8}
	out := []int32{1, 2, 4, 5, 6, 3, 7, 8}
	roll(in[2:6], 3)
	for i, x := range in {
		if out[i] != x {
			t.Error(in, out)
			break
		}
	}
}
