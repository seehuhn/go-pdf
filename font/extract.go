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

	Type1                 // Type 1 fonts
	Builtin               // built-in fonts
	SimpleCFF             // CFF font data without wrapper (simple font)
	SimpleOpenTypeCFF     // CFF font data in an OpenType wrapper (simple font)
	MMType1               // Multiple Master type 1 fonts
	SimpleTrueType        // TrueType fonts (simple font)
	SimpleOpenTypeGlyf    // OpenType fonts with glyf outline (simple font)
	Type3                 // Type 3 fonts
	CompositeCFF          // CFF font data without wrapper (composite font)
	CompositeOpenTypeCFF  // CFF fonts in an OpenType wrapper (composite font)
	CompositeTrueType     // TrueType fonts (composite font)
	CompositeOpenTypeGlyf // OpenType fonts with glyf outline (composite font)
)

func (t EmbeddingType) String() string {
	switch t {
	case Type1:
		return "Type 1"
	case Builtin:
		return "Type 1 (built-in)"
	case SimpleCFF:
		return "Simple CFF"
	case SimpleOpenTypeCFF:
		return "Simple OpenType/CFF"
	case MMType1:
		return "MMType1"
	case SimpleTrueType:
		return "Simple TrueType"
	case SimpleOpenTypeGlyf:
		return "Simple OpenType/glyf"
	case Type3:
		return "Type 3"
	case CompositeCFF:
		return "Composite CFF"
	case CompositeOpenTypeCFF:
		return "Composite OpenType/CFF"
	case CompositeTrueType:
		return "Composite TrueType"
	case CompositeOpenTypeGlyf:
		return "Composite OpenType/glyf"
	default:
		return fmt.Sprintf("EmbeddingType(%d)", int(t))
	}
}

func (t EmbeddingType) IsComposite() bool {
	switch t {
	case CompositeCFF, CompositeOpenTypeCFF, CompositeTrueType, CompositeOpenTypeGlyf:
		return true
	default:
		return false
	}
}

type Dicts struct {
	FontDict       pdf.Dict
	CIDFontDict    pdf.Dict
	FontDescriptor pdf.Dict
	FontProgram    *pdf.Stream
	Type           EmbeddingType
}

func ExtractDicts(r pdf.Getter, ref pdf.Reference) (*Dicts, error) {
	res := &Dicts{}

	fontDict, err := pdf.GetDict(r, ref)
	if err != nil {
		return nil, err
	}
	err = pdf.CheckDictType(r, fontDict, "Font")
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

		cidFontDict, err := pdf.GetDict(r, descendantFonts[0])
		if err != nil {
			return nil, err
		}
		err = pdf.CheckDictType(r, cidFontDict, "Font")
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

	fontDescriptor, err := pdf.GetDict(r, fontDict["FontDescriptor"])
	if err != nil {
		return nil, err
	}
	var fontKey pdf.Name
	if fontDescriptor != nil {
		res.FontDescriptor = fontDescriptor
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
		res.Type = SimpleCFF
	case fontType == "Type1" && fontKey == "FontFile3" && subType == "OpenType":
		res.Type = SimpleOpenTypeCFF
	case fontType == "MMType1":
		res.Type = MMType1
	case fontType == "TrueType" && fontKey == "FontFile2":
		res.Type = SimpleTrueType
	case fontType == "TrueType" && fontKey == "FontFile3" && subType == "OpenType":
		res.Type = SimpleOpenTypeGlyf
	case fontType == "Type3":
		res.Type = Type3
	case fontType == "Type0" && cidFontType == "CIDFontType0" && fontKey == "FontFile3" && subType == "CIDFontType0C":
		res.Type = CompositeCFF
	case fontType == "Type0" && cidFontType == "CIDFontType0" && fontKey == "FontFile3" && subType == "OpenType":
		res.Type = CompositeOpenTypeCFF
	case fontType == "Type0" && cidFontType == "CIDFontType2" && fontKey == "FontFile2":
		res.Type = CompositeTrueType
	case fontType == "Type0" && cidFontType == "CIDFontType2" && fontKey == "FontFile3" && subType == "OpenType":
		res.Type = CompositeOpenTypeGlyf
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
