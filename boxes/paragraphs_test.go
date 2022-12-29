package boxes

import (
	"math"
	"strings"
	"testing"

	"golang.org/x/text/language"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font/simple"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/pages"
	"seehuhn.de/go/pdf/sfnt/funit"
	"seehuhn.de/go/pdf/sfnt/glyph"
)

func TestTryLength(t *testing.T) {
	const fontSize = 10

	out, err := pdf.Create("test_tryLength.pdf")
	if err != nil {
		t.Fatal(err)
	}

	F1, err := simple.EmbedFile(out, "../sfnt/otf/SourceSerif4-Regular.otf", "F1",
		language.BritishEnglish)
	if err != nil {
		t.Fatal(err)
	}

	space := F1.Layout([]rune{' '})
	var spaceWidth funit.Int
	if len(space) == 1 && space[0].Gid != 0 {
		spaceWidth = funit.Int(space[0].Advance)
	} else {
		space = nil
		spaceWidth = funit.Int(F1.UnitsPerEm / 4)
	}

	q := fontSize / float64(F1.UnitsPerEm)
	lineLength := funit.Int(math.Round(15 / 2.54 * 72 / q))

	pageTree := pages.NewTree(out, &pages.InheritableAttributes{
		MediaBox: pages.A4,
	})
	g, err := graphics.NewPage(out)
	if err != nil {
		t.Fatal(err)
	}
	g.BeginText()
	g.SetFont(F1, fontSize)

	ff := strings.Fields(testText)
	var xPos funit.Int
	var line []glyph.Info
	lineNo := 0
	for _, f := range ff {
		gg := F1.Typeset(f, fontSize)
		var totalLength funit.Int
		for _, g := range gg {
			totalLength += funit.Int(g.Advance)
		}

		if len(line) == 0 {
			line = append(line, gg...)
			xPos = totalLength
		} else if xPos+spaceWidth+totalLength <= lineLength {
			// there is space for another word
			if space != nil {
				line = append(line, space...)
			} else {
				line[len(line)-1].Advance += funit.Int16(spaceWidth)
			}
			xPos += spaceWidth

			line = append(line, gg...)
			xPos += totalLength
		} else {
			// add the line to the page ...
			if lineNo == 0 {
				g.StartLine(72, 25/2.54*72)
			} else if lineNo == 1 {
				g.StartNextLine(0, -float64(F1.BaseLineSkip)*q)
			} else {
				g.NewLine()
			}
			g.ShowGlyphs(line)
			lineNo++

			// ... and start a new line
			line = append(line[:0], gg...)
			xPos = totalLength
		}
	}
	if len(line) > 0 {
		if lineNo == 0 {
			g.StartLine(72, 25/2.54*72)
		} else if lineNo == 1 {
			g.StartNextLine(0, -float64(F1.BaseLineSkip)*q)
		} else {
			g.NewLine()
		}
		g.ShowGlyphs(line)
		lineNo++
	}

	g.EndText()
	dict, err := g.Close()
	if err != nil {
		t.Fatal(err)
	}

	_, err = pageTree.AppendPage(dict)
	if err != nil {
		t.Fatal(err)
	}

	ref, err := pageTree.Close()
	if err != nil {
		t.Error(err)
	}
	out.Catalog.Pages = ref

	err = out.Close()
	if err != nil {
		t.Error(err)
	}
}

const testText = `Call me Ishmael. Some years ago—never mind how long precisely—having little or no money in my purse, and nothing particular to interest me on shore, I thought I would sail about a little and see the watery part of the world. It is a way I have of driving off the spleen and regulating the circulation. Whenever I find myself growing grim about the mouth; whenever it is a damp, drizzly November in my soul; whenever I find myself involuntarily pausing before coffin warehouses, and bringing up the rear of every funeral I meet; and especially whenever my hypos get such an upper hand of me, that it requires a strong moral principle to prevent me from deliberately stepping into the street, and methodically knocking people’s hats off—then, I account it high time to get to sea as soon as I can. This is my substitute for pistol and ball. With a philosophical flourish Cato throws himself upon his sword; I quietly take to the ship. There is nothing surprising in this. If they but knew it, almost all men in their degree, some time or other, cherish very nearly the same feelings towards the ocean with me.`
