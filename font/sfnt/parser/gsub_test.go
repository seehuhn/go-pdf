package parser

import (
	"testing"

	"seehuhn.de/go/pdf/font"
)

func TestGsub4_1(t *testing.T) {
	in := []font.GlyphID{
		0, 0, 1, 2, 3, 1, 2, 4, 1, 2, 0, 0, 2, 1, 0, 0,
	}
	expected := []font.GlyphID{
		0, 0, 123, 124, 1, 2, 0, 0, 21, 0, 0,
	}
	gsub := &gsub4_1{
		flags: 0,
		cov: map[font.GlyphID]int{
			1: 0,
			2: 1,
		},
		repl: [][]ligature{
			{
				ligature{
					in:  []font.GlyphID{2, 2},
					out: 122,
				},
				ligature{
					in:  []font.GlyphID{2, 3},
					out: 123,
				},
				ligature{
					in:  []font.GlyphID{2, 4},
					out: 124,
				},
			},
			{
				ligature{
					in:  []font.GlyphID{1},
					out: 21,
				},
			},
		},
	}
	pos := 0
	for pos < len(in) {
		next, out := gsub.Replace(pos, in)
		if next == pos {
			if !isEqual(in, out) {
				t.Errorf("change without progress: %d vs %d",
					in, out)
			}
			pos++
		} else {
			if isEqual(in, out) {
				t.Errorf("progress %d -> %d without change: %d",
					pos, next, out)
			}
			pos = next
		}
		in = out
	}

	if !isEqual(in, expected) {
		t.Errorf("wrong output: %d vs %d", in, expected)
	}
}

func isEqual(in []font.GlyphID, expected []font.GlyphID) bool {
	equal := len(in) == len(expected)
	if equal {
		for i, gid := range in {
			if expected[i] != gid {
				equal = false
				break
			}
		}
	}
	return equal
}
