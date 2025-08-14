// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2025  Jochen Voss <voss@seehuhn.de>
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

package shading

import (
	"fmt"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics"
)

// Extract extracts a shading from a PDF file and returns a graphics.Shading.
func Extract(r pdf.Getter, obj pdf.Object) (graphics.Shading, error) {
	// Check if original object was a reference before resolving
	_, isIndirect := obj.(pdf.Reference)

	obj, err := pdf.Resolve(r, obj)
	if err != nil {
		return nil, err
	} else if obj == nil {
		return nil, &pdf.MalformedFileError{
			Err: fmt.Errorf("missing shading object"),
		}
	}

	var dict pdf.Dict
	switch obj := obj.(type) {
	case pdf.Dict:
		dict = obj
	case *pdf.Stream:
		dict = obj.Dict
	default:
		return nil, &pdf.MalformedFileError{
			Err: fmt.Errorf("shading must be a dictionary or stream"),
		}
	}

	st, ok := dict["ShadingType"]
	if !ok {
		return nil, &pdf.MalformedFileError{
			Err: fmt.Errorf("missing /ShadingType entry"),
		}
	}

	stNum, err := pdf.GetInteger(r, st)
	if err != nil {
		return nil, err
	}

	switch stNum {
	case 1:
		return extractType1(r, dict, isIndirect)
	case 2:
		return extractType2(r, dict, isIndirect)
	case 3:
		return extractType3(r, dict, isIndirect)
	case 4:
		if stream, ok := obj.(*pdf.Stream); ok {
			return extractType4(r, stream, isIndirect)
		}
		return nil, &pdf.MalformedFileError{
			Err: fmt.Errorf("type 4 shading must be a stream"),
		}
	default:
		return nil, &pdf.MalformedFileError{
			Err: fmt.Errorf("unsupported shading type %d", stNum),
		}
	}
}
