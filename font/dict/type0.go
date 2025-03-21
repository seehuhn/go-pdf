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

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/glyphdata"
	"seehuhn.de/go/pdf/font/subset"
)

var (
	_ font.Dict = (*CIDFontType0)(nil)
)

// CIDFontType0 holds the information from the font dictionary and CIDFont
// dictionary of a Type 0 (CFF-based) CIDFont.
type CIDFontType0 struct {
	// PostScriptName is the PostScript name of the font
	// (without any subset tag).
	PostScriptName string

	// SubsetTag can be set to indicate that the font has been subsetted.
	// If non-empty, the value must be a sequence of 6 uppercase letters.
	SubsetTag string

	// Descriptor is the font descriptor.
	Descriptor *font.Descriptor

	// ROS describes the character collection covered by the font.
	ROS *cid.SystemInfo

	// CMap specifies how character codes are mapped to CID values.
	//
	// The CMap.ROS field must either be compatible with the ROS field
	// above, or the cmap must be one of Identity-H or Identity-V.
	CMap *cmap.File

	// Width (optional) is a map from CID values to glyph widths (in PDF glyph
	// space units).  Only widths which are different from the default width
	// need to be specified.
	Width map[cmap.CID]float64

	// DefaultWidth is the glyph width for CID values not in the Width map
	// (in PDF glyph space units).
	DefaultWidth float64

	// VMetrics (optional) maps CIDs to their vertical metrics.
	// These are used when the CMap specifies vertical writing mode.
	VMetrics map[cmap.CID]VMetrics

	// DefaultVMetrics contains the default vertical metrics.
	// These are used when the CMap specifies vertical writing mode,
	// and the CID is not in the VMetrics map.
	//
	// For horizontal writing mode, set this to DefaultVMetricsDefault.
	DefaultVMetrics DefaultVMetrics

	// Text (optional) specifies how character codes are mapped to Unicode
	// strings.
	Text *cmap.ToUnicodeFile

	// FontType gives the type of glyph outline data. Possible values are
	// [glyphdata.CFF] and [glyphdata.OpenTypeCFF],
	// or [glyphdata.None] if the font is not embedded.
	FontType glyphdata.Type

	// FontRef is the reference to the glyph outline data in the PDF file,
	// if the font is embedded.
	FontRef pdf.Reference
}

// ExtractCIDFontType0 extracts the information from a Type 0 CIDFont from a PDF file.
func ExtractCIDFontType0(r pdf.Getter, obj pdf.Object) (*CIDFontType0, error) {
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

	d := &CIDFontType0{}

	// fields in the font dictionary

	d.CMap, err = cmap.Extract(r, fontDict["Encoding"])
	if err != nil {
		return nil, err
	}

	d.Text, err = cmap.ExtractToUnicode(r, fontDict["ToUnicode"])
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

	d.ROS, _ = font.ExtractCIDSystemInfo(r, cidFontDict["CIDSystemInfo"])

	d.Width, err = decodeCompositeWidths(r, cidFontDict["W"])
	if err != nil {
		return nil, err
	}
	if obj, ok := cidFontDict["DW"]; ok {
		dw, err := pdf.GetNumber(r, obj)
		if pdf.IsReadError(err) {
			return nil, err
		}
		d.DefaultWidth = float64(dw)
	} else {
		d.DefaultWidth = DefaultWidthDefault
	}

	dw2, err := decodeVDefault(r, cidFontDict["DW2"])
	if err != nil {
		return nil, err
	}
	d.DefaultVMetrics = dw2
	w2, err := decodeVMetrics(r, cidFontDict["W2"])
	if err != nil {
		return nil, err
	}
	d.VMetrics = w2

	d.FontType = glyphdata.None
	if ref, _ := fdDict["FontFile3"].(pdf.Reference); ref != 0 {
		if stm, _ := pdf.GetStream(r, ref); stm != nil {
			subType, _ := pdf.GetName(r, stm.Dict["Subtype"])
			switch subType {
			case "CIDFontType0C":
				d.FontType = glyphdata.CFF
				d.FontRef = ref
			case "OpenType":
				d.FontType = glyphdata.OpenTypeCFF
				d.FontRef = ref
			}
		}
	}

	d.repair()

	return d, nil
}

