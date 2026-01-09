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

package extract

import (
	"errors"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/dict"
)

// Font extracts a font from a PDF file as an immutable font object.
// This combines Dict with MakeFont() for convenience.
func Font(x *pdf.Extractor, obj pdf.Object) (font.Instance, error) {
	d, err := Dict(x, obj)
	if err != nil {
		return nil, err
	}
	return d.MakeFont(), nil
}

// Dict reads a font dictionary from a PDF file.
// This returns a concrete type implementing dict.Dict,
// allowing access to font-specific properties.
func Dict(x *pdf.Extractor, obj pdf.Object) (dict.Dict, error) {
	fontDict, err := x.GetDictTyped(obj, "Font")
	if err != nil {
		return nil, err
	} else if fontDict == nil {
		return nil, pdf.Error("missing font dictionary")
	}

	fontType, err := x.GetName(fontDict["Subtype"])
	if err != nil {
		return nil, err
	}
	fontDict["Subtype"] = fontType

	if fontType == "Type0" {
		a, err := x.GetArray(fontDict["DescendantFonts"])
		if err != nil {
			return nil, err
		} else if len(a) < 1 {
			return nil, &pdf.MalformedFileError{
				Err: errors.New("composite font with no descendant fonts"),
			}
		}
		fontDict["DescendantFonts"] = a

		cidFontDict, err := x.GetDictTyped(a[0], "Font")
		if err != nil {
			return nil, err
		}
		a[0] = cidFontDict

		fontType, err = x.GetName(cidFontDict["Subtype"])
		if err != nil {
			return nil, err
		}
		cidFontDict["Subtype"] = fontType
	}

	switch fontType {
	case "Type1":
		return extractFontType1(x, fontDict)
	case "TrueType":
		return extractFontTrueType(x, fontDict)
	case "Type3":
		return extractFontType3(x, fontDict)
	case "CIDFontType0":
		return extractFontCIDType0(x, fontDict)
	case "CIDFontType2":
		return extractFontCIDType2(x, fontDict)
	default:
		return nil, pdf.Errorf("unsupported font type: %s", fontType)
	}
}
