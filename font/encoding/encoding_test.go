package encoding

import (
	"testing"

	"seehuhn.de/go/sfnt/glyph"
)

// TestSimpleEncoder tests whether the SimpleEncoder can allocate 256 codes,
// and whether it reuses codes when possible.
func TestSimpleEncoder(t *testing.T) {
	e := NewSimpleEncoder()

	codes := make(map[byte]int)
	for i := 0; i < 256; i++ {
		s := e.AppendEncoded(nil, glyph.ID(i+1), []rune{rune(i + 32)})
		if len(s) != 1 {
			t.Fatalf("unexpected length %d", len(s))
		}

		c := s[0]
		if _, seen := codes[c]; seen {
			t.Errorf("%d: code %d used twice", i, c)
		}
		codes[c] = i
	}

	if e.Overflow() {
		t.Errorf("unexpected overflow")
	}
	if len(e.cache) != 256 {
		t.Errorf("unexpected cache length %d", len(e.cache))
	}

	for i := 0; i < 256; i++ {
		s := e.AppendEncoded(nil, glyph.ID(i+1), []rune{rune(i + 32)})
		if len(s) != 1 {
			t.Errorf("%d: unexpected length %d", i, len(s))
			continue
		}

		c := s[0]
		prevI, seen := codes[c]
		if !seen {
			t.Errorf("code %d not seen before", c)
		} else if prevI != i {
			t.Errorf("previous code %d != %d", prevI, i)
		}
	}
}
