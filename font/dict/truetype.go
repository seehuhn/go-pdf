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
	_ font.Dict = (*TrueType)(nil)
)

// TrueType holds the information from a TrueType font dictionary.
type TrueType struct {
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
	// (in PDF glyph space units).
	Width [256]float64

	// ToUnicode (optional) specifies how character codes are mapped to Unicode
	// strings.  This overrides the mapping implied by the glyph names.
	ToUnicode *cmap.ToUnicodeFile

	// FontFile contains the embedded font file stream.
	// If the font is not embedded, this is nil.
	FontFile *glyphdata.Stream
}

// DecodeTrueType reads a TrueType font dictionary from a PDF file.
func DecodeTrueType(x *pdf.Extractor, obj pdf.Object) (*TrueType, error) {
	fontDict, err := pdf.GetDictTyped(x.R, obj, "Font")
	if err != nil {
		return nil, err
	} else if fontDict == nil {
		return nil, &pdf.MalformedFileError{
			Err: errors.New("missing font dictionary"),
		}
	}
	subtype, err := pdf.GetName(x.R, fontDict["Subtype"])
	if err != nil {
		return nil, err
	}
	if subtype != "" && subtype != "TrueType" {
		return nil, pdf.Errorf("expected font subtype TrueType, got %q", subtype)
	}

	d := &TrueType{}

	baseFont, err := pdf.GetName(x.R, fontDict["BaseFont"])
	if err != nil {
		return nil, err
	}
	if m := subset.TagRegexp.FindStringSubmatch(string(baseFont)); m != nil {
		d.PostScriptName = m[2]
		d.SubsetTag = m[1]
	} else {
		d.PostScriptName = string(baseFont)
	}

	d.Name, _ = pdf.GetName(x.R, fontDict["Name"])

	// StdInfo will be non-nil, if the PostScript name indicates one of the
	// standard 14 fonts. In this case, we use the corresponding metrics as
	// default values, in case they are missing from the font dictionary.
	stdInfo := stdmtx.Metrics[d.PostScriptName]

	fdDict, err := pdf.GetDictTyped(x.R, fontDict["FontDescriptor"], "FontDescriptor")
	if pdf.IsReadError(err) {
		return nil, err
	}
	fd, _ := font.ExtractDescriptor(x.R, fdDict)
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

	for _, key := range []pdf.Name{"FontFile2", "FontFile3"} {
		if fontFile, err := pdf.ExtractorGetOptional(x, fdDict[key],
			func(x *pdf.Extractor, obj pdf.Object) (*glyphdata.Stream, error) {
				return glyphdata.ExtractStream(x, obj, "TrueType", key)
			}); err != nil {
			return nil, err
		} else if fontFile != nil {
			d.FontFile = fontFile
			break
		}
	}

	isNonSymbolic := fd != nil && !fd.IsSymbolic
	isExternal := d.FontFile == nil
	nonSymbolicExt := isNonSymbolic && isExternal
	enc, err := encoding.ExtractType1(x.R, fontDict["Encoding"], nonSymbolicExt)
	if err != nil {
		return nil, err
	}
	d.Encoding = enc

	var defaultWidth float64
	if fd != nil {
		defaultWidth = fd.MissingWidth
	}
	if !getSimpleWidths(d.Width[:], x.R, fontDict, defaultWidth) && stdInfo != nil {
		for c := range 256 {
			w, ok := stdInfo.Width[enc(byte(c))]
			if !ok {
				w = stdInfo.Width[".notdef"]
			}
			d.Width[c] = w
		}
	}

	d.ToUnicode, _ = cmap.ExtractToUnicode(x.R, fontDict["ToUnicode"])

	d.repair(x.R)

	return d, nil
}

// repair fixes invalid data in the font dictionary.
// After repair() has been called, validate() will return nil.
func (d *TrueType) repair(r pdf.Getter) {
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
	if d.FontFile == nil {
		d.SubsetTag = ""
	}
	d.Descriptor.FontName = subset.Join(d.SubsetTag, d.PostScriptName)
}

// validate performs some basic checks on the font dictionary.
// This is guaranteed to pass after repair has been run.
func (d *TrueType) validate(w *pdf.Writer) error {
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

	if d.SubsetTag != "" && d.FontFile == nil {
		return errors.New("external font data cannot be subsetted")
	}

	return nil
}

