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

	"golang.org/x/text/language"

	"seehuhn.de/go/postscript/funit"

	"seehuhn.de/go/sfnt/glyph"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/color"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cff"
	"seehuhn.de/go/pdf/font/gofont"
	"seehuhn.de/go/pdf/font/opentype"
	"seehuhn.de/go/pdf/font/truetype"
	"seehuhn.de/go/pdf/font/type1"
)

func main() {
	err := doit()
	if err != nil {
		panic(err)
	}
}

const exampleText = `“Hello World!”`

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
		leftMargin:  108.0,
		rightMargin: 144.0,
	}

	F, err := type1.TimesRoman.Embed(doc.Out, "F")
	if err != nil {
		return err
	}
	l.addFont("text", F, 10)

	I, err := type1.TimesItalic.Embed(doc.Out, "I")
	if err != nil {
		return err
	}
	l.addFont("it", I, 10)

	S, err := type1.Helvetica.Embed(doc.Out, "S")
	if err != nil {
		return err
	}
	l.addFont("code", S, 9)
	l.addFont("dict", S, 9)

	SB, err := type1.HelveticaBold.Embed(doc.Out, "B")
	if err != nil {
		return err
	}
	l.addFont("chapter", SB, 24)
	l.addFont("section", SB, 18)

	pageNo := 1
	fontNo := 1
	for _, s := range sections {
		title := s.title
		intro := s.lines

		var X font.Font
		var ffKey pdf.Name
		switch title {
		case "Simple PDF Fonts":
			// part 1
		case "Type1 Fonts":
			t1, err := gofont.Type1(gofont.GoRegular)
			if err != nil {
				return err
			}
			X, err = type1.New(t1)
			if err != nil {
				return err
			}
			ffKey = "FontFile"
		case "Builtin Fonts":
			X = type1.Helvetica
		case "Simple CFF Fonts":
			otf, err := gofont.OpenType(gofont.GoRegular)
			if err != nil {
				return err
			}
			X, err = cff.NewSimple(otf, language.English)
			if err != nil {
				return err
			}
			ffKey = "FontFile3"
		case "Simple CFF-based OpenType Fonts":
			ttf, err := gofont.OpenType(gofont.GoRegular)
			if err != nil {
				return err
			}
			X, err = opentype.NewSimpleCFF(ttf, language.English)
			if err != nil {
				return err
			}
			ffKey = "FontFile3"
		case "Multiple Master Fonts":
			// not supported
		case "Simple TrueType Fonts":
			ttf, err := gofont.TrueType(gofont.GoRegular)
			if err != nil {
				return err
			}
			X, err = truetype.NewSimple(ttf, language.English)
			if err != nil {
				return err
			}
			ffKey = "FontFile2"
		case "Glyf-based OpenType Fonts":
			otf, err := gofont.TrueType(gofont.GoRegular)
			if err != nil {
				return err
			}
			X, err = opentype.NewSimpleGlyf(otf, language.English)
			if err != nil {
				return err
			}
			if err != nil {
				return err
			}
			ffKey = "FontFile3"
		case "Type3 Fonts":
			X, err = gofont.Type3(gofont.GoRegular)
			if err != nil {
				return err
			}
		case "Composite PDF Fonts":
			// part 2
		case "Composite CFF Fonts":
			otf, err := gofont.OpenType(gofont.GoRegular)
			if err != nil {
				return err
			}
			X, err = cff.NewComposite(otf, language.English)
			if err != nil {
				return err
			}
			ffKey = "FontFile3"
		case "Composite CFF-based OpenType Fonts":
			otf, err := gofont.OpenType(gofont.GoRegular)
			if err != nil {
				return err
			}
			X, err = opentype.NewCompositeCFF(otf, language.English)
			if err != nil {
				return err
			}
			ffKey = "FontFile3"
		case "Composite TrueType Fonts":
			ttf, err := gofont.TrueType(gofont.GoRegular)
			if err != nil {
				return err
			}
			X, err = truetype.NewComposite(ttf, language.English)
			if err != nil {
				return err
			}
			ffKey = "FontFile2"
		case "Composite Glyf-based OpenType Fonts":
			otf, err := gofont.TrueType(gofont.GoRegular)
			if err != nil {
				return err
			}
			X, err = opentype.NewCompositeGlyf(otf, language.English)
			if err != nil {
				return err
			}
			ffKey = "FontFile3"
		default:
			panic("unexpected section " + title)
		}

		page := doc.AddPage()

		page.TextStart()
		if s.level == 1 {
			gg := SB.Layout(title, l.F["chapter"].ptSize)
			w := l.F["chapter"].geom.ToPDF(l.F["chapter"].ptSize, gg.AdvanceWidth())
			l.yPos = paper.URy - l.topMargin - 72 - l.F["chapter"].ascent
			xPos := (paper.URx-l.rightMargin-l.leftMargin-w)/2 + l.leftMargin
			page.SetFillColor(color.Gray(0.3))
			page.TextSetFont(l.F["chapter"].F, l.F["chapter"].ptSize)
			page.TextFirstLine(xPos, l.yPos)
			l.yPos -= -l.F["chapter"].descent + 2*l.F["text"].baseLineSkip + l.F["text"].ascent
		} else {
			l.yPos = paper.URy - l.topMargin - l.F["section"].ascent
			page.SetFillColor(color.Gray(0.15))
			page.TextSetFont(l.F["section"].F, l.F["section"].ptSize)
			page.TextFirstLine(l.leftMargin, l.yPos)
			l.yPos -= -l.F["section"].descent + 2*l.F["text"].baseLineSkip + l.F["text"].ascent
		}
		page.TextShow(title)
		page.TextEnd()
		page.SetFillColor(color.Gray(0))

		page.TextStart()
		if X != nil {
			intro = append(intro, "", fmt.Sprintf("Example (see `fonts%02d.pdf`):", fontNo))
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

		if X != nil {
			err = writeSinglePage(X, fontNo)
			if err != nil {
				return err
			}
			fontNo++
		}

		if X != nil {
			Y, err := X.Embed(doc.Out, "X")
			if err != nil {
				return err
			}

			l.yPos -= 20
			page.TextStart()
			page.TextFirstLine(l.leftMargin, l.yPos)
			page.TextSetFont(Y, 24)
			page.TextShow(exampleText)
			page.TextEnd()
			l.yPos -= 30

			err = Y.Close()
			if err != nil {
				return err
			}

			fontDict, err := pdf.GetDict(data, Y.Reference())
			if err != nil {
				return err
			}
			yFD := l.ShowDict(page, fontDict, "Font Dictionary", Y.Reference())
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
		}

		// add the page number
		page.TextSetFont(l.F["text"].F, l.F["text"].ptSize)
		page.TextStart()
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

func writeSinglePage(F font.Font, no int) error {
	fname := fmt.Sprintf("fonts%02d.pdf", no)

	page, err := document.CreateSinglePage(fname, document.A5r, nil)
	if err != nil {
		return err
	}

	X, err := F.Embed(page.Out, "X")
	if err != nil {
		return err
	}

	page.TextStart()
	page.TextFirstLine(72, 72)
	page.TextSetFont(X, 24)
	page.TextShow(exampleText)
	page.TextEnd()

	err = X.Close()
	if err != nil {
		return err
	}

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
		desc := pdf.Format(fontDict[key])
		if key == "CharProcs" && len(desc) > 20 {
			desc = "<< ... >>"
		}
		gg = append(gg, l.F["dict"].F.Layout(desc, l.F["dict"].ptSize)...)

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

	l.yPos -= 18

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

var (
	heading    = regexp.MustCompile(`^(#+)\s+(.*?)\s*$`)
	pdfVersion = regexp.MustCompile(`\s+\(PDF \d\.\d\)\s*$`)
	findCode   = regexp.MustCompile("`.*?`|\\*.*?\\*")
)
