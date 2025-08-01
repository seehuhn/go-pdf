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

	"seehuhn.de/go/postscript/cid"

	"seehuhn.de/go/sfnt/glyph"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/glyphdata"
	"seehuhn.de/go/pdf/font/mapping"
	"seehuhn.de/go/pdf/font/subset"
)

var (
	_ font.Dict = (*CIDFontType2)(nil)
)

// CIDFontType2 holds the information from a Type 2 CIDFont dictionary.
type CIDFontType2 struct {
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
	// above, or the CMap must be one of Identity-H or Identity-V.
	CMap *cmap.File

	// Width (optional) is a map from CID values to glyph widths (in PDF glyph
	// space units).  Only widths which are different from DefaultWidth need to
	// be specified.
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

	// ToUnicode (optional) specifies how character codes are mapped to Unicode
	// strings.  This overrides any mapping implied by the CID values.
	ToUnicode *cmap.ToUnicodeFile

	// CIDToGID (optional, only allowed if the font is embedded) maps CID
	// values to GID values. A nil value for embedded fonts means the
	// identity mapping.
	CIDToGID []glyph.ID

	// FontType gives the type of glyph outline data. Possible values are
	// [glyphdata.TrueType] and [glyphdata.OpenTypeGlyf], or [glyphdata.None]
	// if the font is not embedded.
	FontType glyphdata.Type

	// FontRef is the reference to the glyph outline data in the PDF file.
	// If the font is not embedded, this is 0.
	FontRef pdf.Reference
}

