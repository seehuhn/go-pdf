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
	"flag"
	"fmt"
	"os"
	"regexp"
	"sort"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/standard"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/internal/fonttypes"
)

var writeIndividual = flag.Bool("a", false, "write individual font examples")

func main() {
	flag.Parse()

	err := doit()
	if err != nil {
		panic(err)
	}
}

const exampleText = `“Hello World!”`

func doit() error {
	sections, err := parseNotes("NOTES.md")
	if err != nil {
		return err
	}

	paper := document.A4
	l := &layout{
		topMargin:   54.0,
		leftMargin:  108.0,
		rightMargin: 144.0,
	}

	outName := "test.pdf"
	fmt.Println("writing", outName, "...")
	doc, err := document.CreateMultiPage(outName, paper, pdf.V1_7, nil)
	if err != nil {
		return err
	}

	F := standard.TimesRoman.New()
	l.addFont("text", F, 10)

	I := standard.TimesItalic.New()
	l.addFont("it", I, 10)

	S := standard.Helvetica.New()
	l.addFont("code", S, 9)
	l.addFont("dict", S, 9)

	SB := standard.HelveticaBold.New()
	l.addFont("chapter", SB, 24)
	l.addFont("section", SB, 18)

	pageNo := 1
	fontNo := 1
	for _, s := range sections {
		title := s.title
		intro := s.lines

		fmt.Println("-", title)

		var gen func() font.Layouter
		var ffKey pdf.Name
		switch title {
		case "Simple PDF Fonts":
			// part 1
		case "Type 1 Fonts":
			gen = fonttypes.Type1WithMetrics
			ffKey = "FontFile"
		case "Standard Fonts":
			gen = fonttypes.Standard
		case "Simple CFF Fonts":
			gen = fonttypes.CFFSimple
			ffKey = "FontFile3"
		case "Simple CFF-based OpenType Fonts":
			gen = fonttypes.OpenTypeCFFSimple
			ffKey = "FontFile3"
		case "Multiple Master Fonts":
			// not supported
		case "Simple TrueType Fonts":
			gen = fonttypes.TrueTypeSimple
			ffKey = "FontFile2"
		case "Simple Glyf-based OpenType Fonts":
			gen = fonttypes.OpenTypeGlyfSimple
			ffKey = "FontFile3"
		case "Type 3 Fonts":
			gen = fonttypes.Type3
		case "Composite PDF Fonts":
			// part 2
		case "Composite CFF Fonts":
			gen = fonttypes.CFFComposite
			ffKey = "FontFile3"
		case "Composite CFF-based OpenType Fonts":
			gen = fonttypes.OpenTypeCFFComposite
			ffKey = "FontFile3"
		case "Composite TrueType Fonts":
			gen = fonttypes.TrueTypeComposite
			ffKey = "FontFile2"
		case "Composite Glyf-based OpenType Fonts":
			gen = fonttypes.OpenTypeGlyfComposite
			ffKey = "FontFile3"
		default:
			panic("unexpected section " + title)
		}
		var X font.Font
		if gen != nil {
			X = gen()
		}

		page := doc.AddPage()

		page.TextBegin()
		if s.level == 1 {
			gg := SB.Layout(nil, l.F["chapter"].ptSize, title)
			w := gg.TotalWidth()
			l.yPos = paper.URy - l.topMargin - 72 - l.F["chapter"].ascent
			xPos := (paper.URx-l.rightMargin-l.leftMargin-w)/2 + l.leftMargin
			page.SetFillColor(color.DeviceGray(0.3))
			page.TextSetFont(l.F["chapter"].F, l.F["chapter"].ptSize)
			page.TextFirstLine(xPos, l.yPos)
			l.yPos -= -l.F["chapter"].descent + 2*l.F["text"].baseLineSkip + l.F["text"].ascent
		} else {
			l.yPos = paper.URy - l.topMargin - l.F["section"].ascent
			page.SetFillColor(color.DeviceGray(0.15))
			page.TextSetFont(l.F["section"].F, l.F["section"].ptSize)
			page.TextFirstLine(l.leftMargin, l.yPos)
			l.yPos -= -l.F["section"].descent + 2*l.F["text"].baseLineSkip + l.F["text"].ascent
		}
		page.TextShow(title)
		page.TextEnd()
		page.SetFillColor(color.Black)

		page.TextBegin()
		if gen != nil && *writeIndividual {
			intro = append(intro, "", fmt.Sprintf("Example (see `test%02d.pdf`):", fontNo))
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

		if gen != nil && *writeIndividual {
			err = writeSinglePage(gen, fontNo)
			if err != nil {
				return err
			}
			fontNo++
		}

		if gen != nil {
			refY, Y, err := pdf.ResourceManagerEmbed(page.RM, X)
			if err != nil {
				return err
			}

			l.yPos -= 20
			page.TextBegin()
			page.TextFirstLine(l.leftMargin, l.yPos)
			page.TextSetFont(X, 24)
			page.TextShow(exampleText)
			page.TextEnd()
			l.yPos -= 30

			err = Y.(pdf.Finisher).Finish(page.RM)
			if err != nil {
				return err
			}

			fontDict, err := pdf.GetDict(doc.Out, refY)
			if err != nil {
				return err
			}
			ref, _ := refY.(pdf.Reference)
			yFD := l.ShowDict(page, fontDict, "Font Dictionary", ref)
			fd := fontDict["FontDescriptor"]
			y0FontDesc := yFD["FontDescriptor"]

			df := fontDict["DescendantFonts"]
			if df != nil {
				dfArray, err := pdf.GetArray(doc.Out, df)
				if err != nil {
					return err
				}
				cidFontDict, err := pdf.GetDict(doc.Out, dfArray[0])
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
				fdDict, err := pdf.GetDict(doc.Out, fd)
				if err != nil {
					return err
				}
				ref, _ := fd.(pdf.Reference)
				yFontDesc := l.ShowDict(page, fdDict, "Font Descriptor", ref)
				l.connect(page, y0FontDesc, yFontDesc[""], 20)

				ff := fdDict[ffKey]
				if ff != nil {
					ffStream, err := pdf.GetStream(doc.Out, ff)
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
				cp, err := pdf.GetDict(doc.Out, fontDict["CharProcs"])
				if err != nil {
					return err
				}
				yCP := l.ShowDict(page, cp, "CharProcs", 0)
				l.connect(page, yFD["CharProcs"], yCP[""], 20)
			}
		}

		// add the page number
		page.TextSetFont(l.F["text"].F, l.F["text"].ptSize)
		page.TextBegin()
		xMid := (l.leftMargin + (paper.URx - l.rightMargin)) / 2
		page.TextFirstLine(xMid, 36)
		page.TextShowAligned(fmt.Sprintf("%d", pageNo), 0, 0.5)
		page.TextEnd()
		pageNo++

		err = page.Close()
		if err != nil {
			return err
		}
	}

	err = doc.Close()
	if err != nil {
		return err
	}

	fmt.Println("done")

	return nil
}

func writeSinglePage(gen func() font.Layouter, no int) error {
	fname := fmt.Sprintf("test%02d.pdf", no)

	opt := &pdf.WriterOptions{
		HumanReadable: true,
	}
	page, err := document.CreateSinglePage(fname, document.A5r, pdf.V1_7, opt)
	if err != nil {
		return err
	}

	F := gen()

	page.TextBegin()
	page.TextFirstLine(72, 72)
	page.TextSetFont(F, 24)
	page.TextShow(exampleText)
	page.TextEnd()

	return page.Close()
}

type section struct {
	level int
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
		if m := heading.FindStringSubmatch(line); m != nil {
			if current.title != "" {
				sections = append(sections, current)
			}
			title := m[2]
			title = pdfVersion.ReplaceAllString(title, "")
			current = section{
				level: len(m[1]),
				title: title,
			}
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
	var keys []pdf.Name
	for key, val := range fontDict {
		if val == nil {
			continue
		}
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if order(keys[i]) != order(keys[j]) {
			return order(keys[i]) < order(keys[j])
		}
		return keys[i] < keys[j]
	})

	keyGlyphs := make([]*font.GlyphSeq, len(keys))
	var maxWidth float64
	for i, key := range keys {
		fDict := l.F["dict"]
		gg := fDict.F.Layout(nil, fDict.ptSize, pdf.AsString(key)+" ")
		keyGlyphs[i] = gg
		w := gg.TotalWidth()
		if w > maxWidth {
			maxWidth = w
		}
	}
	maxWidth += 10
	for _, gg := range keyGlyphs {
		w := gg.TotalWidth()
		delta := maxWidth - w
		if delta > 0 {
			gg.Seq[len(gg.Seq)-1].Advance += delta
		}
	}

	xBase := l.leftMargin + 5

	y0 := l.yPos + 2
	page.TextBegin()
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

	page.TextBegin()
	lineNo := 0

	flagsY := 0.0
	var flagsVal pdf.Integer
	for i, key := range keys {
		if title == "Font Descriptor" && key == "Flags" {
			flagsVal, _ = fontDict[key].(pdf.Integer)
			flagsY = l.yPos - l.F["dict"].ascent
		}

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
		desc := pdf.AsString(fontDict[key])
		if key == "CharProcs" && len(desc) > 20 {
			desc = "<< ... >>"
		}
		gg.Append(l.F["dict"].F.Layout(nil, l.F["dict"].ptSize, desc))

		page.TextSetFont(l.F["dict"].F, l.F["dict"].ptSize)
		page.TextShowGlyphs(gg)

		w := gg.TotalWidth()
		if w > maxWidth {
			maxWidth = w
		}
	}
	l.yPos++
	page.TextEnd()

	if maxWidth < titleWitdhPDF {
		maxWidth = titleWitdhPDF
	}
	if maxWidth < 170 {
		maxWidth = 170
	}

	if flagsY != 0 {
		page.PushGraphicsState()
		page.TextBegin()
		page.TextFirstLine(xBase+maxWidth+10, flagsY)
		page.SetFillColor(color.DeviceGray(0.5))
		page.TextShow("# " + font.FormatFlags(flagsVal))
		page.TextEnd()
		page.PopGraphicsState()
	}

	y2 := l.yPos - 2
	l.yPos -= 4

	_ = y1
	page.Rectangle(xBase-4, y2, maxWidth+8, y0-y2)
	page.MoveTo(xBase-4, y1)
	page.LineTo(xBase+maxWidth+4, y1)
	page.Stroke()

	l.yPos -= 18

	return yy
}

func (l *layout) connect(page *document.Page, y0, y1 float64, delta float64) {
	vLinePos := l.leftMargin - delta
	xBase := l.leftMargin
	page.PushGraphicsState()
	col := color.DeviceGray(0.5)
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

func (l *layout) addFont(key string, F font.Layouter, ptSize float64) {
	if l.F == nil {
		l.F = make(map[string]*pdfFont)
	}
	geom := F.GetGeometry()
	ascent := ptSize * geom.Ascent
	descent := ptSize * geom.Descent
	baseLineSkip := ptSize * geom.Leading

	l.F[key] = &pdfFont{F, ptSize, geom, ascent, descent, baseLineSkip}
}

type pdfFont struct {
	F            font.Layouter
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

var (
	heading    = regexp.MustCompile(`^(#+)\s+(.*?)\s*$`)
	pdfVersion = regexp.MustCompile(`\s+\(PDF \d\.\d\)\s*$`)
	findCode   = regexp.MustCompile("`.*?`|\\*.*?\\*")
)
