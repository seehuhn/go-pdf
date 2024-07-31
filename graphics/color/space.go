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
	"errors"
	"fmt"

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

func ReadSpace(r pdf.Getter, obj pdf.Object) (Space, error) {
	obj, err := pdf.Resolve(r, obj)
	if err != nil {
		return nil, err
	}

	switch obj := obj.(type) {
	case pdf.Name:
		switch obj {
		case FamilyDeviceGray:
			return SpaceDeviceGray{}, nil
		case FamilyDeviceRGB:
			return SpaceDeviceRGB{}, nil
		case FamilyDeviceCMYK:
			return SpaceDeviceCMYK{}, nil
		case FamilyPattern:
			return spacePatternColored{}, nil
		case "CalCMYK":
			// "A PDF reader shall ignore CalCMYK colour space attributes and
			// render colours specified in this family as if they had been
			// specified using DeviceCMYK."
			return SpaceDeviceCMYK{}, nil
		default:
			return nil, &pdf.MalformedFileError{
				Err: fmt.Errorf("unsupported color space %q", obj),
			}
		}
	case pdf.Array:
		if len(obj) == 0 {
			return nil, &pdf.MalformedFileError{
				Err: errors.New("empty color space array"),
			}
		}
		name, err := pdf.GetName(r, obj[0])
		if err != nil {
			return nil, pdf.Wrap(err, "color space name")
		}

		switch name {
		case FamilyCalGray:
			return readSpaceCalGray(r, obj)
		default:
			panic("not implemented") // TODO(voss): finish this
		}
	default:
		return nil, &pdf.MalformedFileError{
			Err: fmt.Errorf("unexpected color space object %T", obj),
		}
	}
}
