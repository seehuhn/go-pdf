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

package main

import (
	"errors"
	"fmt"
	"slices"

	"seehuhn.de/go/postscript/afm"
	"seehuhn.de/go/postscript/type1"
	"seehuhn.de/go/postscript/type1/names"

	"seehuhn.de/go/sfnt"
	"seehuhn.de/go/sfnt/cff"
	"seehuhn.de/go/sfnt/glyph"
	"seehuhn.de/go/sfnt/os2"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/encoding"
	"seehuhn.de/go/pdf/font/subset"
	"seehuhn.de/go/pdf/font/widths"
	"seehuhn.de/go/pdf/graphics"
)

var _ font.Font = (*Type1Font)(nil)

type Type1Font struct {
	Font    *type1.Font
	Metrics *afm.Metrics
}

func (f *Type1Font) PostScriptName() string {
	if f.Font != nil {
		return f.Font.FontName
	}
	if f.Metrics != nil {
		return f.Metrics.FontName
	}
	return ""
}

func (f *Type1Font) Embed(rm *pdf.ResourceManager) (pdf.Native, font.Embedded, error) {
	ref := rm.Out.Alloc()
	fd := &font.Descriptor{}
	var builtinEncoding []string
	if ps := f.Font; ps != nil {
		fd.FontName = ps.FontName
		fd.FontFamily = ps.FamilyName
		fd.FontWeight = os2.WeightFromString(ps.Weight)
		if len(ps.Encoding) == 256 {
			builtinEncoding = ps.Encoding
		}
		fd.FontBBox = ps.FontBBoxPDF()
		fd.IsItalic = ps.ItalicAngle != 0
		fd.ItalicAngle = ps.ItalicAngle
		fd.IsFixedPitch = ps.IsFixedPitch
		fd.ForceBold = ps.Private.ForceBold
		fd.StemV = ps.Private.StdVW
		fd.StemH = ps.Private.StdHW
	}
	if m := f.Metrics; m != nil {
		fd.FontName = m.FontName
		if len(m.Encoding) == 256 {
			builtinEncoding = m.Encoding
		}
		fd.FontBBox = m.FontBBoxPDF()
		fd.CapHeight = m.CapHeight
		fd.XHeight = m.XHeight
		fd.Ascent = m.Ascent
		fd.Descent = m.Descent
		fd.IsItalic = m.ItalicAngle != 0
		fd.ItalicAngle = m.ItalicAngle
		fd.IsFixedPitch = m.IsFixedPitch
	}
	dicts := &Type1Dicts{
		Ref:             ref,
		PostScriptName:  f.Font.FontName,
		Descriptor:      fd,
		BuiltinEncoding: builtinEncoding,
		Encoding:        encoding.New(),
	}
	return ref, dicts, nil
}

type Type1Dicts struct {
	Ref            pdf.Reference
	Name           pdf.Name
	SubsetTag      string
	PostScriptName string

	// Descriptor is the font descriptor dictionary.
	// To following fields are ignored: FontName, MissingWidth.
	Descriptor *font.Descriptor

	BuiltinEncoding []string
	Encoding        *encoding.Encoding
	Font            any // TODO(voss): use an interface type here?
	Width           [256]float64
	Text            [256]string
}

func (d *Type1Dicts) WritingMode() cmap.WritingMode {
	return cmap.Horizontal
}

func (d *Type1Dicts) DecodeWidth(s pdf.String) (float64, int) {
	if len(s) == 0 {
		return 0, 0
	}
	code := s[0]
	return d.Width[code], 1
}

