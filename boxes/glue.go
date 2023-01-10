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

package boxes

import "seehuhn.de/go/pdf/graphics"

type stretcher interface {
	Stretch() *stretchAmount
}

type shrinker interface {
	Shrink() *stretchAmount
}

type stretchAmount struct {
	Val   float64
	Level int
}

type glue struct {
	Length float64
	Plus   stretchAmount
	Minus  stretchAmount
}

func (obj *glue) Add(other *glue) *glue {
	if other == nil {
		return obj
	}
	res := &glue{
		Length: obj.Length + other.Length,
	}
	if obj.Plus.Level > other.Plus.Level {
		res.Plus = obj.Plus
	} else if obj.Plus.Level < other.Plus.Level {
		res.Plus = other.Plus
	} else {
		res.Plus = stretchAmount{
			Val:   obj.Plus.Val + other.Plus.Val,
			Level: obj.Plus.Level,
		}
	}
	if obj.Minus.Level > other.Minus.Level {
		res.Minus = obj.Minus
	} else if obj.Minus.Level < other.Minus.Level {
		res.Minus = other.Minus
	} else {
		res.Minus = stretchAmount{
			Val:   obj.Minus.Val + other.Minus.Val,
			Level: obj.Minus.Level,
		}
	}
	return res
}

// Glue returns a new "glue" box with the given natural length and
// stretchability.
func Glue(length float64, plus float64, plusLevel int, minus float64, minusLevel int) Box {
	return &glue{
		Length: length,
		Plus:   stretchAmount{plus, plusLevel},
		Minus:  stretchAmount{minus, minusLevel},
	}
}

func (obj *glue) Extent() *BoxExtent {
	return &BoxExtent{
		Width:          obj.Length,
		Height:         obj.Length,
		WhiteSpaceOnly: true,
	}
}

func (obj *glue) Draw(page *graphics.Page, xPos, yPos float64) {}

func (obj *glue) Stretch() *stretchAmount {
	return &obj.Plus
}

func (obj *glue) Shrink() *stretchAmount {
	return &obj.Minus
}
