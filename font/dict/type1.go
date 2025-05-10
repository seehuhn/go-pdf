// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2024  Jochen Voss <voss@seehuhn.de>
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

	"seehuhn.de/go/sfnt/os2"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/encoding"
	"seehuhn.de/go/pdf/font/glyphdata"
	"seehuhn.de/go/pdf/font/subset"
	"seehuhn.de/go/pdf/internal/stdmtx"
)

var (
	_ font.Dict = (*Type1)(nil)
)

// Type1 holds the information from a Type 1 font dictionary.
type Type1 struct {
	// PostScriptName is the PostScript name of the font
	// (without any subset tag).
	PostScriptName string

	// SubsetTag can be set to indicate that the font has been subsetted.
	// If non-empty, the value must be a sequence of 6 uppercase letters.
	SubsetTag string

	// Name is deprecated and should be left empty.
	// Only used in PDF 1.0 where it was the name used to reference the font
	// from within content streams.
	Name pdf.Name

	// Descriptor is the font descriptor.
	Descriptor *font.Descriptor

	// Encoding maps character codes to glyph names.
	Encoding encoding.Simple

	// Width contains the glyph widths for all character codes
	// (PDF glyph space units).
	Width [256]float64

	// ToUnicode (optional) specifies how character codes are mapped to Unicode
	// strings.  This overrides the mapping implied by the glyph names.
	ToUnicode *cmap.ToUnicodeFile

	// FontType gives the type of glyph outline data. Possible values are
	// [glyphdata.Type1], [glyphdata.CFFSimple], and [glyphdata.OpenTypeCFFSimple],
	// or [glyphdata.None] if the font is not embedded.
	FontType glyphdata.Type

	// FontRef is the reference to the glyph outline data in the PDF file,
	// if the font is embedded.
	FontRef pdf.Reference
}

// ExtractType1 reads a Type 1 font dictionary from a PDF file.
func ExtractType1(r pdf.Getter, obj pdf.Object) (*Type1, error) {
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
	if subtype != "" && subtype != "Type1" {
		return nil, pdf.Errorf("expected font subtype Type1, got %q", subtype)
	}

	d := &Type1{}

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

	// StdInfo will be non-nil, if the PostScript name indicates one of the
	// standard 14 fonts. In this case, we use the corresponding metrics as
	// default values, in case they are missing from the font dictionary.
	stdInfo := stdmtx.Metrics[d.PostScriptName]

	fdDict, err := pdf.GetDictTyped(r, fontDict["FontDescriptor"], "FontDescriptor")
	if pdf.IsReadError(err) {
		return nil, err
	}
	fd, _ := font.ExtractDescriptor(r, fdDict)
	if fd == nil && stdInfo != nil {
		fd = &font.Descriptor{
			FontName:     d.PostScriptName,
			FontFamily:   stdInfo.FontFamily,
			FontStretch:  os2.WidthNormal,
			FontWeight:   stdInfo.FontWeight,
			IsFixedPitch: stdInfo.IsFixedPitch,
			IsSerif:      stdInfo.IsSerif,
			IsItalic:     stdInfo.ItalicAngle != 0,
			IsSymbolic:   stdInfo.IsSymbolic,
			FontBBox:     stdInfo.FontBBox,
			ItalicAngle:  stdInfo.ItalicAngle,
			Ascent:       stdInfo.Ascent,
			Descent:      stdInfo.Descent,
			CapHeight:    stdInfo.CapHeight,
			XHeight:      stdInfo.XHeight,
			StemV:        stdInfo.StemV,
			StemH:        stdInfo.StemH,
			MissingWidth: stdInfo.Width[".notdef"],
		}
	}
	d.Descriptor = fd

	d.FontType = glyphdata.None
	if ref, _ := fdDict["FontFile"].(pdf.Reference); ref != 0 {
		d.FontType = glyphdata.Type1
		d.FontRef = ref
	} else if ref, _ := fdDict["FontFile3"].(pdf.Reference); ref != 0 {
		if stm, _ := pdf.GetStream(r, ref); stm != nil {
			subType, _ := pdf.GetName(r, stm.Dict["Subtype"])
			switch subType {
			case "Type1C":
				d.FontType = glyphdata.CFFSimple
				d.FontRef = ref
			case "OpenType":
				d.FontType = glyphdata.OpenTypeCFFSimple
				d.FontRef = ref
			}
		}
	}

	isNonSymbolic := fd != nil && !fd.IsSymbolic
	isExternal := d.FontRef == 0
	nonSymbolicExt := isNonSymbolic && isExternal
	enc, err := encoding.ExtractType1(r, fontDict["Encoding"], nonSymbolicExt)
	if err != nil {
		return nil, err
	}
	d.Encoding = enc

	var defaultWidth float64
	if fd != nil {
		defaultWidth = fd.MissingWidth
	}
	if !getSimpleWidths(d.Width[:], r, fontDict, defaultWidth) && stdInfo != nil {
		for c := range 256 {
			w, ok := stdInfo.Width[enc(byte(c))]
			if !ok {
				w = stdInfo.Width[".notdef"]
			}
			d.Width[c] = w
		}
	}

	toUnicode, err := cmap.ExtractToUnicode(r, fontDict["ToUnicode"])
	if pdf.IsReadError(err) {
		return nil, err
	}
	d.ToUnicode = toUnicode

	d.repair(r)

	return d, nil
}

