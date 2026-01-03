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

	"seehuhn.de/go/sfnt/glyph"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/glyphdata"
	"seehuhn.de/go/pdf/font/mapping"
	"seehuhn.de/go/pdf/font/subset"
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

	// FontFile contains the embedded font file stream.
	// If the font is not embedded, this is nil.
	FontFile *glyphdata.Stream
}

var _ Dict = (*CIDFontType2)(nil)

// validate performs some basic checks on the font dictionary.
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

	if d.SubsetTag != "" && d.FontFile == nil {
		return errors.New("external font data cannot be subsetted")
	}

	if d.FontFile == nil && d.CIDToGID != nil {
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

// Embed adds the font dictionary to a PDF file.
// This implements the [Dict] interface.
func (d *CIDFontType2) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) {
	ref := rm.AllocSelf()
	w := rm.Out()

	if d.FontFile != nil {
		switch d.FontFile.Type {
		case glyphdata.TrueType:
			if err := pdf.CheckVersion(w, "embedded composite TrueType font", pdf.V1_3); err != nil {
				return nil, err
			}
		case glyphdata.OpenTypeGlyf:
			if err := pdf.CheckVersion(w, "embedded composite OpenType/glyf font", pdf.V1_6); err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("invalid font type %s", d.FontFile.Type)
		}
	}

	err := d.validate()
	if err != nil {
		return nil, err
	}

	if d.FontFile == nil && !d.CMap.IsPredefined() {
		return nil, errors.New("custom encoding not allowed for external font")
	}

	baseFont := subset.Join(d.SubsetTag, d.PostScriptName)

	cidSystemInfo, err := pdf.EmbedHelperEmbedFunc(rm, font.WriteCIDSystemInfo, d.ROS)
	if err != nil {
		return nil, err
	}

	var encoding pdf.Object
	if d.CMap.IsPredefined() {
		encoding = pdf.Name(d.CMap.Name)
	} else {
		encoding, err = rm.Embed(d.CMap)
		if err != nil {
			return nil, err
		}
	}

	var toUni pdf.Object
	if d.ToUnicode != nil {
		toUni, err = rm.Embed(d.ToUnicode)
		if err != nil {
			return nil, err
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
	if d.FontFile != nil {
		fontFileRef, err := rm.Embed(d.FontFile)
		if err != nil {
			return nil, err
		}
		switch d.FontFile.Type {
		case glyphdata.TrueType:
			fdDict["FontFile2"] = fontFileRef
		case glyphdata.OpenTypeGlyf:
			fdDict["FontFile3"] = fontFileRef
		}
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
	} else if d.FontFile != nil {
		cidFontDict["CIDToGIDMap"] = pdf.Name("Identity")
	}

	err = w.WriteCompressed(compressedRefs, compressedObjects...)
	if err != nil {
		return nil, fmt.Errorf("CIDFontType2 dicts: %w", err)
	}

	if c2gRef != 0 {
		c2gStm, err := w.OpenStream(c2gRef, nil,
			pdf.FilterCompress{
				"Predictor": pdf.Integer(12),
				"Columns":   pdf.Integer(2),
			})
		if err != nil {
			return nil, err
		}
		cid2gid := make([]byte, 2*len(d.CIDToGID))
		for cid, gid := range d.CIDToGID {
			cid2gid[2*cid] = byte(gid >> 8)
			cid2gid[2*cid+1] = byte(gid)
		}
		_, err = c2gStm.Write(cid2gid)
		if err != nil {
			return nil, err
		}
		err = c2gStm.Close()
		if err != nil {
			return nil, err
		}
	}

	return ref, nil
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
				Width:          width / 1000,
				Text:           textMap[code],
				UseWordSpacing: len(buf) == 1 && buf[0] == 0x20,
			}
			if !yield(code, info) {
				return
			}
		}
	}
}

// FontInfo returns information about the embedded font file.
// The returned value is of type [*FontInfoGlyfEmbedded] or [*FontInfoGlyfExternal].
func (d *CIDFontType2) FontInfo() any {
	if d.FontFile != nil {
		return &FontInfoGlyfEmbedded{
			PostScriptName: d.PostScriptName,
			FontFile:       d.FontFile,
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
func (d *CIDFontType2) MakeFont() font.Instance {
	codec := d.Codec()
	textMap := d.makeTextMap(codec)
	return &t2Font{
		CIDFontType2: d,
		codec:        codec,
		text:         textMap,
		cache:        make(map[charcode.Code]*font.Code),
	}
}

type t2Font struct {
	*CIDFontType2
	codec *charcode.Codec
	text  map[charcode.Code]string
	cache map[charcode.Code]*font.Code
}

var _ font.Instance = (*t2Font)(nil)

func (f *t2Font) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) {
	ref, err := rm.Embed(f.CIDFontType2)
	if err != nil {
		return nil, err
	}
	return ref, nil
}

// GetName returns a human-readable name for the font.
func (f *t2Font) PostScriptName() string {
	return f.CIDFontType2.PostScriptName
}

func (f *t2Font) GetDict() Dict {
	return f.CIDFontType2
}

// GetCodec returns the codec for the encoding used by this font.
func (f *t2Font) GetCodec() *charcode.Codec {
	return f.CIDFontType2.Codec()
}

func (f *t2Font) WritingMode() font.WritingMode {
	return f.CMap.WMode
}

func (f *t2Font) Codes(str pdf.String) iter.Seq[*font.Code] {
	return func(yield func(*font.Code) bool) {
		for len(str) > 0 {
			code, k, isValid := f.codec.Decode(str)

			res, seen := f.cache[code]
			if !seen {
				res = &font.Code{}
				codeBytes := str[:k]
				if isValid {
					res.CID = f.CMap.LookupCID(codeBytes)
					res.Notdef = f.CMap.LookupNotdefCID(codeBytes)
				} else {
					res.CID = f.CMap.LookupNotdefCID(codeBytes)
				}
				w, ok := f.Width[res.CID]
				if ok {
					res.Width = w / 1000
				} else {
					res.Width = f.DefaultWidth / 1000
				}
				res.UseWordSpacing = k == 1 && code == 0x20
				res.Text = f.text[code]
				f.cache[code] = res
			}

			str = str[k:]
			if !yield(res) {
				return
			}
		}
	}
}
