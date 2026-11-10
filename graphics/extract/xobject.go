// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package extract

import (
	"errors"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/image"
	"seehuhn.de/go/pdf/graphics/xobject"
)

// XObject extracts an XObject from a PDF file.
func XObject(c pdf.Cursor, obj pdf.Object, isDirect bool) (graphics.XObject, error) {
	stm, err := c.Stream(obj)
	if err != nil {
		return nil, err
	} else if stm == nil {
		return nil, &pdf.MalformedFileError{
			Err: errors.New("missing XObject"),
		}
	}
	err = c.CheckDictType(stm.Dict, "XObject")
	if err != nil {
		return nil, err
	}

	subtype, err := c.Name(stm.Dict["Subtype"])
	if err != nil {
		return nil, err
	}

	switch subtype {
	case "Image":
		isImageMask, _, err := c.OptionalBoolean(stm.Dict["ImageMask"])
		if err != nil {
			return nil, err
		}
		if isImageMask {
			return image.ExtractMask(c, obj, isDirect)
		}
		img, err := image.ExtractDict(c, stm, isDirect)
		return img, err
	case "Form":
		f, err := Form(c, stm, isDirect)
		return f, err
	case "PS":
		ps, err := xobject.ExtractPostScript(c, stm)
		return ps, err
	default:
		return nil, pdf.Errorf("unsupported XObject Subtype %q", subtype)
	}
}
