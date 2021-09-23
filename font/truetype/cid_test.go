package truetype

import (
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/sfnt"
	"seehuhn.de/go/pdf/pages"
)

func TestCID(t *testing.T) {
	w, err := pdf.Create("test.pdf")
	if err != nil {
		t.Fatal(err)
	}

	tt, err := sfnt.Open("ttf/SourceSerif4-Regular.ttf", nil)
	if err != nil {
		t.Fatal(err)
	}
	F, err := EmbedFontCID(w, tt, "F")
	if err != nil {
		t.Fatal(err)
	}

	page, err := pages.SinglePage(w, &pages.Attributes{
		Resources: &pages.Resources{
			Font: map[pdf.Name]pdf.Object{
				F.InstName: F.Ref,
			},
		},
		MediaBox: &pdf.Rectangle{
			URx: 10 + 16*20,
			URy: 5 + 32*20 + 5,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	text := map[font.GlyphID]rune{}
	for r, gid := range tt.CMap {
		rOld, ok := text[gid]
		if !ok || r < rOld {
			text[gid] = r
		}
	}

	for i := 0; i < 512; i++ {
		row := i / 16
		col := i % 16
		gid := font.GlyphID(i + 2)

		gg, _ := F.Layout([]rune{text[gid]}) // trye to establish glyph -> rune mapping
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
		layout.Draw(page, float64(10+20*col), float64(32*20-10-20*row))
	}

	err = w.Close()
	if err != nil {
		t.Fatal(err)
	}
}
