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

package opentype

import (
	"fmt"
	"math"

	"seehuhn.de/go/postscript/funit"

	"seehuhn.de/go/sfnt"
	sfntcmap "seehuhn.de/go/sfnt/cmap"
	"seehuhn.de/go/sfnt/glyf"
	"seehuhn.de/go/sfnt/glyph"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/encoding"
	"seehuhn.de/go/pdf/font/subset"
	"seehuhn.de/go/pdf/font/truetype"
	"seehuhn.de/go/pdf/font/widths"
)

type embeddedGlyfSimple struct {
	w   pdf.Putter
	ref pdf.Reference

	sfnt *sfnt.Font

	*encoding.SimpleEncoder

	closed bool
}

func (f *embeddedGlyfSimple) ForeachWidth(s pdf.String, yield func(width float64, isSpace bool)) {
	for _, c := range s {
		gid := f.Encoding[c]
		width := float64(f.sfnt.GlyphWidth(gid)) / float64(f.sfnt.UnitsPerEm)
		yield(width, c == ' ')
	}
}

func (f *embeddedGlyfSimple) CodeAndWidth(s pdf.String, gid glyph.ID, rr []rune) (pdf.String, float64, bool) {
	width := float64(f.sfnt.GlyphWidth(gid)) / float64(f.sfnt.UnitsPerEm)
	c := f.GIDToCode(gid, rr)
	return append(s, c), width, c == ' '
}

func (f *embeddedGlyfSimple) Close() error {
	if f.closed {
		return nil
	}
	f.closed = true

	if f.SimpleEncoder.Overflow() {
		return fmt.Errorf("too many distinct glyphs used in font %q",
			f.sfnt.PostScriptName())
	}
	encoding := f.SimpleEncoder.Encoding

	origOTF := f.sfnt.Clone()
	origOTF.CMapTable = nil
	origOTF.Gdef = nil
	origOTF.Gsub = nil
	origOTF.Gpos = nil

	// subset the font
	subsetGID := f.SimpleEncoder.Subset()
	subsetTag := subset.Tag(subsetGID, origOTF.NumGlyphs())
	subsetOTF, err := origOTF.Subset(subsetGID)
	if err != nil {
		return fmt.Errorf("font subset: %w", err)
	}

	subsetGid := make(map[glyph.ID]glyph.ID)
	for gNew, gOld := range subsetGID {
		subsetGid[gOld] = glyph.ID(gNew)
	}
	subsetEncoding := make([]glyph.ID, 256)
	for i, gid := range encoding {
		subsetEncoding[i] = subsetGid[gid]
	}

	m := f.SimpleEncoder.ToUnicode()
	toUnicode := cmap.NewToUnicode(charcode.Simple, m)
	// TODO(voss): check whether a ToUnicode CMap is actually needed

	info := FontDictGlyfSimple{
		Font:      subsetOTF,
		SubsetTag: subsetTag,
		Encoding:  subsetEncoding,
		ToUnicode: toUnicode,
	}
	return info.Embed(f.w, f.ref)
}

// FontDictGlyfSimple is the information needed to embed a simple OpenType/glyf font.
type FontDictGlyfSimple struct {
	// Font is the font to embed (already subsetted, if needed).
	Font *sfnt.Font

	// SubsetTag should be a unique tag for the font subset,
	// or the empty string if this is the full font.
	SubsetTag string

	// Encoding is the encoding vector used by the client (a slice of length 256).
	// Together with the font's built-in encoding, this is used to determine
	// the `Encoding` entry of the PDF font dictionary.
	Encoding []glyph.ID

	ForceBold bool

	IsAllCap   bool
	IsSmallCap bool

	// ToUnicode (optional) is a map from character codes to unicode strings.
	ToUnicode *cmap.ToUnicode
}

