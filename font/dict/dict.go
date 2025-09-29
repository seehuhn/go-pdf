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

package dict

import (
	"errors"
	"iter"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
)

// Dict represents a font dictionary in a PDF file.
//
// This interface is implemented by the following types, corresponding to the
// different font dictionary types supported by PDF:
//   - [seehuhn.de/go/pdf/font/dict.Type1]
//   - [seehuhn.de/go/pdf/font/dict.TrueType]
//   - [seehuhn.de/go/pdf/font/dict.Type3]
//   - [seehuhn.de/go/pdf/font/dict.CIDFontType0]
//   - [seehuhn.de/go/pdf/font/dict.CIDFontType2]
type Dict interface {
	pdf.Embedder

	// MakeFont returns a new font object that can be used to typeset text.
	// The font is immutable, i.e. no new glyphs can be added and no new codes
	// can be defined via the returned font object.
	MakeFont() font.Instance

	// FontInfo returns information about the embedded font file.
	// The information can be used to load the font file and to extract
	// the the glyph corresponding to a character identifier.
	// The result is a pointer to one of the FontInfo* types.
	FontInfo() any

	// Codec allows to interpret character codes for the font.
	Codec() *charcode.Codec

	// TODO(voss): remove? keep?
	Characters() iter.Seq2[charcode.Code, font.Code]
}

// ExtractDict reads a font dictionary from a PDF file.
func ExtractDict(x *pdf.Extractor, obj pdf.Object) (Dict, error) {
	fontDict, err := pdf.GetDictTyped(x.R, obj, "Font")
	if err != nil {
		return nil, err
	} else if fontDict == nil {
		return nil, pdf.Error("missing font dictionary")
	}

	fontType, err := pdf.GetName(x.R, fontDict["Subtype"])
	if err != nil {
		return nil, err
	}
	fontDict["Subtype"] = fontType

	if fontType == "Type0" {
		a, err := pdf.GetArray(x.R, fontDict["DescendantFonts"])
		if err != nil {
			return nil, err
		} else if len(a) < 1 {
			return nil, &pdf.MalformedFileError{
				Err: errors.New("composite font with no descendant fonts"),
			}
		}
		fontDict["DescendantFonts"] = a

		cidFontDict, err := pdf.GetDictTyped(x.R, a[0], "Font")
		if err != nil {
			return nil, err
		}
		a[0] = cidFontDict

		fontType, err = pdf.GetName(x.R, cidFontDict["Subtype"])
		if err != nil {
			return nil, err
		}
		cidFontDict["Subtype"] = fontType
	}

	switch fontType {
	case "Type1":
		return extractType1(x, fontDict)
	case "TrueType":
		return extractTrueType(x, fontDict)
	case "Type3":
		return extractType3(x, fontDict)
	case "CIDFontType0":
		return extractCIDFontType0(x, fontDict)
	case "CIDFontType2":
		return extractCIDFontType2(x, fontDict)
	default:
		return nil, pdf.Errorf("unsupported font type: %s", fontType)
	}
}
