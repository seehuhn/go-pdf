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
	"math"

	"seehuhn.de/go/postscript/cid"
	"seehuhn.de/go/postscript/type1/names"

	"seehuhn.de/go/sfnt/os2"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/encoding"
	"seehuhn.de/go/pdf/font/glyphdata"
	"seehuhn.de/go/pdf/font/subset"
	"seehuhn.de/go/pdf/internal/stdmtx"
)

// Type1 represents a Type 1 font dictionary.
type Type1 struct {
	// Ref is the reference to the font dictionary in the PDF file.
	Ref pdf.Reference

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
	Encoding encoding.Type1

	// Width contains the glyph widths for all character codes
	// (PDF glyph space units).
	Width [256]float64

	// Text gives the text content for each character code.
	Text [256]string

	// FontType gives the type of glyph outline data.
	// Possible values are [glyphdata.Type1], [glyphdata.CFFSimple],
	// and [glyphdata.OpenTypeCFFSimple], or [glyphdata.None] if the font
	// is not embedded.
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
		return nil, &pdf.MalformedFileError{
			Err: fmt.Errorf("expected font subtype Type1, got %q", subtype),
		}
	}

	d := &Type1{}
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

	if fd == nil {
		// prevent invalid PDF files from causing panics
		fd = &font.Descriptor{}
	}

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
			default:
				d.FontType = glyphdata.None
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

	defaultWidth := fd.MissingWidth
	firstChar, _ := pdf.GetInteger(r, fontDict["FirstChar"])
	widths, _ := pdf.GetArray(r, fontDict["Widths"])
	if widths != nil && len(widths) <= 256 && firstChar >= 0 && firstChar < 256 {
		for c := range d.Width {
			d.Width[c] = defaultWidth
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
	for code := range 256 {
		glyphName := enc(byte(code))
		if d.FontRef == 0 && stdInfo != nil && glyphName == encoding.UseBuiltin {
			glyphName = stdInfo.Encoding[code]
		}
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

	d.Repair(r)

	return d, nil
}

// Repair fixes invalid data in the font dictionary.
// After Repair has been called, [Type1.validate] will return nil.
func (d *Type1) Repair(r pdf.Getter) {
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
	d.Descriptor.FontName = subset.Join(d.SubsetTag, d.PostScriptName)

	if d.FontRef == 0 {
		d.FontType = glyphdata.None
	}
}

// Check that all data are valid and consistent.
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

// WriteToPDF adds the font dictionary to the PDF file.
func (d *Type1) WriteToPDF(rm *pdf.ResourceManager) error {
	if d.Ref == 0 {
		return errors.New("missing font dictionary reference")
	}

	w := rm.Out

	err := d.validate(w)
	if err != nil {
		return err
	}

	switch d.FontType {
	case glyphdata.None:
		// not embedded
	case glyphdata.Type1:
		// ok
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

	isNonSymbolic := !d.Descriptor.IsSymbolic
	isExternal := d.FontRef == 0
	baseIsStd := isNonSymbolic && isExternal
	if stdInfo != nil {
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
	compressedRefs := []pdf.Reference{d.Ref}

	trimFontDict := ((d.FontRef == 0) &&
		stdInfo != nil &&
		w.GetOptions().HasAny(pdf.OptTrimStandardFonts) &&
		widthsAreCompatible(d.Width[:], d.Encoding, stdInfo) &&
		fontDescriptorIsCompatible(d.Descriptor, stdInfo))
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

		// TODO(voss): Introduce a helper function for constructing the widths
		// array.
		defaultWidth := d.Descriptor.MissingWidth
		firstChar, lastChar := 0, 255
		for lastChar > 0 && (d.Encoding(byte(lastChar)) == "" || d.Width[lastChar] == defaultWidth) {
			lastChar--
		}
		for firstChar < lastChar && (d.Encoding(byte(firstChar)) == "" || d.Width[firstChar] == defaultWidth) {
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
	}

	toUnicodeData := make(map[byte]string)
	for code := range 256 {
		if d.Text[code] == "" {
			continue
		}

		glyphName := d.Encoding(byte(code))
		switch glyphName {
		case "":
			// unused character code, nothing to do

		case encoding.UseBuiltin:
			toUnicodeData[byte(code)] = d.Text[code]

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
		return fmt.Errorf("font dicts: %w", err)
	}

	return nil
}

func (d *Type1) GetScanner() (font.Embedded, error) {
	return d, nil
}

func (d *Type1) WritingMode() font.WritingMode {
	return font.Horizontal
}

func (d *Type1) Codes(s pdf.String) iter.Seq[*font.Code] {
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

// widthsAreCompatible returns true, if the glyph widths ww are compatible with
// the standard font metrics.  The object encObj is the value of the font
// dictionary's Encoding entry.
//
// EncObj must be valid and must be a direct object.  Do not pass encObj values
// read from files without validation.
func widthsAreCompatible(ww []float64, enc encoding.Type1, info *stdmtx.FontData) bool {
	for code := range 256 {
		glyphName := enc(byte(code))
		if glyphName == "" {
			continue
		}
		if math.Abs(ww[code]-info.Width[glyphName]) > 0.5 {
			return false
		}
	}
	return true
}

func fontDescriptorIsCompatible(fd *font.Descriptor, stdInfo *stdmtx.FontData) bool {
	if fd.FontFamily != "" && fd.FontFamily != stdInfo.FontFamily {
		return false
	}
	if fd.FontWeight != 0 && fd.FontWeight != stdInfo.FontWeight {
		return false
	}

	if fd.IsFixedPitch != stdInfo.IsFixedPitch {
		return false
	}
	if fd.IsSerif != stdInfo.IsSerif {
		return false
	}
	if fd.IsSymbolic != stdInfo.IsSymbolic {
		return false
	}
	if fd.IsScript || fd.IsItalic || fd.IsAllCap || fd.IsSmallCap || fd.ForceBold {
		return false
	}

	if math.Abs(fd.ItalicAngle-stdInfo.ItalicAngle) > 0.1 {
		return false
	}
	if fd.Ascent != 0 && math.Abs(fd.Ascent-stdInfo.Ascent) > 0.5 {
		return false
	}
	if fd.Descent != 0 && math.Abs(fd.Descent-stdInfo.Descent) > 0.5 {
		return false
	}
	if fd.CapHeight != 0 && math.Abs(fd.CapHeight-stdInfo.CapHeight) > 0.5 {
		return false
	}
	if fd.XHeight != 0 && math.Abs(fd.XHeight-stdInfo.XHeight) > 0.5 {
		return false
	}
	if fd.StemV != 0 && math.Abs(fd.StemV-stdInfo.StemV) > 0.5 {
		return false
	}
	if fd.StemH != 0 && math.Abs(fd.StemH-stdInfo.StemH) > 0.5 {
		return false
	}
	return true
}

func init() {
	font.RegisterReader("Type1", func(r pdf.Getter, obj pdf.Object) (font.FromFile, error) {
		return ExtractType1(r, obj)
	})
}
