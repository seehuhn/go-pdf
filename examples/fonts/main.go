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
	"bufio"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"golang.org/x/exp/maps"
	"golang.org/x/text/language"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/color"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/builtin"
	"seehuhn.de/go/pdf/font/cid"
	"seehuhn.de/go/pdf/font/simple"
	"seehuhn.de/go/pdf/font/type3"
	"seehuhn.de/go/sfnt/funit"
	"seehuhn.de/go/sfnt/glyph"
	"seehuhn.de/go/sfnt/gofont"
)

func main() {
	err := doit()
	if err != nil {
		panic(err)
	}
}

func doit() error {
	paper := document.A4

	sections, err := parseNotes("NOTES.md")
	if err != nil {
		return err
	}

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

	I, err := builtin.Embed(doc.Out, builtin.TimesItalic, "I")
	if err != nil {
		return err
	}
	l.addFont("it", I, 10)

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

	for _, s := range sections {
		title := s.title
		intro := s.lines
		example := "Hello World!"

		var X font.Embedded
		var ffKey pdf.Name
		switch title {
		case "Type1 Fonts":
			X, err = builtin.Embed(doc.Out, builtin.TimesRoman, "F")
			if err != nil {
				return err
			}
			ffKey = "FontFile"
		case "CFF Fonts":
			X, err = simple.EmbedFile(doc.Out, "../../../otf/SourceSerif4-Regular.otf", "X", language.English)
			if err != nil {
				return err
			}
			ffKey = "FontFile3"
		case "CFF-based OpenType Fonts":
			X, err = embedOpenTypeSimple(doc.Out, "../../../otf/SourceSerif4-Regular.otf", "X", language.English)
			if err != nil {
				return err
			}
			ffKey = "FontFile3"
		case "TrueType Fonts":
			ttf, err := gofont.TrueType(gofont.GoRegular)
			if err != nil {
				return err
			}
			F, err := simple.Font(ttf, language.English)
			if err != nil {
				return err
			}
			X, err = F.Embed(doc.Out, "X")
			if err != nil {
				return err
			}
			ffKey = "FontFile2"
		case "Glyf-based OpenType Fonts":
			ffKey = "FontFile2"
		case "Type3 Fonts":
			X, err = embedType3Font(doc.Out)
			if err != nil {
				return err
			}
			example = "ABC"
		case "CFF CIDFonts":
			X, err = cid.EmbedFile(doc.Out, "../../../otf/SourceSerif4-Regular.otf", "X", language.English)
			if err != nil {
				return err
			}
			ffKey = "FontFile3"
		case "CFF-based OpenType CIDFonts":
		case "TrueType CIDFonts":
			X, err = cid.EmbedFile(doc.Out, "../../../ttf/SourceSerif4-Regular.ttf", "X", language.English)
			if err != nil {
				return err
			}
			ffKey = "FontFile2"
		case "Glyf-based OpenType CIDFonts":
		}

		page := doc.AddPage()

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
		if X != nil {
			intro = append(intro, "", "Example:")
		}
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
					switch line[mm[0]] {
					case '`':
						page.TextSetFont(l.F["code"].F, l.F["code"].ptSize)
					case '*':
						page.TextSetFont(l.F["it"].F, l.F["it"].ptSize)
					}
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
		page.TextShow(example)
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
		yFD := l.ShowDict(page, fontDict, "Font Dictionary", X.Reference())
		fd := fontDict["FontDescriptor"]
		y0FontDesc := yFD["FontDescriptor"]

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
			ref, _ := dfArray[0].(pdf.Reference)
			yCF := l.ShowDict(page, cidFontDict, "CIDFont Dictionary", ref)
			fd = cidFontDict["FontDescriptor"]
			y0FontDesc = yCF["FontDescriptor"]

			l.connect(page, yFD["DescendantFonts"], yCF[""], 20)
		}

		if fd != nil {
			fdDict, err := pdf.GetDict(data, fd)
			if err != nil {
				return err
			}
			ref, _ := fd.(pdf.Reference)
			yFontDesc := l.ShowDict(page, fdDict, "Font Descriptor", ref)
			l.connect(page, y0FontDesc, yFontDesc[""], 20)

			ff := fdDict[ffKey]
			if ff != nil {
				ffStream, err := pdf.GetStream(data, ff)
				if err != nil {
					return err
				}
				if ffStream != nil {
					ref, _ := ff.(pdf.Reference)
					yStreamDict := l.ShowDict(page, ffStream.Dict, "Font file stream dictionary", ref)
					l.connect(page, yFontDesc[ffKey], yStreamDict[""], 20)
				}
			}
		}

		if title == "Type3 Fonts" {
			cp, err := pdf.GetDict(data, fontDict["CharProcs"])
			if err != nil {
				return err
			}
			yCP := l.ShowDict(page, cp, "CharProcs", 0)
			l.connect(page, yFD["CharProcs"], yCP[""], 20)
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

type section struct {
	title string
	lines []string
}

func parseNotes(fname string) ([]section, error) {
	f, err := os.Open(fname)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var sections []section
	var current section
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "### ") {
			if current.title != "" {
				sections = append(sections, current)
			}
			title := line[4:]
			title = pdfVersion.ReplaceAllString(title, "")
			current = section{title: title}
		} else {
			current.lines = append(current.lines, line)
		}
	}
	if current.title != "" {
		sections = append(sections, current)
	}

	return sections, nil
}