// Embed adds the font to a PDF file.
//
// This implements the [font.Dict] interface.
func (info *FontDictGlyfSimple) Embed(w pdf.Putter, fontDictRef pdf.Reference) error {
	err := pdf.CheckVersion(w, "simple OpenType/glyf fonts", pdf.V1_6)
	if err != nil {
		return err
	}

	otf := info.Font.Clone()
	if !otf.IsGlyf() {
		return fmt.Errorf("not an OpenType/glyf font")
	}

	fontName := otf.PostScriptName()
	if info.SubsetTag != "" {
		fontName = info.SubsetTag + "+" + fontName
	}

	unitsPerEm := otf.UnitsPerEm

	ww := make([]float64, 256)
	q := 1000 / float64(unitsPerEm)
	for i := range ww {
		ww[i] = float64(otf.GlyphWidth(info.Encoding[i])) * q
	}
	widthsInfo := widths.EncodeSimple(ww)

	// Mark the font as "symbolic", and use a (1, 0) "cmap" subtable to map
	// character codes to glyphs.
	//
	// TODO(voss): also try the two allowed encodings for "non-symbolic" fonts.
	//
	// TODO(voss): revisit this, once
	// https://github.com/pdf-association/pdf-issues/issues/316 is resolved.
	isSymbolic := true
	subtable := sfntcmap.Format4{}
	for i, gid := range info.Encoding {
		if gid == 0 {
			continue
		}
		subtable[uint16(i)] = gid
	}
	otf.CMapTable = sfntcmap.Table{
		{PlatformID: 1, EncodingID: 0}: subtable.Encode(0),
	}

	bbox := otf.BBox()
	fontBBox := &pdf.Rectangle{
		LLx: bbox.LLx.AsFloat(q),
		LLy: bbox.LLy.AsFloat(q),
		URx: bbox.URx.AsFloat(q),
		URy: bbox.URy.AsFloat(q),
	}

	widthsRef := w.Alloc()
	fontDescriptorRef := w.Alloc()
	fontFileRef := w.Alloc()

	// See section 9.6.2.1 of PDF 32000-1:2008.
	fontDict := pdf.Dict{
		"Type":           pdf.Name("Font"),
		"Subtype":        pdf.Name("TrueType"),
		"BaseFont":       pdf.Name(fontName),
		"FirstChar":      widthsInfo.FirstChar,
		"LastChar":       widthsInfo.LastChar,
		"Widths":         widthsRef,
		"FontDescriptor": fontDescriptorRef,
	}
	var toUnicodeRef pdf.Reference
	if info.ToUnicode != nil {
		toUnicodeRef = w.Alloc()
		fontDict["ToUnicode"] = toUnicodeRef
	}
	fd := &font.Descriptor{
		FontName:     fontName,
		IsFixedPitch: otf.IsFixedPitch(),
		IsSerif:      otf.IsSerif,
		IsSymbolic:   isSymbolic,
		IsScript:     otf.IsScript,
		IsItalic:     otf.IsItalic,
		IsAllCap:     info.IsAllCap,
		IsSmallCap:   info.IsSmallCap,
		ForceBold:    info.ForceBold,
		FontBBox:     fontBBox,
		ItalicAngle:  otf.ItalicAngle,
		Ascent:       otf.Ascent.AsFloat(q),
		Descent:      otf.Descent.AsFloat(q),
		CapHeight:    otf.CapHeight.AsFloat(q),
		MissingWidth: widthsInfo.MissingWidth,
	}
	fontDescriptor := fd.AsDict()
	fontDescriptor["FontFile3"] = fontFileRef

	compressedRefs := []pdf.Reference{fontDictRef, fontDescriptorRef, widthsRef}
	compressedObjects := []pdf.Object{fontDict, fontDescriptor, widthsInfo.Widths}
	err = w.WriteCompressed(compressedRefs, compressedObjects...)
	if err != nil {
		return pdf.Wrap(err, "simple OpenType/glyf font dicts")
	}

	// See section 9.9 of PDF 32000-1:2008 for details.
	length1 := pdf.NewPlaceholder(w, 10)
	fontFileDict := pdf.Dict{
		"Length1": length1, // TODO(voss): needed?
		"Subtype": pdf.Name("OpenType"),
	}
	fontFileStream, err := w.OpenStream(fontFileRef, fontFileDict, pdf.FilterCompress{})
	if err != nil {
		return err
	}
	n, err := otf.WriteTrueTypePDF(fontFileStream)
	if err != nil {
		return fmt.Errorf("embedding TrueType font %q: %w", fontName, err)
	}
	err = length1.Set(pdf.Integer(n))
	if err != nil {
		return err
	}
	err = fontFileStream.Close()
	if err != nil {
		return err
	}

	if toUnicodeRef != 0 {
		err = info.ToUnicode.Embed(w, toUnicodeRef)
		if err != nil {
			return err
		}
	}

	return nil
}

// ExtractGlyfSimple extracts information about a simple OpenType font.
func ExtractGlyfSimple(r pdf.Getter, dicts *font.Dicts) (*FontDictGlyfSimple, error) {
	if err := dicts.Type.MustBe(font.OpenTypeGlyfSimple); err != nil {
		return nil, err
	}

	res := &FontDictGlyfSimple{}

	baseFont, _ := pdf.GetName(r, dicts.FontDict["BaseFont"])
	if m := subset.TagRegexp.FindStringSubmatch(string(baseFont)); m != nil {
		res.SubsetTag = m[1]
	}

	if dicts.FontProgram != nil {
		stm, err := pdf.DecodeStream(r, dicts.FontProgram, 0)
		if err != nil {
			return nil, pdf.Wrap(err, "uncompressing OpenType/glyf font stream")
		}
		otf, err := sfnt.Read(stm)
		if err != nil {
			return nil, pdf.Wrap(err, "decoding OpenType/glyf font")
		}
		_, ok := otf.Outlines.(*glyf.Outlines)
		if !ok {
			return nil, fmt.Errorf("expected glyf outlines, got %T", otf.Outlines)
		}
		if otf.FamilyName == "" {
			otf.FamilyName = dicts.FontDescriptor.FontFamily
		}
		if otf.Width == 0 {
			otf.Width = dicts.FontDescriptor.FontStretch
		}
		if otf.Weight == 0 {
			otf.Weight = dicts.FontDescriptor.FontWeight
		}
		q := 1000 / float64(otf.UnitsPerEm)
		if otf.CapHeight == 0 {
			capHeight := dicts.FontDescriptor.CapHeight
			otf.CapHeight = funit.Int16(math.Round(float64(capHeight) / q))
		}
		if otf.XHeight == 0 {
			xHeight := dicts.FontDescriptor.XHeight
			otf.XHeight = funit.Int16(math.Round(float64(xHeight) / q))
		}
		res.Font = otf
	}

	if res.Font != nil {
		res.Encoding = truetype.ExtractEncoding(r, dicts.FontDict["Encoding"], res.Font)
	}

	res.IsAllCap = dicts.FontDescriptor.IsAllCap
	res.IsSmallCap = dicts.FontDescriptor.IsSmallCap
	res.ForceBold = dicts.FontDescriptor.ForceBold

	if info, _ := cmap.ExtractToUnicode(r, dicts.FontDict["ToUnicode"], charcode.Simple); info != nil {
		res.ToUnicode = info
	}

	return res, nil
}
