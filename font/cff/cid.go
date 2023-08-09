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

package cff

import (
	"fmt"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/tounicode"
	"seehuhn.de/go/postscript/funit"
	"seehuhn.de/go/postscript/type1"
	"seehuhn.de/go/sfnt/cff"
)

type PDFInfoCID struct {
	Font      *cff.Font
	SubsetTag string
	ToUnicode map[charcode.CharCode][]rune

	CS   charcode.CodeSpaceRange
	CMap map[charcode.CharCode]type1.CID
	ROS  *type1.CIDSystemInfo

	UnitsPerEm uint16 // TODO(voss): get this from the font matrix instead

	IsSerif    bool
	IsScript   bool
	IsAllCap   bool
	IsSmallCap bool

	Ascent    funit.Int16
	Descent   funit.Int16
	CapHeight funit.Int16
}

// Section 9.7.4.2 of ISO-32000-2 ("Glyph selection in CIDFonts"):
//
// If the "CFF" font program has a Top DICT that does not use CIDFont
// operators: The CIDs shall be used directly as GID values, and the glyph
// procedure shall be retrieved using the CharStrings INDEX.
//
// If the "CFF" font program has a Top DICT that uses CIDFont operators:
// The CIDs shall be used to determine the GID value for the glyph
// procedure using the charset table in the CFF program. The GID value
// shall then be used to look up the glyph procedure using the CharStrings
// INDEX table.

func (info *PDFInfoCID) WritePDF(w pdf.Putter, fontDictRef pdf.Reference) error {
	err := pdf.CheckVersion(w, "CFF CIDFonts", pdf.V1_3)
	if err != nil {
		return err
	}

	cffFont := info.Font

	// CidFontName shall be the value of the CIDFontName entry in the CIDFont program.
	// The name may have a subset prefix if appropriate.
	var cidFontName string
	if info.SubsetTag == "" {
		cidFontName = cffFont.FontInfo.FontName
	} else {
		cidFontName = info.SubsetTag + "+" + cffFont.FontInfo.FontName
	}

	var cmapName string     // TODO
	var encoding pdf.Object // TODO

	unitsPerEm := info.UnitsPerEm

	var DW pdf.Number // TODO
	var W pdf.Array   // TODO

	q := 1000 / float64(unitsPerEm)
	bbox := cffFont.BBox()
	fontBBox := &pdf.Rectangle{
		LLx: bbox.LLx.AsFloat(q),
		LLy: bbox.LLy.AsFloat(q),
		URx: bbox.URx.AsFloat(q),
		URy: bbox.URy.AsFloat(q),
	}

	var isSymbolic bool // TODO

	cidFontRef := w.Alloc()
	var toUnicodeRef pdf.Reference
	fontDescriptorRef := w.Alloc()
	fontFileRef := w.Alloc()

	fontDict := pdf.Dict{
		"Type":            pdf.Name("Font"),
		"Subtype":         pdf.Name("Type0"),
		"BaseFont":        pdf.Name(cidFontName + "-" + cmapName),
		"Encoding":        encoding,
		"DescendantFonts": pdf.Array{cidFontRef},
	}
	if info.ToUnicode != nil {
		toUnicodeRef = w.Alloc()
		fontDict["ToUnicode"] = toUnicodeRef
	}

	ROS := pdf.Dict{
		"Registry":   pdf.String(info.ROS.Registry),
		"Ordering":   pdf.String(info.ROS.Ordering),
		"Supplement": pdf.Integer(info.ROS.Supplement),
	}

	cidFontDict := pdf.Dict{
		"Type":           pdf.Name("Font"),
		"Subtype":        pdf.Name("CIDFontType0"),
		"BaseFont":       pdf.Name(cidFontName),
		"CIDSystemInfo":  ROS,
		"FontDescriptor": fontDescriptorRef,
	}
	if DW != 1000 {
		cidFontDict["DW"] = DW
	}
	if W != nil {
		cidFontDict["W"] = W
	}

	fd := &font.Descriptor{
		FontName:     cidFontName,
		IsFixedPitch: info.Font.IsFixedPitch,
		IsSerif:      info.IsSerif,
		IsScript:     info.IsScript,
		IsItalic:     info.Font.ItalicAngle != 0,
		IsAllCap:     info.IsAllCap,
		IsSmallCap:   info.IsSmallCap,
		ForceBold:    info.Font.Private[0].ForceBold,
		FontBBox:     fontBBox,
		ItalicAngle:  info.Font.ItalicAngle,
		Ascent:       info.Ascent.AsFloat(q),
		Descent:      info.Descent.AsFloat(q),
		CapHeight:    info.CapHeight.AsFloat(q),
	}
	fontDescriptor := fd.AsDict(isSymbolic)
	fontDescriptor["FontFile3"] = fontFileRef

	compressedRefs := []pdf.Reference{fontDictRef, cidFontRef, fontDescriptorRef}
	compressedObjects := []pdf.Object{fontDict, cidFontDict, fontDescriptor}
	err = w.WriteCompressed(compressedRefs, compressedObjects...)
	if err != nil {
		return err
	}

	// See section 9.9 of PDF 32000-1:2008 for details.
	fontFileDict := pdf.Dict{
		"Subtype": pdf.Name("CIDFontType0C"),
	}
	fontFileStream, err := w.OpenStream(fontFileRef, fontFileDict, pdf.FilterCompress{})
	if err != nil {
		return err
	}
	err = cffFont.Encode(fontFileStream)
	if err != nil {
		return fmt.Errorf("embedding CFF CIDFont %q: %w", cidFontName, err)
	}
	err = fontFileStream.Close()
	if err != nil {
		return err
	}

	if toUnicodeRef != 0 {
		err = tounicode.Embed(w, toUnicodeRef, charcode.Simple, info.ToUnicode)
		if err != nil {
			return err
		}
	}

	return nil
}
