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

package type1

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"iter"

	"seehuhn.de/go/postscript/type1"
	"seehuhn.de/go/postscript/type1/names"

	"seehuhn.de/go/sfnt"
	"seehuhn.de/go/sfnt/cff"
	"seehuhn.de/go/sfnt/os2"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/encoding"
	"seehuhn.de/go/pdf/font/subset"
	"seehuhn.de/go/pdf/internal/stdmtx"
)

var (
	_ FontData = (*type1.Font)(nil)
	_ FontData = (*cff.Font)(nil)
	_ FontData = (*sfnt.Font)(nil)
)

// FontData is a font which can be used with a Type 1 font dictionary.
// This must be one of [*type1.Font], [*cff.Font] or [*sfnt.Font].
type FontData interface {
	PostScriptName() string
	BuiltinEncoding() []string
}

var _ font.Embedded = (*FontDict)(nil)

// FontDict represents a Type 1 font dictionary.
type FontDict struct {
	Ref            pdf.Reference
	PostScriptName string
	SubsetTag      string
	Name           pdf.Name

	// Descriptor is the font descriptor.
	Descriptor *font.Descriptor

	// Encoding maps character codes to glyph names.
	Encoding encoding.Type1

	// Width contains the glyph widths for all character codes
	// (PDF glyph space units).
	Width [256]float64

	// Text gives the text content for each character code.
	Text [256]string

	// GetFont (optional) returns the font data to embed.
	// If this is nil, the font data is not embedded in the PDF file.
	GetFont func() (FontData, error)
}

func (d *FontDict) WritingMode() cmap.WritingMode {
	return cmap.Horizontal
}

func (d *FontDict) DecodeWidth(s pdf.String) (float64, int) {
	if len(s) == 0 {
		return 0, 0
	}
	return d.Width[s[0]], 1
}

// ExtractDict reads a Type 1 font dictionary from a PDF file.
func ExtractDict(r pdf.Getter, obj pdf.Object) (*FontDict, error) {
	fontDict, err := pdf.GetDictTyped(r, obj, "Font")
	if err != nil {
		return nil, err
	}
	subtype, err := pdf.GetName(r, fontDict["Subtype"])
	if err != nil {
		return nil, err
	}
	if subtype != "" && subtype != "Type1" {
		return nil, &pdf.MalformedFileError{
			Err: fmt.Errorf("expected font subtype Type1, got %q", subtype),
		}
	}

	d := &FontDict{}
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

	// StdInfo will be non-nil, if the PostScript name indicates one of the
	// standard 14 fonts. In this case, we use the corresponding metrics as
	// default values, in case they are missing from the font dictionary.
	stdInfo := stdmtx.Metrics[d.PostScriptName]

	d.Name, _ = pdf.GetName(r, fontDict["Name"])

	fdDict, err := pdf.GetDictTyped(r, fontDict["FontDescriptor"], "FontDescriptor")
	if err != nil && !pdf.IsMalformed(err) {
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
			FontBBox:     stdInfo.FontBBox,
			ItalicAngle:  stdInfo.ItalicAngle,
			Ascent:       stdInfo.Ascent,
			Descent:      stdInfo.Descent,
			CapHeight:    stdInfo.CapHeight,
			XHeight:      stdInfo.XHeight,
			StemV:        stdInfo.StemV,
			StemH:        stdInfo.StemH,
		}
		if stdInfo.FontFamily == "Symbol" || stdInfo.FontFamily == "ZapfDingbats" {
			fd.IsSymbolic = true
		}
	} else if fd == nil { // only possible for invalid PDF files
		fd = &font.Descriptor{
			FontName: d.PostScriptName,
		}
	}
	d.Descriptor = fd

	isNonSymbolic := !fd.IsSymbolic
	isExternal := fdDict["FontFile"] == nil && fdDict["FontFile3"] == nil
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
	} else if stdInfo != nil {
		for c := range 256 {
			w, ok := stdInfo.Width[enc(byte(c))]
			if !ok {
				w = stdInfo.Width[".notdef"]
			}
			d.Width[c] = w
		}
	}

	// First try to derive text content from the glyph names.
	// This can be overridden by the ToUnicode CMap, below.
	for code := range 256 {
		glyphName := enc(byte(code))
		if glyphName == "" || glyphName == encoding.UseBuiltin || glyphName == ".notdef" {
			continue
		}

		rr := names.ToUnicode(glyphName, d.PostScriptName == "ZapfDingbats")
		d.Text[code] = string(rr)
	}

	toUnicode, err := cmap.ExtractToUnicodeNew(r, fontDict["ToUnicode"])
	if err != nil && !pdf.IsMalformed(err) {
		return nil, err
	}
	if toUnicode != nil {
		// TODO(voss): implement an iterator on toUnicode to do this
		// more efficiently?
		for code := range 256 {
			rr := toUnicode.Lookup([]byte{byte(code)})
			d.Text[code] = string(rr)
		}
	}

	getFont, err := makeFontReader(r, fdDict)
	if err != nil && !pdf.IsMalformed(err) {
		return nil, err
	}
	d.GetFont = getFont

	return d, nil
}

