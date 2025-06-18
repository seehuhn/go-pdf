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

package function

import (
	"fmt"

	"seehuhn.de/go/pdf"
)

// Read extracts a function from a PDF file and returns a pdf.Function.
func Read(r pdf.Getter, obj pdf.Object) (pdf.Function, error) {
	cycleChecker := pdf.NewCycleChecker()
	return readWithCycleChecker(r, obj, cycleChecker)
}

// readWithCycleChecker extracts a function with cycle detection to prevent infinite recursion.
func readWithCycleChecker(r pdf.Getter, obj pdf.Object, cycleChecker *pdf.CycleChecker) (pdf.Function, error) {
	// Check for cycles before processing the object
	if err := cycleChecker.Check(obj); err != nil {
		return nil, err
	}

	obj, err := pdf.Resolve(r, obj)
	if err != nil {
		return nil, err
	} else if obj == nil {
		return nil, &pdf.MalformedFileError{
			Err: fmt.Errorf("missing function object"),
		}
	}

	switch obj := obj.(type) {
	case pdf.Dict:
		ft, ok := obj["FunctionType"]
		if !ok {
			return nil, &pdf.MalformedFileError{
				Err: fmt.Errorf("missing /FunctionType entry"),
			}
		}

		ftNum, err := pdf.GetInteger(r, ft)
		if err != nil {
			return nil, err
		}
		switch ftNum {
		case 2:
			return readType2(r, obj)
		case 3:
			return readType3(r, obj, cycleChecker)
		default:
			return nil, &pdf.MalformedFileError{
				Err: fmt.Errorf("unsupported function type %d for dictionary", ftNum),
			}
		}
	case *pdf.Stream:
		ft, ok := obj.Dict["FunctionType"]
		if !ok {
			return nil, &pdf.MalformedFileError{
				Err: fmt.Errorf("missing /FunctionType entry"),
			}
		}

		ftNum, err := pdf.GetInteger(r, ft)
		if err != nil {
			return nil, err
		}
		switch ftNum {
		case 0:
			return readType0(r, obj)
		case 4:
			return readType4(r, obj)
		default:
			return nil, &pdf.MalformedFileError{
				Err: fmt.Errorf("unsupported function type %d for stream", ftNum),
			}
		}
	default:
		return nil, &pdf.MalformedFileError{
			Err: fmt.Errorf("function must be a dictionary or stream"),
		}
	}
}
