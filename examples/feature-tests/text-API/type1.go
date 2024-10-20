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
	"math"
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
	"seehuhn.de/go/pdf/internal/stdmtx"
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
	if ps := f.Font; ps != nil {
		fd.FontName = ps.FontName
		fd.FontFamily = ps.FamilyName
		fd.FontWeight = os2.WeightFromString(ps.Weight)
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
		Ref:            ref,
		PostScriptName: f.Font.FontName,
		Descriptor:     fd,
		Encoding:       encoding.New(),
		Font:           f.Font,
	}
	return ref, dicts, nil
}

type Type1FontData interface {
	PostScriptName() string
	GetEncoding() []string
	WidthsMapPDF() map[string]float64
}

var (
	_ Type1FontData = (*type1.Font)(nil)
	_ Type1FontData = (*cff.Font)(nil)
	_ Type1FontData = (*sfnt.Font)(nil)
)

type Type1Dicts struct {
	Ref            pdf.Reference
	PostScriptName string
	SubsetTag      string
	Name           pdf.Name

	// Descriptor is the font descriptor dictionary.
	// To following fields are ignored: FontName, MissingWidth.
	Descriptor *font.Descriptor

	Encoding *encoding.Encoding
	Width    [256]float64
	Text     [256]string

	// Font (optional) is the font data to embed.
	// This must be one of *type1.Font, *cff.Font, or *sfnt.Font.
	Font Type1FontData
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
		switch f := d.Font.(type) {
		case *cff.Font:
			if f.IsCIDKeyed() {
				return errors.New("CID-keyed fonts not allowed")
			}
		case *sfnt.Font:
			o, _ := f.Outlines.(*cff.Outlines)
			if o == nil {
				return errors.New("missing CFF table")
			} else if o.IsCIDKeyed() {
				return errors.New("CID-keyed fonts not allowed")
			}
		default:
			return fmt.Errorf("unsupported font type: %T", d.Font)
		}

		fontName := d.Font.PostScriptName()
		if fontName != d.PostScriptName {
			return fmt.Errorf("font name mismatch: %s != %s", fontName, d.PostScriptName)
		}
	}
	if d.SubsetTag != "" && !subset.IsValidTag(d.SubsetTag) {
		return fmt.Errorf("invalid subset tag: %s", d.SubsetTag)
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

	isNonSymbolic := !d.Descriptor.IsSymbolic
	isExternal := d.Font == nil
	encodingObject, err := d.Encoding.AsPDFType1(isNonSymbolic && isExternal, w.GetOptions())
	if err != nil {
		return err
	}
	if encodingObject != nil {
		fontDict["Encoding"] = encodingObject
	}

	// TODO(voss): to construct the widths array, we need to know the actual
	// encoding used.  Since `encodingObject` may map more codes than `d.Encoding`,
	// we need to use `encodingObject` to construct the widths array.

	compressedObjects := []pdf.Object{fontDict}
	compressedRefs := []pdf.Reference{d.Ref}

	stdInfo, isStdFont := stdmtx.Metrics[d.PostScriptName]

	var builtinEncoding []string
	if d.Font != nil {
		builtinEncoding = d.Font.GetEncoding()
	} else if isStdFont {
		builtinEncoding = stdInfo.Encoding
	}

	var fontFileRef pdf.Reference
	trimFontDict := (d.Font == nil &&
		isStdFont &&
		rm.Out.GetOptions().HasAny(pdf.OptTrimStandardFonts) &&
		widthsAreCompatible() &&
		fontDescriptorIsCompatible(d.Descriptor, stdInfo))
	if !trimFontDict {
		descRef := w.Alloc()
		desc := d.Descriptor.AsDict()
		desc["FontName"] = pdf.Name(baseFont)
		if d.Font != nil {
			fontFileRef = w.Alloc()
			switch d.Font.(type) {
			case *type1.Font:
				desc["FontFile"] = fontFileRef
			case *cff.Font, *sfnt.Font:
				desc["FontFile3"] = fontFileRef
			}
		}

		fontDict["FontDescriptor"] = descRef

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

		compressedObjects = append(compressedObjects, desc, widthInfo.Widths)
		compressedRefs = append(compressedRefs, descRef, widthRef)
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
		if glyphName == "" && code < len(builtinEncoding) && builtinEncoding[code] != ".notdef" {
			glyphName = builtinEncoding[code]
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

func widthsAreCompatible() bool {
	// TODO(voss): implement
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
