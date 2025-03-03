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
	"fmt"
	"iter"

	"seehuhn.de/go/postscript/cid"
	"seehuhn.de/go/postscript/type1/names"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/encoding"
	"seehuhn.de/go/pdf/font/glyphdata"
	"seehuhn.de/go/pdf/font/subset"
)

var (
	_ font.Dict = (*TrueType)(nil)
)

// TrueType represents a TrueType font dictionary.
// This can correspond either to a TrueType or an OpenType font.
type TrueType struct {
	// Ref is the reference to the font dictionary in the PDF file.
	Ref pdf.Reference

	// PostScriptName is the PostScript name of the font
	// (without any subset tag).
	PostScriptName string

	// SubsetTag can be set to indicate that the font has been subsetted.
	// If non-empty, the value must be a sequence of 6 uppercase letters.
	SubsetTag string

	// Name is deprecated and is normally empty.
	// For PDF 1.0 this was the name the font was referenced by from
	// within content streams.
	Name pdf.Name

	// Descriptor is the font descriptor.
	Descriptor *font.Descriptor

	// Encoding maps character codes to glyph names.
	Encoding encoding.Type1

	// Width contains the glyph widths for all character codes
	// (PDF glyph space units).
	Width [256]float64

	// Text gives the text content for each character code.
	Text [256]string

	// FontType gives the type of glyph outline data. Possible values are
	// [glyphdata.TrueType] and [glyphdata.OpenTypeGlyf], or [glyphdata.None]
	// if the font is not embedded.
	FontType glyphdata.Type

	// FontRef is the reference to the glyph outline data in the PDF file,
	// if the font is embedded.
	FontRef pdf.Reference
}

// ExtractTrueType reads a TrueType font dictionary from a PDF file.
func ExtractTrueType(r pdf.Getter, obj pdf.Object) (*TrueType, error) {
	fontDict, err := pdf.GetDictTyped(r, obj, "Font")
	if err != nil {
		return nil, err
	} else if fontDict == nil {
		return nil, &pdf.MalformedFileError{
			Err: errors.New("missing font dictionary"),
		}
	}
	subtype, err := pdf.GetName(r, fontDict["Subtype"])
	if err != nil {
		return nil, err
	}
	if subtype != "" && subtype != "TrueType" {
		return nil, &pdf.MalformedFileError{
			Err: fmt.Errorf("expected font subtype TrueType, got %q", subtype),
		}
	}

	d := &TrueType{}
	d.Ref, _ = obj.(pdf.Reference)

	baseFont, err := pdf.GetName(r, fontDict["BaseFont"])
	if err != nil {
		return nil, err
	}
	if m := subset.TagRegexp.FindStringSubmatch(string(baseFont)); m != nil {
		d.PostScriptName = m[2]
		d.SubsetTag = m[1]
	} else {
		d.PostScriptName = string(baseFont)
	}

	d.Name, _ = pdf.GetName(r, fontDict["Name"])

	fdDict, err := pdf.GetDictTyped(r, fontDict["FontDescriptor"], "FontDescriptor")
	if pdf.IsReadError(err) {
		return nil, err
	}
	fd, _ := font.ExtractDescriptor(r, fdDict)
	if fd == nil { // only possible for invalid PDF files
		fd = &font.Descriptor{
			FontName: d.PostScriptName,
		}
	}
	d.Descriptor = fd

	if ref, _ := fontDict["FontFile2"].(pdf.Reference); ref != 0 {
		d.FontType = glyphdata.TrueType
		d.FontRef = ref
	} else if ref, _ := fontDict["FontFile3"].(pdf.Reference); ref != 0 {
		if stm, _ := pdf.GetStream(r, ref); stm != nil {
			subType, _ := pdf.GetName(r, stm.Dict["Subtype"])
			switch subType {
			case "OpenType":
				d.FontType = glyphdata.OpenTypeGlyf
				d.FontRef = ref
			}
		}
	}

	isNonSymbolic := !fd.IsSymbolic
	isExternal := d.FontRef == 0
	nonSymbolicExt := isNonSymbolic && isExternal
	enc, err := encoding.ExtractType1(r, fontDict["Encoding"], nonSymbolicExt)
	if err != nil {
		return nil, err
	}
	d.Encoding = enc

	firstChar, _ := pdf.GetInteger(r, fontDict["FirstChar"])
	widths, _ := pdf.GetArray(r, fontDict["Widths"])
	if widths != nil && len(widths) <= 256 && firstChar >= 0 && firstChar < 256 {
		for c := range d.Width {
			d.Width[c] = fd.MissingWidth
		}
		for i, w := range widths {
			w, err := pdf.GetNumber(r, w)
			if err != nil {
				continue
			}
			if code := firstChar + pdf.Integer(i); code < 256 {
				d.Width[byte(code)] = float64(w)
			}
		}
	}

	// First try to derive text content from the glyph names.
	for code := range 256 {
		glyphName := enc(byte(code))
		if glyphName == "" || glyphName == encoding.UseBuiltin || glyphName == ".notdef" {
			continue
		}

		rr := names.ToUnicode(glyphName, d.PostScriptName == "ZapfDingbats")
		d.Text[code] = string(rr)
	}
	// the ToUnicode cmap, if present, overrides the derived text content
	toUnicode, err := cmap.ExtractToUnicode(r, fontDict["ToUnicode"])
	if pdf.IsReadError(err) {
		return nil, err
	}
	if toUnicode != nil {
		// TODO(voss): implement an iterator on toUnicode to do this
		// more efficiently?
		for code := range 256 {
			rr, found := toUnicode.Lookup([]byte{byte(code)})
			if found {
				d.Text[code] = rr
			}
		}
	}

	return d, nil
}

