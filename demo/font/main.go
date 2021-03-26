// seehuhn.de/go/pdf - support for reading and writing PDF files
// Copyright (C) 2021  Jochen Voss <voss@seehuhn.de>
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
	"fmt"
	"log"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/builtin"
	"seehuhn.de/go/pdf/pages"
)

const (
	FontName = "Times-Roman"
	FontSize = 48.0
)

func WritePage(out *pdf.Writer, text string, width, height float64) error {
	subset := make(map[rune]bool)
	for _, r := range text {
		subset[r] = true
	}

	F1, err := builtin.Embed(out, FontName, subset)
	// F1, err := truetype.Embed(out, "../../font/truetype/FreeSerif.ttf", subset)
	if err != nil {
		return err
	}

	page, err := pages.SinglePage(out, &pages.Attributes{
		Resources: pdf.Dict{
			"Font": pdf.Dict{"F1": F1.Ref},
		},
		MediaBox: &pdf.Rectangle{
			URx: width,
			URy: height,
		},
	})
	if err != nil {
		return err
	}

	margin := 50.0
	baseLineSkip := 1.2 * FontSize

	q := FontSize / 1000

	_, err = page.Write([]byte("q\n1 .5 .5 RG\n"))
	if err != nil {
		return err
	}
	yPos := height - margin - F1.Ascent*q
	for y := yPos; y > margin; y -= baseLineSkip {
		_, err = page.Write([]byte(fmt.Sprintf("%.1f %.1f m %.1f %.1f l\n",
			margin, y, width-margin, y)))
		if err != nil {
			return err
		}
	}
	_, err = page.Write([]byte("s\nQ\n"))
	if err != nil {
		return err
	}

	var codes []font.GlyphIndex
	var last font.GlyphIndex
	for _, r := range text {
		c, ok := F1.CMap[r]
		if !ok {
			panic("character " + string([]rune{r}) + " not in font")
		}

		if len(codes) > 0 && F1.Ligatures != nil {
			pair := font.GlyphPair{last, c}
			lig, ok := F1.Ligatures[pair]
			if ok {
				codes = codes[:len(codes)-1]
				c = lig
			}
		}

		codes = append(codes, c)
		last = c
	}

	_, err = page.Write([]byte("q\n.2 1 .2 RG\n"))
	if err != nil {
		return err
	}
	var formatted pdf.Array
	pos := 0
	xPos := margin
	for i, c := range codes {
		bbox := F1.GlyphExtent[c]
		if !bbox.IsZero() {
			_, err = page.Write([]byte(fmt.Sprintf("%.2f %.2f %.2f %.2f re\n",
				xPos+float64(bbox.LLx)*q,
				yPos+float64(bbox.LLy)*q,
				float64(bbox.URx)*q-float64(bbox.LLx)*q,
				float64(bbox.URy)*q-float64(bbox.LLy)*q)))
			if err != nil {
				return err
			}
		}
		xPos += float64(F1.Width[c]) * q

		if i == len(codes)-1 {
			formatted = append(formatted, pdf.String(F1.Enc(codes[pos:]...)))
			break
		}

		kern, ok := F1.Kerning[font.GlyphPair{c, codes[i+1]}]
		if !ok {
			continue
		}
		xPos += float64(kern) * q
		kObj := pdf.Number(-kern)
		formatted = append(formatted,
			pdf.String(F1.Enc(codes[pos:(i+1)]...)), kObj)
		pos = i + 1
	}
	_, err = page.Write([]byte("s\nQ\n"))
	if err != nil {
		return err
	}

	_, err = page.Write([]byte(fmt.Sprintf("BT\n/F1 %f Tf\n%.1f %.1f Td\n",
		FontSize, margin, yPos)))
	if err != nil {
		return err
	}
	err = formatted.PDF(page)
	if err != nil {
		return err
	}
	_, err = page.Write([]byte(" TJ\nET"))
	if err != nil {
		return err
	}

	err = page.Close()
	if err != nil {
		return err
	}
	return nil
}

func main() {
	out, err := pdf.Create("test.pdf")
	if err != nil {
		log.Fatal(err)
	}

	const width = 8 * 72
	const height = 6 * 72

	text := "Waterﬂask & ﬁsh bucket"
	err = WritePage(out, text, width, height)
	if err != nil {
		log.Fatal(err)
	}

	err = out.SetInfo(pdf.Dict{
		"Title":  pdf.TextString("PDF Test Document"),
		"Author": pdf.TextString("Jochen Voß"),
	})
	if err != nil {
		log.Fatal(err)
	}

	err = out.Close()
	if err != nil {
		log.Fatal(err)
	}
}