// repair can fix some problems with a font dictionary.
// After repair has been run, validate is guaranteed to pass.
func (d *CIDFontType0) repair() {
	if d.Descriptor == nil {
		d.Descriptor = &font.Descriptor{}
	}

	if d.FontRef == 0 {
		d.FontType = glyphdata.None
	} else if d.FontType == glyphdata.None {
		d.FontRef = 0
	}

	m := subset.TagRegexp.FindStringSubmatch(d.Descriptor.FontName)
	if m != nil {
		if d.SubsetTag == "" {
			d.SubsetTag = m[1]
		}
		if d.PostScriptName == "" {
			d.PostScriptName = m[2]
		}
	} else if d.PostScriptName == "" {
		d.PostScriptName = d.Descriptor.FontName
	}
	if d.PostScriptName == "" {
		d.PostScriptName = "Font"
	}
	if !subset.IsValidTag(d.SubsetTag) {
		d.SubsetTag = ""
	}
	if d.FontType == glyphdata.None {
		d.SubsetTag = ""
	}
	d.Descriptor.FontName = subset.Join(d.SubsetTag, d.PostScriptName)

	d.Descriptor.MissingWidth = 0
}

// validate performs some basic checks on the font dictionary.
// This is guaranteed to pass after repair has been run.
func (d *CIDFontType0) validate() error {
	if d.Descriptor == nil {
		return errors.New("missing font descriptor")
	}

	if d.PostScriptName == "" {
		return errors.New("missing PostScript name")
	}
	if d.SubsetTag != "" && !subset.IsValidTag(d.SubsetTag) {
		return fmt.Errorf("invalid subset tag: %s", d.SubsetTag)
	}
	baseFont := subset.Join(d.SubsetTag, d.PostScriptName)
	if d.Descriptor.FontName != baseFont {
		return fmt.Errorf("font name mismatch: %s != %s",
			baseFont, d.Descriptor.FontName)
	}

	if d.SubsetTag != "" && d.FontType == glyphdata.None {
		return errors.New("external font data cannot be subsetted")
	}

	if (d.FontType == glyphdata.None) != (d.FontRef == 0) {
		return errors.New("missing font reference or type")
	}

	if d.Descriptor.MissingWidth != 0 {
		return errors.New("MissingWidth must be 0 for composite fonts")
	}

	return nil
}

func (d *CIDFontType0) WriteToPDF(rm *pdf.ResourceManager, ref pdf.Reference) error {
	w := rm.Out

	switch d.FontType {
	case glyphdata.None:
		// pass
	case glyphdata.CFF:
		if err := pdf.CheckVersion(w, "embedded composite CFF fonts", pdf.V1_3); err != nil {
			return err
		}
	case glyphdata.OpenTypeCFF:
		if err := pdf.CheckVersion(w, "embedded composite OpenType/CFF fonts", pdf.V1_6); err != nil {
			return err
		}
	default:
		return fmt.Errorf("invalid font type %s", d.FontType)
	}

	err := d.validate()
	if err != nil {
		return err
	}

	baseFont := subset.Join(d.SubsetTag, d.PostScriptName)

	cidSystemInfo, err := pdf.ResourceManagerEmbedFunc(rm, font.WriteCIDSystemInfo, d.ROS)
	if err != nil {
		return err
	}

	var encoding pdf.Object
	if d.CMap.IsPredefined() {
		encoding = pdf.Name(d.CMap.Name)
	} else {
		encoding, _, err = pdf.ResourceManagerEmbed(rm, d.CMap)
		if err != nil {
			return err
		}
	}

	var toUni pdf.Object
	if d.Text != nil {
		toUni, _, err = pdf.ResourceManagerEmbed(rm, d.Text)
		if err != nil {
			return err
		}
	}

	cidFontRef := w.Alloc()
	fdRef := w.Alloc()

	fontDict := pdf.Dict{
		"Type":            pdf.Name("Font"),
		"Subtype":         pdf.Name("Type0"),
		"BaseFont":        pdf.Name(baseFont + "-" + d.CMap.Name),
		"Encoding":        encoding,
		"DescendantFonts": pdf.Array{cidFontRef},
		"ToUnicode":       toUni,
	}

	cidFontDict := pdf.Dict{
		"Type":           pdf.Name("Font"),
		"Subtype":        pdf.Name("CIDFontType0"),
		"BaseFont":       pdf.Name(baseFont),
		"CIDSystemInfo":  cidSystemInfo,
		"FontDescriptor": fdRef,
		// we set the glyph width information later
	}

	fdDict := d.Descriptor.AsDict()
	switch d.FontType {
	case glyphdata.CFF, glyphdata.OpenTypeCFF:
		fdDict["FontFile3"] = d.FontRef
	}

	compressedObjects := []pdf.Object{fontDict, cidFontDict, fdDict}
	compressedRefs := []pdf.Reference{ref, cidFontRef, fdRef}

	ww := encodeCompositeWidths(d.Width)
	switch {
	case moreThanTen(ww):
		wwRef := w.Alloc()
		cidFontDict["W"] = wwRef
		compressedObjects = append(compressedObjects, ww)
		compressedRefs = append(compressedRefs, wwRef)
	case len(ww) != 0:
		cidFontDict["W"] = ww
	}
	if d.DefaultWidth != DefaultWidthDefault {
		cidFontDict["DW"] = pdf.Number(d.DefaultWidth)
	}

	if dw2 := encodeVDefault(d.DefaultVMetrics); dw2 != nil {
		cidFontDict["DW2"] = dw2
	}
	if w2 := encodeVMetrics(d.VMetrics); w2 != nil {
		cidFontDict["W2"] = w2
	}

	err = w.WriteCompressed(compressedRefs, compressedObjects...)
	if err != nil {
		return fmt.Errorf("CIDFontType0 dicts: %w", err)
	}

	return nil
}