// WriteToPDF adds the font dictionary to the PDF file.
func (d *TrueType) WriteToPDF(rm *pdf.ResourceManager) error {
	// Check that all data are valid and consistent.
	if d.Ref == 0 {
		return errors.New("missing font dictionary reference")
	}
	if (d.FontType == glyphdata.None) != (d.FontRef == 0) {
		return errors.New("missing font reference or type")
	}
	switch d.FontType {
	case glyphdata.None:
		// not embedded
	case glyphdata.TrueType:
		if err := pdf.CheckVersion(rm.Out, "embedded TrueType font", pdf.V1_1); err != nil {
			return err
		}
	case glyphdata.OpenTypeGlyf:
		if err := pdf.CheckVersion(rm.Out, "embedded OpenType/glyf font", pdf.V1_6); err != nil {
			return err
		}
	default:
		return fmt.Errorf("invalid font type %s", d.FontType)
	}
	if d.SubsetTag != "" && !subset.IsValidTag(d.SubsetTag) {
		return fmt.Errorf("invalid subset tag: %s", d.SubsetTag)
	}

	w := rm.Out

	var baseFont pdf.Name
	if d.SubsetTag != "" {
		baseFont = pdf.Name(d.SubsetTag + "+" + d.PostScriptName)
	} else {
		baseFont = pdf.Name(d.PostScriptName)
	}

	fontDict := pdf.Dict{
		"Type":     pdf.Name("Font"),
		"Subtype":  pdf.Name("TrueType"),
		"BaseFont": baseFont,
	}
	if d.Name != "" {
		fontDict["Name"] = d.Name
	}

	isNonSymbolic := !d.Descriptor.IsSymbolic
	isExternal := d.FontRef == 0
	// TODO(voss): implement TrueType constraints
	encodingObj, err := d.Encoding.AsPDFType1(isNonSymbolic && isExternal, w.GetOptions())
	if err != nil {
		return err
	}
	if encodingObj != nil {
		fontDict["Encoding"] = encodingObj
	}

	compressedObjects := []pdf.Object{fontDict}
	compressedRefs := []pdf.Reference{d.Ref}

	fdRef := w.Alloc()
	fdDict := d.Descriptor.AsDict()
	switch d.FontType {
	case glyphdata.TrueType:
		fdDict["FontFile2"] = d.FontRef
	case glyphdata.OpenTypeGlyf:
		fdDict["FontFile3"] = d.FontRef
	}
	fontDict["FontDescriptor"] = fdRef
	compressedObjects = append(compressedObjects, fdDict)
	compressedRefs = append(compressedRefs, fdRef)

	// TODO(voss): Introduce a helper function for constructing the widths
	// array.
	firstChar, lastChar := 0, 255
	for lastChar > 0 && d.Width[lastChar] == d.Descriptor.MissingWidth {
		lastChar--
	}
	for firstChar < lastChar && d.Width[firstChar] == d.Descriptor.MissingWidth {
		firstChar++
	}
	widths := make(pdf.Array, lastChar-firstChar+1)
	for i := range widths {
		widths[i] = pdf.Number(d.Width[firstChar+i])
	}

	fontDict["FirstChar"] = pdf.Integer(firstChar)
	fontDict["LastChar"] = pdf.Integer(lastChar)
	if len(widths) > 10 {
		widthRef := w.Alloc()
		fontDict["Widths"] = widthRef
		compressedObjects = append(compressedObjects, widths)
		compressedRefs = append(compressedRefs, widthRef)
	} else {
		fontDict["Widths"] = widths
	}

	toUnicodeData := make(map[byte]string)
	for code := range 256 {
		glyphName := d.Encoding(byte(code))
		switch glyphName {
		case "", ".notdef":
			// unused character code, nothing to do

		case encoding.UseBuiltin:
			if d.Text[code] != "" {
				toUnicodeData[byte(code)] = d.Text[code]
			}

		default:
			rr := names.ToUnicode(glyphName, d.PostScriptName == "ZapfDingbats")
			if text := d.Text[code]; text != string(rr) {
				toUnicodeData[byte(code)] = text
			}
		}
	}
	if len(toUnicodeData) > 0 {
		tuInfo := cmap.MakeSimpleToUnicode(toUnicodeData)
		ref, _, err := pdf.ResourceManagerEmbed(rm, tuInfo)
		if err != nil {
			return fmt.Errorf("ToUnicode cmap: %w", err)
		}
		fontDict["ToUnicode"] = ref
	}

	err = w.WriteCompressed(compressedRefs, compressedObjects...)
	if err != nil {
		return pdf.Wrap(err, "Type 1 font dicts")
	}

	return nil
}

func (d *TrueType) MakeFont() (font.FromFile, error) {
	return d, nil
}

func (d *TrueType) GetDict() font.Dict {
	return d
}

func (d *TrueType) WritingMode() font.WritingMode {
	return font.Horizontal
}

func (d *TrueType) Codes(s pdf.String) iter.Seq[*font.Code] {
	return func(yield func(*font.Code) bool) {
		var code font.Code
		for _, c := range s {
			code.CID = cid.CID(c) + 1 // leave CID 0 for notdef
			code.Width = d.Width[c]
			code.Text = d.Text[c]
			code.UseWordSpacing = (c == 0x20)
			if !yield(&code) {
				return
			}
		}
	}
}

func init() {
	font.RegisterReader("TrueType", func(r pdf.Getter, obj pdf.Object) (font.Dict, error) {
		return ExtractTrueType(r, obj)
	})
}
