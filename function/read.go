// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package function

import (
	"fmt"

	"seehuhn.de/go/pdf"
)

// Extract extracts a function from a PDF file.
// Cycle detection for recursive Type 3 functions is handled by routing
// sub-function extraction through [pdf.Cursor.Decode].
func Extract(c pdf.Cursor, obj pdf.Object, _ bool) (pdf.Function, error) {
	resolved, err := c.Resolve(obj)
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

		ftNum, err := c.Integer(ft)
		if err != nil {
			return nil, err
		}
		switch ftNum {
		case 2:
			return extractType2(c, obj)
		case 3:
			return extractType3(c, obj)
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

		ftNum, err := c.Integer(ft)
		if err != nil {
			return nil, err
		}
		switch ftNum {
		case 0:
			return extractType0(c, obj)
		case 4:
			return extractType4(c, obj)
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
func getFloatArray(c pdf.Cursor, obj pdf.Object) ([]float64, error) {
	a, err := c.Array(obj)
	if a == nil {
		return nil, err
	}

	res := make([]float64, len(a))
	for i, obj := range a {
		num, err := c.Number(obj)
		if err != nil {
			return nil, fmt.Errorf("array element %d: %w", i, err)
		}
		res[i] = num
	}
	return res, nil
}

// readInts extracts a slice of int from a PDF Array.
func readInts(c pdf.Cursor, obj pdf.Object) ([]int, error) {
	a, err := c.Array(obj)
	if a == nil {
		return nil, err
	}

	res := make([]int, len(a))
	for i, obj := range a {
		num, err := c.Integer(obj)
		if err != nil {
			return nil, err
		}
		res[i] = int(num)
	}
	return res, nil
}