// repair fixes invalid data in the font dictionary.
// After repair() has been called, validate() will return nil.
func (d *Type1) repair(r pdf.Getter) {
	if d.Descriptor == nil {
		d.Descriptor = &font.Descriptor{}
	}

	if v := pdf.GetVersion(r); v == pdf.V1_0 {
		if d.Name == "" {
			d.Name = "Font"
		}
	} else if v >= pdf.V2_0 {
		d.Name = ""
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
}

// validate performs some basic checks on the font dictionary.
// This is guaranteed to pass after repair has been run.
func (d *Type1) validate(w *pdf.Writer) error {
	if d.Descriptor == nil {
		return errors.New("missing font descriptor")
	}

	if v := pdf.GetVersion(w); v == pdf.V1_0 {
		if d.Name == "" {
			return errors.New("missing font name")
		}
	} else if v >= pdf.V2_0 {
		if d.Name != "" {
			return errors.New("unexpected font name")
		}
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

	return nil
}

func (d *Type1) WriteToPDF(rm *pdf.ResourceManager, ref pdf.Reference) error {
	w := rm.Out

	switch d.FontType {
	case glyphdata.None:
		// pass
	case glyphdata.Type1:
		// always ok
	case glyphdata.CFFSimple:
		if err := pdf.CheckVersion(w, "embedded CFF font", pdf.V1_2); err != nil {
			return err
		}
	case glyphdata.OpenTypeCFFSimple:
		if err := pdf.CheckVersion(w, "embedded OpenType/CFF font", pdf.V1_6); err != nil {
			return err
		}
	default:
		return fmt.Errorf("invalid font type %s", d.FontType)
	}

	err := d.validate(w)
	if err != nil {
		return err
	}

	baseFont := subset.Join(d.SubsetTag, d.PostScriptName)
	fontDict := pdf.Dict{
		"Type":     pdf.Name("Font"),
		"Subtype":  pdf.Name("Type1"),
		"BaseFont": pdf.Name(baseFont),
	}
	if d.Name != "" {
		fontDict["Name"] = d.Name
	}

	stdInfo := stdmtx.Metrics[d.PostScriptName]
	trimFontDict := ((d.FontType == glyphdata.None) &&
		stdInfo != nil &&
		w.GetOptions().HasAny(pdf.OptTrimStandardFonts) &&
		widthsAreCompatible(d.Width[:], d.Encoding, stdInfo) &&
		fontDescriptorIsCompatible(d.Descriptor, stdInfo))

	isNonSymbolic := !d.Descriptor.IsSymbolic
	isExternal := d.FontRef == 0
	baseIsStd := isNonSymbolic && isExternal
	if trimFontDict {
		// Don't make any assumptions about the base encoding for the
		// standard fonts.
		baseIsStd = false
	}
	encodingObj, err := d.Encoding.AsPDFType1(baseIsStd, w.GetOptions())
	if err != nil {
		return err
	}
	if encodingObj != nil {
		fontDict["Encoding"] = encodingObj
	}

	compressedObjects := []pdf.Object{fontDict}
	compressedRefs := []pdf.Reference{ref}

	if !trimFontDict {
		fdRef := w.Alloc()
		fdDict := d.Descriptor.AsDict()
		switch d.FontType {
		case glyphdata.Type1:
			fdDict["FontFile"] = d.FontRef
		case glyphdata.CFFSimple, glyphdata.OpenTypeCFFSimple:
			fdDict["FontFile3"] = d.FontRef
		}
		fontDict["FontDescriptor"] = fdRef
		compressedObjects = append(compressedObjects, fdDict)
		compressedRefs = append(compressedRefs, fdRef)

		defaultWidth := d.Descriptor.MissingWidth
		oo, rr := setSimpleWidths(w, fontDict, d.Width[:], d.Encoding, defaultWidth)
		compressedObjects = append(compressedObjects, oo...)
		compressedRefs = append(compressedRefs, rr...)
	}

	if d.ToUnicode != nil {
		ref, _, err := pdf.ResourceManagerEmbed(rm, d.ToUnicode)
		if err != nil {
			return err
		}
		fontDict["ToUnicode"] = ref
	}

	err = w.WriteCompressed(compressedRefs, compressedObjects...)
	if err != nil {
		return fmt.Errorf("Type 1 font dict: %w", err)
	}

	return nil
}

// DefaultTextMapping returns the default text content for a character identifier.
// This is based on the glyph name alone, and does not use information from the
// ToUnicode cmap or the font file.
//
// CID values are taken to be the character code, plus one.
func (d *Type1) DefaultTextMapping() map[cid.CID]string {
	m := make(map[cid.CID]string)
	for code := range 256 {
		glyphName := d.Encoding(byte(code))
		s := names.ToUnicode(glyphName, d.PostScriptName)
		if s != "" {
			cid := cid.CID(code) + 1
			m[cid] = s
		}
	}
	return m
}

// TextMapping returns the mapping from character identifiers to text
// content. This uses information from the ToUnicode cmap, if available,
// with glyph names as a fallback.
//
// CID values are taken to be the character code, plus one.
func (d *Type1) TextMapping() map[cid.CID]string {
	implied := d.DefaultTextMapping()
	if d.ToUnicode == nil {
		return implied
	}

	codec, _ := charcode.NewCodec(charcode.Simple)
	for code, s := range d.ToUnicode.All(codec) {
		cid := cid.CID(code) + 1
		implied[cid] = s
	}
	return implied
}

func (d *Type1) GlyphData() (glyphdata.Type, pdf.Reference) {
	return d.FontType, d.FontRef
}

// MakeFont returns a new font object that can be used to typeset text.
// The font is immutable, i.e. no new glyphs can be added and no new codes
// can be defined via the returned font object.
func (d *Type1) MakeFont() (font.FromFile, error) {
	return t1Font{d}, nil
}

var (
	_ font.FromFile = t1Font{}
)

type t1Font struct {
	Dict *Type1
}

func (f t1Font) GetDict() font.Dict {
	return f.Dict
}

func (t1Font) WritingMode() font.WritingMode {
	return font.Horizontal
}

func (f t1Font) Codes(s pdf.String) iter.Seq[*font.Code] {
	return func(yield func(*font.Code) bool) {
		var code font.Code
		for _, c := range s {
			if f.Dict.Encoding(c) == "" {
				code.CID = 0
			} else {
				code.CID = cid.CID(c) + 1
			}
			code.Width = f.Dict.Width[c]
			code.UseWordSpacing = (c == 0x20)
			if !yield(&code) {
				return
			}
		}
	}
}

func init() {
	font.RegisterReader("Type1", func(r pdf.Getter, obj pdf.Object) (font.Dict, error) {
		return ExtractType1(r, obj)
	})
}
