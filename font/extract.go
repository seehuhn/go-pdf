// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2023  Jochen Voss <voss@seehuhn.de>
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

package font

import (
	"errors"
	"fmt"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font/subset"
)

// EmbeddingType represents the different ways font data
// can be embedded in a PDF file.
//
// The different ways of embedding fonts in a PDF file are
// represented by values of type [EmbeddingType]. There are seven different
// types of embedded simple fonts:
//   - Type 1: see [seehuhn.de/go/pdf/font/type1.FontDict]
//   - Multiple Master Type 1 (not supported by this library)
//   - CFF font data: see [seehuhn.de/go/pdf/font/cff.EmbedInfoSimple]
//   - TrueType: see [seehuhn.de/go/pdf/font/truetype.EmbedInfoSimple]
//   - OpenType with CFF glyph outlines: see [seehuhn.de/go/pdf/font/opentype.EmbedInfoCFFSimple]
//   - OpenType with "glyf" glyph outlines: see [seehuhn.de/go/pdf/font/opentype.EmbedInfoGlyfSimple]
//   - Type 3: see [seehuhn.de/go/pdf/font/type3.EmbedInfo]
//
// There are four different types of embedded composite fonts:
//   - CFF font data: see [seehuhn.de/go/pdf/font/cff.EmbedInfoComposite]
//   - TrueType: see [seehuhn.de/go/pdf/font/truetype.EmbedInfoComposite]
//   - OpenType with CFF glyph outlines: see [seehuhn.de/go/pdf/font/opentype.EmbedInfoCFFComposite]
//   - OpenType with "glyf" glyph outlines: see [seehuhn.de/go/pdf/font/opentype.EmbedInfoGlyfComposite]
type EmbeddingType int

// List of all embedding types supported by PDF.
const (
	Unknown EmbeddingType = iota

	Type1              // Type 1 (simple)
	MMType1            // Multiple Master Type 1 (simple)
	CFFSimple          // CFF font data (simple)
	TrueTypeSimple     // TrueType (simple)
	OpenTypeCFFSimple  // OpenType with CFF glyph outlines (simple)
	OpenTypeGlyfSimple // OpenType with "glyf" glyph outlines (simple)
	Type3              // Type 3 (simple)

	CFFComposite          // CFF font data (composite)
	TrueTypeComposite     // TrueType (composite)
	OpenTypeCFFComposite  // OpenType with CFF glyph outlines (composite)
	OpenTypeGlyfComposite // OpenType with "glyf" glyph outlines (composite)

	ExternalCFFComposite  // extern font, CFF-based (composite)
	ExternalGlyfComposite // extern font, glyf-based (composite)
)

func (t EmbeddingType) String() string {
	switch t {
	case Type1:
		return "Type 1"
	case CFFSimple:
		return "Simple CFF"
	case OpenTypeCFFSimple:
		return "Simple OpenType/CFF"
	case MMType1:
		return "MMType1"
	case TrueTypeSimple:
		return "Simple TrueType"
	case OpenTypeGlyfSimple:
		return "Simple OpenType/glyf"
	case Type3:
		return "Type 3"
	case CFFComposite:
		return "Composite CFF"
	case OpenTypeCFFComposite:
		return "Composite OpenType/CFF"
	case TrueTypeComposite:
		return "Composite TrueType"
	case OpenTypeGlyfComposite:
		return "Composite OpenType/glyf"

	case ExternalCFFComposite:
		return "Composite CFF (external)"
	case ExternalGlyfComposite:
		return "Composite glyf (external)"
	default:
		return fmt.Sprintf("EmbeddingType(%d)", int(t))
	}
}

// IsComposite returns true if the embedded font is a composite PDF font.
// If the function returns false, the font is a simple PDF font.
//
// Fonts can be embedded into a PDF file either as "simple fonts" or as
// "composite fonts".  Simple fonts lead to smaller PDF files, but only allow
// to use up to 256 glyphs per embedded copy of the font.  Composite fonts
// allow to use more than 256 glyphs per embedded copy of the font, but lead to
// larger PDF files.
func (t EmbeddingType) IsComposite() bool {
	switch t {
	case CFFComposite, OpenTypeCFFComposite, TrueTypeComposite, OpenTypeGlyfComposite:
		return true
	default:
		return false
	}
}

// MustBe returns an error if the embedding type is not as expected.
func (t EmbeddingType) MustBe(expected EmbeddingType) error {
	if t != expected {
		return fmt.Errorf("expected %q, got %q", expected, t)
	}
	return nil
}

// Dicts collects all information about a font embedded in a PDF file.
type Dicts struct {
	// PostScriptName is the BaseFont entry of the font dictionary
	// or CIDFont dictionary.  It does not include the subset tag.
	PostScriptName pdf.Name

	// SubsetTag is the tag used to identify a font subset.
	// This is the part of the BaseFont entry of the font dictionary
	// before the "+" character, if any.
	SubsetTag string

	FontDict       pdf.Dict
	CIDFontDict    pdf.Dict
	FontDescriptor *Descriptor
	FontProgramRef pdf.Reference
	FontProgram    *pdf.Stream
	Type           EmbeddingType
}

