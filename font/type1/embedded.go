// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2023  Jochen Voss <voss@seehuhn.de>
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
	"errors"
	"fmt"
	"math"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/subset"
	"seehuhn.de/go/pdf/font/tounicode"
	"seehuhn.de/go/postscript/funit"
	"seehuhn.de/go/postscript/type1"
	"seehuhn.de/go/sfnt/glyph"
)

type embedded struct {
	*Font

	w pdf.Putter
	pdf.Resource

	cmap.SimpleEncoder
	closed bool
}

func (e *embedded) Close() error {
	if e.closed {
		return nil
	}
	e.closed = true

	if e.SimpleEncoder.Overflow() {
		return fmt.Errorf("too many distinct glyphs used in font %q (%s)",
			e.Name, e.outlines.FontInfo.FontName)
	}
	e.SimpleEncoder = cmap.NewFrozenSimpleEncoder(e.SimpleEncoder)

	encodingGid := e.SimpleEncoder.Encoding()
	encoding := make([]string, 256)
	for i, gid := range encodingGid {
		encoding[i] = e.names[gid]
	}

	psFont := e.outlines
	var psSubset *type1.Font
	var subsetTag string
	if psFont.Outlines != nil {
		psSubset = &type1.Font{}
		*psSubset = *psFont
		psSubset.Outlines = make(map[string]*type1.Glyph)
		psSubset.GlyphInfo = make(map[string]*type1.GlyphInfo)

		if _, ok := psFont.Outlines[".notdef"]; ok {
			psSubset.Outlines[".notdef"] = psFont.Outlines[".notdef"]
			psSubset.GlyphInfo[".notdef"] = psFont.GlyphInfo[".notdef"]
		}
		for _, name := range encoding {
			if _, ok := psFont.Outlines[name]; ok {
				psSubset.Outlines[name] = psFont.Outlines[name]
				psSubset.GlyphInfo[name] = psFont.GlyphInfo[name]
			}
		}
		psSubset.Encoding = encoding

		var ss []subset.Glyph
		for origGid, name := range e.names {
			if _, ok := psSubset.Outlines[name]; ok {
				ss = append(ss, subset.Glyph{
					OrigGID: glyph.ID(origGid),
					CID:     type1.CID(len(ss)),
				})
			}
		}
		subsetTag = subset.Tag(ss, psFont.NumGlyphs())
	} else {
		psSubset = psFont
	}

	// TODO(voss): implement ToUnicode

	t1 := &EmbedInfo{
		PSFont:    psSubset,
		SubsetTag: subsetTag,
		Encoding:  encoding,
		ResName:   e.Name,
	}
	return t1.Embed(e.w, e.Ref)
}

// EmbedInfo is all the information about a Type 1 font which is stored when
// the font is embedded in a PDF file.
type EmbedInfo struct {
	// PSFont is the (subsetted as needed) font to embed.
	PSFont *type1.Font

	// SubsetTag should be a unique tag for the font subset,
	// or the empty string if this is the full font.
	SubsetTag string

	// Encoding (a slice of length 256) is the encoding vector used by the client.
	// This is used to determine the `Encoding` entry of the PDF font dictionary.
	Encoding []string

	// ToUnicode (optional) is a map from character codes to unicode strings.
	// Character codes must be in the range 0, ..., 255.
	ToUnicode map[charcode.CharCode][]rune

	// ResName is the resource name for the font.
	// This is only used for PDF version 1.0.
	ResName pdf.Name

	IsSerif    bool
	IsScript   bool
	IsAllCap   bool
	IsSmallCap bool
}

