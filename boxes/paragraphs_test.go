package boxes

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"golang.org/x/text/language"
	"seehuhn.de/go/dijkstra"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font/simple"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/pages"
	"seehuhn.de/go/pdf/sfnt/funit"
)

func TestLineBreaks(t *testing.T) {
	const fontSize = 10
	hSize := math.Round(15 / 2.54 * 72)
	parFillSkip := &glue{
		Plus: stretchAmount{Val: 1, Level: 1},
		Text: "\n",
	}

	out, err := pdf.Create("test_LineBreaks.pdf")
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
	pdfSpaceWidth := F1.ToPDF(fontSize, spaceWidth)

	spaceGlue := &glue{
		Length: pdfSpaceWidth,
		Plus:   stretchAmount{Val: pdfSpaceWidth / 2},
		Minus:  stretchAmount{Val: pdfSpaceWidth / 3},
		Text:   " ",
	}
	xSpaceGlue := &glue{
		Length: 1.5 * pdfSpaceWidth,
		Plus:   stretchAmount{Val: pdfSpaceWidth * 1.5},
		Minus:  stretchAmount{Val: pdfSpaceWidth},
		Text:   " ",
	}

	var hModeMaterial []interface{}
	endOfSentence := false
	for i, f := range strings.Fields(testText) {
		if i > 0 {
			if endOfSentence {
				hModeMaterial = append(hModeMaterial, xSpaceGlue)
				endOfSentence = false
			} else {
				hModeMaterial = append(hModeMaterial, spaceGlue)
			}
		}
		gg := F1.Typeset(f, fontSize)
		hModeMaterial = append(hModeMaterial, &hGlyphs{
			glyphs:   gg,
			font:     F1,
			fontSize: fontSize,
			width:    F1.ToPDF(fontSize, gg.AdvanceWidth()),
		})
	}

	// TODO(voss):
	// - check that no node has infinite shrinkability (since otherwise the
	//   whole paragraph would fit into a single line)
	// - remove trailing space or glue, if any
	// - add an infinite penalty before the ParFillSkip glue
	hModeMaterial = append(hModeMaterial, parFillSkip)

	g := &lineBreakGraph{
		hlist:     hModeMaterial,
		textWidth: hSize,
		rightSkip: &glue{Plus: stretchAmount{Val: 36, Level: 0}},
	}
	start := &breakNode{}
	breaks, err := dijkstra.ShortestPathSet[*breakNode, int, float64](g, start, func(v *breakNode) bool {
		return v.pos == len(g.hlist)
	})
	if err != nil {
		t.Fatal(err)
	}

	parms := &Parameters{
		BaseLineSkip: F1.ToPDF16(fontSize, F1.BaseLineSkip),
	}

	var lines []Box
	v := start
	for _, e := range breaks {
		var line []Box
		if g.leftSkip != nil {
			line = append(line, g.leftSkip)
		}
		for _, item := range g.hlist[v.pos:e] {
			switch h := item.(type) {
			case *glue:
				line = append(line, h)
			case *hGlyphs:
				line = append(line, &TextBox{
					Font:     h.font,
					FontSize: h.fontSize,
					Glyphs:   h.glyphs,
				})
			default:
				panic(fmt.Sprintf("unexpected type %T in horizontal mode list", h))
			}
		}
		if g.rightSkip != nil {
			line = append(line, g.rightSkip)
		}
		lines = append(lines, HBoxTo(g.textWidth, line...))
		v = g.To(v, e)
	}
	paragraph := parms.VTop(lines...)

	pageTree := pages.InstallTree(out, &pages.InheritableAttributes{
		MediaBox: pages.A4,
	})

	gr, err := graphics.AppendPage(pageTree)
	if err != nil {
		t.Fatal(err)
	}

	paragraph.Draw(gr, 72, 25/2.54*72)

	_, err = gr.Close()
	if err != nil {
		t.Fatal(err)
	}

	err = out.Close()
	if err != nil {
		t.Error(err)
	}
}

var _ dijkstra.Graph[*breakNode, int, float64] = (*lineBreakGraph)(nil)

const testText = `Call me Ishmael. Some years ago—never mind how long precisely—having little or no money in my purse, and nothing particular to interest me on shore, I thought I would sail about a little and see the watery part of the world. It is a way I have of driving off the spleen and regulating the circulation. Whenever I find myself growing grim about the mouth; whenever it is a damp, drizzly November in my soul; whenever I find myself involuntarily pausing before coffin warehouses, and bringing up the rear of every funeral I meet; and especially whenever my hypos get such an upper hand of me, that it requires a strong moral principle to prevent me from deliberately stepping into the street, and methodically knocking people’s hats off—then, I account it high time to get to sea as soon as I can. This is my substitute for pistol and ball. With a philosophical flourish Cato throws himself upon his sword; I quietly take to the ship. There is nothing surprising in this. If they but knew it, almost all men in their degree, some time or other, cherish very nearly the same feelings towards the ocean with me.`
