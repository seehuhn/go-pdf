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
	"slices"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font/pdfenc"
	"seehuhn.de/go/pdf/font/subset"
	"seehuhn.de/go/pdf/internal/stdmtx"
)

// DictType represents the different types of font dictionaries in PDF.
//
// TODO(voss): remove
//
// Deprecated: use [glyphdata.Type] instead.
type DictType int

func (t DictType) String() string {
	switch t {
	case DictTypeSimpleType1:
		return "Type 1"
	case DictTypeSimpleTrueType:
		return "TrueType"
	case DictTypeType3:
		return "Type 3"
	case DictTypeCompositeCFF:
		return "Type 1 (composite)"
	case DictTypeCompositeTrueType:
		return "TrueType (composite)"
	default:
		return fmt.Sprintf("DictType(%d)", int(t))
	}
}

// List of all font dictionary types supported by PDF.
const (
	DictTypeUnknown DictType = iota
	DictTypeSimpleType1
	DictTypeSimpleTrueType
	DictTypeType3
	DictTypeCompositeCFF
	DictTypeCompositeTrueType
)

// DataType represents the different types of font data in PDF.
//
// TODO(voss): remove
//
// Deprecated: use [glyphdata.Type] instead.
type DataType int

// List of all font data types supported by PDF.
const (
	DataUnknown DataType = iota
	DataType1
	DataType3
	DataCFF
	DataTrueType
	DataOpenType
)

// EmbeddingTypeOld represents the different ways font data
// can be embedded in a PDF file.
//
// The different ways of embedding fonts in a PDF file are
// represented by values of type [EmbeddingTypeOld]. There are seven different
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
//
// TODO(voss): Migrate to use [DictType] instead.
type EmbeddingTypeOld int

