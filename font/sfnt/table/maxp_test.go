package table

import (
	"bytes"
	"testing"
)

func TestMaxp(t *testing.T) {
	for _, numGlyphs := range []int{1, 2, 3, 255, 256, 1000, 65535} {
		maxp, err := EncodeMaxp(numGlyphs)
		if err != nil {
			t.Errorf("EncodeMaxp(%d): %v", numGlyphs, err)
			continue
		}
		gotNumGlyphs, err := ReadMaxp(bytes.NewReader(maxp))
		if err != nil {
			t.Errorf("ReadMaxp(%d): %v", numGlyphs, err)
			continue
		}
		if gotNumGlyphs != numGlyphs {
			t.Errorf("ReadMaxp(%d): got %d glyphs, want %d", numGlyphs, gotNumGlyphs, numGlyphs)
		}
	}
}
