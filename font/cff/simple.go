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
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"

	"golang.org/x/text/language"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/subset"
	"seehuhn.de/go/pdf/font/tounicode"
	"seehuhn.de/go/postscript/funit"
	"seehuhn.de/go/postscript/type1"
	"seehuhn.de/go/sfnt"
	"seehuhn.de/go/sfnt/cff"
	"seehuhn.de/go/sfnt/glyph"
	"seehuhn.de/go/sfnt/opentype/gtab"
)

type SimpleFont struct {
	info        *sfnt.Font
	gsubLookups []gtab.LookupIndex
	gposLookups []gtab.LookupIndex
	*font.Geometry
}

func NewSimple(info *sfnt.Font, loc language.Tag) (*SimpleFont, error) {
	if !info.IsCFF() {
		return nil, errors.New("wrong font type")
	}

	geometry := &font.Geometry{
		UnitsPerEm:   info.UnitsPerEm,
		GlyphExtents: info.Extents(),
		Widths:       info.Widths(),

		Ascent:             info.Ascent,
		Descent:            info.Descent,
		BaseLineSkip:       info.Ascent - info.Descent + info.LineGap,
		UnderlinePosition:  info.UnderlinePosition,
		UnderlineThickness: info.UnderlineThickness,
	}

	res := &SimpleFont{
		info:        info,
		gsubLookups: info.Gsub.FindLookups(loc, gtab.GsubDefaultFeatures),
		gposLookups: info.Gpos.FindLookups(loc, gtab.GposDefaultFeatures),
		Geometry:    geometry,
	}
	return res, nil
}

func (f *SimpleFont) Embed(w pdf.Putter, resName pdf.Name) (font.Embedded, error) {
	err := pdf.CheckVersion(w, "use of OpenType fonts", pdf.V1_6)
	if err != nil {
		return nil, err
	}
	res := &embeddedSimple{
		SimpleFont:    f,
		w:             w,
		Resource:      pdf.Resource{Ref: w.Alloc(), Name: resName},
		SimpleEncoder: cmap.NewSimpleEncoder(),
		text:          map[glyph.ID][]rune{},
	}
	w.AutoClose(res)
	return res, nil
}

func (f *SimpleFont) Layout(s string, ptSize float64) glyph.Seq {
	rr := []rune(s)
	return f.info.Layout(rr, f.gsubLookups, f.gposLookups)
}

type embeddedSimple struct {
	*SimpleFont
	w pdf.Putter
	pdf.Resource

	cmap.SimpleEncoder
	text   map[glyph.ID][]rune
	closed bool
}

func (f *embeddedSimple) AppendEncoded(s pdf.String, gid glyph.ID, rr []rune) pdf.String {
	f.text[gid] = rr
	return f.SimpleEncoder.AppendEncoded(s, gid, rr)
}

func (f *embeddedSimple) Close() error {
	if f.closed {
		return nil
	}
	f.closed = true

	if f.SimpleEncoder.Overflow() {
		return fmt.Errorf("too many distinct glyphs used in font %q (%s)",
			f.Name, f.info.PostscriptName())
	}
	f.SimpleEncoder = cmap.NewFrozenSimpleEncoder(f.SimpleEncoder)

	// subset the font
	var ss []subset.Glyph
	ss = append(ss, subset.Glyph{OrigGID: 0, CID: 0})
	encoding := f.SimpleEncoder.Encoding()
	for cid, gid := range encoding {
		if gid != 0 {
			ss = append(ss, subset.Glyph{OrigGID: gid, CID: type1.CID(cid)})
		}
	}
	subsetTag := subset.Tag(ss, f.info.NumGlyphs())
	subsetInfo, err := subset.Simple(f.info, ss)
	if err != nil {
		return fmt.Errorf("font subset: %w", err)
	}

	toUnicode := make(map[charcode.CharCode][]rune)
	for code, gid := range encoding {
		if gid == 0 || len(f.text[gid]) == 0 {
			continue
		}
		toUnicode[charcode.CharCode(code)] = f.text[gid]
	}
	info := EmbedInfoSimple{
		Font:       subsetInfo.AsCFF(),
		SubsetTag:  subsetTag,
		Encoding:   subsetInfo.Outlines.(*cff.Outlines).Encoding,
		ToUnicode:  toUnicode,
		UnitsPerEm: f.info.UnitsPerEm,
		Ascent:     f.info.Ascent,
		Descent:    f.info.Descent,
		CapHeight:  f.info.CapHeight,
		IsSerif:    f.info.IsScript,
		IsScript:   f.info.IsScript,
	}
	return info.Embed(f.w, f.Ref)
}