// List of all embedding types supported by PDF.
const (
	Unknown EmbeddingTypeOld = iota

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

func (t EmbeddingTypeOld) String() string {
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
func (t EmbeddingTypeOld) IsComposite() bool {
	switch t {
	case CFFComposite, OpenTypeCFFComposite, TrueTypeComposite, OpenTypeGlyfComposite:
		return true
	default:
		return false
	}
}

// MustBe returns an error if the embedding type is not as expected.
func (t EmbeddingTypeOld) MustBe(expected EmbeddingTypeOld) error {
	if t != expected {
		return fmt.Errorf("expected %q, got %q", expected, t)
	}
	return nil
}

// Dicts collects all information about a font embedded in a PDF file.
type Dicts struct {
	DictType DictType

	// PostScriptName is the BaseFont entry of the font dictionary
	// or CIDFont dictionary.  It does not include the subset tag.
	PostScriptName string

	// SubsetTag is the tag used to identify a font subset.
	// This is the part of the BaseFont entry of the font dictionary
	// before the "+" character, if any.
	SubsetTag string

	// FontDict contains the font dictionary.
	FontDict pdf.Dict

	// CIDFontDict contains the CIDFont dictionary.
	// This is only non-nil for composite fonts.
	CIDFontDict pdf.Dict

	// FontDescriptor contains information about the font metrics.
	FontDescriptor *Descriptor

	FontDataKey pdf.Name
	FontData    *pdf.Stream

	FontTypeOld EmbeddingTypeOld
}

// ExtractDicts reads information about a font from a PDF file.
//
// If the fonts is one of the standard 14 PDF fonts, the function provides
// a font descriptor and glyph widths, in case these were not present in the
// font dictionary.
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
			return nil, fmt.Errorf("invalid DescendantFonts: %v", descendantFonts)
		}

		cidFontDict, err := pdf.GetDictTyped(r, descendantFonts[0], "Font")
		if err != nil {
			return nil, pdf.Wrap(err, "CIDFont dict")
		} else if cidFontDict == nil {
			return nil, errors.New("missing CIDFont dict")
		}
		res.CIDFontDict = cidFontDict

		cidFontType, err = pdf.GetName(r, cidFontDict["Subtype"])
		if err != nil {
			return nil, pdf.Wrap(err, "CIDFont Subtype")
		}

		fontDict = cidFontDict
	}

	var dictType DictType
	switch {
	case fontType == "Type1" || fontType == "MMType1":
		dictType = DictTypeSimpleType1
	case fontType == "TrueType":
		dictType = DictTypeSimpleTrueType
	case fontType == "Type3":
		dictType = DictTypeType3
	case fontType == "Type0" && cidFontType == "CIDFontType0":
		dictType = DictTypeCompositeCFF
	case fontType == "Type0" && cidFontType == "CIDFontType2":
		dictType = DictTypeCompositeTrueType
	default:
		if fontType == "Type0" {
			return nil, pdf.Errorf("unknown font type: %s/%s", fontType, cidFontType)
		}
		return nil, pdf.Errorf("unknown font type: %s", fontType)
	}
	res.DictType = dictType

	fontName, err := pdf.GetName(r, fontDict["BaseFont"])
	if err == nil {
		if m := subset.TagRegexp.FindStringSubmatch(string(fontName)); m != nil {
			res.SubsetTag = m[1]
			fontName = pdf.Name(m[2])
		}
		res.PostScriptName = string(fontName)
	}

	fontDescriptor, err := pdf.GetDictTyped(r, fontDict["FontDescriptor"], "FontDescriptor")
	if err != nil {
		return nil, pdf.Wrap(err, "FontDescriptor")
	}
	if fontDescriptor != nil {
		res.FontDescriptor, err = ExtractDescriptor(r, fontDescriptor)
		if err != nil {
			return nil, pdf.Wrap(err, "FontDescriptor")
		}
	}

	// If the font is one of the standard 14 PDF fonts, we can fill in
	// missing information, including the font descriptor.
	if info, ok := stdmtx.Metrics[res.PostScriptName]; ok && res.DictType == DictTypeSimpleType1 {
		res.fixStandardFont(r, info)
	}

	if res.FontDescriptor == nil && res.DictType != DictTypeType3 {
		return nil, pdf.Errorf("font %q: missing FontDescriptor", res.PostScriptName)
	}

	var fontKey pdf.Name
	var fontRef pdf.Reference
	for _, key := range []pdf.Name{"FontFile", "FontFile2", "FontFile3"} {
		if ref, _ := fontDescriptor[key].(pdf.Reference); ref != 0 {
			fontKey = key
			fontRef = ref
			break
		}
	}

	var subType pdf.Name
	if fontRef != 0 {
		stmObj, err := pdf.GetStream(r, fontRef)
		if err != nil {
			return nil, pdf.Wrap(err, string(fontKey))
		}
		res.FontDataKey = fontKey
		res.FontData = stmObj
		subType, err = pdf.GetName(r, stmObj.Dict["Subtype"])
		if err != nil {
			return nil, pdf.Wrap(err, "Subtype")
		}
	}
	switch {
	case fontType == "Type1" && (fontKey == "FontFile" || fontKey == ""):
		res.FontTypeOld = Type1
	case fontType == "Type1" && fontKey == "FontFile3" && subType == "Type1C":
		res.FontTypeOld = CFFSimple
	case fontType == "Type1" && fontKey == "FontFile3" && subType == "OpenType":
		res.FontTypeOld = OpenTypeCFFSimple
	case fontType == "MMType1":
		res.FontTypeOld = MMType1
	case fontType == "TrueType" && fontKey == "FontFile2":
		res.FontTypeOld = TrueTypeSimple
	case fontType == "TrueType" && fontKey == "FontFile3" && subType == "OpenType":
		res.FontTypeOld = OpenTypeGlyfSimple
	case fontType == "Type3":
		res.FontTypeOld = Type3
	case fontType == "Type0" && cidFontType == "CIDFontType0" && fontKey == "FontFile3" && subType == "CIDFontType0C":
		res.FontTypeOld = CFFComposite
	case fontType == "Type0" && cidFontType == "CIDFontType0" && fontKey == "FontFile3" && subType == "OpenType":
		res.FontTypeOld = OpenTypeCFFComposite
	case fontType == "Type0" && cidFontType == "CIDFontType0" && fontKey == "":
		res.FontTypeOld = ExternalCFFComposite
	case fontType == "Type0" && cidFontType == "CIDFontType2" && fontKey == "FontFile2":
		res.FontTypeOld = TrueTypeComposite
	case fontType == "Type0" && cidFontType == "CIDFontType2" && fontKey == "FontFile3" && subType == "OpenType":
		res.FontTypeOld = OpenTypeGlyfComposite
	case fontType == "Type0" && cidFontType == "CIDFontType2" && fontKey == "":
		res.FontTypeOld = ExternalGlyfComposite
	default:
		return nil, &pdf.MalformedFileError{
			Err: errors.New("unknown font type"),
			Loc: []string{pdf.AsString(fontDictRef)},
		}
	}

	return res, nil
}

