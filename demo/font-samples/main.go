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
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/builtin"
	"seehuhn.de/go/pdf/font/cid"
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

	var fileNames []string
	if *fontNamesFile != "" {
		f, err := os.Open(*fontNamesFile)
		if err != nil {
			log.Fatal(err)
		}
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			fileNames = append(fileNames, sc.Text())
		}
		if err := sc.Err(); err != nil {
			log.Fatal(err)
		}
	}
	fileNames = append(fileNames, flag.Args()...)

	title := fmt.Sprintf("%d Font Samples", len(fileNames))
	err = f.AddTitle(title, 10, 0, 24)
	if err != nil {
		log.Fatal(err)
	}

	for _, fileName := range fileNames {
		info, err := sfnt.ReadFile(fileName)
		if err != nil {
			log.Print(fileName + ":" + err.Error())
			continue
		}

		// disable any interaction between the glyphs
		info.Gdef = nil
		info.Gsub = nil
		info.Gpos = nil

		err = f.AddFontSample(fileName, info)
		if err != nil {
			log.Print(fileName + ":" + err.Error())
		}
	}

	err = f.ClosePage()
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

	page *pages.Page

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
	if f.page != nil && f.used+vSpace < f.textHeight {
		// If we have enough space, just return ...
		return nil
	}

	// ... otherwise start a new page.
	err := f.ClosePage()
	if err != nil {
		return err
	}

	page, err := pages.AppendPage(f.tree)
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
	f.page.StartLine(f.margin, f.margin+f.textHeight-f.used)
	f.page.ShowText(title)
	f.page.EndText()

	f.used += b

	return nil
}

func (f *fontSamples) AddFontSample(fileName string, info *sfnt.Info) error {
	instName := pdf.Name(fmt.Sprintf("X%d", f.fontNo))
	f.fontNo++
	X, err := cid.Embed(f.tree.Out, info, instName, language.AmericanEnglish)
	if err != nil {
		return err
	}

	bodyFont := f.bodyFont
	v1 := bodyFont.ToPDF16(10, bodyFont.Ascent)
	v2 := bodyFont.ToPDF16(10, bodyFont.BaseLineSkip-bodyFont.Ascent) +
		bodyFont.ToPDF16(7, bodyFont.Ascent)
	v3 := bodyFont.ToPDF16(7, bodyFont.BaseLineSkip-bodyFont.Ascent) +
		X.ToPDF16(24, X.Ascent)
	v4 := X.ToPDF16(24, X.BaseLineSkip-X.Ascent) + 12
	totalPartHeight := v1 + v2 + v3 + v4

	var parts []string
	parts = append(parts, info.FullName())
	parts = append(parts, fmt.Sprintf("%d glyphs", info.NumGlyphs()))
	if info.IsGlyf() {
		parts = append(parts, "glyf outlines")
	} else if info.IsCFF() {
		parts = append(parts, "CFF outlines")
		outlines := info.Outlines.(*cff.Outlines)
		if outlines.ROS != nil {
			parts = append(parts, "CID-keyed")
		}
	}
	if info.UnitsPerEm != 1000 {
		parts = append(parts, fmt.Sprintf("%d/em", info.UnitsPerEm))
	}
	subTitle := strings.Join(parts, ", ")

	var seq []glyph.Info
	total := 0.
	for gid := 0; gid < info.NumGlyphs() && len(seq) < 256; gid++ {
		if info.GlyphExtent(glyph.ID(gid)).IsZero() {
			continue
		}
		w := info.GlyphWidth(glyph.ID(gid))
		wf := X.ToPDF16(24, w)
		if total+wf > f.textWidth {
			break
		}
		seq = append(seq, glyph.Info{
			Gid:     glyph.ID(gid),
			Advance: w,
		})
		total += wf
	}

	err = f.MakeSpace(totalPartHeight)
	if err != nil {
		return err
	}

	page := f.page
	page.BeginText()
	page.StartLine(f.margin, f.margin+f.textHeight-f.used-v1)
	page.SetFont(bodyFont, 10)
	page.ShowText(subTitle)
	page.StartLine(0, -v2)
	page.SetFont(bodyFont, 7)
	page.ShowText(fileName)
	page.StartLine(0, -v3)
	page.SetFont(X, 24)
	page.ShowGlyphs(seq)
	page.EndText()

	f.used += totalPartHeight

	return nil
}