type layout struct {
	F           map[string]*pdfFont
	yPos        float64
	topMargin   float64
	leftMargin  float64
	rightMargin float64
}

func (l *layout) ShowDict(page *document.Page, fontDict pdf.Dict, title string, ref pdf.Reference) map[pdf.Name]float64 {
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

	xBase := l.leftMargin + 5

	y0 := l.yPos + 2
	page.TextStart()
	page.TextFirstLine(xBase, l.yPos-l.F["text"].ascent)
	page.TextSetFont(l.F["text"].F, l.F["text"].ptSize)
	titleWitdhPDF := page.TextShow(title)
	if ref != 0 {
		titleWitdhPDF += page.TextShow(" (")
		page.TextSetFont(l.F["code"].F, l.F["code"].ptSize)
		titleWitdhPDF += page.TextShow(fmt.Sprintf("%d %d obj", ref.Number(), ref.Generation()))
		page.TextSetFont(l.F["text"].F, l.F["text"].ptSize)
		titleWitdhPDF += page.TextShow(")")
	}
	page.TextEnd()
	l.yPos -= l.F["text"].baseLineSkip
	l.yPos -= 9
	y1 := l.yPos + 5
	yy[""] = y1

	page.TextStart()
	lineNo := 0
	for i, key := range keys {
		switch lineNo {
		case 0:
			page.TextFirstLine(xBase, l.yPos-l.F["dict"].ascent)
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
	page.Rectangle(xBase-4, y2, wPDF+8, y0-y2)
	page.MoveTo(xBase-4, y1)
	page.LineTo(xBase+wPDF+4, y1)
	page.Stroke()

	l.yPos -= 10

	return yy
}

func (l *layout) connect(page *document.Page, y0, y1 float64, delta float64) {
	vLinePos := l.leftMargin - delta
	xBase := l.leftMargin
	page.PushGraphicsState()
	col := color.Gray(0.5)
	page.SetFillColor(col)
	page.SetStrokeColor(col)
	page.MoveTo(xBase+3.5, y0)
	page.LineTo(vLinePos, y0)
	page.LineTo(vLinePos, y1)
	page.LineTo(xBase-5, y1)
	page.Stroke()
	// draw an arrow head
	page.MoveTo(xBase, y1)
	page.LineTo(xBase-6, y1-2.5)
	page.LineTo(xBase-5.5, y1)
	page.LineTo(xBase-6, y1+2.5)
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

func embedType3Font(out pdf.Putter) (font.Embedded, error) {
	b := type3.New(1000)
	b.Ascent = 800
	b.Descent = -200
	b.BaseLineSkip = 1000

	A, err := b.AddGlyph("A", 1000, funit.Rect16{LLx: 0, LLy: 0, URx: 800, URy: 800}, true)
	if err != nil {
		return nil, err
	}
	A.MoveTo(0, 0)
	A.LineTo(800, 0)
	A.LineTo(800, 800)
	A.LineTo(0, 800)
	A.Fill()
	err = A.Close()
	if err != nil {
		return nil, err
	}

	B, err := b.AddGlyph("B", 900, funit.Rect16{LLx: 0, LLy: 0, URx: 800, URy: 800}, true)
	if err != nil {
		return nil, err
	}
	B.Circle(400, 400, 400)
	B.Fill()
	err = B.Close()
	if err != nil {
		return nil, err
	}

	C, err := b.AddGlyph("C", 1000, funit.Rect16{LLx: 0, LLy: 0, URx: 800, URy: 800}, true)
	if err != nil {
		return nil, err
	}
	C.MoveTo(0, 0)
	C.LineTo(800, 0)
	C.LineTo(400, 800)
	C.Fill()
	err = C.Close()
	if err != nil {
		return nil, err
	}

	return b.EmbedFont(out, "X")
}

var (
	pdfVersion = regexp.MustCompile(`\s+\(PDF \d\.\d\)\s*$`)
	findCode   = regexp.MustCompile("`.*?`|\\*.*?\\*")
)
