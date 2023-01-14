// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2022  Jochen Voss <voss@seehuhn.de>
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

	"golang.org/x/text/language"

	"seehuhn.de/go/sfnt"
	"seehuhn.de/go/sfnt/cff"
	"seehuhn.de/go/sfnt/glyph"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/boxes"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/builtin"
	"seehuhn.de/go/pdf/font/cid"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/pages"
)

func main() {
	fontNamesFile := flag.String("f", "", "file containing font names")
	flag.Parse()

	w, err := pdf.Create("out.pdf")
	if err != nil {
		log.Fatal(err)
	}

	labelFont, err := builtin.Embed(w, "Helvetica", "L")
	if err != nil {
		log.Fatal(err)
	}
	titleFont, err := builtin.Embed(w, "Helvetica-Bold", "T")
	if err != nil {
		log.Fatal(err)
	}

	paper := pages.A4
	pageTree := pages.InstallTree(w, &pages.InheritableAttributes{
		MediaBox: pages.A4,
	})

	const margin = 50
	f := fontSamples{
		tree: pageTree,

		textWidth:  paper.URx - 2*margin,
		textHeight: paper.URy - 2*margin,
		margin:     margin,

		bodyFont:  labelFont,
		titleFont: titleFont,
	}
	_ = f // TODO(voss): finish the conversion to the system

	c := make(chan boxes.Box)
	res := make(chan error)
	go func() {
		res <- makePages(w, pageTree, c, labelFont)
	}()

	var fnames []string
	if *fontNamesFile != "" {
		f, err := os.Open(*fontNamesFile)
		if err != nil {
			log.Fatal(err)
		}
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			fnames = append(fnames, sc.Text())
		}
		if err := sc.Err(); err != nil {
			log.Fatal(err)
		}
	}
	fnames = append(fnames, flag.Args()...)

	title := boxes.Text(titleFont, 10, fmt.Sprintf("%d Font Samples", len(fnames)))
	c <- title
	c <- boxes.Kern(12)

	for i, fname := range fnames {
		r, err := os.Open(fname)
		if err != nil {
			log.Print(fname + ":" + err.Error())
			continue
		}
		info, err := sfnt.Read(r)
		if err != nil {
			log.Print(fname + ":" + err.Error())
			r.Close()
			continue
		}
		err = r.Close()
		if err != nil {
			log.Fatal(err)
		}

		info.Gdef = nil
		info.Gsub = nil
		info.Gpos = nil

		var title []string
		title = append(title, info.FullName())
		title = append(title, fmt.Sprintf("%d glyphs", info.NumGlyphs()))
		if info.IsGlyf() {
			title = append(title, "glyf outlines")
		} else if info.IsCFF() {
			title = append(title, "CFF outlines")
			outlines := info.Outlines.(*cff.Outlines)
			if outlines.ROS != nil {
				title = append(title, "CID-keyed")
			}
		}
		if info.UnitsPerEm != 1000 {
			title = append(title, fmt.Sprintf("%d/em", info.UnitsPerEm))
		}
		c <- boxes.Text(labelFont, 10, strings.Join(title, ", "))
		c <- boxes.Text(labelFont, 7, fname)

		var seq []glyph.Info
		total := 0.
		for gid := 0; gid < info.NumGlyphs(); gid++ {
			if info.GlyphExtent(glyph.ID(gid)).IsZero() {
				continue
			}
			w := info.GlyphWidth(glyph.ID(gid))
			wf := w.AsFloat(24 / float64(info.UnitsPerEm))
			if total+wf > 72*6 {
				break
			}
			seq = append(seq, glyph.Info{
				Gid:     glyph.ID(gid),
				Advance: w,
			})
			total += wf
			if len(seq) >= 100 {
				break
			}
		}

		if len(seq) > 0 {
			F, err := cid.Embed(w, info, pdf.Name(fmt.Sprintf("F%d", i)), language.AmericanEnglish)
			if err != nil {
				log.Fatal(err)
			}
			c <- &boxes.TextBox{
				Font:     F,
				FontSize: 24,
				Glyphs:   seq,
			}
		} else {
			c <- boxes.Text(labelFont, 10, "(no glyphs)")
		}
		c <- boxes.Kern(12)
	}

	close(c)
	err = <-res
	if err != nil {
		log.Fatal(err)
	}

	err = w.Close()
	if err != nil {
		log.Fatal(err)
	}
}

type fontSamples struct {
	tree *pages.Tree

	textWidth  float64
	textHeight float64
	margin     float64

	used float64 // vertical amount of page space currently used

	bodyFont  *font.Font
	titleFont *font.Font

	page *graphics.Page

	pageNo int
	fontNo int
}

func (f *fontSamples) ClosePage() error {
	if f.page == nil {
		return nil
	}

	f.pageNo++
	f.page.BeginText()
	f.page.SetFont(f.bodyFont, 10)
	f.page.StartLine(f.margin+0.5*f.textWidth, f.margin-20)
	f.page.ShowTextAligned(fmt.Sprintf("- %d -", f.pageNo), 0, 0.5)
	f.page.EndText()

	_, err := f.page.Close()
	f.page = nil

	return err
}

func (f *fontSamples) MakeSpace(vSpace float64) error {
	if f.page != nil && f.used+vSpace < f.textWidth {
		// If we have enough space, just return ...
		return nil
	}

	// ... otherwise start a new page.
	err := f.ClosePage()
	if err != nil {
		return err
	}

	page, err := graphics.AppendPage(f.tree)
	if err != nil {
		return err
	}

	f.page = page
	f.used = 0
	return nil
}

func (f *fontSamples) AddTitle(title string, fontSize, a, b float64) error {
	err := f.MakeSpace(a + b + 72)
	if err != nil {
		return err
	}

	f.used += a
	f.page.BeginText()
	f.page.SetFont(f.titleFont, fontSize)
	f.page.StartLine(f.margin+0.5*f.textWidth, f.margin+f.textHeight-f.used)
	f.page.ShowTextAligned(title, 0, 0.5)
	f.page.EndText()

	f.used += b

	return nil
}

func makePages(w *pdf.Writer, tree *pages.Tree, c <-chan boxes.Box, labelFont *font.Font) error {
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

		page, err := graphics.NewPage(w)
		if err != nil {
			return err
		}

		withMargins.Draw(page, 0, withMargins.Extent().Depth)

		dict, err := page.Close()
		if err != nil {
			return err
		}
		_, err = tree.AppendPage(dict)
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