func makeFontReader(r pdf.Getter, fd pdf.Dict) (func() (FontData, error), error) {
	s, err := pdf.GetStream(r, fd["FontFile"])
	if err != nil && !pdf.IsMalformed(err) {
		return nil, err
	}
	if s != nil {
		getFont := func() (FontData, error) {
			fontData, err := pdf.DecodeStream(r, s, 0)
			if err != nil {
				return nil, err
			}
			font, err := type1.Read(fontData)
			if err != nil {
				return nil, err
			}
			return font, nil
		}
		return getFont, nil
	}

	s, err = pdf.GetStream(r, fd["FontFile3"])
	if err != nil && !pdf.IsMalformed(err) {
		return nil, err
	}
	if s == nil {
		return nil, nil
	}

	subType, _ := pdf.GetName(r, s.Dict["Subtype"])
	switch subType {
	case "Type1C":
		getFont := func() (FontData, error) {
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
		getFont := func() (FontData, error) {
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

// Embed adds the font dictionary to the PDF file.
//
// The FontName field in the font descriptor is ignored and the correct value
// is set automatically.
func (d *FontDict) Embed(rm *pdf.ResourceManager) (pdf.Native, pdf.Unused, error) {
	var zero pdf.Unused

	var psFont FontData
	if d.GetFont != nil {
		font, err := d.GetFont()
		if err != nil {
			return nil, zero, err
		}
		psFont = font
	}

	// Check that all data are valid and consistent.
	if d.Ref == 0 {
		return nil, zero, errors.New("missing font dictionary reference")
	}
	if psFont != nil {
		switch f := psFont.(type) {
		case *type1.Font:
			// pass
		case *cff.Font:
			if f.IsCIDKeyed() {
				return nil, zero, errors.New("CID-keyed fonts not allowed")
			}
		case *sfnt.Font:
			o, _ := f.Outlines.(*cff.Outlines)
			if o == nil {
				return nil, zero, errors.New("missing CFF table")
			} else if o.IsCIDKeyed() {
				return nil, zero, errors.New("CID-keyed fonts not allowed")
			}
		default:
			return nil, zero, fmt.Errorf("unsupported font type %T", psFont)
		}
	}
	if d.SubsetTag != "" && !subset.IsValidTag(d.SubsetTag) {
		return nil, zero, fmt.Errorf("invalid subset tag: %s", d.SubsetTag)
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
		"Subtype":  pdf.Name("Type1"),
		"BaseFont": baseFont,
	}
	if d.Name != "" {
		fontDict["Name"] = d.Name
	}

	isNonSymbolic := !d.Descriptor.IsSymbolic
	isExternal := psFont == nil
	encodingObj, err := d.Encoding.AsPDF(isNonSymbolic && isExternal, w.GetOptions())
	if err != nil {
		return nil, zero, err
	}
	if encodingObj != nil {
		fontDict["Encoding"] = encodingObj
	}

	compressedObjects := []pdf.Object{fontDict}
	compressedRefs := []pdf.Reference{d.Ref}

	stdInfo := stdmtx.Metrics[d.PostScriptName]

	var fontFileRef pdf.Reference
	// TODO(voss): make sure that this matches the code in
	// [embeddedSimple.Finish] (file "font.go")
	trimFontDict := (psFont == nil &&
		stdInfo != nil &&
		w.GetOptions().HasAny(pdf.OptTrimStandardFonts) &&
		widthsAreCompatible(d.Width[:], d.Encoding, stdInfo) &&
		fontDescriptorIsCompatible(d.Descriptor, stdInfo))
	if !trimFontDict {
		fdRef := w.Alloc()
		fdDict := d.Descriptor.AsDict()
		fdDict["FontName"] = pdf.Name(baseFont)
		if psFont != nil {
			fontFileRef = w.Alloc()
			switch psFont.(type) {
			case *type1.Font:
				fdDict["FontFile"] = fontFileRef
			case *cff.Font, *sfnt.Font:
				fdDict["FontFile3"] = fontFileRef
			}
		}

		fontDict["FontDescriptor"] = fdRef

		// TODO(voss): Introduce a helper function for constructing the widths
		// array.
		lastChar := 255
		for lastChar > 0 && d.Width[lastChar] == d.Descriptor.MissingWidth {
			lastChar--
		}
		firstChar := 0
		for firstChar < lastChar && d.Width[firstChar] == d.Descriptor.MissingWidth {
			firstChar++
		}
		widths := make(pdf.Array, 0, lastChar-firstChar+1)
		for i := firstChar; i <= lastChar; i++ {
			widths = append(widths, pdf.Number(d.Width[i]))
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

		compressedObjects = append(compressedObjects, fdDict)
		compressedRefs = append(compressedRefs, fdRef)
	}

	toUnicodeData := make(map[byte]string)
	for code := range 256 {
		glyphName := d.Encoding(byte(code))
		switch glyphName {
		case "":
			// unused code, nothing to do

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
			return nil, zero, fmt.Errorf("ToUnicode cmap: %w", err)
		}
		fontDict["ToUnicode"] = ref
	}

	err = w.WriteCompressed(compressedRefs, compressedObjects...)
	if err != nil {
		return nil, zero, pdf.Wrap(err, "Type 1 font dicts")
	}

	switch f := psFont.(type) {
	case *type1.Font:
		length1 := pdf.NewPlaceholder(w, 10)
		length2 := pdf.NewPlaceholder(w, 10)
		fontStmDict := pdf.Dict{
			"Length1": length1,
			"Length2": length2,
			"Length3": pdf.Integer(0),
		}
		fontStm, err := w.OpenStream(fontFileRef, fontStmDict, pdf.FilterCompress{})
		if err != nil {
			return nil, zero, fmt.Errorf("open Type1 stream: %w", err)
		}
		l1, l2, err := f.WritePDF(fontStm)
		if err != nil {
			return nil, zero, fmt.Errorf("write Type1 stream: %w", err)
		}
		err = length1.Set(pdf.Integer(l1))
		if err != nil {
			return nil, zero, fmt.Errorf("Type1 stream: length1: %w", err)
		}
		err = length2.Set(pdf.Integer(l2))
		if err != nil {
			return nil, zero, fmt.Errorf("Type1 stream: length2: %w", err)
		}
		err = fontStm.Close()
		if err != nil {
			return nil, zero, fmt.Errorf("close Type1 stream: %w", err)
		}

	case *cff.Font:
		fontStmDict := pdf.Dict{
			"Subtype": pdf.Name("Type1C"),
		}
		fontStm, err := w.OpenStream(fontFileRef, fontStmDict, pdf.FilterCompress{})
		if err != nil {
			return nil, zero, fmt.Errorf("open CFF stream: %w", err)
		}
		err = f.Write(fontStm)
		if err != nil {
			return nil, zero, fmt.Errorf("write CFF stream: %w", err)
		}
		err = fontStm.Close()
		if err != nil {
			return nil, zero, fmt.Errorf("close CFF stream: %w", err)
		}

	case *sfnt.Font:
		fontStmDict := pdf.Dict{
			"Subtype": pdf.Name("OpenType"),
		}
		fontStm, err := w.OpenStream(fontFileRef, fontStmDict, pdf.FilterCompress{})
		if err != nil {
			return nil, zero, fmt.Errorf("open OpenType stream: %w", err)
		}
		err = f.WriteOpenTypeCFFPDF(fontStm)
		if err != nil {
			return nil, zero, fmt.Errorf("write OpenType stream: %w", err)
		}
		err = fontStm.Close()
		if err != nil {
			return nil, zero, fmt.Errorf("close OpenType stream: %w", err)
		}
	}

	return d.Ref, zero, nil
}

// Codes returns an iterator over the character codes in the given PDF string.
// The iterator yields Code instances that provide access to the CID, width,
// and text content associated with each character code.
func (d *FontDict) Codes(s pdf.String) iter.Seq[Code] {
	return func(yield func(Code) bool) {
		pos := &type1Code{d: d}
		for _, c := range s {
			pos.c = c
			if !yield(pos) {
				return
			}
		}
	}
}

// Code represents a character code in a font. It provides methods to access
// the CID, width, and text content associated with the character code.
type Code interface {
	// CID returns the CID (Character Identifier) for the current character code.
	CID() cmap.CID

	// Width returns the width of the glyph for the current character code.
	Width() float64

	// Text returns the text content for the current character code.
	Text() string
}

// type1Code is an implementation of the Code interface for a simple font.
type type1Code struct {
	d *FontDict
	c byte
}

// CID returns the CID (Character Identifier) for the current character code.
func (c *type1Code) CID() cmap.CID {
	// TODO(voss): find a way to select glyphs using CID values
	return cmap.CID(c.c)
}

// Width returns the width of the glyph for the current character code.
func (c *type1Code) Width() float64 {
	return c.d.Width[c.c]
}

// Text returns the text content for the current character code.
func (c *type1Code) Text() string {
	return c.d.Text[c.c]
}
