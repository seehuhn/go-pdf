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
	"errors"
	"fmt"
	"math"

	"golang.org/x/text/language"
	"seehuhn.de/go/postscript/funit"
	"seehuhn.de/go/postscript/type1"

	"seehuhn.de/go/sfnt"
	"seehuhn.de/go/sfnt/cff"
	"seehuhn.de/go/sfnt/glyph"
	"seehuhn.de/go/sfnt/opentype/gtab"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/subset"
	"seehuhn.de/go/pdf/font/tounicode"
)

type SimpleFontCFF struct {
	info        *sfnt.Font
	gsubLookups []gtab.LookupIndex
	gposLookups []gtab.LookupIndex
	*font.Geometry
}

func NewSimpleCFF(info *sfnt.Font, loc language.Tag) (*SimpleFontCFF, error) {
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

	res := &SimpleFontCFF{
		info:        info,
		gsubLookups: info.Gsub.FindLookups(loc, gtab.GsubDefaultFeatures),
		gposLookups: info.Gpos.FindLookups(loc, gtab.GposDefaultFeatures),
		Geometry:    geometry,
	}
	return res, nil
}

func (f *SimpleFontCFF) Embed(w pdf.Putter, resName pdf.Name) (font.Embedded, error) {
	err := pdf.CheckVersion(w, "use of OpenType fonts", pdf.V1_6)
	if err != nil {
		return nil, err
	}
	res := &embeddedSimpleCFF{
		SimpleFontCFF: f,
		w:             w,
		Resource:      pdf.Resource{Ref: w.Alloc(), Name: resName},
		SimpleEncoder: cmap.NewSimpleEncoder(),
		text:          map[glyph.ID][]rune{},
	}
	w.AutoClose(res)
	return res, nil
}

func (f *SimpleFontCFF) Layout(s string, ptSize float64) glyph.Seq {
	rr := []rune(s)
	return f.info.Layout(rr, f.gsubLookups, f.gposLookups)
}

type embeddedSimpleCFF struct {
	*SimpleFontCFF
	w pdf.Putter
	pdf.Resource

	cmap.SimpleEncoder
	text   map[glyph.ID][]rune
	closed bool
}

func (f *embeddedSimpleCFF) AppendEncoded(s pdf.String, gid glyph.ID, rr []rune) pdf.String {
	f.text[gid] = rr
	return f.SimpleEncoder.AppendEncoded(s, gid, rr)
}

func (f *embeddedSimpleCFF) Close() error {
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
	info := EmbedInfoSimpleCFF{
		Font:      subsetInfo,
		SubsetTag: subsetTag,
		Encoding:  subsetInfo.Outlines.(*cff.Outlines).Encoding,
		ToUnicode: toUnicode,
	}
	return info.Embed(f.w, f.Ref)
}

type EmbedInfoSimpleCFF struct {
	// Font is the font to embed (already subsetted, if needed).
	Font *sfnt.Font

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

	IsAllCap   bool
	IsSmallCap bool
}

