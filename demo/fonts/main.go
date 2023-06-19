// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2023  Jochen Voss <voss@seehuhn.de>
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

package main

import (
	"os"
	"regexp"
	"sort"

	"golang.org/x/exp/maps"
	"golang.org/x/text/language"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/color"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/builtin"
	"seehuhn.de/go/pdf/font/cid"
	"seehuhn.de/go/pdf/font/simple"
	"seehuhn.de/go/sfnt/funit"
	"seehuhn.de/go/sfnt/glyph"
)

func main() {
	err := doit()
	if err != nil {
		panic(err)
	}
}

func doit() error {
	paper := document.A4

	data := pdf.NewData(pdf.V1_7)
	doc, err := document.AddMultiPage(data, paper)
	if err != nil {
		return err
	}

	l := &layout{
		topMargin:   54.0,
		leftMargin:  72.0,
		rightMargin: 72.0,
	}

	F, err := builtin.Embed(doc.Out, builtin.TimesRoman, "F")
	if err != nil {
		return err
	}
	l.addFont("text", F, 10)

	S, err := builtin.Embed(doc.Out, builtin.Helvetica, "S")
	if err != nil {
		return err
	}
	l.addFont("code", S, 9)
	l.addFont("dict", S, 9)

	SB, err := builtin.Embed(doc.Out, builtin.HelveticaBold, "B")
	if err != nil {
		return err
	}
	l.addFont("title", SB, 18)

	for i := 0; i < 10; i++ {
		page := doc.AddPage()

		var X font.Embedded
		var title string
		var intro []string
		var ffKey pdf.Name
		switch i {
		case 0:
			title = "Type1 Fonts"
			intro = []string{
				"Type1 fonts use `Type1` as the `Subtype` in the font dictionary.",
				"Font data is embedded via the `FontFile` entry in the font descriptor.",
				"The 14 built-in standard fonts are of this type.",
				"",
				"The `Encoding` entry in the font dictionary describes the mapping from",
				"character codes to glyph names.",
			}
			X, err = builtin.Embed(doc.Out, builtin.TimesRoman, "F")
			if err != nil {
				return err
			}
			ffKey = "FontFile"
		case 1:
			title = "CFF Fonts"
			intro = []string{
				"These use `Type1` as the `Subtype` in the font dictionary.",
				"Font data is embedded via the `FontFile3` entry in the font descriptor,",
				"and the `Subtype` entry in the font file stream dictionary is `Type1C`.",
				"",
				"The CFF data is not allowed to be CID-keyed, *i.e.* the CFF font must not",
				"contain a `ROS` operator.  Usually, `Encoding` is omitted from the font dictionary,",
				"and the mapping from character codes to glyph names is described by",
				"the “builtin encoding” of the CFF font.",
			}
			X, err = simple.EmbedFile(doc.Out, "../../../otf/SourceSerif4-Regular.otf", "X", language.English)
			if err != nil {
				return err
			}
			ffKey = "FontFile3"
		case 2:
			title = "CFF-based OpenType Fonts"
		case 3:
			title = "TrueType Fonts"
			X, err = simple.EmbedFile(doc.Out, "../../../ttf/SourceSerif4-Regular.ttf", "X", language.English)
			if err != nil {
				return err
			}
			ffKey = "FontFile2"
		case 4:
			title = "Glypf-based OpenType Fonts"
		case 5:
			title = "Type3 Fonts"
		case 6:
			title = "CFF CIDFonts"
			X, err = cid.EmbedFile(doc.Out, "../../../otf/SourceSerif4-Regular.otf", "X", language.English)
			if err != nil {
				return err
			}
			ffKey = "FontFile3"
		case 7:
			title = "CFF-based OpenType CIDFonts"
		case 8:
			title = "TrueType CIDFonts"
			X, err = cid.EmbedFile(doc.Out, "../../../ttf/SourceSerif4-Regular.ttf", "X", language.English)
			if err != nil {
				return err
			}
			ffKey = "FontFile2"
		case 9:
			title = "Glypf-based OpenType CIDFonts"
		default:
			title = "To Be Done"
		}

		gg := SB.Layout(title, 18)
		w := l.F["title"].geom.ToPDF(l.F["title"].ptSize, gg.AdvanceWidth())
		l.yPos = paper.URy - l.topMargin - l.F["title"].ascent
		xPos := (paper.URx-l.rightMargin-l.leftMargin-w)/2 + l.leftMargin
		page.TextStart()
		page.TextSetFont(l.F["title"].F, l.F["title"].ptSize)
		page.TextFirstLine(xPos, l.yPos)
		page.TextShow(title)
		page.TextEnd()
		l.yPos -= -l.F["title"].descent + 2*l.F["text"].baseLineSkip + l.F["text"].ascent

		page.TextStart()
		intro = append(intro, "", "Example:")
		for i, line := range intro {
			switch i {
			case 0:
				page.TextSetFont(l.F["text"].F, l.F["text"].ptSize)
				page.TextFirstLine(l.leftMargin, l.yPos)
			case 1:
				page.TextSecondLine(0, -l.F["text"].baseLineSkip)
			default:
				page.TextNextLine()
			}
			if line != "" {
				mmm := findCode.FindAllStringIndex(line, -1)
				start := 0
				for _, mm := range mmm {
					if start < mm[0] {
						page.TextShow(line[start:mm[0]])
					}
					page.TextSetFont(l.F["code"].F, l.F["code"].ptSize)
					page.TextShow(line[mm[0]+1 : mm[1]-1])
					page.TextSetFont(l.F["text"].F, l.F["text"].ptSize)
					start = mm[1]
				}
				if start < len(line) {
					page.TextShow(line[start:])
				}
			}
			l.yPos -= l.F["text"].baseLineSkip
		}
		page.TextEnd()
		l.yPos -= -l.F["text"].descent

		if X == nil {
			page.Close()
			continue
		}

		l.yPos -= 20
		page.TextStart()
		page.TextFirstLine(l.leftMargin, l.yPos)
		page.TextSetFont(X, 24)
		page.TextShow("Hello World!")
		page.TextEnd()
		l.yPos -= 30

		err = X.Close()
		if err != nil {
			return err
		}

		fontDict, err := pdf.GetDict(data, X.Reference())
		if err != nil {
			return err
		}
		yFD := l.ShowDict(page, fontDict, "Font Dictionary")
		fd := fontDict["FontDescriptor"]

		df := fontDict["DescendantFonts"]
		if df != nil {
			dfArray, err := pdf.GetArray(data, df)
			if err != nil {
				return err
			}
			cidFontDict, err := pdf.GetDict(data, dfArray[0])
			if err != nil {
				return err
			}
			yCF := l.ShowDict(page, cidFontDict, "CIDFont Dictionary")
			fd = cidFontDict["FontDescriptor"]

			l.connect(page, yFD["DescendantFonts"], yCF[""], 20)
		}

		if fd != nil {
			fdDict, err := pdf.GetDict(data, fd)
			if err != nil {
				return err
			}
			l.ShowDict(page, fdDict, "Font Descriptor")

			ff := fdDict[ffKey]
			if ff != nil {
				ffStream, err := pdf.GetStream(data, ff)
				if err != nil {
					return err
				}
				if ffStream != nil {
					l.ShowDict(page, ffStream.Dict, "Font file stream dictionary")
				}
			}
		}

		err = page.Close()
		if err != nil {
			return err
		}
	}

	err = doc.Close()
	if err != nil {
		return err
	}

	fd, err := os.Create("fonts.pdf")
	if err != nil {
		return err
	}
	defer fd.Close()
	err = data.Write(fd)
	if err != nil {
		return err
	}

	return nil
}

