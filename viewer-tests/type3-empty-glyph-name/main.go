// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"fmt"
	"iter"
	"os"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/standard"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/text"
)

func main() {
	err := createDocument("test.pdf")
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

const (
	margin    = 72.0
	wrapWidth = 450.0
	boxX      = margin + 200 // left edge of the boxes
	boxSize   = 24.0
	rowHeight = 40.0
)

func createDocument(filename string) error {
	page, err := document.CreateSinglePage(filename, document.A4, pdf.V1_7, nil)
	if err != nil {
		return err
	}

	titleFont := font.Must(standard.TimesBold.New())
	title := text.F{Font: titleFont, Size: 14, Color: color.DeviceGray(0.2)}
	noteFont := font.Must(standard.TimesRoman.New())
	note := text.F{Font: noteFont, Size: 11, Color: color.DeviceGray(0.1)}

	text.Show(page.Builder,
		text.M{X: margin, Y: 770},
		title, "Type 3 glyph names", text.NL,
		note, text.Wrap(wrapWidth,
			"Each row uses its own Type 3 font with a single check-mark glyph,",
			"whose name is given in the row label.  If the viewer handles the",
			"name, a check mark appears in the box; otherwise the box stays empty.",
		),
	)

	rows := []struct {
		label string
		name  pdf.Name
	}{
		{"control (must have check)", "A"},
		{"accepts empty name", ""},
	}
	for _, n := range []int{16, 32, 64, 128, 256, 512} {
		rows = append(rows, struct {
			label string
			name  pdf.Name
		}{fmt.Sprintf("accepts names of length %d", n), longName(n)})
	}

	for i, r := range rows {
		y := 660 - rowHeight*float64(i)

		text.Show(page.Builder,
			text.M{X: margin, Y: y + 8},
			note, r.label,
		)

		page.PushGraphicsState()
		page.SetStrokeColor(color.DeviceGray(0.2))
		page.SetLineWidth(0.8)
		page.Rectangle(boxX, y, boxSize, boxSize)
		page.Stroke()
		page.PopGraphicsState()

		page.SetFillColor(color.DeviceRGB{0, 0.45, 0.15})
		page.TextBegin()
		page.TextSetFont(&checkFont{name: r.name}, boxSize)
		page.TextFirstLine(boxX, y)
		page.TextShowRaw(pdf.String{checkCode})
		page.TextEnd()
	}

	return page.Close()
}

// longName returns a glyph name of n lower-case letters.
func longName(n int) pdf.Name {
	b := make([]byte, n)
	for i := range b {
		b[i] = 'a' + byte(i%26)
	}
	return pdf.Name(b)
}

// checkCode is the character code which shows the check mark.
const checkCode = 65

// checkFont is a Type 3 font with a single check-mark glyph.  It is written
// as raw objects, so that the file holds exactly the glyph name under test
// and an empty box shows what the viewer does, not what go-pdf's font code
// wrote.
type checkFont struct {
	name pdf.Name // name of the glyph for checkCode
}

func (f *checkFont) PostScriptName() string        { return "" }
func (f *checkFont) WritingMode() font.WritingMode { return font.Horizontal }
func (f *checkFont) ResourceName() pdf.Name        { return "" }
func (f *checkFont) FontInfo() any                 { return nil }

func (f *checkFont) Codec() *charcode.Codec {
	c, err := charcode.NewCodec(charcode.Simple)
	if err != nil {
		panic(err)
	}
	return c
}

func (f *checkFont) Codes(s pdf.String) iter.Seq[font.Code] {
	return func(yield func(font.Code) bool) {
		for _, c := range s {
			if !yield(font.Code{CID: 0, Width: 1, UseWordSpacing: c == ' '}) {
				return
			}
		}
	}
}

func (f *checkFont) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) {
	content := "1000 0 0 0 1000 1000 d1\n" +
		"100 420 m\n380 120 l\n900 820 l\n790 900 l\n370 340 l\n210 520 l\nf\n"
	charProcs := pdf.Dict{}
	{
		ref := rm.Alloc()
		stm, err := rm.Out().OpenStream(ref, nil)
		if err != nil {
			return nil, err
		}
		if _, err := stm.Write([]byte(content)); err != nil {
			return nil, err
		}
		if err := stm.Close(); err != nil {
			return nil, err
		}
		charProcs[f.name] = ref
	}

	fontDict := pdf.Dict{
		"Type":       pdf.Name("Font"),
		"Subtype":    pdf.Name("Type3"),
		"FontBBox":   pdf.Array{pdf.Integer(0), pdf.Integer(0), pdf.Integer(1000), pdf.Integer(1000)},
		"FontMatrix": pdf.Array{pdf.Number(0.001), pdf.Integer(0), pdf.Integer(0), pdf.Number(0.001), pdf.Integer(0), pdf.Integer(0)},
		"CharProcs":  charProcs,
		"Encoding": pdf.Dict{
			"Type": pdf.Name("Encoding"),
			"Differences": pdf.Array{
				pdf.Integer(checkCode), f.name,
			},
		},
		"FirstChar": pdf.Integer(checkCode),
		"LastChar":  pdf.Integer(checkCode),
		"Widths":    pdf.Array{pdf.Integer(1000)},
	}
	ref := rm.Alloc()
	if err := rm.Out().Put(ref, fontDict); err != nil {
		return nil, err
	}
	return ref, nil
}
