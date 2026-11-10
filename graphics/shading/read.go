// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package shading

import (
	"fmt"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics"
)

// Extract extracts a shading from a PDF file and returns a graphics.Shading.
func Extract(c pdf.Cursor, obj pdf.Object, isDirect bool) (graphics.Shading, error) {

	obj, err := c.Resolve(obj)
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

	stNum, err := c.Integer(st)
	if err != nil {
		return nil, err
	}

	switch stNum {
	case 1:
		return extractType1(c, dict, isDirect)
	case 2:
		return extractType2(c, dict, isDirect)
	case 3:
		return extractType3(c, dict, isDirect)
	case 4:
		if stream, ok := obj.(*pdf.Stream); ok {
			return extractType4(c, stream)
		}
		return nil, &pdf.MalformedFileError{
			Err: fmt.Errorf("type 4 shading must be a stream"),
		}
	case 5:
		if stream, ok := obj.(*pdf.Stream); ok {
			return extractType5(c, stream)
		}
		return nil, &pdf.MalformedFileError{
			Err: fmt.Errorf("type 5 shading must be a stream"),
		}
	case 6:
		if stream, ok := obj.(*pdf.Stream); ok {
			return extractType6(c, stream)
		}
		return nil, &pdf.MalformedFileError{
			Err: fmt.Errorf("type 6 shading must be a stream"),
		}
	case 7:
		if stream, ok := obj.(*pdf.Stream); ok {
			return extractType7(c, stream)
		}
		return nil, &pdf.MalformedFileError{
			Err: fmt.Errorf("type 7 shading must be a stream"),
		}
	default:
		return nil, &pdf.MalformedFileError{
			Err: fmt.Errorf("unsupported shading type %d", stNum),
		}
	}
}
