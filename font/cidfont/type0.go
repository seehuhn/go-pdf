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

package cidfont

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/subset"
	"seehuhn.de/go/pdf/font/widths"
	"seehuhn.de/go/sfnt"
	"seehuhn.de/go/sfnt/cff"
)

// Type0FontData is a font which can be used with a Type 0 CIDFont.
// This must be one of [*cff.Font] or [*sfnt.Font].
type Type0FontData interface{}

var _ font.Embedded = (*Type0Dict)(nil)

// Type0Dict holds the information from the font dictionary and CIDFont dictionary
// of a Type 0 (CFF-based) CIDFont.
type Type0Dict struct {
	// Ref is the reference to the font dictionary in the PDF file.
	Ref pdf.Reference

	// PostScriptName is the PostScript name of the font
	// (without any subset tag).
	PostScriptName string

	SubsetTag string

	// Descriptor is the font descriptor.
	Descriptor *font.Descriptor

	// Encoding specifies how character codes are mapped to CID values.
	Encoding *cmap.InfoNew

	// Width is a map from CID values to glyph widths (in PDF glyph space units).
	Width map[cmap.CID]float64

	// DefaultWidth is the glyph width for CID values not in the Width map
	// (in PDF glyph space units).
	DefaultWidth float64

	// TODO(voss): vertical glyph metrics

	// Text specifies how character codes are mapped to Unicode strings.
	Text *cmap.ToUnicodeInfo

	// GetFont (optional) returns the font data to embed.
	// If this is nil, the font data is not embedded in the PDF file.
	GetFont func() (Type0FontData, error)
}

// ExtractType0 extracts the information from a Type 0 CIDFont from a PDF file.
func ExtractType0(r pdf.Getter, obj pdf.Object) (*Type0Dict, error) {
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
	if subtype != "" && subtype != "Type0" {
		return nil, &pdf.MalformedFileError{
			Err: fmt.Errorf("expected font subtype Type0, got %q", subtype),
		}
	}

	a, err := pdf.GetArray(r, fontDict["DescendantFonts"])
	if err != nil {
		return nil, err
	} else if len(a) != 1 {
		return nil, &pdf.MalformedFileError{
			Err: errors.New("invalid DescendantFonts array"),
		}
	}
	cidFontDict, err := pdf.GetDictTyped(r, a[0], "Font")
	if err != nil {
		return nil, err
	} else if cidFontDict == nil {
		return nil, &pdf.MalformedFileError{
			Err: errors.New("missing CIDFont dictionary"),
		}
	}
	subtype, err = pdf.GetName(r, cidFontDict["Subtype"])
	if err != nil {
		return nil, err
	} else if subtype != "CIDFontType0" {
		return nil, &pdf.MalformedFileError{
			Err: fmt.Errorf("expected CIDFontType0, got %q", subtype),
		}
	}

	d := &Type0Dict{}
	d.Ref, _ = obj.(pdf.Reference)

	// fields in the font dictionary

	d.Encoding, err = cmap.ExtractNew(r, fontDict["Encoding"])
	if err != nil {
		return nil, err
	}

	d.Text, err = cmap.ExtractToUnicodeNew(r, fontDict["ToUnicode"])
	if pdf.IsReadError(err) {
		return nil, err
	}

	// fields in the CIDFont dictionary

	baseFont, err := pdf.GetName(r, cidFontDict["BaseFont"])
	if err != nil {
		return nil, err
	}
	if m := subset.TagRegexp.FindStringSubmatch(string(baseFont)); m != nil {
		d.PostScriptName = m[2]
		d.SubsetTag = m[1]
	} else {
		d.PostScriptName = string(baseFont)
	}

	fdDict, err := pdf.GetDictTyped(r, cidFontDict["FontDescriptor"], "FontDescriptor")
	if pdf.IsReadError(err) {
		return nil, err
	}
	d.Descriptor, _ = font.ExtractDescriptor(r, fdDict)
	if d.Descriptor == nil { // only possible for invalid PDF files
		d.Descriptor = &font.Descriptor{
			FontName: d.PostScriptName,
		}
	}

	d.Width, err = widths.ExtractComposite(r, cidFontDict["W"])
	if err != nil {
		return nil, err
	}
	dw, err := pdf.GetNumber(r, cidFontDict["DW"])
	if pdf.IsReadError(err) {
		return nil, err
	}
	d.DefaultWidth = float64(dw)

	getFont, err := makeFontReader(r, fdDict)
	if pdf.IsReadError(err) {
		return nil, err
	}
	d.GetFont = getFont

	return d, nil
}

