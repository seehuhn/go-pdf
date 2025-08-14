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

// Extract extracts a function from a PDF file.
func Extract(r pdf.Getter, obj pdf.Object) (pdf.Function, error) {
	// Type 3 functions are recursive, so we need to check for cycles.
	cycleChecker := pdf.NewCycleChecker()
	return safeExtract(r, obj, cycleChecker)
}

// safeExtract extracts a function with cycle detection to prevent infinite recursion.
func safeExtract(r pdf.Getter, obj pdf.Object, cycleChecker *pdf.CycleChecker) (pdf.Function, error) {
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
				Err: fmt.Errorf("missing /FunctionType"),
			}
		}

		ftNum, err := pdf.GetInteger(r, ft)
		if err != nil {
			return nil, err
		}
		switch ftNum {
		case 2:
			return extractType2(r, obj)
		case 3:
			return extractType3(r, obj, cycleChecker)
		default:
			return nil, &pdf.MalformedFileError{
				Err: fmt.Errorf("invalid function type %d for dict", ftNum),
			}
		}
	case *pdf.Stream:
		ft, ok := obj.Dict["FunctionType"]
		if !ok {
			return nil, &pdf.MalformedFileError{
				Err: fmt.Errorf("missing /FunctionType"),
			}
		}

		ftNum, err := pdf.GetInteger(r, ft)
		if err != nil {
			return nil, err
		}
		switch ftNum {
		case 0:
			return extractType0(r, obj)
		case 4:
			return extractType4(r, obj)
		default:
			return nil, &pdf.MalformedFileError{
				Err: fmt.Errorf("invalid function type %d for stream", ftNum),
			}
		}
	default:
		return nil, &pdf.MalformedFileError{
			Err: fmt.Errorf("function must be a dictionary or stream"),
		}
	}
}

// readInts extracts a slice of int from a PDF Array.
func readInts(r pdf.Getter, obj pdf.Object) ([]int, error) {
	a, err := pdf.GetArray(r, obj)
	if a == nil {
		return nil, err
	}

	res := make([]int, len(a))
	for i, obj := range a {
		num, err := pdf.GetInteger(r, obj)
		if err != nil {
			return nil, err
		}
		res[i] = int(num)
	}
	return res, nil
}
