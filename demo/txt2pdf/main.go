// seehuhn.de/go/pdf - a library for reading and writing PDF files
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
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/boxes"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/builtin"
	"seehuhn.de/go/pdf/font/sfnt"
	"seehuhn.de/go/pdf/font/sfnt/simple"
	"seehuhn.de/go/pdf/locale"
	"seehuhn.de/go/pdf/pages"
)

const tabWidth = 4

var (
	fontFile = flag.String("f", "Courier", "the font to use")
	version  = flag.String("V", pdf.V1_7.String(), "PDF version to write")
)

func main() {
	flag.Parse()

	V, err := pdf.ParseVersion(*version)
	if err != nil {
		log.Fatal(err)
	}

	for _, inName := range flag.Args() {
		baseName := strings.TrimSuffix(inName, ".txt")
		var outName string
		for i := 1; ; i++ {
			if i == 1 {
				outName = baseName + ".pdf"
			} else {
				outName = fmt.Sprintf("%s-%d.pdf", baseName, i)
			}
			_, err := os.Stat(outName)
			if os.IsNotExist(err) {
				break
			} else if err != nil {
				log.Fatal(err)
			}
		}
		err := typesetFile(inName, outName, V)
		if err != nil {
			log.Fatal(err)
		}
	}
}

func typesetFile(inName, outName string, V pdf.Version) error {
	fmt.Println(inName, "->", outName)

	in, err := os.Open(inName)
	if err != nil {
		return err
	}
	defer in.Close()

	fd, err := os.Create(outName)
	if err != nil {
		return err
	}
	defer fd.Close()
	opt := &pdf.WriterOptions{
		Version: V,
	}
	out, err := pdf.NewWriter(fd, opt)
	if err != nil {
		return err
	}

	out.SetInfo(&pdf.Info{
		Title:        inName,
		Producer:     "seehuhn.de/go/pdf/demo/txt2pdf",
		CreationDate: time.Now(),
	})

	var Font *font.Font
	if strings.HasSuffix(*fontFile, ".ttf") || strings.HasSuffix(*fontFile, ".otf") {
		fd, err := os.Open(*fontFile)
		if err != nil {
			return err
		}
		info, err := sfnt.Read(fd)
		if err != nil {
			fd.Close()
			return err
		}
		err = fd.Close()
		if err != nil {
			return err
		}
		Font, err = simple.Embed(out, info, "F", locale.EnUS)
		if err != nil {
			return err
		}
	} else {
		Font, err = builtin.Embed(out, *fontFile, "F")
		if err != nil {
			return err
		}
	}

	labelFont, err := builtin.Embed(out, "Helvetica", "L")
	if err != nil {
		log.Fatal(err)
	}

	pageTree := pages.NewPageTree(out, &pages.DefaultAttributes{
		MediaBox: pages.A4,
	})

	c := make(chan boxes.Box)
	res := make(chan error)
	go func() {
		res <- makePages(out, pageTree, c, labelFont)
	}()

	scanner := bufio.NewScanner(in)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "\t") {
			var rr []rune
			col := 0
			for _, r := range line {
				if r == '\t' {
					for {
						rr = append(rr, ' ')
						col++
						if col%tabWidth == 0 {
							break
						}
					}
				} else {
					rr = append(rr, r)
				}
			}
			line = string(rr)
		}
		c <- boxes.Text(Font, 10, line)
	}

	close(c)
	err = <-res
	if err != nil {
		return err
	}

	return out.Close()
}

func makePages(w *pdf.Writer, tree *pages.PageTree, c <-chan boxes.Box, labelFont *font.Font) error {
	topMargin := 36.
	rightMargin := 50.
	bottomMargin := 36.
	leftMargin := 50.
	paperWidth := pages.A4.URx
	textWidth := paperWidth - rightMargin - leftMargin
	paperHeight := pages.A4.URy
	maxHeight := paperHeight - topMargin - bottomMargin

	p := boxes.Parameters{
		BaseLineSkip: 0,
	}

	var body []boxes.Box
	pageNo := 1
	flush := func() error {
		pageList := []boxes.Box{
			boxes.Kern(topMargin),
		}
		pageList = append(pageList, body...)
		pageList = append(pageList,
			boxes.Glue(0, 1, 1, 1, 1),
			boxes.HBoxTo(textWidth,
				boxes.Glue(0, 1, 1, 1, 1),
				boxes.Text(labelFont, 10, fmt.Sprintf("- %d -", pageNo)),
				boxes.Glue(0, 1, 1, 1, 1),
			),
			boxes.Kern(18),
		)
		pageBody := p.VBoxTo(paperHeight, pageList...)
		withMargins := boxes.HBoxTo(paperWidth, boxes.Kern(leftMargin), pageBody)

		pageFonts := pdf.Dict{}
		boxes.Walk(pageBody, func(box boxes.Box) {
			switch b := box.(type) {
			case *boxes.TextBox:
				font := b.Layout.Font
				pageFonts[font.InstName] = font.Ref
			}
		})
		attr := &pages.Attributes{
			Resources: &pages.Resources{
				Font: pageFonts,
			},
		}
		page, err := tree.NewPage(attr)
		if err != nil {
			return err
		}
		withMargins.Draw(page, 0, withMargins.Extent().Depth)
		err = page.Close()
		if err != nil {
			return err
		}

		body = body[:0]
		pageNo++

		return nil
	}

	var totalHeight float64
	for box := range c {
		ext := box.Extent()
		h := ext.Height + ext.Depth
		if len(body) > 0 && totalHeight+h > maxHeight {
			err := flush()
			if err != nil {
				return err
			}
			totalHeight = 0
		}
		body = append(body, box)
		totalHeight += h
	}
	return flush()
}
