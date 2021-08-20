package builtin

import (
	"testing"
)

func TestEnc(t *testing.T) {
	for _, fontName := range []string{"Times-Roman", "Courier"} {
		afm, err := Afm(fontName)
		if err != nil {
			t.Fatal(err)
		}

		b := newBuiltin(afm, nil, "F")

		glyphs := b.FullLayout([]rune("ý×A×˚"))
		if len(glyphs) != 5 {
			t.Fatal("wrong number of glyphs")
		}

		codes := map[rune]byte{
			'A': 65,
			'ý': 0o375, // from WinAnsiEncoding
			'×': 0o327, // from WinAnsiEncoding
			'˚': 0o373, // from MacRomanEncoding
		}
		hits := map[string]int{
			"":                  1, // only "A" is in the font's builtin encoding
			"WinAnsiEncoding":   3, // we have "A", "ý" and "×"
			"MacRomanEncoding":  2, // we have "A" and "˚"
			"MacExpertEncoding": 0, // only contains funny characters
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
				t.Errorf("%s.hits == %d, not %d",
					cand.name, cand.hits, hits[cand.name])
			}
		}
	}
}
