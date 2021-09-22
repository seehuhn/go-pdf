package truetype

import (
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/pages"
)

func TestCID(t *testing.T) {
	w, err := pdf.Create("test.pdf")
	if err != nil {
		t.Fatal(err)
	}

	F, err := EmbedCID(w, "F", "ttf/SourceSerif4-Regular.ttf", nil)
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

	for i := 0; i < 512; i++ {
		row := i / 16
		col := i % 16
		gid := font.GlyphID(i + 2)

		layout := &font.Layout{
			Font:     F,
			FontSize: 16,
			Glyphs: []font.Glyph{
				{Gid: gid},
			},
		}
		layout.Draw(page, float64(10+20*col), float64(32*20-10-20*row))
	}

	err = w.Close()
	if err != nil {
		t.Fatal(err)
	}
}
