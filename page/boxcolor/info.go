// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

// Package boxcolor implements box colour information dictionaries for PDF pages.
package boxcolor

import (
	"errors"

	"seehuhn.de/go/pdf"
)

// PDF 2.0 sections: 14.11.2

// Info specifies display styles for page boundary guidelines.
type Info struct {
	// CropBox (optional) describes how guidelines for the page's crop box
	// should be shown.
	CropBox *Style

	// BleedBox (optional) describes how guidelines for the page's bleed box
	// should be shown.
	BleedBox *Style

	// TrimBox (optional) describes how guidelines for the page's trim box
	// should be shown.
	TrimBox *Style

	// ArtBox (optional) describes how guidelines for the page's art box
	// should be shown.
	ArtBox *Style

	// SingleUse determines if Embed returns a dictionary (true) or
	// a reference (false).
	SingleUse bool
}

// ExtractInfo extracts a box colour information dictionary from a PDF object.
func ExtractInfo(c pdf.Cursor, obj pdf.Object, isDirect bool) (*Info, error) {

	dict, err := c.Dict(obj)
	if err != nil {
		return nil, err
	}
	if dict == nil {
		return nil, &pdf.MalformedFileError{
			Err: errors.New("missing box colour information dictionary"),
		}
	}

	info := &Info{}
	info.SingleUse = isDirect

	if style, err := c.DecodeOptional(dict["CropBox"], ExtractStyle); err != nil {
		return nil, err
	} else {
		info.CropBox = style
	}

	if style, err := c.DecodeOptional(dict["BleedBox"], ExtractStyle); err != nil {
		return nil, err
	} else {
		info.BleedBox = style
	}

	if style, err := c.DecodeOptional(dict["TrimBox"], ExtractStyle); err != nil {
		return nil, err
	} else {
		info.TrimBox = style
	}

	if style, err := c.DecodeOptional(dict["ArtBox"], ExtractStyle); err != nil {
		return nil, err
	} else {
		info.ArtBox = style
	}

	return info, nil
}

// Embed converts the box colour information dictionary to a PDF dictionary.
func (i *Info) Embed(e *pdf.EmbedHelper) (pdf.Native, error) {
	if err := pdf.CheckVersion(e.Out(), "box colour information dictionary", pdf.V1_4); err != nil {
		return nil, err
	}

	dict := pdf.Dict{}

	if i.CropBox != nil {
		obj, err := e.Embed(i.CropBox)
		if err != nil {
			return nil, err
		}
		dict["CropBox"] = obj
	}

	if i.BleedBox != nil {
		obj, err := e.Embed(i.BleedBox)
		if err != nil {
			return nil, err
		}
		dict["BleedBox"] = obj
	}

	if i.TrimBox != nil {
		obj, err := e.Embed(i.TrimBox)
		if err != nil {
			return nil, err
		}
		dict["TrimBox"] = obj
	}

	if i.ArtBox != nil {
		obj, err := e.Embed(i.ArtBox)
		if err != nil {
			return nil, err
		}
		dict["ArtBox"] = obj
	}

	if i.SingleUse {
		return dict, nil
	}

	ref := e.Alloc()
	err := e.Out().Put(ref, dict)
	if err != nil {
		return nil, err
	}
	return ref, nil
}