type EmbedInfoSimple struct {
	// Font is the font to embed (already subsetted, if needed).
	Font *cff.Font

	// SubsetTag should be a unique tag for the font subset,
	// or the empty string if this is the full font.
	SubsetTag string

	// Encoding is the encoding vector used by the client (a slice of length 256).
	// Together with the font's built-in encoding, this is used to determine
	// the `Encoding` entry of the PDF font dictionary.
	Encoding []glyph.ID

	// ToUnicode (optional) is a map from character codes to unicode strings.
	// Character codes must be in the range 0, ..., 255.
	// TODO(voss): or else?
	ToUnicode map[charcode.CharCode][]rune

	UnitsPerEm uint16 // TODO(voss): get this from the font matrix instead

	Ascent    funit.Int16
	Descent   funit.Int16
	CapHeight funit.Int16

	IsSerif    bool
	IsScript   bool
	IsAllCap   bool
	IsSmallCap bool
}

func (info *EmbedInfoSimple) Embed(w pdf.Putter, fontDictRef pdf.Reference) error {
	err := pdf.CheckVersion(w, "simple CFF fonts", pdf.V1_2)
	if err != nil {
		return err
	}

	cff := info.Font
	if len(cff.Encoding) != 256 ||
		len(cff.Private) != 1 ||
		len(cff.Glyphs) == 0 ||
		len(cff.Glyphs[0].Name) == 0 {
		return errors.New("font is not a simple CFF font")
	}

	fontName := cff.FontInfo.FontName
	if info.SubsetTag != "" {
		fontName = info.SubsetTag + "+" + fontName
	}

	unitsPerEm := info.UnitsPerEm

	ww := make([]funit.Int16, 256)
	for i := range ww {
		ww[i] = cff.Glyphs[info.Encoding[i]].Width
	}
	widthsInfo := font.CompressWidths(ww, unitsPerEm)

	encoding := make([]string, 256)
	builtin := make([]string, 256)
	for i := 0; i < 256; i++ {
		encoding[i] = cff.Glyphs[info.Encoding[i]].Name
		builtin[i] = cff.Glyphs[cff.Encoding[i]].Name
	}

	q := 1000 / float64(unitsPerEm)
	bbox := cff.BBox()
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
		"Subtype":        pdf.Name("Type1"),
		"BaseFont":       pdf.Name(fontName),
		"FirstChar":      widthsInfo.FirstChar,
		"LastChar":       widthsInfo.LastChar,
		"Widths":         widthsRef,
		"FontDescriptor": fontDescriptorRef,
	}
	if enc := font.DescribeEncodingType1(encoding, builtin); enc != nil {
		fontDict["Encoding"] = enc
	}
	var toUnicodeRef pdf.Reference
	if info.ToUnicode != nil {
		toUnicodeRef = w.Alloc()
		fontDict["ToUnicode"] = toUnicodeRef
	}

	fd := &font.Descriptor{
		FontName:     fontName,
		IsFixedPitch: cff.IsFixedPitch,
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
		StemV:        cff.Private[0].StdVW * q,
		MissingWidth: widthsInfo.MissingWidth,
	}
	fontDescriptor := fd.AsDict(true)
	fontDescriptor["FontFile3"] = fontFileRef

	compressedRefs := []pdf.Reference{fontDictRef, fontDescriptorRef, widthsRef}
	compressedObjects := []pdf.Object{fontDict, fontDescriptor, widthsInfo.Widths}
	err = w.WriteCompressed(compressedRefs, compressedObjects...)
	if err != nil {
		return pdf.Wrap(err, "simple CFF font dicts")
	}

	// See section 9.9 of PDF 32000-1:2008 for details.
	fontFileDict := pdf.Dict{
		"Subtype": pdf.Name("Type1C"),
	}
	fontFileStream, err := w.OpenStream(fontFileRef, fontFileDict, pdf.FilterCompress{})
	if err != nil {
		return err
	}
	err = cff.Encode(fontFileStream)
	if err != nil {
		return fmt.Errorf("embedding CFF font %q: %w", fontName, err)
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

func Extract(r pdf.Getter, dicts *font.Dicts) (*EmbedInfoSimple, error) {
	if dicts.Type != font.SimpleCFF {
		return nil, fmt.Errorf("expected %q, got %q", font.SimpleCFF, dicts.Type)
	}
	res := &EmbedInfoSimple{}

	if dicts.FontProgram != nil {
		stm, err := pdf.DecodeStream(r, dicts.FontProgram, 0)
		if err != nil {
			return nil, pdf.Wrap(err, "uncompressing CFF font stream")
		}
		data, err := io.ReadAll(stm)
		if err != nil {
			return nil, pdf.Wrap(err, "reading CFF font stream")
		}
		cff, err := cff.Read(bytes.NewReader(data))
		if err != nil {
			return nil, pdf.Wrap(err, "decoding CFF font")
		}

		res.Font = cff
	}

	baseFont, _ := pdf.GetName(r, dicts.FontDict["BaseFont"])
	if m := subset.TagRegexp.FindStringSubmatch(string(baseFont)); m != nil {
		res.SubsetTag = m[1]
	}

	var unitsPerEm uint16 = 1000
	if res.Font != nil {
		cff := res.Font
		builtin := make([]string, 256)
		for i := 0; i < 256; i++ {
			builtin[i] = cff.Glyphs[cff.Encoding[i]].Name
		}
		nameEncoding, err := font.UndescribeEncodingType1(
			r, dicts.FontDict["Encoding"], builtin)
		if err != nil {
			return nil, pdf.Wrap(err, "font encoding")
		}

		rev := make(map[string]glyph.ID)
		for i, g := range cff.Glyphs {
			rev[g.Name] = glyph.ID(i)
		}

		encoding := make([]glyph.ID, 256)
		for i, name := range nameEncoding {
			encoding[i] = rev[name]
		}
		res.Encoding = encoding

		if cff.FontMatrix[0] != 0 {
			unitsPerEm = uint16(math.Round(1 / cff.FontMatrix[0]))
		}
	}

	if info, _ := tounicode.Extract(r, dicts.FontDict["ToUnicode"]); info != nil {
		// TODO(voss): check that the codespace ranges are compatible with the cmap.
		res.ToUnicode = info.GetMapping()
	}

	res.UnitsPerEm = unitsPerEm

	q := 1000 / float64(unitsPerEm)
	ascent, err := pdf.GetNumber(r, dicts.FontDescriptor["Ascent"])
	if err == nil {
		res.Ascent = funit.Int16(math.Round(float64(ascent) / q))
	}
	descent, err := pdf.GetNumber(r, dicts.FontDescriptor["Descent"])
	if err == nil {
		res.Descent = funit.Int16(math.Round(float64(descent) / q))
	}
	capHeight, err := pdf.GetNumber(r, dicts.FontDescriptor["CapHeight"])
	if err == nil {
		res.CapHeight = funit.Int16(math.Round(float64(capHeight) / q))
	}

	flagsInt, _ := pdf.GetInteger(r, dicts.FontDescriptor["Flags"])
	flags := font.Flags(flagsInt)
	res.IsSerif = flags&font.FlagSerif != 0
	res.IsScript = flags&font.FlagScript != 0
	res.IsAllCap = flags&font.FlagAllCap != 0
	res.IsSmallCap = flags&font.FlagSmallCap != 0

	return res, nil
}