// fixStandardFont fills in missing information for the 14 standard fonts.
// The information concerned is the Font Descriptor and the glyph widths,
// both of which were optional before PDF 2.0.
func (d *Dicts) fixStandardFont(r pdf.Getter, info *stdmtx.FontData) {
	missingWidth := info.Width[".notdef"]

	if d.FontDescriptor == nil {
		fontName := d.PostScriptName
		if d.SubsetTag != "" {
			fontName = d.SubsetTag + "+" + fontName
		}
		d.FontDescriptor = &Descriptor{
			FontName:     fontName,
			FontFamily:   info.FontFamily,
			FontWeight:   info.FontWeight,
			IsFixedPitch: info.IsFixedPitch,
			IsSerif:      info.IsSerif,
			IsItalic:     info.ItalicAngle != 0,
			FontBBox:     info.FontBBox,
			ItalicAngle:  info.ItalicAngle,
			Ascent:       info.Ascent,
			Descent:      info.Descent,
			CapHeight:    info.CapHeight,
			XHeight:      info.XHeight,
			StemV:        info.StemV,
			StemH:        info.StemH,
			MissingWidth: missingWidth,
		}
		if info.FontFamily == "Symbol" || info.FontFamily == "ZapfDingbats" {
			d.FontDescriptor.IsSymbolic = true
		}
	}

	_, hasFirstChar := d.FontDict["FirstChar"]
	_, hasWidths := d.FontDict["Widths"]
	if !hasFirstChar || !hasWidths {
		// Decode the `Encoding` entry by hand, so that we can determine
		// which width to use for each code.  We ignore invalid information
		// where possible.  Since `Encoding` may be parsed again, we always
		// replace `Encoding` with a valid entry here, to avoid
		// inconsistencies.
		encoding := pdfenc.Standard.Encoding[:]
		switch d.PostScriptName {
		case "Symbol":
			encoding = pdfenc.Symbol.Encoding[:]
		case "ZapfDingbats":
			encoding = pdfenc.ZapfDingbats.Encoding[:]
		}
		enc, _ := pdf.Resolve(r, d.FontDict["Encoding"])
		var sanitisedEnc pdf.Object
		switch enc := enc.(type) {
		case pdf.Name:
			sanitisedEnc = enc
			switch enc {
			case "WinAnsiEncoding":
				encoding = pdfenc.WinAnsi.Encoding[:]
			case "MacRomanEncoding":
				encoding = pdfenc.MacRoman.Encoding[:]
			case "MacExpertEncoding":
				encoding = pdfenc.MacExpert.Encoding[:]
			default:
				sanitisedEnc = nil
			}
		case pdf.Dict:
			base, _ := pdf.GetName(r, enc["BaseEncoding"])
			sanitisedBase := base
			switch base {
			case "WinAnsiEncoding":
				encoding = pdfenc.WinAnsi.Encoding[:]
			case "MacRomanEncoding":
				encoding = pdfenc.MacRoman.Encoding[:]
			case "MacExpertEncoding":
				encoding = pdfenc.MacExpert.Encoding[:]
			default:
				sanitisedBase = ""
			}

			diff, _ := pdf.GetArray(r, enc["Differences"])
			var sanitisedDiff pdf.Array
			if len(diff) > 0 {
				encoding = slices.Clone(encoding)
				code := -1
				for _, obj := range diff {
					obj, _ := pdf.Resolve(r, obj)
					switch obj := obj.(type) {
					case pdf.Integer:
						if obj >= 0 && obj < 256 {
							code = int(obj)
							sanitisedDiff = append(sanitisedDiff, obj)
						} else {
							code = -1
						}
					case pdf.Name:
						if code >= 0 && code < 256 {
							sanitisedDiff = append(sanitisedDiff, obj)
							encoding[code] = string(obj)
							code++
						}
					default:
						code = -1
					}
				}
			}
			if len(sanitisedDiff) > 0 {
				if sanitisedBase != "" {
					sanitisedEnc = pdf.Dict{
						"BaseEncoding": sanitisedBase,
						"Differences":  sanitisedDiff,
					}
				} else {
					sanitisedEnc = pdf.Dict{
						"Differences": sanitisedDiff,
					}
				}
			} else if sanitisedBase != "" {
				sanitisedEnc = sanitisedBase
			}
		}
		if sanitisedEnc != nil {
			d.FontDict["Encoding"] = sanitisedEnc
		} else {
			delete(d.FontDict, "Encoding")
		}

		firstChar := 0
		for firstChar < 255 && encoding[firstChar] == ".notdef" {
			firstChar++
		}
		lastChar := 255
		for lastChar > firstChar && encoding[lastChar] == ".notdef" {
			lastChar--
		}
		widths := make(pdf.Array, lastChar-firstChar+1)
		for i := firstChar; i <= lastChar; i++ {
			w, ok := info.Width[encoding[i]]
			if !ok {
				w = missingWidth
			}
			widths[i-firstChar] = pdf.Integer(w)
		}
		d.FontDict["FirstChar"] = pdf.Integer(firstChar)
		d.FontDict["LastChar"] = pdf.Integer(lastChar)
		d.FontDict["Widths"] = widths
		d.FontDescriptor.MissingWidth = missingWidth
	}
}

