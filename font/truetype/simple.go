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

package truetype

import (
	"fmt"
	"math"

	"seehuhn.de/go/postscript/funit"
	"seehuhn.de/go/postscript/type1/names"

	"seehuhn.de/go/sfnt"
	sfntcmap "seehuhn.de/go/sfnt/cmap"
	"seehuhn.de/go/sfnt/glyf"
	"seehuhn.de/go/sfnt/glyph"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/encoding"
	"seehuhn.de/go/pdf/font/pdfenc"
	"seehuhn.de/go/pdf/font/subset"
	"seehuhn.de/go/pdf/font/widths"
)

type embeddedSimple struct {
	w   *pdf.Writer
	ref pdf.Reference

	sfnt *sfnt.Font

	*encoding.SimpleEncoder

	closed bool
}

// WritingMode implements the [font.Embedded] interface.
func (f *embeddedSimple) WritingMode() cmap.WritingMode {
	return 0
}

func (f *embeddedSimple) DecodeWidth(s pdf.String) (float64, int) {
	if len(s) == 0 {
		return 0, 0
	}
	gid := f.Encoding[s[0]]
	return f.sfnt.GlyphWidthPDF(gid), 1
}

func (f *embeddedSimple) CodeAndWidth(s pdf.String, gid glyph.ID, rr []rune) (pdf.String, float64, bool) {
	width := float64(f.sfnt.GlyphWidth(gid)) / float64(f.sfnt.UnitsPerEm)
	c := f.GIDToCode(gid, rr)
	return append(s, c), width, c == ' '
}

