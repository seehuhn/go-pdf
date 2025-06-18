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

package halftone

import (
	"fmt"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics"
)

// Read extracts a halftone from a PDF file.
func Read(r pdf.Getter, obj pdf.Object) (graphics.Halftone, error) {
	resolved, err := pdf.Resolve(r, obj)
	if err != nil {
		return nil, err
	}

	if resolved == nil {
		return nil, &pdf.MalformedFileError{
			Err: fmt.Errorf("missing halftone object"),
		}
	}

	switch resolved := resolved.(type) {
	case pdf.Dict:
		halftoneType, err := pdf.GetInteger(r, resolved["HalftoneType"])
		if err != nil {
			return nil, err
		}

		switch halftoneType {
		case 1:
			return readType1(r, resolved)
		case 5:
			return readType5(r, resolved)
		default:
			return nil, &pdf.MalformedFileError{
				Err: fmt.Errorf("unsupported halftone type %d for dictionary", halftoneType),
			}
		}

	case *pdf.Stream:
		halftoneType, err := pdf.GetInteger(r, resolved.Dict["HalftoneType"])
		if err != nil {
			return nil, err
		}

		switch halftoneType {
		case 6:
			return readType6(r, resolved)
		case 10:
			return readType10(r, resolved)
		case 16:
			return readType16(r, resolved)
		default:
			return nil, &pdf.MalformedFileError{
				Err: fmt.Errorf("unsupported halftone type %d for stream", halftoneType),
			}
		}

	default:
		return nil, &pdf.MalformedFileError{
			Err: fmt.Errorf("halftone must be a dictionary or stream, got %T", resolved),
		}
	}
}
