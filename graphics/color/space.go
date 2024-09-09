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

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/function"
	"seehuhn.de/go/pdf/metadata"
)

// Space represents a PDF color space which can be embedded in a PDF file.
type Space interface {
	// ColorSpaceFamily returns the family of the color space.
	ColorSpaceFamily() pdf.Name

	// Channels returns the dimensionality of the color space.
	Channels() int

	// Default returns the default color of the color space.
	Default() Color

	pdf.Embedder[pdf.Unused]
}

// IsSpecial reports whether the color space is a special color space.
// The special color spaces are Pattern, Indexed, Separation, and DeviceN.
func IsSpecial(s Space) bool {
	switch s.ColorSpaceFamily() {
	case FamilyPattern, FamilyIndexed, FamilySeparation, FamilyDeviceN:
		return true
	default:
		return false
	}
}

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

// Singleton objects for the color spaces which do not require any parameters.
var (
	DeviceGraySpace     = spaceDeviceGray{}
	DeviceRGBSpace      = spaceDeviceRGB{}
	DeviceCMYKSpace     = spaceDeviceCMYK{}
	PatternColoredSpace = spacePatternColored{}
	SRGBSpace           = spaceSRGB{}
)

// ExtractSpace extracts a color space from a PDF file.
//
// The argument desc is typically a value in the ColorSpace sub-dictionary of
// a Resources dictionary.
func ExtractSpace(r pdf.Getter, desc pdf.Object) (Space, error) {
	d := newDecoder(r, desc)

	var res Space
	var err error
	switch d.name {
	case FamilyDeviceGray:
		res = spaceDeviceGray{}

	case FamilyDeviceRGB:
		res = spaceDeviceRGB{}

	case FamilyDeviceCMYK:
		res = spaceDeviceCMYK{}

	case FamilyPattern:
		if len(d.args) == 0 {
			res = spacePatternColored{}
		} else {
			base, err := ExtractSpace(r, d.args[0])
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

	case FamilyICCBased:
		var meta *metadata.Stream
		if ref, ok := d.dict["Metadata"]; ok {
			meta, err = metadata.ExtractStream(r, ref)
			if err != nil {
				d.SetError(err)
			}
		}
		res, err = ICCBased(d.data, meta)
		if err != nil {
			d.SetError(err)
		}

	case FamilyIndexed:
		if len(d.args) < 3 {
			d.MarkAsInvalid()
			break
		}
		base, err := ExtractSpace(r, d.args[0])
		if err != nil {
			d.SetError(pdf.Wrap(err, "base color space"))
			break
		}
		hiVal, err := pdf.GetInteger(r, d.args[1])
		if err != nil {
			d.SetError(pdf.Wrap(err, "high value"))
			break
		} else if hiVal < 1 || hiVal > 255 {
			d.MarkAsInvalid()
			break
		}

		var lookup pdf.String
		lookupData, err := pdf.Resolve(r, d.args[2])
		if err != nil {
			d.SetError(pdf.Wrap(err, "lookup table"))
			break
		}
		switch obj := lookupData.(type) {
		case pdf.String:
			lookup = obj
		case *pdf.Stream:
			data, err := pdf.ReadAll(r, obj)
			if err != nil {
				d.SetError(pdf.Wrap(err, "lookup table"))
				break
			}
			lookup = data
		default:
			d.MarkAsInvalid()
		}
		res = &SpaceIndexed{
			NumCol: int(hiVal) + 1,
			base:   base,
			lookup: lookup,
		}

	case FamilySeparation:
		if len(d.args) < 2 {
			d.MarkAsInvalid()
			break
		}

		colorant, err := pdf.GetName(r, d.args[0])
		if err != nil {
			d.SetError(pdf.Wrap(err, "colorant name"))
			break
		}

		alternate, err := ExtractSpace(r, d.args[1])
		if err != nil {
			d.SetError(pdf.Wrap(err, "alternate color space"))
			break
		}

		trfm, err := function.Read(r, d.args[2])
		if err != nil {
			d.SetError(pdf.Wrap(err, "tint transform"))
			break
		}

		res = &SpaceSeparation{
			colorant:  colorant,
			alternate: alternate,
			trfm:      trfm,
		}

	case FamilyDeviceN:
		if len(d.args) < 3 {
			d.MarkAsInvalid()
			break
		}

		colorants, err := getNames(r, d.args[0])
		if err != nil {
			d.SetError(pdf.Wrap(err, "colorant names"))
			break
		}

		alternate, err := ExtractSpace(r, d.args[1])
		if err != nil {
			d.SetError(pdf.Wrap(err, "alternate color space"))
			break
		}

		trfm, err := function.Read(r, d.args[2])
		if err != nil {
			d.SetError(pdf.Wrap(err, "tint transform"))
			break
		}

		var attr pdf.Dict
		if len(d.args) >= 4 {
			attr, err = pdf.GetDict(r, d.args[3])
			if err != nil {
				d.SetError(pdf.Wrap(err, "attributes"))
				break
			}
		}

		res = &SpaceDeviceN{
			colorants: colorants,
			alternate: alternate,
			trfm:      trfm,
			attr:      attr,
		}

	case "CalCMYK": // deprecated
		res = spaceDeviceCMYK{}

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
			body, err := pdf.ReadAll(r, y)
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
		desc = pdf.AsString(d.obj)
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
