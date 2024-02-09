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

package testcases

import (
	"math"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font/gofont"
	"seehuhn.de/go/pdf/graphics"
)

var Paper = &pdf.Rectangle{
	LLx: -50,
	LLy: -50,
	URx: 450,
	URy: 450,
}

type TestCase func(*document.Page) error

var All = []TestCase{
	func(p *document.Page) error {
		E, err := gofont.GoRegular.Embed(p.Out, nil)
		if err != nil {
			return err
		}
		p.TextSetFont(E, 24)
		return nil
	},
	func(p *document.Page) error {
		E, err := gofont.GoRegular.Embed(p.Out, nil)
		if err != nil {
			return err
		}
		p.TextSetFont(E, 24)
		p.TextFirstLine(10, 10)
		p.TextShow("Hello, world!")
		return nil
	},
	func(p *document.Page) error {
		E, err := gofont.GoRegular.Embed(p.Out, nil)
		if err != nil {
			return err
		}
		p.TextSetFont(E, 24)
		p.TextFirstLine(10, 50)
		p.TextShow("Hello, world!")
		p.TextSecondLine(0, -25)
		p.TextShow("Hello, world!")
		return nil
	},
	func(p *document.Page) error {
		E, err := gofont.GoRegular.Embed(p.Out, nil)
		if err != nil {
			return err
		}
		p.TextSetFont(E, 24)
		p.TextSetMatrix(graphics.Rotate(math.Pi / 4))
		p.TextShow("Hello, world!")
		return nil
	},
	func(p *document.Page) error {
		E, err := gofont.GoRegular.Embed(p.Out, nil)
		if err != nil {
			return err
		}
		p.TextSetFont(E, 48)
		p.TextSetHorizontalScaling(50)
		p.TextShow("Hello, world!")
		return nil
	},
}

type Answer struct {
	X, Y float64
}

//go:generate go run generate.go
