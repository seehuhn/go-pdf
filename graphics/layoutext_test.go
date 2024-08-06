// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2024  Jochen Voss <voss@seehuhn.de>
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

package graphics_test

import (
	"bytes"
	"io"
	"math"
	"math/rand"
	"strings"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/gofont"
	"seehuhn.de/go/pdf/font/standard"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/internal/fonttypes"
	"seehuhn.de/go/pdf/reader"
)

func TestGlyphWidths(t *testing.T) {
	data := pdf.NewData(pdf.V1_7)
	rm := pdf.NewResourceManager(data)

	F, err := standard.TimesRoman.New(nil)
	if err != nil {
		t.Fatal(err)
	}

	gg0 := F.Layout(nil, 50, "AB")
	if len(gg0.Seq) != 2 {
		t.Fatal("wrong number of glyphs")
	}

	buf := &bytes.Buffer{}
	out := graphics.NewWriter(buf, rm)
	out.TextBegin()
	out.TextSetHorizontalScaling(2)
	out.TextSetFont(F, 50)
	out.TextFirstLine(100, 100)
	gg := &font.GlyphSeq{
		Seq: []font.Glyph{
			{
				GID:     gg0.Seq[0].GID,
				Advance: 100,
				Text:    []rune("A"),
			},
			{
				GID:  gg0.Seq[1].GID,
				Text: []rune("B"),
			},
		},
	}
	out.TextShowGlyphs(gg)
	out.TextEnd()

	err = rm.Close()
	if err != nil {
		t.Fatal(err)
	}

	in := reader.New(data, nil)
	var ggOut []font.Glyph
	var xxOut []float64
	in.DrawGlyph = func(g font.Glyph) error {
		ggOut = append(ggOut, g)
		x, _ := in.GetTextPositionDevice()
		xxOut = append(xxOut, x)
		return nil
	}
	in.Reset()
	in.Resources = out.Resources
	err = in.ParseContentStream(buf)
	if err != nil {
		t.Fatal(err)
	}

	if len(xxOut) != 2 {
		t.Fatal("wrong number of glyphs")
	}
	if math.Abs(xxOut[0]-100) > 0.01 {
		t.Errorf("wrong glyph position: %f != 100", xxOut[0])
	}
	if math.Abs(xxOut[1]-200) > 0.01 {
		t.Errorf("wrong glyph position: %f != 200", xxOut[1])
	}
}

// TestSpaceAdvance checks that kerning is not applied before a space.
func TestSpaceAdvance(t *testing.T) {
	t.Skip() // TODO(voss): re-enable this test once TextShowGlyphs is fixed.

	data := pdf.NewData(pdf.V2_0)
	rm := pdf.NewResourceManager(data)

	F, err := gofont.Regular.New(nil)
	if err != nil {
		t.Fatal(err)
	}

	buf := &bytes.Buffer{}
	out := graphics.NewWriter(buf, rm)
	out.TextSetFont(F, 10)

	gg := F.Layout(nil, 10, "A B")
	if len(gg.Seq) != 3 || string(gg.Seq[1].Text) != " " {
		t.Fatal("unexpected glyph sequence")
	}
	// Shift the invisible space to the right, leaving the other glyphs in
	// place:
	gg.Seq[0].Advance += 100
	gg.Seq[1].Advance -= 100

	out.TextBegin()
	out.TextShowGlyphs(gg)
	out.TextEnd()

	// There should be no kerning required, i.e. "Tj" should be used instead of
	// the "TJ" operator.
	if strings.Contains(buf.String(), "TJ") {
		t.Error("unexpected TJ operator")
	} else if !strings.Contains(buf.String(), "(A B) Tj") {
		t.Error("missing Tj operator")
	}
}

func BenchmarkTextLayout(b *testing.B) {
	for _, info := range fonttypes.All {
		b.Run(info.Label, func(b *testing.B) {
			b.ResetTimer()
			for range b.N {
				writeDummyDocument(io.Discard, info.MakeFont)
			}
		})
	}
}

func writeDummyDocument(w io.Writer, makeFont func(*pdf.ResourceManager) font.Layouter) error {
	words1 := strings.Fields(sampleText1)
	words2 := strings.Fields(sampleText2)

	paper := document.A4
	doc, err := document.WriteMultiPage(w, paper, pdf.V1_7, nil)
	if err != nil {
		return err
	}

	F := makeFont(doc.RM)

	setStyle := func(page *document.Page) {
		page.TextSetFont(F, 10)
		page.TextSetLeading(12)
		page.SetFillColor(color.DeviceGray(0))
	}

	page := doc.AddPage()
	setStyle(page)

	spaceWidth := page.TextLayout(nil, " ").TotalWidth()

	page.TextBegin()
	yPos := paper.URy - 72
	page.TextFirstLine(72, yPos)
	width := paper.Dx() - 2*72

	gg := &font.GlyphSeq{}

	showLine := func(line string) error {
		if yPos < 72 {
			page.TextEnd()
			err = page.Close()
			if err != nil {
				return err
			}
			page = doc.AddPage()
			setStyle(page)
			page.TextBegin()
			yPos = paper.URy - 72
			page.TextFirstLine(72, yPos)
		}
		page.TextShow(line)
		page.TextNextLine()
		yPos -= page.TextLeading
		return nil
	}

	rng := rand.New(rand.NewSource(0))

	var par []string
	for i := 0; i < 100; i++ {
		n := rng.Intn(9) + 1
		par = par[:0]
		for j := 0; j < n; j++ {
			if rng.Intn(2) == 0 {
				par = append(par, words1...)
			} else {
				par = append(par, words2...)
			}
		}

		var line []string
		var lineWidth float64
		for len(par) > 0 {
			var word string
			word, par = par[0], par[1:]
			gg.Reset()
			w := page.TextLayout(gg, word).TotalWidth()
			if len(line) == 0 {
				line = append(line, word)
				lineWidth = w
			} else if lineWidth+w+spaceWidth <= width {
				line = append(line, word)
				lineWidth += w + spaceWidth
			} else {
				err = showLine(strings.Join(line, " "))
				if err != nil {
					return err
				}
				line = line[:0]
				line = append(line, word)
				lineWidth = w
			}
		}
		err = showLine(strings.Join(line, " "))
		if err != nil {
			return err
		}
		if yPos >= 72 {
			showLine("")
		}
	}

	page.TextEnd()
	err = page.Close()
	if err != nil {
		return err
	}

	err = doc.Close()
	if err != nil {
		return err
	}

	return nil
}

// Thanks Google Bard, for making up this sentence for me.
// https://g.co/gemini/share/784105073f35
const sampleText1 = "I was weary of sight, weary of acquaintance, weary of familiarity, weary of myself, and weary of all the world; and henceforth all places were alike to me."

// This one is from the actual Moby Dick novel.
const sampleText2 = "With a philosophical flourish Cato throws himself upon his sword; I quietly take to the ship."