// Finish writes the font dictionary to the PDF file.
// This implements [pdf.Finisher].
func (d *Type1Dicts) Finish(rm *pdf.ResourceManager) error {
	w := rm.Out

	if d.Font != nil {
		var fontName string
		switch f := d.Font.(type) {
		case *type1.Font:
			fontName = f.FontName
		case *cff.Font:
			if f.IsCIDKeyed() {
				return errors.New("CID-keyed fonts not allowed")
			}
			fontName = f.FontName
		case *sfnt.Font:
			o, _ := f.Outlines.(*cff.Outlines)
			if o == nil {
				return errors.New("missing CFF table")
			} else if o.IsCIDKeyed() {
				return errors.New("CID-keyed fonts not allowed")
			}
			fontName = f.PostScriptName()
		default:
			return fmt.Errorf("unsupported font type: %T", d.Font)
		}
		if fontName != d.PostScriptName {
			return fmt.Errorf("font name mismatch: %s != %s", fontName, d.PostScriptName)
		}
	}
	if d.SubsetTag != "" && !subset.IsValidTag(d.SubsetTag) {
		return fmt.Errorf("invalid subset tag: %s", d.SubsetTag)
	}
	canOmit := font.IsStandard[d.PostScriptName] && pdf.GetVersion(w) < pdf.V2_0 && d.Font == nil
	if d.Descriptor == nil && !canOmit {
		return errors.New("missing font descriptor")
	}

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

	var isNonSymbolic bool
	if d.Descriptor != nil {
		isNonSymbolic = !d.Descriptor.IsSymbolic
	} else {
		isNonSymbolic = font.IsStandardNonSymbolic[d.PostScriptName]
	}
	isExternal := d.Font == nil
	encoding, err := d.Encoding.AsPDFType1(isNonSymbolic && isExternal, w.GetOptions())
	if err != nil {
		return err
	}
	if encoding != nil {
		fontDict["Encoding"] = encoding
	}

	compressedObjects := []pdf.Object{fontDict}
	compressedRefs := []pdf.Reference{d.Ref}

	var fontFileRef pdf.Reference
	if d.Descriptor != nil {
		desc := d.Descriptor.AsDict()
		desc["FontName"] = pdf.Name(baseFont)

		widthRef := w.Alloc()
		widthInfo := widths.EncodeSimple(d.Width[:])
		fontDict["FirstChar"] = widthInfo.FirstChar
		fontDict["LastChar"] = widthInfo.LastChar
		fontDict["Widths"] = widthRef
		if widthInfo.MissingWidth != 0 {
			desc["MissingWidth"] = pdf.Number(widthInfo.MissingWidth)
		} else {
			delete(desc, "MissingWidth")
		}

		descRef := w.Alloc()
		fontDict["FontDescriptor"] = descRef

		compressedObjects = append(compressedObjects, desc, widthInfo.Widths)
		compressedRefs = append(compressedRefs, descRef, widthRef)

		if d.Font != nil {
			fontFileRef = w.Alloc()
			switch d.Font.(type) {
			case *type1.Font:
				desc["FontFile"] = fontFileRef
			case *cff.Font, *sfnt.Font:
				desc["FontFile3"] = fontFileRef
			}
		}
	}

	needsToUnicode := false
	for code := range 256 {
		cid := d.Encoding.Decode(byte(code))
		if cid == 0 {
			if d.Text[code] != "" {
				needsToUnicode = true
				break
			}
			continue
		}

		glyphName := d.Encoding.GlyphName(cid)
		if glyphName == "" && code < len(d.BuiltinEncoding) && d.BuiltinEncoding[code] != ".notdef" {
			glyphName = d.BuiltinEncoding[code]
		}

		if glyphName == "" {
			if d.Text[code] != "" {
				needsToUnicode = true
				break
			}
			continue
		}

		rr := names.ToUnicode(glyphName, d.PostScriptName == "ZapfDingbats")
		if d.Text[code] != string(rr) {
			needsToUnicode = true
			break
		}
	}
	if needsToUnicode {
		tuInfo := cmap.MakeSimpleToUnicode(d.Text[:])
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

	switch f := d.Font.(type) {
	case *type1.Font:
		length1 := pdf.NewPlaceholder(w, 10)
		length2 := pdf.NewPlaceholder(w, 10)
		fontFileDict := pdf.Dict{
			"Length1": length1,
			"Length2": length2,
			"Length3": pdf.Integer(0),
		}
		fontFileStream, err := w.OpenStream(fontFileRef, fontFileDict, pdf.FilterCompress{})
		if err != nil {
			return fmt.Errorf("open Type1 stream: %w", err)
		}
		l1, l2, err := f.WritePDF(fontFileStream)
		if err != nil {
			return fmt.Errorf("write Type1 stream: %w", err)
		}
		err = length1.Set(pdf.Integer(l1))
		if err != nil {
			return fmt.Errorf("Type1 stream: length1: %w", err)
		}
		err = length2.Set(pdf.Integer(l2))
		if err != nil {
			return fmt.Errorf("Type1 stream: length2: %w", err)
		}
		err = fontFileStream.Close()
		if err != nil {
			return fmt.Errorf("close Type1 stream: %w", err)
		}

	case *cff.Font:
		fontFileDict := pdf.Dict{
			"Subtype": pdf.Name("Type1C"),
		}
		fontFileStream, err := w.OpenStream(fontFileRef, fontFileDict, pdf.FilterCompress{})
		if err != nil {
			return fmt.Errorf("open CFF stream: %w", err)
		}
		err = f.Write(fontFileStream)
		if err != nil {
			return fmt.Errorf("write CFF stream: %w", err)
		}
		err = fontFileStream.Close()
		if err != nil {
			return fmt.Errorf("close CFF stream: %w", err)
		}

	case *sfnt.Font:
		fontFileDict := pdf.Dict{
			"Subtype": pdf.Name("OpenType"),
		}
		fontFileStream, err := w.OpenStream(fontFileRef, fontFileDict, pdf.FilterCompress{})
		if err != nil {
			return fmt.Errorf("open OpenType stream: %w", err)
		}
		err = f.WriteOpenTypeCFFPDF(fontFileStream)
		if err != nil {
			return fmt.Errorf("write OpenType stream: %w", err)
		}
		err = fontFileStream.Close()
		if err != nil {
			return fmt.Errorf("close OpenType stream: %w", err)
		}
	}

	return nil
}

func type1Rune(w *graphics.Writer, f *type1.Font, r rune) {
	cmap := make(map[rune]string)
	for glyphName := range f.Glyphs {
		rr := names.ToUnicode(glyphName, f.FontName == "ZapfDingbats")
		if len(rr) != 1 {
			panic("unexpected number of runes")
		}
		cmap[rr[0]] = glyphName
	}
	enc := encoding.New()

	// -----------------------------------------------------------------------

	glyphName, ok := cmap[r]
	if !ok {
		panic("missing rune")
	}
	gidInt := slices.Index(f.GlyphList(), glyphName)
	if gidInt < 0 {
		panic("missing")
	}
	gid := glyph.ID(gidInt)
	text := string([]rune{r})

	code, isNew := allocateCode(gid, text)
	if isNew {
		// builtinEncoding[code] = glyphName

		cid := enc.Allocate(glyphName)
		w := f.Glyphs[glyphName].WidthX

		info := &font.CodeInfo{
			CID:    cid,
			Notdef: 0,
			Text:   string([]rune{r}),
			W:      w,
		}
		setCodeInfo(code, info)
	}

	w.TextShowRaw(code)
}
