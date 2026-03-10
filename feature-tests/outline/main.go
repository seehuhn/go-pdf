// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2026  Jochen Voss <voss@seehuhn.de>
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
	"log"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font/standard"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/outline"
)

func main() {
	page, err := document.CreateSinglePage("test.pdf", document.A4, pdf.V2_0, nil)
	if err != nil {
		log.Fatal(err)
	}

	F := standard.Helvetica.New()

	page.TextBegin()
	page.TextSetFont(F, 14)
	page.TextFirstLine(72, 700)
	page.TextShow("see outline items (bookmarks)")
	page.TextEnd()

	type style struct {
		name   string
		bold   bool
		italic bool
	}
	styles := []style{
		{"normal", false, false},
		{"bold", true, false},
		{"italic", false, true},
		{"bold italic", true, true},
	}

	type namedColor struct {
		name string
		rgb  color.DeviceRGB
	}
	colors := []namedColor{
		{"red", color.DeviceRGB{1, 0, 0}},
		{"orange", color.DeviceRGB{1, 0.5, 0}},
		{"yellow", color.DeviceRGB{0.8, 0.8, 0}},
		{"green", color.DeviceRGB{0, 0.5, 0}},
		{"blue", color.DeviceRGB{0, 0, 1}},
		{"purple", color.DeviceRGB{0.5, 0, 0.5}},
	}

	ol := &outline.Outline{}
	for _, s := range styles {
		item := ol.AddItem(s.name)
		item.Bold = s.bold
		item.Italic = s.italic
		item.Open = true
		for _, c := range colors {
			child := item.AddChild(c.name)
			child.Bold = s.bold
			child.Italic = s.italic
			child.Color = c.rgb
		}
	}

	outlineRef, err := ol.Encode(page.RM)
	if err != nil {
		log.Fatal(err)
	}
	if ref, ok := outlineRef.(pdf.Reference); ok {
		page.Out.GetMeta().Catalog.Outlines = ref
	}

	err = page.Close()
	if err != nil {
		log.Fatal(err)
	}
}
