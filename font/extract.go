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
	"fmt"

	"seehuhn.de/go/pdf"
)

type EmbeddingType int

// EmbeddingType values representing all the different ways font data
// can be embedded in a PDF file.
const (
	Unknown EmbeddingType = iota

	Builtin               // built-in fonts
	CFFComposite          // CFF font data without wrapper (composite font)
	CFFSimple             // CFF font data without wrapper (simple font)
	MMType1               // Multiple Master type 1 fonts
	OpenTypeCFFComposite  // CFF fonts in an OpenType wrapper (composite font)
	OpenTypeCFFSimple     // CFF font data in an OpenType wrapper (simple font)
	OpenTypeGlyfComposite // OpenType fonts with glyf outline (composite font)
	OpenTypeGlyfSimple    // OpenType fonts with glyf outline (simple font)
	TrueTypeComposite     // TrueType fonts (composite font)
	TrueTypeSimple        // TrueType fonts (simple font)
	Type1                 // Type 1 fonts
	Type3                 // Type 3 fonts
)

func (t EmbeddingType) String() string {
	switch t {
	case Type1:
		return "Type 1"
	case Builtin:
		return "Type 1 (built-in)"
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
	default:
		return fmt.Sprintf("EmbeddingType(%d)", int(t))
	}
}

func (t EmbeddingType) IsComposite() bool {
	switch t {
	case CFFComposite, OpenTypeCFFComposite, TrueTypeComposite, OpenTypeGlyfComposite:
		return true
	default:
		return false
	}
}

func (t EmbeddingType) MustBe(expected EmbeddingType) error {
	if t != expected {
		return fmt.Errorf("expected %q, got %q", expected, t)
	}
	return nil
}

type Dicts struct {
	FontDict       pdf.Dict
	CIDFontDict    pdf.Dict
	FontDescriptor *Descriptor
	FontProgram    *pdf.Stream
	Type           EmbeddingType
}

func ExtractDicts(r pdf.Getter, ref pdf.Reference) (*Dicts, error) {
	res := &Dicts{}

	fontDict, err := pdf.GetDictTyped(r, ref, "Font")
	if err != nil {
		return nil, err
	}
	res.FontDict = fontDict

	fontType, err := pdf.GetName(r, fontDict["Subtype"])
	if err != nil {
		return nil, err
	}

	var cidFontType pdf.Name
	if fontType == "Type0" {
		descendantFonts, err := pdf.GetArray(r, fontDict["DescendantFonts"])
		if err != nil {
			return nil, err
		} else if len(descendantFonts) != 1 {
			return nil, fmt.Errorf("invalid descendant fonts: %v", descendantFonts)
		}

		cidFontDict, err := pdf.GetDictTyped(r, descendantFonts[0], "Font")
		if err != nil {
			return nil, err
		}
		res.CIDFontDict = cidFontDict

		cidFontType, err = pdf.GetName(r, cidFontDict["Subtype"])
		if err != nil {
			return nil, err
		}

		fontDict = cidFontDict
	}

	fontDescriptor, err := pdf.GetDictTyped(r, fontDict["FontDescriptor"], "FontDescriptor")
	if err != nil {
		return nil, err
	}
	var fontKey pdf.Name
	if fontDescriptor != nil {
		res.FontDescriptor, err = DecodeDescriptor(r, fontDescriptor)
		if err != nil {
			return nil, err
		}
		for _, key := range []pdf.Name{"FontFile", "FontFile2", "FontFile3"} {
			if ref, _ := fontDescriptor[key].(pdf.Reference); ref != 0 {
				stm, err := pdf.GetStream(r, ref)
				if err != nil {
					return nil, err
				}
				fontKey = key
				res.FontProgram = stm
				break
			}
		}
	}

	var subType pdf.Name
	if res.FontProgram != nil {
		subType, err = pdf.GetName(r, res.FontProgram.Dict["Subtype"])
		if err != nil {
			return nil, err
		}
	}

	switch {
	case fontType == "Type1" && (fontKey == "FontFile" || fontKey == ""):
		baseFont, _ := pdf.GetName(r, fontDict["BaseFont"])
		if fontKey == "" && isBuiltinFont[baseFont] {
			res.Type = Builtin
		} else {
			res.Type = Type1
		}
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
	case fontType == "Type0" && cidFontType == "CIDFontType2" && fontKey == "FontFile2":
		res.Type = TrueTypeComposite
	case fontType == "Type0" && cidFontType == "CIDFontType2" && fontKey == "FontFile3" && subType == "OpenType":
		res.Type = OpenTypeGlyfComposite
	default:
		return nil, fmt.Errorf("unknown font type: %s/%s/%s/%s",
			fontType, cidFontType, fontKey, subType)
	}

	return res, nil
}

var isBuiltinFont = map[pdf.Name]bool{
	"Courier":               true,
	"Courier-Bold":          true,
	"Courier-BoldOblique":   true,
	"Courier-Oblique":       true,
	"Helvetica":             true,
	"Helvetica-Bold":        true,
	"Helvetica-BoldOblique": true,
	"Helvetica-Oblique":     true,
	"Times-Roman":           true,
	"Times-Bold":            true,
	"Times-BoldItalic":      true,
	"Times-Italic":          true,
	"Symbol":                true,
	"ZapfDingbats":          true,
}
