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

// Package separation implements separation dictionaries for preseparated PDF files.
//
// Separation dictionaries are deprecated with PDF 2.0.
// See section 14.11.4 of ISO 32000-2:2020.
package separation

import (
	"errors"
	"slices"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics/color"
)

// Dict represents a separation dictionary (Table 400 in the PDF spec).
//
// In preseparated PDF files, separations for a page are described as separate
// page objects, each painting only a single colorant. The separation dictionary
// links these pages together and identifies the colorant for each separation.
type Dict struct {
	// Pages is an array of references to page objects representing
	// separations of the same document page. One page in this array
	// is the one with which this dictionary is associated.
	Pages []pdf.Reference

	// DeviceColorant is the name of the device colorant used in rendering
	// this separation (e.g., "Cyan" or "PANTONE 35 CV").
	DeviceColorant pdf.Name

	// ColorSpace (optional) is a Separation or DeviceN color space that
	// provides alternate color space and tint transformation for preview.
	// The colorant name must match DeviceColorant.
	ColorSpace color.Space
}

var _ pdf.Encoder = (*Dict)(nil)

// Encode writes the separation dictionary to a PDF file.
func (d *Dict) Encode(rm *pdf.ResourceManager) (pdf.Native, error) {
	if err := pdf.CheckVersion(rm.Out, "separation dictionaries", pdf.V1_3); err != nil {
		return nil, err
	}

	if len(d.Pages) == 0 {
		return nil, errors.New("separation dictionary: missing Pages")
	}

	dict := pdf.Dict{
		"Pages":          pagesArray(d.Pages),
		"DeviceColorant": d.DeviceColorant,
	}

	if d.ColorSpace != nil {
		if err := d.validateColorSpace(); err != nil {
			return nil, err
		}
		cs, err := rm.Embed(d.ColorSpace)
		if err != nil {
			return nil, err
		}
		dict["ColorSpace"] = cs
	}

	return dict, nil
}

// validateColorSpace checks that ColorSpace is Separation or DeviceN
// and that DeviceColorant matches the color space's colorant name(s).
func (d *Dict) validateColorSpace() error {
	switch cs := d.ColorSpace.(type) {
	case *color.SpaceSeparation:
		if cs.Colorant != d.DeviceColorant {
			return errors.New("DeviceColorant must match ColorSpace colorant")
		}
	case *color.SpaceDeviceN:
		if !slices.Contains(cs.Colorants, d.DeviceColorant) {
			return errors.New("DeviceColorant must be one of ColorSpace colorants")
		}
	case nil: // ColorSpace is optional
		return nil
	default:
		return errors.New("ColorSpace must be Separation or DeviceN")
	}
	return nil
}

func pagesArray(refs []pdf.Reference) pdf.Array {
	arr := make(pdf.Array, len(refs))
	for i, ref := range refs {
		arr[i] = ref
	}
	return arr
}

// Decode reads a separation dictionary from a PDF object.
func Decode(x *pdf.Extractor, obj pdf.Object) (*Dict, error) {
	dict, err := pdf.GetDict(x.R, obj)
	if err != nil {
		return nil, err
	}
	if dict == nil {
		return nil, nil
	}

	d := &Dict{}

	// Pages (required)
	pagesArr, err := x.GetArray(dict["Pages"])
	if err != nil {
		return nil, pdf.Wrap(err, "Pages")
	}
	for _, item := range pagesArr {
		if ref, ok := item.(pdf.Reference); ok {
			d.Pages = append(d.Pages, ref)
		}
	}
	if len(d.Pages) == 0 {
		return nil, pdf.Error("missing Pages")
	}

	// DeviceColorant (required) - can be name or string
	colorantObj, err := x.Resolve(dict["DeviceColorant"])
	if err != nil {
		return nil, pdf.Wrap(err, "DeviceColorant")
	}
	switch c := colorantObj.(type) {
	case pdf.Name:
		d.DeviceColorant = c
	case pdf.String:
		d.DeviceColorant = pdf.Name(c)
	}
	if d.DeviceColorant == "" {
		return nil, pdf.Error("missing DeviceColorant")
	}

	// ColorSpace (optional)
	d.ColorSpace, _ = color.ExtractSpace(x, dict["ColorSpace"])
	if d.validateColorSpace() != nil {
		d.ColorSpace = nil
	}

	return d, nil
}
