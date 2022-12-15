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

package color

import (
	"fmt"
	"io"

	"seehuhn.de/go/pdf/internal/float"
)

type Color interface {
	SetStroke(w io.Writer) error
	SetFill(w io.Writer) error
}

type rgb struct {
	R, G, B float64
}

// RGB returns a color in the /DeviceRGB color space.
// Each component must be in the range [0, 1].
func RGB(r, g, b float64) Color {
	return &rgb{r, g, b}
}

func (c *rgb) SetStroke(w io.Writer) error {
	rString := float.Format(c.R, 3)
	gString := float.Format(c.G, 3)
	bString := float.Format(c.B, 3)
	_, err := fmt.Fprintln(w, rString, gString, bString, "RG")
	return err
}

func (c *rgb) SetFill(w io.Writer) error {
	rString := float.Format(c.R, 3)
	gString := float.Format(c.G, 3)
	bString := float.Format(c.B, 3)
	_, err := fmt.Fprintln(w, rString, gString, bString, "rg")
	return err
}

type gray float64

// Gray returns a color in the /DeviceGray color space.
// The value must be in the range from 0 (black) to 1 (white).
func Gray(g float64) Color {
	return gray(g)
}

func (c gray) SetStroke(w io.Writer) error {
	gString := float.Format(float64(c), 3)
	_, err := fmt.Fprintln(w, gString, "G")
	return err
}

func (c gray) SetFill(w io.Writer) error {
	gString := float.Format(float64(c), 3)
	_, err := fmt.Fprintln(w, gString, "g")
	return err
}
