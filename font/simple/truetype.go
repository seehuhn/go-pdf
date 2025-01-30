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

package simple

import (
	"errors"
	"fmt"
	"iter"

	"seehuhn.de/go/postscript/cid"
	"seehuhn.de/go/postscript/type1/names"

	"seehuhn.de/go/sfnt"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/encoding"
	"seehuhn.de/go/pdf/font/subset"
)

// TrueTypeDict represents a TrueType font dictionary.
// This can correspond either to a TrueType or an OpenType font.
type TrueTypeDict struct {
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

	// IsOpenType is true if the font is embedded as OpenType font.
	IsOpenType bool

	// GetFont (optional) returns the font data to embed.
	// If this is nil, the font data is not embedded in the PDF file.
	// Otherwise, this is an [*sfnt.Font].
	GetFont func() (any, error)
}

// ExtractTrueTypeDict reads a TrueType font dictionary from a PDF file.
func ExtractTrueTypeDict(r pdf.Getter, obj pdf.Object) (*TrueTypeDict, error) {
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

	d := &TrueTypeDict{}
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

	isNonSymbolic := !fd.IsSymbolic
	isExternal := fdDict["FontFile2"] == nil && fdDict["FontFile3"] == nil
	nonSymbolicExt := isNonSymbolic && isExternal
	enc, err := encoding.ExtractType1(r, fontDict["Encoding"], nonSymbolicExt)
	if err != nil {
		return nil, err
	}
	d.Encoding = enc

	firstChar, _ := pdf.GetInteger(r, fontDict["FirstChar"])
	widths, _ := pdf.GetArray(r, fontDict["Widths"])
	if widths != nil && len(widths) <= 256 && firstChar >= 0 && firstChar < 256 {
		for c := range widths {
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

	_, d.IsOpenType = fontDict["FontFile3"]

	getFont, err := makeTrueTypeReader(r, fdDict)
	if pdf.IsReadError(err) {
		return nil, err
	}
	d.GetFont = getFont

	return d, nil
}

func makeTrueTypeReader(r pdf.Getter, fd pdf.Dict) (func() (any, error), error) {
	s, err := pdf.GetStream(r, fd["FontFile2"])
	if pdf.IsReadError(err) {
		return nil, err
	}
	if s != nil {
		getFont := func() (any, error) {
			fontData, err := pdf.DecodeStream(r, s, 0)
			if err != nil {
				return nil, err
			}
			font, err := sfnt.Read(fontData)
			if err != nil {
				return nil, err
			}
			if !font.IsGlyf() {
				return nil, errors.New("missing glyf table")
			}
			return font, nil
		}
		return getFont, nil
	}

	s, err = pdf.GetStream(r, fd["FontFile3"])
	if pdf.IsReadError(err) {
		return nil, err
	}
	if s == nil {
		return nil, nil
	}

	subType, _ := pdf.GetName(r, s.Dict["Subtype"])
	switch subType {
	case "OpenType":
		getFont := func() (any, error) {
			fontData, err := pdf.DecodeStream(r, s, 0)
			if err != nil {
				return nil, err
			}
			font, err := sfnt.Read(fontData)
			if err != nil {
				return nil, err
			}
			if !font.IsGlyf() {
				return nil, errors.New("missing glyf table")
			}
			return font, nil
		}
		return getFont, nil

	default:
		return nil, nil
	}
}

// WriteToPDF adds the font dictionary to the PDF file.
func (d *TrueTypeDict) WriteToPDF(rm *pdf.ResourceManager) error {
	var psFont any
	if d.GetFont != nil {
		font, err := d.GetFont()
		if err != nil {
			return err
		}
		psFont = font
	}

	// Check that all data are valid and consistent.
	if d.Ref == 0 {
		return errors.New("missing font dictionary reference")
	}
	switch f := psFont.(type) {
	case nil:
		// pass

	case *sfnt.Font:
		if !f.IsGlyf() {
			return errors.New("missing glyf table")
		}
		if err := pdf.CheckVersion(rm.Out, "embedded TrueType font", pdf.V1_1); err != nil {
			return err
		}

	default:
		return fmt.Errorf("unsupported font type %T", psFont)
	}
	if d.IsOpenType && d.GetFont == nil {
		return errors.New("missing OpenType font data")
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
		"Type":       pdf.Name("Font"),
		"Subtype":    pdf.Name("TrueType"),
		"BaseFont":   baseFont,
		"XX_Seehuhn": pdf.Boolean(true), // TODO(voss): remove
	}
	if d.Name != "" {
		fontDict["Name"] = d.Name
	}

	isNonSymbolic := !d.Descriptor.IsSymbolic
	isExternal := psFont == nil
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

	var fontFileRef pdf.Reference
	fdRef := w.Alloc()
	fdDict := d.Descriptor.AsDict()
	if psFont != nil {
		fontFileRef = w.Alloc()
		if !d.IsOpenType {
			fdDict["FontFile2"] = fontFileRef
		} else {
			fdDict["FontFile3"] = fontFileRef
		}
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

	if f := psFont.(*sfnt.Font); f != nil {
		if !d.IsOpenType {
			length1 := pdf.NewPlaceholder(w, 10)
			fontStmDict := pdf.Dict{
				"Length1": length1,
			}
			fontStm, err := w.OpenStream(fontFileRef, fontStmDict, pdf.FilterCompress{})
			if err != nil {
				return fmt.Errorf("open TrueType stream: %w", err)
			}
			l1, err := f.WriteTrueTypePDF(fontStm)
			if err != nil {
				return fmt.Errorf("write TrueType stream: %w", err)
			}
			err = length1.Set(pdf.Integer(l1))
			if err != nil {
				return fmt.Errorf("TrueType stream: length1: %w", err)
			}
			err = fontStm.Close()
			if err != nil {
				return fmt.Errorf("close TrueType stream: %w", err)
			}
		} else {
			fontFileDict := pdf.Dict{
				"Subtype": pdf.Name("OpenType"),
			}
			fontStm, err := w.OpenStream(fontFileRef, fontFileDict, pdf.FilterCompress{})
			if err != nil {
				return fmt.Errorf("open OpenType stream: %w", err)
			}
			_, err = f.WriteTrueTypePDF(fontStm)
			if err != nil {
				return fmt.Errorf("write OpenType stream: %w", err)
			}
			err = fontStm.Close()
			if err != nil {
				return fmt.Errorf("close OpenType stream: %w", err)
			}
		}
	}

	return nil
}

func (d *TrueTypeDict) GetScanner() (font.Scanner, error) {
	return d, nil
}

func (d *TrueTypeDict) Codes(s pdf.String) iter.Seq[*font.Code] {
	return func(yield func(*font.Code) bool) {
		var code font.Code
		for _, c := range s {
			code.CID = cid.CID(c) + 1 // leave CID 0 for notdef
			code.Width = d.Width[c]
			code.Text = d.Text[c]

			if !yield(&code) {
				return
			}
		}
	}
}

func init() {
	font.RegisterReader("TrueType", func(r pdf.Getter, obj pdf.Object) (font.FromFile, error) {
		return ExtractTrueTypeDict(r, obj)
	})
}
