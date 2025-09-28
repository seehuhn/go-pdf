// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2024  Jochen Voss <voss@seehuhn.de>
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

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/graphics/color"
)

const (
	fontSize = 12
	leading  = 15
)

type output struct {
	doc  *document.MultiPage
	page *document.Page

	top, bottom float64

	yPos   float64
	lineNo int

	err error
}

func NewOutput(fname string, v pdf.Version) (*output, error) {
	paper := document.A4
	opt := &pdf.WriterOptions{
		// HumanReadable: true,
	}
	doc, err := document.CreateMultiPage(fname, paper, v, opt)
	if err != nil {
		return nil, err
	}

	page := doc.AddPage()
	page.TextBegin()

	res := &output{
		doc:  doc,
		page: page,

		top:    paper.URy - 36 - 12,
		bottom: 36,
		yPos:   paper.URy - 36 - 12,
	}
	return res, nil
}

func (o *output) Close() error {
	if o.err != nil {
		return o.err
	}

	o.page.TextEnd()
	err := o.page.Close()
	if err != nil {
		o.err = err
		return err
	}
	err = o.doc.Close()
	if err != nil {
		o.err = err
		return err
	}
	return nil
}

func (o *output) Println(args ...any) {
	if o.err != nil {
		return
	}

	if o.yPos < o.bottom {
		o.page.TextEnd()
		err := o.page.Close()
		if err != nil {
			o.err = err
			return
		}

		o.page = o.doc.AddPage()
		o.page.TextBegin()
		o.yPos = o.top
		o.lineNo = 0
	}

	switch o.lineNo {
	case 0:
		o.page.TextFirstLine(72, o.yPos)
	case 1:
		o.page.TextSecondLine(0, -leading)
	default:
		o.page.TextNextLine()
	}
	o.lineNo++
	o.yPos -= leading

	for _, arg := range args {
		switch arg := arg.(type) {
		case string:
			o.page.TextShow(arg)
		case pdf.String:
			o.page.TextShowRaw(arg)
		case font.Instance:
			o.page.TextSetFont(arg, fontSize)
		case color.Color:
			o.page.SetFillColor(arg)
		default:
			panic(fmt.Sprintf("unexpected type %T", arg))
		}
	}
}
