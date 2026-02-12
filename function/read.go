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
func Extract(x *pdf.Extractor, obj pdf.Object) (pdf.Function, error) {
	// Type 3 functions are recursive, so we need to check for cycles.
	cycleChecker := pdf.NewCycleChecker()
	return safeExtract(x, obj, cycleChecker)
}

// safeExtract extracts a function with cycle detection to prevent infinite recursion.
func safeExtract(x *pdf.Extractor, obj pdf.Object, cycleChecker *pdf.CycleChecker) (pdf.Function, error) {
	if err := cycleChecker.Check(obj); err != nil {
		return nil, err
	}

	resolved, err := x.Resolve(obj)
	if err != nil {
		return nil, err
	} else if resolved == nil {
		return nil, &pdf.MalformedFileError{
			Err: fmt.Errorf("missing function object"),
		}
	}

	switch obj := resolved.(type) {
	case pdf.Dict:
		ft, ok := obj["FunctionType"]
		if !ok {
			return nil, &pdf.MalformedFileError{
				Err: fmt.Errorf("missing /FunctionType"),
			}
		}

		ftNum, err := x.GetInteger(ft)
		if err != nil {
			return nil, err
		}
		switch ftNum {
		case 2:
			return extractType2(x, obj)
		case 3:
			return extractType3(x, obj, cycleChecker)
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

		ftNum, err := x.GetInteger(ft)
		if err != nil {
			return nil, err
		}
		switch ftNum {
		case 0:
			return extractType0(x, obj)
		case 4:
			return extractType4(x, obj)
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

// getFloatArray extracts a slice of float64 from a PDF Array.
func getFloatArray(x *pdf.Extractor, obj pdf.Object) ([]float64, error) {
	a, err := x.GetArray(obj)
	if a == nil {
		return nil, err
	}

	res := make([]float64, len(a))
	for i, obj := range a {
		num, err := x.GetNumber(obj)
		if err != nil {
			return nil, fmt.Errorf("array element %d: %w", i, err)
		}
		res[i] = num
	}
	return res, nil
}

// readInts extracts a slice of int from a PDF Array.
func readInts(x *pdf.Extractor, obj pdf.Object) ([]int, error) {
	a, err := x.GetArray(obj)
	if a == nil {
		return nil, err
	}

	res := make([]int, len(a))
	for i, obj := range a {
		num, err := x.GetInteger(obj)
		if err != nil {
			return nil, err
		}
		res[i] = int(num)
	}
	return res, nil
}
