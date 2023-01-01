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
	"fmt"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/text/language"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/boxes"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/builtin"
	"seehuhn.de/go/pdf/font/cid"
	"seehuhn.de/go/pdf/pages"
	"seehuhn.de/go/pdf/sfnt"
	"seehuhn.de/go/pdf/sfnt/glyph"
	"seehuhn.de/go/pdf/sfnt/header"
	"seehuhn.de/go/pdf/sfnt/opentype/gdef"
)

const (
	glyphBoxWidth = 36
	glyphFontSize = 24
)

var courier, theFont *font.Font
var rev map[glyph.ID]rune
var gdefInfo *gdef.Table

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: tt-glyph-table font.ttf")
		os.Exit(1)
	}
	fontFileName := os.Args[1]

	fd, err := os.Open(fontFileName)
	if err != nil {
		log.Fatal(err)
	}
	tt, err := sfnt.Read(fd)
	if err != nil {
		log.Fatal(err)
	}
	header, err := header.Read(fd)
	if err != nil {
		log.Fatal(err)
	}
	err = fd.Close()
	if err != nil {
		log.Fatal(err)
	}

	gdefInfo = tt.Gdef

	// gsub, err := gtab.ReadGsubTable()
	// if err != nil && !table.IsMissing(err) {
	// 	log.Fatal(err)
	// }

	out, err := pdf.Create("test.pdf")
	if err != nil {
		log.Fatal(err)
	}

	labelFont, err := builtin.Embed(out, "Helvetica", "L")
	if err != nil {
		log.Fatal(err)
	}
	courier, err = builtin.Embed(out, "Courier", "C")
	if err != nil {
		log.Fatal(err)
	}
	italic, err := builtin.Embed(out, "Times-Italic", "I")
	if err != nil {
		log.Fatal(err)
	}
	theFont, err = cid.Embed(out, tt, "X", language.AmericanEnglish)
	if err != nil {
		log.Fatal(err)
	}

	pageTree := pages.InstallTree(out, &pages.InheritableAttributes{
		MediaBox: pages.A4,
	})

	c := make(chan boxes.Box)
	res := make(chan error)
	go func() {
		res <- makePages(out, pageTree, c, labelFont)
	}()

	stretch := boxes.Glue(0, 1, 1, 1, 1)

	numGlyph := len(theFont.Widths)

	c <- boxes.Kern(36)
	c <- boxes.HBox(
		boxes.Text(labelFont, 10, "input file: "),
		boxes.Text(courier, 10, fontFileName),
	)
	c <- boxes.Kern(12)
	c <- boxes.Text(labelFont, 10, "family name: "+tt.FamilyName)
	c <- boxes.Text(labelFont, 10, "width: "+tt.Width.String())
	c <- boxes.Text(labelFont, 10, "weight: "+tt.Weight.String())

	var flags []string
	if tt.IsItalic {
		flags = append(flags, "italic")
	}
	if tt.IsBold {
		flags = append(flags, "bold")
	}
	if tt.IsRegular {
		flags = append(flags, "regular")
	}
	if tt.IsOblique {
		flags = append(flags, "oblique")
	}
	if tt.IsSerif {
		flags = append(flags, "serif")
	}
	if tt.IsScript {
		flags = append(flags, "script")
	}
	if len(flags) > 0 {
		c <- boxes.Text(labelFont, 10, "flags: "+strings.Join(flags, ", "))
	}

	c <- boxes.Kern(12)
	if tt.Description != "" {
		c <- boxes.Text(labelFont, 10, "description: "+tt.Description)
	}
	if tt.SampleText != "" {
		c <- boxes.Text(labelFont, 10, "sample text: "+tt.SampleText)
	}
	c <- boxes.Text(labelFont, 10, "version: "+tt.Version.String())
	c <- boxes.Text(labelFont, 10, "creation time: "+tt.CreationTime.Format("2006-01-02 15:04:05"))
	c <- boxes.Text(labelFont, 10, "modification time: "+tt.ModificationTime.Format("2006-01-02 15:04:05"))
	c <- boxes.Kern(12)
	c <- boxes.Text(labelFont, 10, "copyright: "+tt.Copyright)
	c <- boxes.Text(labelFont, 10, "trademark: "+tt.Trademark)
	c <- boxes.Text(labelFont, 10, "permissions: "+tt.PermUse.String())
	c <- boxes.Kern(12)
	c <- boxes.Text(labelFont, 10, fmt.Sprintf("units/em: %d", tt.UnitsPerEm))
	c <- boxes.Kern(12)
	c <- boxes.Text(labelFont, 10, fmt.Sprintf("ascent: %d", tt.Ascent))
	c <- boxes.Text(labelFont, 10, fmt.Sprintf("descent: %d", tt.Descent))
	c <- boxes.Text(labelFont, 10, fmt.Sprintf("linegap: %d", tt.LineGap))
	c <- boxes.Text(labelFont, 10, fmt.Sprintf("cap height: %d", tt.CapHeight))
	c <- boxes.Text(labelFont, 10, fmt.Sprintf("x-height: %d", tt.XHeight))
	c <- boxes.Kern(12)
	c <- boxes.Text(labelFont, 10, fmt.Sprintf("italic angle: %.1f", tt.ItalicAngle))
	c <- boxes.Text(labelFont, 10, fmt.Sprintf("underline position: %d", tt.UnderlinePosition))
	c <- boxes.Text(labelFont, 10, fmt.Sprintf("underline thickness: %d", tt.UnderlineThickness))
	c <- boxes.Kern(12)
	c <- boxes.Text(labelFont, 10, fmt.Sprintf("number of glyphs: %d", numGlyph))
	c <- boxes.Kern(12)
	c <- boxes.Text(labelFont, 10, "SFNT tables:")
	var names []string
	for name := range header.Toc {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		c <- boxes.HBox(
			boxes.Text(courier, 10, "  •"+name+" "),
			boxes.HBoxTo(72,
				stretch,
				boxes.Text(labelFont, 10, fmt.Sprintf("%d bytes", header.Toc[name].Length)),
			),
		)
	}
	c <- nil // new page

	rev = make(map[glyph.ID]rune)
	min, max := tt.CMap.CodeRange()
	for r := min; r <= max; r++ {
		gid := tt.CMap.Lookup(r)
		if gid == 0 {
			continue
		}
		r2 := rev[gid]
		if r2 == 0 || r < r2 {
			rev[gid] = r
		}
	}
	for row := 0; 10*row < numGlyph; row++ {
		colBoxes := []boxes.Box{stretch}
		label := strconv.Itoa(row)
		if label == "0" {
			label = ""
		}
		h := boxes.HBoxTo(20,
			stretch,
			boxes.Text(courier, 10, label),
			boxes.Text(italic, 10, "x"),
		)
		colBoxes = append(colBoxes, h, boxes.Kern(20), rules{})
		for col := 0; col < 10; col++ {
			idx := col + 10*row
			if idx < numGlyph {
				colBoxes = append(colBoxes, glyphBox(idx))
			} else {
				colBoxes = append(colBoxes, boxes.Kern(glyphBoxWidth))
			}
		}
		colBoxes = append(colBoxes, stretch)
		c <- boxes.HBoxTo(textWidth, colBoxes...)
	}

	close(c)
	err = <-res
	if err != nil {
		log.Fatal(err)
	}

	err = out.Close()
	if err != nil {
		log.Fatal(err)
	}
}
