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

package color

import (
	"fmt"
	"io"

	"seehuhn.de/go/pdf"
)

// Color space families supported by PDF.
const (
	FamilyDeviceGray pdf.Name = "DeviceGray"
	FamilyDeviceRGB  pdf.Name = "DeviceRGB"
	FamilyDeviceCMYK pdf.Name = "DeviceCMYK"
	FamilyCalGray    pdf.Name = "CalGray"
	FamilyCalRGB     pdf.Name = "CalRGB"
	FamilyLab        pdf.Name = "Lab"
	FamilyICCBased   pdf.Name = "ICCBased"
	FamilyPattern    pdf.Name = "Pattern"
	FamilyIndexed    pdf.Name = "Indexed"
	FamilySeparation pdf.Name = "Separation"
	FamilyDeviceN    pdf.Name = "DeviceN"
)

// Space represents a PDF color space which can be embedded in a PDF file.
type Space interface {
	Embed(*pdf.ResourceManager) (pdf.Object, pdf.Unused, error)
	ColorSpaceFamily() pdf.Name
	defaultValues() []float64
}

// NumValues returns the number of color values for the given color space.
func NumValues(s Space) int {
	return len(s.defaultValues())
}

// ReadSpace reads a color space from a PDF file.
func ReadSpace(r pdf.Getter, desc pdf.Object) (Space, error) {
	d := newDecoder(r, desc)

	var res Space
	var err error
	switch d.name {
	case FamilyDeviceGray:
		res = DeviceGray

	case FamilyDeviceRGB:
		res = DeviceRGB

	case FamilyDeviceCMYK:
		res = DeviceCMYK

	case FamilyPattern:
		if len(d.args) == 0 {
			res = spacePatternColored{}
		} else {
			base, err := ReadSpace(r, d.args[0])
			if err != nil {
				d.SetError(pdf.Wrap(err, "base color space"))
			} else {
				// TODO(voss): do we need to look this up in the resource dictionary?
				res = spacePatternUncolored{
					base: base,
				}
			}
		}

	case FamilyCalGray:
		whitePoint := d.getArrayN("WhitePoint", 3)
		blackPoint := d.getArrayN("BlackPoint", 3) // optional
		gamma := d.getOptionalNumber("Gamma", 1.0)

		res, err = CalGray(whitePoint, blackPoint, gamma)
		if err != nil {
			d.SetError(&pdf.MalformedFileError{Err: err})
		}

	case FamilyCalRGB:
		whitePoint := d.getArrayN("WhitePoint", 3)
		blackPoint := d.getArrayN("BlackPoint", 3)
		gamma := d.getArrayN("Gamma", 3)
		matrix := d.getArrayN("Matrix", 9)
		res, err = CalRGB(whitePoint, blackPoint, gamma, matrix)
		if err != nil {
			d.SetError(&pdf.MalformedFileError{Err: err})
		}

	case FamilyLab:
		whitePoint := d.getArrayN("WhitePoint", 3)
		blackPoint := d.getArrayN("BlackPoint", 3)
		Range := d.getArrayN("Range", 4)

		res, err = Lab(whitePoint, blackPoint, Range)
		if err != nil {
			d.SetError(&pdf.MalformedFileError{Err: err})
		}

	case "CalCMYK": // deprecated
		res = DeviceCMYK

	default:
		d.MarkAsInvalid()
	}

	if d.err != nil {
		return nil, d.err
	}
	return res, nil
}

type decoder struct {
	r   pdf.Getter
	obj pdf.Object

	name pdf.Name
	args []pdf.Object
	dict pdf.Dict
	data []byte
	err  error
}

func newDecoder(r pdf.Getter, obj pdf.Object) *decoder {
	d := &decoder{
		r:    r,
		obj:  obj,
		dict: pdf.Dict{},
	}

	x, err := pdf.Resolve(r, obj)
	if err != nil {
		d.err = err
		return d
	}
	d.obj = x

	switch x := x.(type) {
	case pdf.Name:
		d.name = x
	case pdf.String:
		d.name = pdf.Name(x)
	case pdf.Array:
		if len(x) == 0 {
			d.MarkAsInvalid()
			break
		}
		name, err := pdf.GetName(r, x[0])
		if err != nil {
			d.SetError(err)
			break
		}
		d.name = name
		x = x[1:]

		if len(x) == 0 {
			break
		}

		y, err := pdf.Resolve(r, x[0])
		if err != nil {
			d.SetError(err)
			break
		}
		switch y := y.(type) {
		case pdf.Dict:
			d.dict = y

		case *pdf.Stream:
			d.dict = y.Dict
			r, err := pdf.DecodeStream(r, y, 0)
			if err != nil {
				d.SetError(err)
				break
			}
			body, err := io.ReadAll(r)
			if err != nil {
				d.SetError(err)
				break
			}
			d.data = body

		default:
			d.args = x
		}

	default:
		d.MarkAsInvalid()
	}

	return d
}

func (d *decoder) SetError(err error) {
	if err == nil {
		panic("invalid error")
	}

	switch {
	case d.err == nil:
		if pdf.IsMalformed(err) {
			d.MarkAsInvalid()
		} else {
			d.err = err
		}
	case pdf.IsMalformed(d.err) && !pdf.IsMalformed(err):
		// read errors take priority over file format errors
		d.err = err
	default:
		// keep the original read error
	}
}

func (d *decoder) MarkAsInvalid() {
	var desc string
	switch d.obj.(type) {
	case *pdf.Stream:
		desc = "stream"
	default:
		desc = pdf.Format(d.obj)
	}
	if len(desc) > 40 {
		desc = desc[:32] + "..." + desc[len(desc)-5:]
	}

	d.err = &pdf.MalformedFileError{
		Err: fmt.Errorf("invalid color space: %s", desc),
	}
}

func (d *decoder) getOptionalNumber(entry pdf.Name, defValue float64) float64 {
	if d.err != nil {
		return defValue
	}

	obj, ok := d.dict[entry]
	if !ok {
		return defValue
	}

	x, err := pdf.GetNumber(d.r, obj)
	if err != nil {
		d.SetError(err)
		return defValue
	}
	return float64(x)
}

func (d *decoder) getArrayN(entry pdf.Name, n int) []float64 {
	if d.err != nil {
		return nil
	}

	obj, ok := d.dict[entry]
	if !ok {
		return nil
	}

	arr, err := pdf.GetArray(d.r, obj)
	if err != nil {
		d.SetError(err)
		return nil
	}

	if len(arr) != n {
		d.SetError(&pdf.MalformedFileError{
			Err: fmt.Errorf("expected array of length %d, got %d", n, len(arr)),
		})
		return nil
	}

	res := make([]float64, n)
	for i, elem := range arr {
		x, err := pdf.GetNumber(d.r, elem)
		if err != nil {
			d.SetError(err)
			return nil
		}
		res[i] = float64(x)
	}
	return res
}