// ImpliedText returns the default text content for a character identifier.
// This is based on the CID and CID System Info alone, and does not use
// information from the ToUnicode cmap or the font file.
func (d *CIDFontType0) ImpliedText(cid cid.CID) string {
	panic("not implemented")
}

func (d *CIDFontType0) GlyphData() (glyphdata.Type, pdf.Reference) {
	return d.FontType, d.FontRef
}

// MakeFont returns a [font.FromFile] object for the font dictionary.
func (d *CIDFontType0) MakeFont() (font.FromFile, error) {
	var csr charcode.CodeSpaceRange
	csr = append(csr, d.CMap.CodeSpaceRange...)
	if d.Text != nil {
		csr = append(csr, d.Text.CodeSpaceRange...)
	}
	codec, err := charcode.NewCodec(csr)
	if err != nil && d.Text != nil {
		// In case the two code spaces are not compatible, try to use only the
		// code space from the encoding.
		csr = append(csr[:0], d.CMap.CodeSpaceRange...)
		codec, err = charcode.NewCodec(csr)
	}
	if err != nil {
		return nil, err
	}

	s := &t0Font{
		CIDFontType0: d,
		codec:        codec,
		cache:        make(map[charcode.Code]*font.Code),
	}
	return s, nil
}

var (
	_ font.FromFile = (*t0Font)(nil)
)

type t0Font struct {
	*CIDFontType0
	codec *charcode.Codec
	cache map[charcode.Code]*font.Code
}

func (s *t0Font) GetDict() font.Dict {
	return s.CIDFontType0
}

func (s *t0Font) WritingMode() font.WritingMode {
	return s.CMap.WMode
}

func (s *t0Font) Codes(str pdf.String) iter.Seq[*font.Code] {
	return func(yield func(*font.Code) bool) {
		for len(str) > 0 {
			code, k, isValid := s.codec.Decode(str)

			res, seen := s.cache[code]
			if !seen {
				code := str[:k]

				res = &font.Code{}
				if isValid {
					res.CID = s.CMap.LookupCID(code)
					res.Notdef = s.CMap.LookupNotdefCID(code)
				} else {
					res.CID = s.CMap.LookupNotdefCID(code)
				}
				w, ok := s.Width[res.CID]
				if ok {
					res.Width = w
				} else {
					res.Width = s.DefaultWidth
				}

				if s.Text != nil {
					res.Text, _ = s.Text.Lookup(code)
				}
				// TODO(voss): as a fallback, try to get the text from the CID
			}

			str = str[k:]
			if !yield(res) {
				return
			}
		}
	}
}

func init() {
	font.RegisterReader("CIDFontType0", func(r pdf.Getter, obj pdf.Object) (font.Dict, error) {
		return ExtractCIDFontType0(r, obj)
	})
}