type layout struct {
	F           map[string]*pdfFont
	yPos        float64
	topMargin   float64
	leftMargin  float64
	rightMargin float64
}

func (l *layout) ShowDict(page *document.Page, fontDict pdf.Dict, title string) map[pdf.Name]float64 {
	yy := make(map[pdf.Name]float64)
	keys := maps.Keys(fontDict)
	sort.Slice(keys, func(i, j int) bool {
		if order(keys[i]) != order(keys[j]) {
			return order(keys[i]) < order(keys[j])
		}
		return keys[i] < keys[j]
	})

	keyGlyphs := make([]glyph.Seq, len(keys))
	var maxWidth funit.Int
	for i, key := range keys {
		gg := l.F["dict"].F.Layout(pdf.Format(key)+" ", 9)
		keyGlyphs[i] = gg
		w := gg.AdvanceWidth()
		if w > maxWidth {
			maxWidth = w
		}
	}
	maxWidth += funit.Int(keyGlyphs[0][len(keyGlyphs[0])-1].Advance)
	for _, gg := range keyGlyphs {
		w := gg.AdvanceWidth()
		delta := maxWidth - w
		if delta > 0 {
			gg[len(gg)-1].Advance += funit.Int16(delta)
		}
	}

	y0 := l.yPos + 2
	page.TextStart()
	page.TextFirstLine(l.leftMargin, l.yPos-l.F["text"].ascent)
	page.TextSetFont(l.F["text"].F, l.F["text"].ptSize)
	gg := l.F["text"].F.Layout(title, l.F["text"].ptSize)
	page.TextShowGlyphs(gg)
	page.TextEnd()
	l.yPos -= l.F["text"].baseLineSkip
	l.yPos -= 9
	y1 := l.yPos + 5
	yy[""] = y1
	titleWitdhPDF := l.F["text"].geom.ToPDF(l.F["text"].ptSize, gg.AdvanceWidth())

	page.TextStart()
	lineNo := 0
	for i, key := range keys {
		switch lineNo {
		case 0:
			page.TextFirstLine(l.leftMargin, l.yPos-l.F["dict"].ascent)
		case 1:
			page.TextSecondLine(0, -l.F["dict"].baseLineSkip)
		default:
			page.TextNextLine()
		}
		lineNo++
		yy[key] = l.yPos - 0.6*l.F["dict"].ascent
		l.yPos -= l.F["dict"].baseLineSkip

		gg := keyGlyphs[i]
		gg = append(gg, l.F["dict"].F.Layout(pdf.Format(fontDict[key]), l.F["dict"].ptSize)...)

		page.TextSetFont(l.F["dict"].F, l.F["dict"].ptSize)
		page.TextShowGlyphs(gg)

		w := gg.AdvanceWidth()
		if w > maxWidth {
			maxWidth = w
		}
	}
	l.yPos += 1
	page.TextEnd()
	y2 := l.yPos - 2
	l.yPos -= 4

	wPDF := l.F["dict"].geom.ToPDF(l.F["dict"].ptSize, maxWidth)
	if wPDF < titleWitdhPDF {
		wPDF = titleWitdhPDF
	}

	_ = y1
	page.Rectangle(l.leftMargin-4, y2, wPDF+8, y0-y2)
	page.MoveTo(l.leftMargin-4, y1)
	page.LineTo(l.leftMargin+wPDF+4, y1)
	page.Stroke()

	l.yPos -= 10

	return yy
}

