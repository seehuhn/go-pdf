package builtin

import (
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/boxes"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/names"
	"seehuhn.de/go/pdf/pages"
)

func TestEnc(t *testing.T) {
	for _, fontName := range []string{"Times-Roman", "Courier"} {
		afm, err := Afm(fontName)
		if err != nil {
			t.Fatal(err)
		}

		b := newBuiltin(afm, nil, "F")

		rr := []rune("ý×A×˚")
		gids := make([]font.GlyphID, len(rr))
		for i, r := range rr {
			gid, ok := b.cmap[r]
			if !ok {
				t.Fatal("missing rune")
			}
			gids[i] = gid
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

		for i, gid := range gids {
			s := b.Enc(gid)
			if len(s) != 1 {
				t.Fatal("wrong number of codes")
			}
			c := s[0]

			if c != codes[rr[i]] {
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

func TestCommaAccent(t *testing.T) {
	rr := names.ToUnicode("commaaccent", false)
	if len(rr) != 1 {
		t.Fatal("wrong number of runes")
	}
	r := rr[0]

	afm, err := Afm("Courier")
	if err != nil {
		t.Fatal(err)
	}

	b := newBuiltin(afm, nil, "F")
	gid := b.cmap[r]

	if afm.Code[gid] != -1 {
		t.Errorf("character wrongly mapped at code %d", afm.Code[gid])
	}
	if afm.Width[gid] != 600 {
		t.Errorf("wrong width %d", afm.Width[gid])
	}
}

func TestComplicatedGyphs(t *testing.T) {
	w, err := pdf.Create("test.pdf")
	if err != nil {
		t.Fatal(err)
	}

	font, err := Embed(w, "F", "Courier")
	if err != nil {
		t.Fatal(err)
	}

	text := []rune{'A'}
	text = append(text, names.ToUnicode("commaaccent", false)...)
	text = append(text, 'B')
	text = append(text, names.ToUnicode("lcommaaccent", false)...)
	text = append(text, 'C')

	page, err := pages.SinglePage(w, &pages.Attributes{
		Resources: map[pdf.Name]pdf.Object{
			"Font": pdf.Dict{
				font.InstName: font.Ref,
			},
		},
		MediaBox: &pdf.Rectangle{
			URx: 100,
			URy: 40,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	box := boxes.Text(font, 24, string(text))
	box.Draw(page, 10, 15)

	err = page.Close()
	if err != nil {
		t.Fatal(err)
	}

	err = w.Close()
	if err != nil {
		t.Fatal(err)
	}
}