func (f *embeddedSimple) Close() error {
	if f.closed {
		return nil
	}
	f.closed = true

	if f.SimpleEncoder.Overflow() {
		return fmt.Errorf("too many distinct glyphs used in font %q",
			f.sfnt.PostScriptName())
	}
	encoding := f.SimpleEncoder.Encoding

	origSfnt := f.sfnt.Clone()
	origSfnt.CMapTable = nil
	origSfnt.Gdef = nil
	origSfnt.Gsub = nil
	origSfnt.Gpos = nil

	// subset the font
	subsetGID := f.SimpleEncoder.Subset()
	subsetTag := subset.Tag(subsetGID, origSfnt.NumGlyphs())
	subsetSfnt, err := origSfnt.Subset(subsetGID)
	if err != nil {
		return fmt.Errorf("TrueType font subset: %w", err)
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

	info := FontDictSimple{
		Font:      subsetSfnt,
		SubsetTag: subsetTag,
		Encoding:  subsetEncoding,
		ToUnicode: toUnicode,
	}
	return info.Embed(f.w, f.ref)
}

// FontDictSimple is the information needed to embed a simple TrueType font.
type FontDictSimple struct {
	// Font is the font to embed (already subsetted, if needed).
	Font *sfnt.Font

	// SubsetTag should be a unique tag for the font subset,
	// or the empty string if this is the full font.
	SubsetTag string

	// Encoding is the encoding vector used by the client (a slice of length 256).
	// This may be different from the font's built-in encoding.
	Encoding []glyph.ID

	ForceBold bool

	IsAllCap   bool
	IsSmallCap bool

	// ToUnicode (optional) is a map from character codes to unicode strings.
	ToUnicode *cmap.ToUnicode
}

// ExtractSimple extracts information about a simple TrueType font.
// This is the inverse of [FontDictSimple.Embed].
func ExtractSimple(r pdf.Getter, dicts *font.Dicts) (*FontDictSimple, error) {
	if err := dicts.Type.MustBe(font.TrueTypeSimple); err != nil {
		return nil, err
	}

	res := &FontDictSimple{}

	baseFont, _ := pdf.GetName(r, dicts.FontDict["BaseFont"])
	if m := subset.TagRegexp.FindStringSubmatch(string(baseFont)); m != nil {
		res.SubsetTag = m[1]
	}

	if dicts.FontProgram != nil {
		stm, err := pdf.DecodeStream(r, dicts.FontProgram, 0)
		if err != nil {
			return nil, pdf.Wrap(err, "TrueType font stream")
		}
		ttf, err := sfnt.Read(stm)
		if err != nil {
			return nil, pdf.Wrap(err, "reading TrueType font stream")
		}
		_, ok := ttf.Outlines.(*glyf.Outlines)
		if !ok {
			return nil, fmt.Errorf("expected glyf outlines, got %T", ttf.Outlines)
		}

		// TODO(voss): use the glyph widths from the font dictionaries.

		res.Font = ttf
	}

	if res.Font != nil {
		res.Encoding = ExtractEncoding(r, dicts.FontDict["Encoding"], res.Font)
	}

	if ttf := res.Font; ttf != nil {
		if ttf.FamilyName == "" {
			ttf.FamilyName = dicts.FontDescriptor.FontFamily
		}
		if ttf.Width == 0 {
			ttf.Width = dicts.FontDescriptor.FontStretch
		}
		if ttf.Weight == 0 {
			ttf.Weight = dicts.FontDescriptor.FontWeight
		}
		q := 1000 / float64(ttf.UnitsPerEm)
		if ttf.CapHeight == 0 {
			capHeight := dicts.FontDescriptor.CapHeight
			ttf.CapHeight = funit.Int16(math.Round(float64(capHeight) / q))
		}
		if ttf.XHeight == 0 {
			xHeight := dicts.FontDescriptor.XHeight
			ttf.XHeight = funit.Int16(math.Round(float64(xHeight) / q))
		}
	}
	res.IsAllCap = dicts.FontDescriptor.IsAllCap
	res.IsSmallCap = dicts.FontDescriptor.IsSmallCap
	res.ForceBold = dicts.FontDescriptor.ForceBold

	if info, _ := cmap.ExtractToUnicode(r, dicts.FontDict["ToUnicode"], charcode.Simple); info != nil {
		res.ToUnicode = info
	}

	return res, nil
}

// Embed adds the font to a PDF file.
// This implements the [font.Dict] interface.
// This is the reverse of [ExtractSimple]
func (info *FontDictSimple) Embed(w *pdf.Writer, fontDictRef pdf.Reference) error {
	err := pdf.CheckVersion(w, "simple TrueType fonts", pdf.V1_1)
	if err != nil {
		return err
	}

	sfnt := info.Font.Clone()
	if !sfnt.IsGlyf() {
		return fmt.Errorf("not a TrueType font")
	}

	fontName := sfnt.PostScriptName()
	if info.SubsetTag != "" {
		fontName = info.SubsetTag + "+" + fontName
	}

	ww := make([]float64, 256)
	q := 1000 / float64(sfnt.UnitsPerEm)
	for i := range ww {
		ww[i] = float64(sfnt.GlyphWidth(info.Encoding[i])) * q
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
	for code, gid := range info.Encoding {
		if gid == 0 {
			continue
		}
		subtable[uint16(code)] = gid
	}
	sfnt.CMapTable = sfntcmap.Table{
		{PlatformID: 1, EncodingID: 0}: subtable.Encode(0),
	}

	bbox := sfnt.BBox()
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
		IsFixedPitch: sfnt.IsFixedPitch(),
		IsSerif:      sfnt.IsSerif,
		IsSymbolic:   isSymbolic,
		IsScript:     sfnt.IsScript,
		IsItalic:     sfnt.IsItalic,
		IsAllCap:     info.IsAllCap,
		IsSmallCap:   info.IsSmallCap,
		ForceBold:    info.ForceBold,
		FontBBox:     fontBBox,
		ItalicAngle:  sfnt.ItalicAngle,
		Ascent:       sfnt.Ascent.AsFloat(q),
		Descent:      sfnt.Descent.AsFloat(q),
		CapHeight:    sfnt.CapHeight.AsFloat(q),
		MissingWidth: widthsInfo.MissingWidth,
	}
	fontDescriptor := fd.AsDict()
	fontDescriptor["FontFile2"] = fontFileRef

	compressedRefs := []pdf.Reference{fontDictRef, fontDescriptorRef, widthsRef}
	compressedObjects := []pdf.Object{fontDict, fontDescriptor, widthsInfo.Widths}
	err = w.WriteCompressed(compressedRefs, compressedObjects...)
	if err != nil {
		return pdf.Wrap(err, "simple TrueType font dicts")
	}

	// See section 9.9 of PDF 32000-1:2008 for details.
	length1 := pdf.NewPlaceholder(w, 10)
	fontFileDict := pdf.Dict{
		"Length1": length1,
	}
	fontFileStream, err := w.OpenStream(fontFileRef, fontFileDict, pdf.FilterCompress{})
	if err != nil {
		return err
	}
	n, err := sfnt.WriteTrueTypePDF(fontFileStream)
	if err != nil {
		return fmt.Errorf("TrueType font program %q: %w", fontName, err)
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

// ExtractEncoding tries to extract an encoding vector from the given encoding
// dictionary.  See section 9.6.5.4 of ISO 32000-2:2020.
//
// TODO(voss): revisit this, once
// https://github.com/pdf-association/pdf-issues/issues/316 is resolved.
func ExtractEncoding(r pdf.Getter, encodingDict pdf.Object, ttf *sfnt.Font) []glyph.ID {
	if encodingEntry, _ := pdf.Resolve(r, encodingDict); encodingEntry != nil {
		encodingNames, _ := encoding.UndescribeEncodingType1(r, encodingEntry, pdfenc.StandardEncoding[:])
		for i, name := range encodingNames {
			if name == ".notdef" {
				encodingNames[i] = pdfenc.StandardEncoding[i]
			}
		}

		cmap, _ := ttf.CMapTable.GetNoLang(3, 1)
		if cmap != nil {
			encoding := make([]glyph.ID, 256)
			for code, name := range encodingNames {
				rr := names.ToUnicode(name, false)
				if len(rr) == 1 {
					encoding[code] = cmap.Lookup(rr[0])
				}
			}
			return encoding
		}
		// TODO(voss): also try to use a (1,0) subtable together with encodingNames
	}

	cmap, _ := ttf.CMapTable.GetNoLang(3, 0)
	if cmap != nil {
		encoding := make([]glyph.ID, 256)
		for code := rune(0); code < 256; code++ {
			for _, pfx := range []rune{0xF000, 0xF100, 0xF200, 0x0000} {
				if cmap.Lookup(pfx+code) != 0 {
					encoding[code] = cmap.Lookup(pfx | code)
					break
				}
			}
		}
		return encoding
	}

	cmap, _ = ttf.CMapTable.GetNoLang(1, 0)
	if cmap != nil {
		encoding := make([]glyph.ID, 256)
		for code := rune(0); code < 256; code++ {
			encoding[code] = cmap.Lookup(code)
		}
		return encoding
	}

	// encoding := make([]glyph.ID, 256)
	// for i := range encoding {
	// 	encoding[i] = glyph.ID(i)
	// }
	// return encoding

	return nil
}