// WriteToPDF adds the font dictionary to a PDF file using the given reference.
// This implements the [font.Dict] interface.
func (d *TrueType) WriteToPDF(rm *pdf.ResourceManager, ref pdf.Reference) error {
	w := rm.Out

	if d.FontFile != nil {
		switch d.FontFile.Type {
		case glyphdata.TrueType:
			if err := pdf.CheckVersion(rm.Out, "embedded TrueType font", pdf.V1_1); err != nil {
				return err
			}
		case glyphdata.OpenTypeGlyf:
			if err := pdf.CheckVersion(rm.Out, "embedded OpenType/glyf font", pdf.V1_6); err != nil {
				return err
			}
		default:
			return fmt.Errorf("invalid font type %s", d.FontFile.Type)
		}
	}

	err := d.validate(w)
	if err != nil {
		return err
	}

	baseFont := subset.Join(d.SubsetTag, d.PostScriptName)
	fontDict := pdf.Dict{
		"Type":     pdf.Name("Font"),
		"Subtype":  pdf.Name("TrueType"),
		"BaseFont": pdf.Name(baseFont),
	}
	if d.Name != "" {
		fontDict["Name"] = d.Name
	}

	isNonSymbolic := !d.Descriptor.IsSymbolic
	isExternal := d.FontFile == nil
	// TODO(voss): implement TrueType constraints
	encodingObj, err := d.Encoding.AsPDFType1(isNonSymbolic && isExternal, w.GetOptions())
	if err != nil {
		return err
	}
	if encodingObj != nil {
		fontDict["Encoding"] = encodingObj
	}

	compressedObjects := []pdf.Object{fontDict}
	compressedRefs := []pdf.Reference{ref}

	fdRef := w.Alloc()
	fdDict := d.Descriptor.AsDict()
	if d.FontFile != nil {
		fontFileRef, _, err := pdf.ResourceManagerEmbed(rm, d.FontFile)
		if err != nil {
			return err
		}
		switch d.FontFile.Type {
		case glyphdata.TrueType:
			fdDict["FontFile2"] = fontFileRef
		case glyphdata.OpenTypeGlyf:
			fdDict["FontFile3"] = fontFileRef
		}
	}
	fontDict["FontDescriptor"] = fdRef
	compressedObjects = append(compressedObjects, fdDict)
	compressedRefs = append(compressedRefs, fdRef)

	defaultWidth := d.Descriptor.MissingWidth
	oo, rr := setSimpleWidths(w, fontDict, d.Width[:], d.Encoding, defaultWidth)
	compressedObjects = append(compressedObjects, oo...)
	compressedRefs = append(compressedRefs, rr...)

	if d.ToUnicode != nil {
		ref, _, err := pdf.ResourceManagerEmbed(rm, d.ToUnicode)
		if err != nil {
			return err
		}
		fontDict["ToUnicode"] = ref
	}

	err = w.WriteCompressed(compressedRefs, compressedObjects...)
	if err != nil {
		return fmt.Errorf("TrueType font dict: %w", err)
	}

	return nil
}

func (d *TrueType) Codec() *charcode.Codec {
	codec, _ := charcode.NewCodec(charcode.Simple)
	return codec
}

func (d *TrueType) Characters() iter.Seq2[charcode.Code, font.Code] {
	return func(yield func(charcode.Code, font.Code) bool) {
		textMap := simpleTextMap(d.PostScriptName, d.Encoding, d.ToUnicode)
		for c := range 256 {
			code := byte(c)
			var info font.Code
			if d.Encoding(code) != "" {
				info = font.Code{
					CID:            cid.CID(code) + 1,
					Width:          d.Width[code],
					Text:           textMap[code],
					UseWordSpacing: code == 0x20,
				}
			} else {
				continue
			}
			if !yield(charcode.Code(code), info) {
				return
			}
		}
	}
}

// FontInfo returns information about the embedded font file.
// The returned value is of type [*FontInfoSimple].
func (d *TrueType) FontInfo() any {
	return &FontInfoSimple{
		PostScriptName: d.PostScriptName,
		FontFile:       d.FontFile,
		Encoding:       d.Encoding,
		IsSymbolic:     d.Descriptor.IsSymbolic,
	}
}

// MakeFont returns a new font object that can be used to typeset text.
// The font is immutable, i.e. no new glyphs can be added and no new codes
// can be defined via the returned font object.
func (d *TrueType) MakeFont() font.FromFile {
	textMap := simpleTextMap(d.PostScriptName, d.Encoding, d.ToUnicode)
	return &ttFont{
		Dict: d,
		Text: textMap,
	}
}

var (
	_ font.FromFile = &ttFont{}
)

type ttFont struct {
	Dict *TrueType
	Text map[byte]string
}

func (f *ttFont) Embed(rm *pdf.EmbedHelper) (pdf.Native, font.Embedded, error) {
	ref := rm.Alloc()
	err := f.Dict.WriteToPDF(rm.GetRM(), ref)
	if err != nil {
		return nil, nil, err
	}
	return ref, f, nil
}

func (f *ttFont) PostScriptName() string {
	return f.Dict.PostScriptName
}

func (f *ttFont) GetDict() font.Dict {
	return f.Dict
}

func (*ttFont) WritingMode() font.WritingMode {
	return font.Horizontal
}

func (f *ttFont) Codes(s pdf.String) iter.Seq[*font.Code] {
	return func(yield func(*font.Code) bool) {
		var res font.Code
		for _, code := range s {
			if f.Dict.Encoding(code) == "" {
				res.CID = 0
			} else {
				res.CID = cid.CID(code) + 1
			}
			res.Width = f.Dict.Width[code]
			res.UseWordSpacing = (code == 0x20)
			res.Text = f.Text[code]
			if !yield(&res) {
				return
			}
		}
	}
}

func init() {
	registerReader("TrueType", func(x *pdf.Extractor, obj pdf.Object) (font.Dict, error) {
		return DecodeTrueType(x, obj)
	})
}