func (info *EmbedInfo) Embed(w pdf.Putter, fontDictRef pdf.Reference) error {
	useBuiltin := w.GetMeta().Version < pdf.V2_0 && IsBuiltin(info.PSFont)

	if len(info.Encoding) != 256 || len(info.PSFont.Encoding) != 256 {
		panic("unreachable") // TODO(voss): remove
	}

	fontName := info.PSFont.FontInfo.FontName
	if info.SubsetTag != "" {
		fontName = info.SubsetTag + "+" + fontName
	}

	var toUnicodeRef pdf.Reference
	var fontFileRef pdf.Reference

	// See section 9.6.2.1 of PDF 32000-1:2008.
	fontDict := pdf.Dict{
		"Type":     pdf.Name("Font"),
		"Subtype":  pdf.Name("Type1"),
		"BaseFont": pdf.Name(fontName),
	}
	if w.GetMeta().Version == pdf.V1_0 {
		fontDict["Name"] = info.ResName
	}
	if enc := font.DescribeEncodingType1(info.Encoding, info.PSFont.Encoding); enc != nil {
		fontDict["Encoding"] = enc
	}
	if info.ToUnicode != nil {
		toUnicodeRef = w.Alloc()
		fontDict["ToUnicode"] = toUnicodeRef
	}
	compressedRefs := []pdf.Reference{fontDictRef}
	compressedObjects := []pdf.Object{fontDict}

	if !useBuiltin {
		psFont := info.PSFont

		widthsRef := w.Alloc()
		ww := make([]funit.Int16, 256)
		for i := range ww {
			ww[i] = psFont.GlyphInfo[info.Encoding[i]].WidthX
		}
		widthsInfo := font.CompressWidths(ww, psFont.UnitsPerEm)
		fontDict["FirstChar"] = widthsInfo.FirstChar
		fontDict["LastChar"] = widthsInfo.LastChar
		fontDict["Widths"] = widthsRef
		compressedRefs = append(compressedRefs, widthsRef)
		compressedObjects = append(compressedObjects, widthsInfo.Widths)

		FontDescriptorRef := w.Alloc()
		fontDict["FontDescriptor"] = FontDescriptorRef

		q := 1000 / float64(psFont.UnitsPerEm)
		bbox := psFont.BBox()
		fontBBox := &pdf.Rectangle{
			LLx: bbox.LLx.AsFloat(q),
			LLy: bbox.LLy.AsFloat(q),
			URx: bbox.URx.AsFloat(q),
			URy: bbox.URy.AsFloat(q),
		}

		// TODO(voss): correctly set the isSymbolic flag?
		isSymbolic := true

		fd := &font.Descriptor{
			FontName:     fontName,
			IsFixedPitch: psFont.FontInfo.IsFixedPitch,
			IsSerif:      info.IsSerif,
			IsScript:     info.IsScript,
			IsItalic:     psFont.FontInfo.ItalicAngle != 0,
			IsAllCap:     info.IsAllCap,
			IsSmallCap:   info.IsSmallCap,
			ForceBold:    psFont.Private.ForceBold,
			FontBBox:     fontBBox,
			ItalicAngle:  psFont.FontInfo.ItalicAngle,
			Ascent:       psFont.Ascent.AsFloat(q),
			Descent:      psFont.Descent.AsFloat(q),
			CapHeight:    psFont.CapHeight.AsFloat(q),
			// XHeight:      psFont.XHeight.AsFloat(q),
			StemV:        psFont.Private.StdVW * q,
			MissingWidth: widthsInfo.MissingWidth,
		}
		fontDescriptor := fd.AsDict(isSymbolic)
		if psFont.Outlines != nil {
			fontFileRef = w.Alloc()
			fontDescriptor["FontFile"] = fontFileRef
		}
		compressedRefs = append(compressedRefs, FontDescriptorRef)
		compressedObjects = append(compressedObjects, fontDescriptor)
	}

	err := w.WriteCompressed(compressedRefs, compressedObjects...)
	if err != nil {
		return pdf.Wrap(err, "Type 1 font dicts")
	}

	if fontFileRef != 0 {
		// See section 9.9 of PDF 32000-1:2008.
		length1 := pdf.NewPlaceholder(w, 10)
		length2 := pdf.NewPlaceholder(w, 10)
		fontFileDict := pdf.Dict{
			"Length1": length1,
			"Length2": length2,
			"Length3": pdf.Integer(0),
		}
		fontFileStream, err := w.OpenStream(fontFileRef, fontFileDict, pdf.FilterCompress{})
		if err != nil {
			return err
		}
		l1, l2, err := info.PSFont.WritePDF(fontFileStream)
		if err != nil {
			return err
		}
		length1.Set(pdf.Integer(l1))
		length2.Set(pdf.Integer(l2))
		err = fontFileStream.Close()
		if err != nil {
			return err
		}
	}

	if toUnicodeRef != 0 {
		err = tounicode.Embed(w, toUnicodeRef, charcode.Simple, info.ToUnicode)
		if err != nil {
			return err
		}
	}

	return nil
}

func Extract(r pdf.Getter, dicts *font.Dicts) (*EmbedInfo, error) {
	subType, err := pdf.GetName(r, dicts.FontDict["Subtype"])
	if err != nil || subType != "Type1" {
		return nil, errors.New("not a Type 1 font")
	}

	res := &EmbedInfo{}

	if dicts.FontProgramKey == "FontFile" {
		stmObj, err := pdf.GetStream(r, dicts.FontProgram)
		if err != nil {
			return nil, err
		}
		stm, err := pdf.DecodeStream(r, stmObj, 0)
		if err != nil {
			return nil, err
		}
		t1, err := type1.Read(stm)
		if err != nil {
			return nil, err
		}

		unitsPerEm := uint16(math.Round(1 / t1.FontInfo.FontMatrix[0]))
		t1.UnitsPerEm = unitsPerEm

		q := 1000 * t1.FontInfo.FontMatrix[0]

		ascent, _ := pdf.GetNumber(r, dicts.FontDescriptor["Ascent"])
		t1.Ascent = funit.Int16(math.Round(float64(ascent) / q))
		descent, _ := pdf.GetNumber(r, dicts.FontDescriptor["Descent"])
		t1.Descent = funit.Int16(math.Round(float64(descent) / q))
		capHeight, _ := pdf.GetNumber(r, dicts.FontDescriptor["CapHeight"])
		t1.CapHeight = funit.Int16(math.Round(float64(capHeight) / q))
		xHeight, _ := pdf.GetNumber(r, dicts.FontDescriptor["XHeight"]) // optional
		t1.XHeight = funit.Int16(math.Round(float64(xHeight) / q))

		res.PSFont = t1
	}

	baseFont, _ := pdf.GetName(r, dicts.FontDict["BaseFont"])
	if m := subset.TagRegexp.FindStringSubmatch(string(baseFont)); m != nil {
		res.SubsetTag = m[1]
	}

	if res.PSFont != nil {
		encoding, err := font.UndescribeEncodingType1(
			r, dicts.FontDict["Encoding"], res.PSFont.Encoding)
		if err != nil {
			return nil, err
		}
		res.Encoding = encoding
	}

	if info, _ := tounicode.Extract(r, dicts.FontDict["ToUnicode"]); info != nil {
		// TODO(voss): check that the codespace ranges are compatible with the cmap.
		res.ToUnicode = info.GetMapping()
	}

	res.ResName, _ = pdf.GetName(r, dicts.FontDict["Name"])

	flagsInt, _ := pdf.GetInteger(r, dicts.FontDescriptor["Flags"])
	flags := font.Flags(flagsInt)
	res.IsSerif = flags&font.FlagSerif != 0
	res.IsScript = flags&font.FlagScript != 0
	res.IsAllCap = flags&font.FlagAllCap != 0
	res.IsSmallCap = flags&font.FlagSmallCap != 0

	return res, nil
}
