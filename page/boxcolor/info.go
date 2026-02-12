// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2026  Jochen Voss <voss@seehuhn.de>
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

// Package boxcolor implements box colour information dictionaries for PDF pages.
package boxcolor

import "seehuhn.de/go/pdf"

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
func ExtractInfo(x *pdf.Extractor, obj pdf.Object) (*Info, error) {
	singleUse := !x.IsIndirect

	dict, err := x.GetDict(obj)
	if err != nil {
		return nil, err
	}
	if dict == nil {
		return nil, nil
	}

	info := &Info{}
	info.SingleUse = singleUse

	if style, err := pdf.ExtractorGetOptional(x, dict["CropBox"], ExtractStyle); err != nil {
		return nil, err
	} else {
		info.CropBox = style
	}

	if style, err := pdf.ExtractorGetOptional(x, dict["BleedBox"], ExtractStyle); err != nil {
		return nil, err
	} else {
		info.BleedBox = style
	}

	if style, err := pdf.ExtractorGetOptional(x, dict["TrimBox"], ExtractStyle); err != nil {
		return nil, err
	} else {
		info.TrimBox = style
	}

	if style, err := pdf.ExtractorGetOptional(x, dict["ArtBox"], ExtractStyle); err != nil {
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