// ExtractDicts reads all information about a font from a PDF file.
func ExtractDicts(r pdf.Getter, fontDictRef pdf.Object) (*Dicts, error) {
	res := &Dicts{}

	fontDict, err := pdf.GetDictTyped(r, fontDictRef, "Font")
	if err != nil {
		return nil, pdf.Wrap(err, "font dict")
	}
	res.FontDict = fontDict

	fontType, err := pdf.GetName(r, fontDict["Subtype"])
	if err != nil {
		return nil, pdf.Wrap(err, "Subtype")
	}

	var cidFontType pdf.Name
	if fontType == "Type0" {
		descendantFonts, err := pdf.GetArray(r, fontDict["DescendantFonts"])
		if err != nil {
			return nil, pdf.Wrap(err, "DescendantFonts")
		} else if len(descendantFonts) != 1 {
			return nil, fmt.Errorf("invalid descendant fonts: %v", descendantFonts)
		}

		cidFontDict, err := pdf.GetDictTyped(r, descendantFonts[0], "Font")
		if err != nil {
			return nil, pdf.Wrap(err, "CIDFont dict")
		}
		res.CIDFontDict = cidFontDict

		cidFontType, err = pdf.GetName(r, cidFontDict["Subtype"])
		if err != nil {
			return nil, pdf.Wrap(err, "CIDFont Subtype")
		}

		fontDict = cidFontDict
	}

	fontName, err := pdf.GetName(r, fontDict["BaseFont"])
	if err == nil {
		if m := subset.TagRegexp.FindStringSubmatch(string(fontName)); m != nil {
			res.SubsetTag = m[1]
			fontName = pdf.Name(m[2])
		}
		res.PostScriptName = fontName
	}

	fontDescriptor, err := pdf.GetDictTyped(r, fontDict["FontDescriptor"], "FontDescriptor")
	if err != nil {
		return nil, pdf.Wrap(err, "FontDescriptor")
	}
	var fontKey pdf.Name
	var fontRef pdf.Reference
	if fontDescriptor != nil {
		res.FontDescriptor, err = ExtractDescriptor(r, fontDescriptor)
		if err != nil {
			return nil, err
		}
		for _, key := range []pdf.Name{"FontFile", "FontFile2", "FontFile3"} {
			if ref, _ := fontDescriptor[key].(pdf.Reference); ref != 0 {
				fontKey = key
				fontRef = ref
				break
			}
		}
	}

	var subType pdf.Name
	if fontRef != 0 {
		stmObj, err := pdf.GetStream(r, fontRef)
		if err != nil {
			return nil, pdf.Wrap(err, string(fontKey))
		}
		res.FontProgramRef = fontRef
		res.FontProgram = stmObj
		subType, err = pdf.GetName(r, stmObj.Dict["Subtype"])
		if err != nil {
			return nil, pdf.Wrap(err, "Subtype")
		}
	}

	switch {
	case fontType == "Type1" && (fontKey == "FontFile" || fontKey == ""):
		res.Type = Type1
	case fontType == "Type1" && fontKey == "FontFile3" && subType == "Type1C":
		res.Type = CFFSimple
	case fontType == "Type1" && fontKey == "FontFile3" && subType == "OpenType":
		res.Type = OpenTypeCFFSimple
	case fontType == "MMType1":
		res.Type = MMType1
	case fontType == "TrueType" && fontKey == "FontFile2":
		res.Type = TrueTypeSimple
	case fontType == "TrueType" && fontKey == "FontFile3" && subType == "OpenType":
		res.Type = OpenTypeGlyfSimple
	case fontType == "Type3":
		res.Type = Type3
	case fontType == "Type0" && cidFontType == "CIDFontType0" && fontKey == "FontFile3" && subType == "CIDFontType0C":
		res.Type = CFFComposite
	case fontType == "Type0" && cidFontType == "CIDFontType0" && fontKey == "FontFile3" && subType == "OpenType":
		res.Type = OpenTypeCFFComposite
	case fontType == "Type0" && cidFontType == "CIDFontType0" && fontKey == "":
		res.Type = ExternalCFFComposite
	case fontType == "Type0" && cidFontType == "CIDFontType2" && fontKey == "FontFile2":
		res.Type = TrueTypeComposite
	case fontType == "Type0" && cidFontType == "CIDFontType2" && fontKey == "FontFile3" && subType == "OpenType":
		res.Type = OpenTypeGlyfComposite
	case fontType == "Type0" && cidFontType == "CIDFontType2" && fontKey == "":
		res.Type = ExternalGlyfComposite
	default:
		return nil, &pdf.MalformedFileError{
			Err: errors.New("unknown font type"),
			Loc: []string{pdf.AsString(fontDictRef)},
		}
	}

	return res, nil
}