func (l *layout) connect(page *document.Page, y0, y1 float64, delta float64) {
	vLinePos := l.leftMargin - delta
	page.PushGraphicsState()
	col := color.Gray(0.5)
	page.SetFillColor(col)
	page.SetStrokeColor(col)
	page.MoveTo(l.leftMargin-4, y0)
	page.LineTo(vLinePos, y0)
	page.LineTo(vLinePos, y1)
	page.LineTo(l.leftMargin-4, y1)
	page.Stroke()
	// draw an arrow head
	page.MoveTo(l.leftMargin-4, y1)
	page.LineTo(l.leftMargin-4-6, y1-3)
	page.LineTo(l.leftMargin-4-6, y1+3)
	page.Fill()
	page.PopGraphicsState()
}

func (l *layout) addFont(key string, F font.Embedded, ptSize float64) {
	if l.F == nil {
		l.F = make(map[string]*pdfFont)
	}
	geom := F.GetGeometry()
	ascent := geom.ToPDF16(ptSize, geom.Ascent)
	descent := geom.ToPDF16(ptSize, geom.Descent)
	baseLineSkip := geom.ToPDF16(ptSize, geom.BaseLineSkip)

	l.F[key] = &pdfFont{F, ptSize, geom, ascent, descent, baseLineSkip}
}

type pdfFont struct {
	F            font.Embedded
	ptSize       float64
	geom         *font.Geometry
	ascent       float64
	descent      float64
	baseLineSkip float64
}

func order(key pdf.Name) int {
	switch key {
	case "Type":
		return 0
	case "Subtype":
		return 1
	case "DescendantFonts":
		return 2
	case "BaseFont":
		return 3
	case "Encoding":
		return 4
	case "FontDescriptor":
		return 5
	case "FirstChar":
		return 10
	case "LastChar":
		return 11
	case "Widths":
		return 12
	default:
		return 999
	}
}

var findCode = regexp.MustCompile("`.*?`")
