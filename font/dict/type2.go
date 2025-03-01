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
	"io"
	"iter"
	"math"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/glyphdata"
	"seehuhn.de/go/pdf/font/subset"
	"seehuhn.de/go/sfnt/glyph"
)

// CIDFontType2 holds the information from the font dictionary and CIDFont
// dictionary of a Type 2 (TrueType-based) CIDFont.
type CIDFontType2 struct {
	// Ref is the reference to the font dictionary in the PDF file.
	Ref pdf.Reference

	// PostScriptName is the PostScript name of the font
	// (without any subset tag).
	PostScriptName string

	// SubsetTag can be set to indicate that the font has been subsetted.
	// If non-empty, the value must be a sequence of 6 uppercase letters.
	SubsetTag string

	// Descriptor is the font descriptor.
	Descriptor *font.Descriptor

	// ROS describes the character collection covered by the font.
	ROS *cmap.CIDSystemInfo

	// Encoding specifies how character codes are mapped to CID values.
	//
	// The Encoding.ROS field must either be compatible with the ROS field
	// above, or the cmap must be one of Identity-H or Identity-V.
	Encoding *cmap.File

	// Width is a map from CID values to glyph widths (in PDF glyph space units).
	Width map[cmap.CID]float64

	// DefaultWidth is the glyph width for CID values not in the Width map
	// (in PDF glyph space units).
	DefaultWidth float64

	// TODO(voss): vertical glyph metrics

	// Text specifies how character codes are mapped to Unicode strings.
	Text *cmap.ToUnicodeFile

	// CIDToGID (optional, only used if GetFont is non-nil) maps CID values to
	// GID values. If this is missing, the GID values are the same as the CID
	// values.
	CIDToGID []glyph.ID

	// FontType gives the type of glyph outline data. Possible values are
	// [glyphdata.TrueType] and [glyphdata.OpenTypeGlyf], or [glyphdata.None]
	// if the font is not embedded.
	FontType glyphdata.Type

	// FontRef is the reference to the glyph outline data in the PDF file,
	// if the font is embedded.
	FontRef pdf.Reference
}

// ExtractCIDFontType2 extracts the information from a Type 0 CIDFont from a PDF file.
func ExtractCIDFontType2(r pdf.Getter, obj pdf.Object) (*CIDFontType2, error) {
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
	} else if subtype != "CIDFontType2" {
		return nil, &pdf.MalformedFileError{
			Err: fmt.Errorf("expected CIDFontType2, got %q", subtype),
		}
	}

	d := &CIDFontType2{}
	d.Ref, _ = obj.(pdf.Reference)

	// fields in the font dictionary

	d.Encoding, err = cmap.Extract(r, fontDict["Encoding"])
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
	if d.Descriptor == nil { // only possible for invalid PDF files
		d.Descriptor = &font.Descriptor{
			FontName: d.PostScriptName,
		}
	}

	d.Width, err = decodeComposite(r, cidFontDict["W"])
	if err != nil {
		return nil, err
	}
	dw, err := pdf.GetNumber(r, cidFontDict["DW"])
	if pdf.IsReadError(err) {
		return nil, err
	}
	d.DefaultWidth = float64(dw)

	c2g, err := pdf.Resolve(r, cidFontDict["CIDToGIDMap"])
	if err != nil {
		return nil, err
	}
	switch c2g := c2g.(type) {
	case pdf.Name:
		if c2g != "Identity" {
			return nil, &pdf.MalformedFileError{
				Err: fmt.Errorf("unsupported CIDToGIDMap: %q", c2g),
			}
		}

	case *pdf.Stream:
		in, err := pdf.DecodeStream(r, c2g, 0)
		if err != nil {
			return nil, err
		}
		cid2gidData, err := io.ReadAll(in)
		if err == nil && len(cid2gidData)%2 != 0 {
			err = &pdf.MalformedFileError{
				Err: errors.New("odd length CIDToGIDMap"),
			}
		}
		if err != nil {
			return nil, err
		}
		d.CIDToGID = make([]glyph.ID, len(cid2gidData)/2)
		for i := range d.CIDToGID {
			d.CIDToGID[i] = glyph.ID(cid2gidData[2*i])<<8 | glyph.ID(cid2gidData[2*i+1])
		}

	default:
		return nil, &pdf.MalformedFileError{
			Err: errors.New("missing/invalid CIDToGIDMap"),
		}
	}

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
			default:
				d.FontType = glyphdata.None
			}
		}
	}

	return d, nil
}

