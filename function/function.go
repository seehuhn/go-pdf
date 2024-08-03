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

// Func is a PDF function.
type Func interface {
	// Shape returns the number of input and output values of the function.
	Shape() (int, int)

	// Embed embeds the function in a PDF file.
	// This method is used by [pdf.ResourceManager].
	Embed(rm *pdf.ResourceManager) (pdf.Object, pdf.Unused, error)
}

// Type2 is an exponential interpolation function.
type Type2 struct {
	Y0    []float64
	Y1    []float64
	Gamma float64

	SingleUse bool
}

// Shape returns the number of input and output values of the function.
func (f *Type2) Shape() (int, int) {
	return 1, len(f.Y0)
}

// Embed embeds the function in a PDF file.
// This method is used by [pdf.ResourceManager].
func (f *Type2) Embed(rm *pdf.ResourceManager) (pdf.Object, pdf.Unused, error) {
	var zero pdf.Unused

	if err := pdf.CheckVersion(rm.Out, "Type 2 functions", pdf.V1_3); err != nil {
		return nil, zero, err
	}

	d := pdf.Dict{
		"FunctionType": pdf.Integer(2),
		"Domain":       pdf.Array{pdf.Number(0), pdf.Number(1)},
		"C0":           toPDF(f.Y0),
		"C1":           toPDF(f.Y1),
		"N":            pdf.Number(f.Gamma),
	}

	var obj pdf.Object
	if f.SingleUse {
		obj = d
	} else {
		ref := rm.Out.Alloc()
		err := rm.Out.Put(ref, d)
		if err != nil {
			return nil, zero, err
		}
		obj = ref
	}

	return obj, zero, nil
}

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