// ReadCIDFontType2 reads a Type 2 CIDFont dictionary from the PDF file.
func ReadCIDFontType2(r pdf.Getter, obj pdf.Object) (*CIDFontType2, error) {
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

	// fields in the font dictionary

	d.CMap, err = cmap.Extract(r, fontDict["Encoding"])
	if err != nil {
		return nil, err
	}

	d.ToUnicode, _ = cmap.ExtractToUnicode(r, fontDict["ToUnicode"])

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

	c2g, err := pdf.Resolve(r, cidFontDict["CIDToGIDMap"])
	if err != nil {
		return nil, err
	}
	switch c2g := c2g.(type) {
	case nil:
		// pass

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

	d.FontType = glyphdata.None
	if ref, _ := fdDict["FontFile2"].(pdf.Reference); ref != 0 {
		d.FontType = glyphdata.TrueType
		d.FontRef = ref
	} else if ref, _ := fdDict["FontFile3"].(pdf.Reference); ref != 0 {
		if stm, _ := pdf.GetStream(r, ref); stm != nil {
			subType, _ := pdf.GetName(r, stm.Dict["Subtype"])
			switch subType {
			case "OpenType":
				d.FontType = glyphdata.OpenTypeGlyf
				d.FontRef = ref
			}
		}
	}

	d.repair()

	if d.FontType == glyphdata.None && !d.CMap.IsPredefined() {
		return nil, errors.New("custom encoding not allowed for external font")
	}

	return d, nil
}

// repair fixes invalid data in the font dictionary.
// After repair() has been called, validate() will return nil.
func (d *CIDFontType2) repair() {
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

	if d.FontType == glyphdata.None {
		d.CIDToGID = nil
	}

	d.Descriptor.MissingWidth = 0

	if d.CMap.Name != "Identity-H" && d.CMap.Name != "Identity-V" ||
		!d.CMap.IsPredefined() {
		if d.ROS.Registry != d.CMap.ROS.Registry ||
			d.ROS.Ordering != d.CMap.ROS.Ordering {
			d.CMap = d.CMap.Clone()
			d.CMap.ROS = d.ROS
		}
	}
}

// validate performs some basic checks on the font dictionary.
// This is guaranteed to pass after repair has been run.
func (d *CIDFontType2) validate() error {
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

	if d.FontType == glyphdata.None && d.CIDToGID != nil {
		return errors.New("CIDToGIDMap not allowed for external font")
	}

	if d.Descriptor.MissingWidth != 0 {
		return errors.New("MissingWidth must be 0 for composite fonts")
	}

	if d.CMap.Name != "Identity-H" && d.CMap.Name != "Identity-V" ||
		!d.CMap.IsPredefined() {
		if d.ROS.Registry != d.CMap.ROS.Registry ||
			d.ROS.Ordering != d.CMap.ROS.Ordering {
			return fmt.Errorf("ROS mismatch: %s %s != %s %s",
				d.ROS.Registry, d.ROS.Ordering,
				d.CMap.ROS.Registry, d.CMap.ROS.Ordering)
		}
	}

	return nil
}

// WriteToPDF adds the font dictionary to a PDF file using the given reference.
// This implements the [font.Dict] interface.
func (d *CIDFontType2) WriteToPDF(rm *pdf.ResourceManager, ref pdf.Reference) error {
	w := rm.Out

	switch d.FontType {
	case glyphdata.None:
		// pass
	case glyphdata.TrueType:
		if err := pdf.CheckVersion(w, "embedded composite TrueType font", pdf.V1_3); err != nil {
			return err
		}
	case glyphdata.OpenTypeGlyf:
		if err := pdf.CheckVersion(w, "embedded composite OpenType/glyf font", pdf.V1_6); err != nil {
			return err
		}
	default:
		return fmt.Errorf("invalid font type %s", d.FontType)
	}

	err := d.validate()
	if err != nil {
		return err
	}

	if d.FontType == glyphdata.None && !d.CMap.IsPredefined() {
		return errors.New("custom encoding not allowed for external font")
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
	if d.ToUnicode != nil {
		toUni, _, err = pdf.ResourceManagerEmbed(rm, d.ToUnicode)
		if err != nil {
			return err
		}
	}

	cidFontRef := w.Alloc()
	fdRef := w.Alloc()

	fontDict := pdf.Dict{
		"Type":            pdf.Name("Font"),
		"Subtype":         pdf.Name("Type0"),
		"BaseFont":        pdf.Name(baseFont),
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
	switch d.FontType {
	case glyphdata.TrueType:
		fdDict["FontFile2"] = d.FontRef
	case glyphdata.OpenTypeGlyf:
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

	var c2gRef pdf.Reference
	if d.CIDToGID != nil {
		c2gRef = w.Alloc()
		cidFontDict["CIDToGIDMap"] = c2gRef
	} else if d.FontType != glyphdata.None {
		cidFontDict["CIDToGIDMap"] = pdf.Name("Identity")
	}

	err = w.WriteCompressed(compressedRefs, compressedObjects...)
	if err != nil {
		return fmt.Errorf("CIDFontType2 dicts: %w", err)
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

func (d *CIDFontType2) Codec() *charcode.Codec {
	return makeCodec(d.CMap, d.ToUnicode)
}

func (d *CIDFontType2) Characters() iter.Seq2[charcode.Code, font.Code] {
	return func(yield func(charcode.Code, font.Code) bool) {
		codec := d.Codec()
		textMap := d.makeTextMap(codec)
		var buf []byte
		for code, cid := range d.CMap.All(codec) {
			buf = codec.AppendCode(buf[:0], code)
			width, ok := d.Width[cid]
			if !ok {
				width = d.DefaultWidth
			}
			info := font.Code{
				CID:            cid,
				Notdef:         d.CMap.LookupNotdefCID(buf),
				Width:          width,
				Text:           textMap[code],
				UseWordSpacing: len(buf) == 1 && buf[0] == 0x20,
			}
			if !yield(code, info) {
				return
			}
		}
	}
}

// FontInfo returns information about the embedded font program.
// The returned value is of type [*FontInfoGlyfEmbedded] or [*FontInfoGlyfExternal].
func (d *CIDFontType2) FontInfo() any {
	if d.FontRef != 0 {
		return &FontInfoGlyfEmbedded{
			PostScriptName: d.PostScriptName,
			Ref:            d.FontRef,
			CIDToGID:       d.CIDToGID,
		}
	}
	return &FontInfoGlyfExternal{
		PostScriptName: d.PostScriptName,
		ROS:            d.ROS,
	}
}

func (d *CIDFontType2) makeTextMap(codec *charcode.Codec) map[charcode.Code]string {
	defaultTextMap, _ := mapping.GetCIDTextMapping(d.ROS.Registry, d.ROS.Ordering)

	textMap := make(map[charcode.Code]string)
	for code, cid := range d.CMap.All(codec) {
		text := defaultTextMap[cid]
		if text != "" {
			textMap[code] = text
		}
	}
	if d.ToUnicode != nil {
		for code, text := range d.ToUnicode.All(codec) {
			if text != "" {
				textMap[code] = text
			}
		}
	}

	return textMap
}

// MakeFont returns a new font object that can be used to typeset text.
// The font is immutable, i.e. no new glyphs can be added and no new codes
// can be defined via the returned font object.
func (d *CIDFontType2) MakeFont() font.FromFile {
	codec := d.Codec()
	textMap := d.makeTextMap(codec)
	return &t2Font{
		CIDFontType2: d,
		codec:        codec,
		text:         textMap,
		cache:        make(map[charcode.Code]*font.Code),
	}
}

var (
	_ font.FromFile = (*t2Font)(nil)
)

type t2Font struct {
	*CIDFontType2
	codec *charcode.Codec
	text  map[charcode.Code]string
	cache map[charcode.Code]*font.Code
}

func (s *t2Font) GetDict() font.Dict {
	return s.CIDFontType2
}

func (s *t2Font) WritingMode() font.WritingMode {
	return s.CMap.WMode
}

func (s *t2Font) Codes(str pdf.String) iter.Seq[*font.Code] {
	return func(yield func(*font.Code) bool) {
		for len(str) > 0 {
			code, k, isValid := s.codec.Decode(str)

			res, seen := s.cache[code]
			if !seen {
				res = &font.Code{}
				codeBytes := str[:k]
				if isValid {
					res.CID = s.CMap.LookupCID(codeBytes)
					res.Notdef = s.CMap.LookupNotdefCID(codeBytes)
				} else {
					res.CID = s.CMap.LookupNotdefCID(codeBytes)
				}
				w, ok := s.Width[res.CID]
				if ok {
					res.Width = w
				} else {
					res.Width = s.DefaultWidth
				}
				res.UseWordSpacing = k == 1 && code == 0x20
				res.Text = s.text[code]
				s.cache[code] = res
			}

			str = str[k:]
			if !yield(res) {
				return
			}
		}
	}
}

func init() {
	registerReader("CIDFontType2", func(r pdf.Getter, obj pdf.Object) (font.Dict, error) {
		return ReadCIDFontType2(r, obj)
	})
}
