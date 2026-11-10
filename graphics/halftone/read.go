// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package halftone

import (
	"fmt"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics"
)

// Extract extracts a halftone from a PDF file.
func Extract(c pdf.Cursor, obj pdf.Object, _ bool) (graphics.Halftone, error) {
	resolved, err := c.Resolve(obj)
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
		halftoneType, err := c.Integer(resolved["HalftoneType"])
		if err != nil {
			return nil, err
		}

		switch halftoneType {
		case 1:
			return extractType1(c, resolved)
		case 5:
			return extractType5(c, resolved)
		default:
			return nil, &pdf.MalformedFileError{
				Err: fmt.Errorf("unsupported halftone type %d for dictionary", halftoneType),
			}
		}

	case *pdf.Stream:
		halftoneType, err := c.Integer(resolved.Dict["HalftoneType"])
		if err != nil {
			return nil, err
		}

		switch halftoneType {
		case 6:
			return extractType6(c, resolved)
		case 10:
			return extractType10(c, resolved)
		case 16:
			return extractType16(c, resolved)
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
