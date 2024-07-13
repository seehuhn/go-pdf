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

package function

import (
	"fmt"

	"seehuhn.de/go/pdf"
)

// A Func represents a PDF function.
//
// TODO(voss): This is a placeholder for now.
//
// TODO(voss): make a distinction between free functions and functions
// already embedded in the PDF file.
type Func pdf.Object

type Interpolate struct {
	XMin, XMax float64
	Y0, Y1     []float64
	YMin, YMax []float64
	Gamma      float64
}

func (f *Interpolate) Check() error {
	if f.XMin >= f.XMax {
		return fmt.Errorf("invalid domain: [%f, %f]", f.XMin, f.XMax)
	}
	if float64(int(f.Gamma)) != f.Gamma && f.XMin < 0 {
		return fmt.Errorf("invalid XMin for non-integer gamma: %f", f.XMin)
	}
	// TODO(voss): Is 0^0 allowed or not?  What is the value?
	if f.Gamma < 0 && f.XMin <= 0 && f.XMax >= 0 {
		return fmt.Errorf("invalid domain for negative gamma: [%f, %f]", f.XMin, f.XMax)
	}

	return nil
}