func (d *Dicts) IsSimple() bool {
	return d.DictType == DictTypeSimpleType1 ||
		d.DictType == DictTypeSimpleTrueType ||
		d.DictType == DictTypeType3
}

func (d *Dicts) IsComposite() bool {
	return d.DictType == DictTypeCompositeCFF ||
		d.DictType == DictTypeCompositeTrueType
}

// IsSymbolic returns true, if the fonts is marked as a symbolic font
// in the font descriptor.  This indicates that the font contains glyphs
// which are not in the Standard Latin character set.
func (d *Dicts) IsSymbolic() bool {
	return d.FontDescriptor.IsSymbolic
}

// IsNonSymbolic returns true, if the fonts is marked as a non-symbolic font
// in the font descriptor.  This indicates that the font contains only glyphs
// from the Standard Latin character set.
func (d *Dicts) IsNonSymbolic() bool {
	return !d.FontDescriptor.IsSymbolic
}

func (d *Dicts) IsStandardFont() bool {
	if d.DictType != DictTypeSimpleType1 {
		return false
	}

	return IsStandard[d.PostScriptName]
}

// IsExternal returns true, if the font data is not included in the PDF
// file but instead must be loaded from an external source by the reader.
func (d *Dicts) IsExternal() bool {
	return d.FontData == nil
}
