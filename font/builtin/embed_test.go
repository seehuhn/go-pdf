package builtin

import (
	"testing"
)

func TestEnc(t *testing.T) {
	afm, err := ReadAfm("Times-Roman")
	if err != nil {
		t.Fatal(err)
	}

	b := newBuiltin(afm, nil, "F")

	glyphs := b.Layout([]rune("½×A×ﬁ"))
	if len(glyphs) != 5 {
		t.Fatal("wrong number of glyphs")
	}

	codes := map[rune]byte{
		'A': 65,
		'½': 0o275, // from WinAnsiEncoding
		'×': 0o327, // from WinAnsiEncoding
		'ﬁ': 0o336, // from MacRomanEncoding
	}
	hits := map[string]int{
		"":                 1, // only "A" is in the font's builtin encoding
		"WinAnsiEncoding":  3, // we have "A", "½" and "×"
		"MacRomanEncoding": 2, // we have "A" and "ﬁ"
	}

	for _, glyph := range glyphs {
		gid := glyph.Gid
		s := b.Enc(gid)
		if len(s) != 1 {
			t.Fatal("wrong number of codes")
		}
		c := s[0]

		if c != codes[glyph.Chars[0]] {
			t.Errorf("wrong char code %d", c)
		}
	}

	for _, cand := range b.candidates {
		if cand.hits != hits[cand.name] {
			t.Errorf("%s.hits == %d, not 1", cand.name, cand.hits)
		}
	}
}