// WriteToPDF embeds the font data in the PDF file.
func (d *CIDFontType2) WriteToPDF(rm *pdf.ResourceManager) error {
	w := rm.Out

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
		if err := pdf.CheckVersion(rm.Out, "composite TrueType font", pdf.V1_3); err != nil {
			return err
		}
	case glyphdata.OpenTypeGlyf:
		if err := pdf.CheckVersion(rm.Out, "composite OpenType/glyf font", pdf.V1_6); err != nil {
			return err
		}
	default:
		return fmt.Errorf("invalid font type %s", d.FontType)
	}
	if d.FontRef == 0 && !d.Encoding.IsPredefined() {
		// If the [...] TrueType font program is not embedded in the PDF file,
		// the Encoding entry shall be a predefined CMap name.
		return fmt.Errorf("invalid CMap for external %s font", d.FontType)
	}
	if d.SubsetTag != "" && !subset.IsValidTag(d.SubsetTag) {
		return fmt.Errorf("invalid subset tag: %s", d.SubsetTag)
	}

	baseFont := pdf.Name(subset.Join(d.SubsetTag, d.PostScriptName))

	cidSystemInfo, _, err := pdf.ResourceManagerEmbed(rm, d.ROS)
	if err != nil {
		return err
	}

	var encoding pdf.Object
	if d.Encoding.IsPredefined() {
		encoding = pdf.Name(d.Encoding.Name)
	} else {
		encoding, _, err = pdf.ResourceManagerEmbed(rm, d.Encoding)
		if err != nil {
			return err
		}
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
		"BaseFont":        baseFont,
		"Encoding":        encoding,
		"DescendantFonts": pdf.Array{cidFontRef},
		"ToUnicode":       toUni,
	}

	cidFontDict := pdf.Dict{
		"Type":           pdf.Name("Font"),
		"Subtype":        pdf.Name("CIDFontType2"),
		"BaseFont":       pdf.Name(baseFont),
		"CIDSystemInfo":  cidSystemInfo,
		"FontDescriptor": fdRef,
		// we set the glyph width information later
		// we set CIDToGIDMap later
	}

	fdDict := d.Descriptor.AsDict()
	fdDict["FontName"] = cidFontDict["BaseFont"]
	switch d.FontType {
	case glyphdata.TrueType:
		fdDict["FontFile2"] = d.FontRef
	case glyphdata.OpenTypeGlyf:
		fdDict["FontFile3"] = d.FontRef
	}

	compressedObjects := []pdf.Object{fontDict, cidFontDict, fdDict}
	compressedRefs := []pdf.Reference{fontDictRef, cidFontRef, fdRef}

	ww := encodeComposite(d.Width, d.DefaultWidth)
	switch {
	case moreThanTen(ww):
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

	var c2gRef pdf.Reference
	if len(d.CIDToGID) != 0 {
		c2gRef = w.Alloc()
		cidFontDict["CIDToGIDMap"] = c2gRef
	} else if d.FontRef != 0 {
		cidFontDict["CIDToGIDMap"] = pdf.Name("Identity")
	}

	err = w.WriteCompressed(compressedRefs, compressedObjects...)
	if err != nil {
		return pdf.Wrap(err, "composite OpenType/CFF font dicts")
	}

	if c2gRef != 0 {
		c2gStm, err := w.OpenStream(c2gRef, nil,
			pdf.FilterCompress{
				"Predictor": pdf.Integer(12),
				"Columns":   pdf.Integer(2),
			})
		if err != nil {
			return err
		}
		cid2gid := make([]byte, 2*len(d.CIDToGID))
		for cid, gid := range d.CIDToGID {
			cid2gid[2*cid] = byte(gid >> 8)
			cid2gid[2*cid+1] = byte(gid)
		}
		_, err = c2gStm.Write(cid2gid)
		if err != nil {
			return err
		}
		err = c2gStm.Close()
		if err != nil {
			return err
		}
	}

	return nil
}

func (d *CIDFontType2) WritingMode() font.WritingMode {
	return d.Encoding.WMode
}

// GetScanner returns a font.Scanner for the font.
func (d *CIDFontType2) GetScanner() (font.Scanner, error) {
	var csr charcode.CodeSpaceRange
	csr = append(csr, d.Encoding.CodeSpaceRange...)
	csr = append(csr, d.Text.CodeSpaceRange...)
	codec, err := charcode.NewCodec(csr)
	if err != nil {
		// In case the two code spaces are not compatible, try to use only the
		// code space from the encoding.
		csr = append(csr[:0], d.Encoding.CodeSpaceRange...)
		codec, err = charcode.NewCodec(csr)
	}
	if err != nil {
		return nil, err
	}

	s := &type2Scanner{
		CIDFontType2: d,
		codec:        codec,
		cache:        make(map[charcode.Code]*font.Code),
	}
	return s, nil
}

type type2Scanner struct {
	*CIDFontType2
	codec *charcode.Codec
	cache map[charcode.Code]*font.Code
}

func (s *type2Scanner) Codes(str pdf.String) iter.Seq[*font.Code] {
	return func(yield func(*font.Code) bool) {
		for len(str) > 0 {
			code, k, isValid := s.codec.Decode(str)

			res, seen := s.cache[code]
			if !seen {
				code := str[:k]

				res = &font.Code{}
				if isValid {
					res.CID = s.Encoding.LookupCID(code)
					res.Notdef = s.Encoding.LookupNotdefCID(code)
				} else {
					res.CID = s.Encoding.LookupNotdefCID(code)
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

func (d *type2Scanner) DecodeWidth(s pdf.String) (float64, int) {
	var w float64
	for c := range d.Codes(s) {
		w += c.Width
	}
	return w / 1000, len(s)
}

func init() {
	font.RegisterReader("CIDFontType2", func(r pdf.Getter, obj pdf.Object) (font.FromFile, error) {
		return ExtractCIDFontType2(r, obj)
	})
}
