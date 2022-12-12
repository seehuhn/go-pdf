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

package layout

import (
	"fmt"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/boxes"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/pages2"
)

// MakePages breaks a stream of boxes into pages.
func MakePages(w *pdf.Writer, tree *pages2.Tree, c <-chan boxes.Box, labelFont *font.Font) error {
	topMargin := 36.
	rightMargin := 50.
	bottomMargin := 36.
	leftMargin := 50.
	paperWidth := pages2.A4.URx
	textWidth := paperWidth - rightMargin - leftMargin
	paperHeight := pages2.A4.URy
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