func makeFontReader(r pdf.Getter, fd pdf.Dict) (func() (Type0FontData, error), error) {
	s, err := pdf.GetStream(r, fd["FontFile3"])
	if pdf.IsReadError(err) {
		return nil, err
	} else if s == nil {
		return nil, nil
	}

	subType, _ := pdf.GetName(r, s.Dict["Subtype"])
	switch subType {
	case "CIDFontType0C":
		getFont := func() (Type0FontData, error) {
			fontData, err := pdf.DecodeStream(r, s, 0)
			if err != nil {
				return nil, err
			}
			body, err := io.ReadAll(fontData)
			if err != nil {
				return nil, err
			}
			font, err := cff.Read(bytes.NewReader(body))
			if err != nil {
				return nil, err
			}
			return font, nil
		}
		return getFont, nil

	case "OpenType":
		getFont := func() (Type0FontData, error) {
			fontData, err := pdf.DecodeStream(r, s, 0)
			if err != nil {
				return nil, err
			}
			font, err := sfnt.Read(fontData)
			if err != nil {
				return nil, err
			}
			return font, nil
		}
		return getFont, nil

	default:
		return nil, nil
	}
}

// Finish embeds the font data in the PDF file.
// This implements the [pdf.Finisher] interface.
func (d *Type0Dict) Finish(rm *pdf.ResourceManager) error {
	w := rm.Out

	var fontData Type0FontData
	if d.GetFont != nil {
		var err error
		fontData, err = d.GetFont()
		if err != nil {
			return err
		}
	}

	// Check that all data are valid and consistent.
	if d.Ref == 0 {
		return errors.New("missing font dictionary reference")
	}
	switch f := fontData.(type) {
	case nil:
		// pass
	case *cff.Font:
		err := pdf.CheckVersion(w, "composite CFF fonts", pdf.V1_3)
		if err != nil {
			return err
		}
	case *sfnt.Font:
		if !f.IsCFF() {
			return errors.New("CFF table missing")
		}
		err := pdf.CheckVersion(w, "composite OpenType/CFF fonts", pdf.V1_6)
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported font type %T", fontData)
	}
	if d.SubsetTag != "" && !subset.IsValidTag(d.SubsetTag) {
		return fmt.Errorf("invalid subset tag: %s", d.SubsetTag)
	}

	var cidFontName pdf.Name
	if d.SubsetTag != "" {
		cidFontName = pdf.Name(d.SubsetTag + "+" + d.PostScriptName)
	} else {
		cidFontName = pdf.Name(d.PostScriptName)
	}

	// TODO(voss): How do we get the correct ROS for Identity-H/V?
	cidSystemInfo, _, err := pdf.ResourceManagerEmbed(rm, d.Encoding.ROS)
	if err != nil {
		return err
	}

	encoding, _, err := pdf.ResourceManagerEmbed(rm, d.Encoding)
	if err != nil {
		return err
	}

	var toUni pdf.Object
	if !d.Text.IsEmpty() {
		toUni, _, err = pdf.ResourceManagerEmbed(rm, d.Text)
		if err != nil {
			return err
		}
	}

	fontDictRef := d.Ref
	cidFontRef := w.Alloc()
	fdRef := w.Alloc()

	fontDict := pdf.Dict{
		"Type":            pdf.Name("Font"),
		"Subtype":         pdf.Name("Type0"),
		"BaseFont":        pdf.Name(cidFontName + "-" + d.Encoding.Name),
		"Encoding":        encoding,
		"DescendantFonts": pdf.Array{cidFontRef},
		"ToUnicode":       toUni,
	}

	cidFontDict := pdf.Dict{
		"Type":           pdf.Name("Font"),
		"Subtype":        pdf.Name("CIDFontType0"),
		"BaseFont":       pdf.Name(cidFontName),
		"CIDSystemInfo":  cidSystemInfo,
		"FontDescriptor": fdRef,
		// we set the glyph width information later
	}

	fdDict := d.Descriptor.AsDict()
	fdDict["FontName"] = cidFontDict["BaseFont"]
	var fontFileRef pdf.Reference
	if fontData != nil {
		fontFileRef = w.Alloc()
		fdDict["FontFile3"] = fontFileRef
	}

	compressedObjects := []pdf.Object{fontDict, cidFontDict, fdDict}
	compressedRefs := []pdf.Reference{fontDictRef, cidFontRef, fdRef}

	ww := widths.EncodeComposite2(d.Width, d.DefaultWidth)
	switch {
	case len(ww) > 10:
		wwRef := w.Alloc()
		cidFontDict["W"] = wwRef
		compressedObjects = append(compressedObjects, ww)
		compressedRefs = append(compressedRefs, wwRef)
	case len(ww) != 0:
		cidFontDict["W"] = ww
	}
	if math.Abs(d.DefaultWidth-1000) > 0.01 {
		cidFontDict["DW"] = pdf.Number(d.DefaultWidth)
	}

	err = w.WriteCompressed(compressedRefs, compressedObjects...)
	if err != nil {
		return pdf.Wrap(err, "composite OpenType/CFF font dicts")
	}

	// See section 9.9 of PDF 32000-1:2008 for details.
	switch f := fontData.(type) {
	case *cff.Font:
		fontFileDict := pdf.Dict{
			"Subtype": pdf.Name("CIDFontType0C"),
		}
		fontFileStream, err := w.OpenStream(fontFileRef, fontFileDict, pdf.FilterCompress{})
		if err != nil {
			return err
		}
		err = f.Write(fontFileStream)
		if err != nil {
			return fmt.Errorf("CFF font program %q: %w", cidFontName, err)
		}
		err = fontFileStream.Close()
		if err != nil {
			return err
		}

	case *sfnt.Font:
		fontFileDict := pdf.Dict{
			"Subtype": pdf.Name("OpenType"),
		}
		fontFileStream, err := w.OpenStream(fontFileRef, fontFileDict, pdf.FilterCompress{})
		if err != nil {
			return err
		}
		err = f.WriteOpenTypeCFFPDF(fontFileStream)
		if err != nil {
			return fmt.Errorf("OpenType/CFF font program %q: %w", cidFontName, err)
		}
		err = fontFileStream.Close()
		if err != nil {
			return err
		}
	}

	return nil
}

func (d *Type0Dict) WritingMode() cmap.WritingMode {
	return d.Encoding.WMode
}

// DecodeWidth reads one character code from the given string and returns
// the width of the corresponding glyph in PDF text space units (still to
// be multiplied by the font size) and the number of bytes read from the
// string.
//
// This implements the [font.Embedded] interface.
func (d *Type0Dict) DecodeWidth(s pdf.String) (float64, int) {
	enc := d.Encoding
	for code, valid := range enc.CodeSpaceRange.AllCodes(s) {
		if valid {
			cid := enc.LookupCID(code)
			if width, ok := d.Width[cid]; ok {
				return width, len(code)
			}
			return d.DefaultWidth, len(code)
		}
		return d.Width[0], 1
	}
	return 0, 0
}
