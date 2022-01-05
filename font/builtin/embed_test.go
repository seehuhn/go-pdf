// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2021  Jochen Voss <voss@seehuhn.de>
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package builtin

import (
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/boxes"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/names"
	"seehuhn.de/go/pdf/pages"
)

func TestSimple(t *testing.T) {
	w, err := pdf.Create("test-builtin-simple.pdf")
	if err != nil {
		t.Fatal(err)
	}

	afm, err := Afm("Times-Roman")
	if err != nil {
		t.Fatal(err)
	}
	F, err := EmbedAfm(w, afm, "F")
	if err != nil {
		t.Fatal(err)
	}

	pageTree := pages.NewPageTree(w, nil)
	page, err := pageTree.NewPage(&pages.Attributes{
		Resources: &pages.Resources{
			Font: map[pdf.Name]pdf.Object{
				F.InstName: F.Ref,
			},
		},
		MediaBox: &pdf.Rectangle{
			URx: 10 + 16*20,
			URy: 5 + 16*20 + 5,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	text := map[font.GlyphID]rune{}
	for i, name := range afm.GlyphName {
		rr := names.ToUnicode(name, false)
		if len(rr) != 1 {
			continue
		}
		r := rr[0]
		gid := font.GlyphID(i)
		rOld, ok := text[gid]
		if !ok || r < rOld {
			text[gid] = r
		}
	}

	for i := 0; i < 256; i++ {
		row := i / 16
		col := i % 16
		gid := font.GlyphID(i + 2)

		gg := F.Layout([]rune{text[gid]}) // try to establish glyph -> rune mapping
		if len(gg) != 1 || gg[0].Gid != gid {
			gg = []font.Glyph{
				{Gid: gid},
			}
		}

		layout := &font.Layout{
			Font:     F,
			FontSize: 16,
			Glyphs:   gg,
		}
		layout.Draw(page, float64(10+20*col), float64(16*20-10-20*row))
	}
	page.Close()

	err = w.Close()
	if err != nil {
		t.Fatal(err)
	}
}

func TestEnc(t *testing.T) {
	for _, fontName := range []string{"Times-Roman", "Courier"} {
		afm, err := Afm(fontName)
		if err != nil {
			t.Fatal(err)
		}

		b := newSimple(afm, nil, "F")

		rr := []rune("ý×A×˚")
		gids := make([]font.GlyphID, len(rr))
		for i, r := range rr {
			gid, ok := b.CMap[r]
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

	b := newSimple(afm, nil, "F")
	gid := b.CMap[r]

	if afm.Code[gid] != -1 {
		t.Errorf("character wrongly mapped at code %d", afm.Code[gid])
	}
	if afm.Width[gid] != 600 {
		t.Errorf("wrong width %d", afm.Width[gid])
	}
}

func TestComplicatedGyphs(t *testing.T) {
	w, err := pdf.Create("test-builtin-gylphs.pdf")
	if err != nil {
		t.Fatal(err)
	}

	font, err := Embed(w, "Courier", "F")
	if err != nil {
		t.Fatal(err)
	}

	text := []rune{'A'}
	text = append(text, names.ToUnicode("commaaccent", false)...)
	text = append(text, 'B')
	text = append(text, names.ToUnicode("lcommaaccent", false)...)
	text = append(text, 'C')

	pageTree := pages.NewPageTree(w, nil)
	page, err := pageTree.NewPage(&pages.Attributes{
		Resources: &pages.Resources{
			Font: pdf.Dict{
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
	page.Close()

	err = w.Close()
	if err != nil {
		t.Fatal(err)
	}
}
