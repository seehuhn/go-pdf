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
	"seehuhn.de/go/pdf/font"
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
		// Test that the text position is (0, 0) after starting a new page.
		// We still need to set a font so that there is a font size available
		// for the text position calculation.
		E, err := gofont.GoRegular.Embed(p.Out, nil)
		if err != nil {
			return err
		}
		p.TextSetFont(E, 24)
		return nil
	},

	func(p *document.Page) error {
		// Test the normal case: we use TextSetFont and print simple string.
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
		// Test a two-line text, using TextSecondLine.
		E, err := gofont.GoRegular.Embed(p.Out, nil)
		if err != nil {
			return err
		}
		p.TextSetFont(E, 24)
		p.TextFirstLine(10, 50)
		p.TextShow("Hello, world!")
		p.TextSecondLine(0, -25)
		p.TextShow("Hello again, world!")
		return nil
	},

	func(p *document.Page) error {
		// Test a three-line text, using TextSecondLine and TextNextLine.
		E, err := gofont.GoRegular.Embed(p.Out, nil)
		if err != nil {
			return err
		}
		p.TextSetFont(E, 24)
		p.TextFirstLine(10, 75)
		p.TextShow("Hello, world!")
		p.TextSecondLine(0, -25)
		p.TextShow("Hello again, world!")
		p.TextNextLine()
		p.TextShow("And again.")
		return nil
	},

	func(p *document.Page) error {
		// Test TextSetCharacterSpacing.
		E, err := gofont.GoRegular.Embed(p.Out, nil)
		if err != nil {
			return err
		}
		p.TextSetFont(E, 24)
		p.TextSetCharacterSpacing(10)
		p.TextShow("Hello, world!")
		return nil
	},

	func(p *document.Page) error {
		// Test TextSetCharacterSpacing with negative spacing.
		E, err := gofont.GoRegular.Embed(p.Out, nil)
		if err != nil {
			return err
		}
		p.TextSetFont(E, 24)
		p.TextSetCharacterSpacing(-5)
		p.TextShow("Hello, world!")
		return nil
	},

	func(p *document.Page) error {
		// Test positive word spacing.
		E, err := gofont.GoRegular.Embed(p.Out, nil)
		if err != nil {
			return err
		}
		p.TextSetFont(E, 24)
		p.TextSetWordSpacing(20)
		p.TextShow("Hello, world!")
		return nil
	},

	func(p *document.Page) error {
		// Test negative word spacing.
		E, err := gofont.GoRegular.Embed(p.Out, nil)
		if err != nil {
			return err
		}
		p.TextSetFont(E, 24)
		p.TextSetWordSpacing(-20)
		p.TextShow("Hello, world!")
		return nil
	},

	func(p *document.Page) error {
		// Test horizontally stretched text.
		E, err := gofont.GoRegular.Embed(p.Out, nil)
		if err != nil {
			return err
		}
		p.TextSetFont(E, 48)
		p.TextFirstLine(-20, 10)
		p.TextSetHorizontalScaling(1.5)
		p.TextShow("Hello, world!")
		return nil
	},

	func(p *document.Page) error {
		// Test horizontally compressed text.
		E, err := gofont.GoRegular.Embed(p.Out, nil)
		if err != nil {
			return err
		}
		p.TextSetFont(E, 48)
		p.TextSetHorizontalScaling(0.5)
		p.TextShow("Hello, world!")
		return nil
	},

	func(p *document.Page) error {
		// Test horizontally mirrored text.
		E, err := gofont.GoRegular.Embed(p.Out, nil)
		if err != nil {
			return err
		}
		p.TextSetFont(E, 48)
		p.TextSetHorizontalScaling(-1)
		p.TextFirstLine(300, 20)
		p.TextShow("Hello, world!")
		return nil
	},

	func(p *document.Page) error {
		// Test TextSetLeading.
		E, err := gofont.GoRegular.Embed(p.Out, nil)
		if err != nil {
			return err
		}
		p.TextSetFont(E, 24)
		p.TextFirstLine(10, 200)
		p.TextShow("Hello, world!")
		p.TextSetLeading(28)
		p.TextNextLine()
		p.TextShow("line 2")
		p.TextNextLine()
		p.TextShow("line 3")
		p.TextNextLine()
		p.TextShow("line 4")
		return nil
	},

	func(p *document.Page) error {
		// Test the text rise.
		E, err := gofont.GoRegular.Embed(p.Out, nil)
		if err != nil {
			return err
		}
		p.TextSetFont(E, 24)
		p.TextShow("Hello, ")
		p.TextSetRise(10)
		p.TextShow("world!")
		return nil
	},

	func(p *document.Page) error {
		// Test text rotated by 45 degrees.
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
		// Test an arbitrary text matrix.
		E, err := gofont.GoRegular.Embed(p.Out, nil)
		if err != nil {
			return err
		}
		p.TextSetFont(E, 24)
		p.TextSetMatrix(graphics.Matrix{1, 2, 3, 4, 50, 60})
		p.TextShow("ABC")
		return nil
	},

	func(p *document.Page) error {
		// Test TextShowRaw.
		E, err := gofont.GoRegular.Embed(p.Out, nil)
		if err != nil {
			return err
		}
		p.TextSetFont(E, 24)
		s := font.EncodeText(E, "Hello, world!")
		p.TextShowRaw(s)
		return nil
	},

	func(p *document.Page) error {
		// Test TextShowNextLineRaw.
		E, err := gofont.GoRegular.Embed(p.Out, nil)
		if err != nil {
			return err
		}
		p.TextFirstLine(10, 100)
		p.TextSetFont(E, 24)
		p.TextSetLeading(28)
		s := font.EncodeText(E, "Hello, world!")
		p.TextShowRaw(s)
		p.TextShowNextLineRaw(s)
		return nil
	},

	func(p *document.Page) error {
		// Test TextShowSpacedRaw.
		E, err := gofont.GoRegular.Embed(p.Out, nil)
		if err != nil {
			return err
		}
		p.TextFirstLine(-20, 50)
		p.TextSetFont(E, 24)
		s := font.EncodeText(E, "Hello, world!")
		p.TextShowSpacedRaw(10, 5, s)
		return nil
	},
}

type Answer struct {
	X, Y float64
}

//go:generate go run generate.go