func (info *EmbedInfoSimpleCFF) Embed(w pdf.Putter, fontDictRef pdf.Reference) error {
	err := pdf.CheckVersion(w, "simple OpenType/CFF fonts", pdf.V1_6)
	if err != nil {
		return err
	}

	otf := info.Font
	cff, ok := otf.Outlines.(*cff.Outlines)
	if !ok ||
		len(cff.Encoding) != 256 ||
		len(cff.Private) != 1 ||
		len(cff.Glyphs) == 0 ||
		len(cff.Glyphs[0].Name) == 0 {
		return errors.New("not a simple OpenType/CFF font")
	}

	fontName := otf.PostscriptName()
	if info.SubsetTag != "" {
		fontName = info.SubsetTag + "+" + fontName
	}

	unitsPerEm := otf.UnitsPerEm

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
	info.Font.CMap = nil
	// TODO(voss): for all other font types, check for unnecessary "cmap" tables.

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

	isSymbolic := true // TODO(voss): check this

	fd := &font.Descriptor{
		FontName:     fontName,
		IsFixedPitch: otf.IsFixedPitch(),
		IsSerif:      otf.IsSerif,
		IsScript:     otf.IsScript,
		IsItalic:     otf.IsItalic,
		IsAllCap:     info.IsAllCap,
		IsSmallCap:   info.IsSmallCap,
		ForceBold:    cff.Private[0].ForceBold,
		FontBBox:     fontBBox,
		ItalicAngle:  otf.ItalicAngle,
		Ascent:       otf.Ascent.AsFloat(q),
		Descent:      otf.Descent.AsFloat(q),
		CapHeight:    otf.CapHeight.AsFloat(q),
		StemV:        cff.Private[0].StdVW * q,
		MissingWidth: widthsInfo.MissingWidth,
	}
	fontDescriptor := fd.AsDict(isSymbolic)
	fontDescriptor["FontFile3"] = fontFileRef

	compressedRefs := []pdf.Reference{fontDictRef, fontDescriptorRef, widthsRef}
	compressedObjects := []pdf.Object{fontDict, fontDescriptor, widthsInfo.Widths}
	err = w.WriteCompressed(compressedRefs, compressedObjects...)
	if err != nil {
		return pdf.Wrap(err, "simple OpenType/CFF font dicts")
	}

	// See section 9.9 of PDF 32000-1:2008 for details.
	fontFileDict := pdf.Dict{
		"Subtype": pdf.Name("OpenType"),
	}
	fontFileStream, err := w.OpenStream(fontFileRef, fontFileDict, pdf.FilterCompress{})
	if err != nil {
		return err
	}
	err = otf.WriteCFFOpenTypePDF(fontFileStream)
	if err != nil {
		return fmt.Errorf("embedding OpenType/CFF font %q: %w", fontName, err)
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

func ExtractSimpleCFF(r pdf.Getter, dicts *font.Dicts) (*EmbedInfoSimpleCFF, error) {
	if dicts.Type != font.SimpleOpenTypeCFF {
		return nil, fmt.Errorf("expected %q, got %q", font.SimpleOpenTypeCFF, dicts.Type)
	}
	res := &EmbedInfoSimpleCFF{}

	baseFont, _ := pdf.GetName(r, dicts.FontDict["BaseFont"])
	if m := subset.TagRegexp.FindStringSubmatch(string(baseFont)); m != nil {
		res.SubsetTag = m[1]
	}

	if dicts.FontProgram != nil {
		stm, err := pdf.DecodeStream(r, dicts.FontProgram, 0)
		if err != nil {
			return nil, pdf.Wrap(err, "uncompressing OpenType font stream")
		}
		otf, err := sfnt.Read(stm)
		if err != nil {
			return nil, pdf.Wrap(err, "decoding OpenType font")
		}
		res.Font = otf
	}

	if res.Font != nil {
		cff, ok := res.Font.Outlines.(*cff.Outlines)
		if !ok {
			return nil, fmt.Errorf("expected CFF outlines, got %T", res.Font.Outlines)
		}

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

		q := 1000 / float64(res.Font.UnitsPerEm)
		ascent, _ := pdf.GetNumber(r, dicts.FontDescriptor["Ascent"])
		res.Font.Ascent = funit.Int16(math.Round(float64(ascent) / q))
		descent, _ := pdf.GetNumber(r, dicts.FontDescriptor["Descent"])
		res.Font.Descent = funit.Int16(math.Round(float64(descent) / q))
		capHeight, _ := pdf.GetNumber(r, dicts.FontDescriptor["CapHeight"])
		res.Font.CapHeight = funit.Int16(math.Round(float64(capHeight) / q))
		xHeight, _ := pdf.GetNumber(r, dicts.FontDescriptor["XHeight"]) // optional
		res.Font.XHeight = funit.Int16(math.Round(float64(xHeight) / q))
		leading, _ := pdf.GetNumber(r, dicts.FontDescriptor["Leading"])
		if ascent-descent < leading {
			res.Font.LineGap = funit.Int16(math.Round(float64(leading-ascent+descent) / q))
		}
	}

	flagsInt, _ := pdf.GetInteger(r, dicts.FontDescriptor["Flags"])
	flags := font.Flags(flagsInt)
	res.IsAllCap = flags&font.FlagAllCap != 0
	res.IsSmallCap = flags&font.FlagSmallCap != 0

	if info, _ := tounicode.Extract(r, dicts.FontDict["ToUnicode"]); info != nil {
		// TODO(voss): check that the codespace ranges are compatible with the cmap.
		res.ToUnicode = info.GetMapping()
	}

	return res, nil
}
