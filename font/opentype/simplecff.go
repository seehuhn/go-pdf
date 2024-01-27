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

	"seehuhn.de/go/postscript/funit"

	"seehuhn.de/go/sfnt"
	"seehuhn.de/go/sfnt/cff"
	sfntcmap "seehuhn.de/go/sfnt/cmap"
	"seehuhn.de/go/sfnt/glyph"
	"seehuhn.de/go/sfnt/opentype/gtab"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/encoding"
	"seehuhn.de/go/pdf/font/subset"
	"seehuhn.de/go/pdf/graphics"
)

// fontCFFSimple is a OpenType/CFF font for embedding into a PDF file as a simple font.
type fontCFFSimple struct {
	sfnt        *sfnt.Font
	cmap        sfntcmap.Subtable
	gsubLookups []gtab.LookupIndex
	gposLookups []gtab.LookupIndex
	*font.Geometry
}

// NewCFFSimple allocates a new OpenType/CFF font for embedding in a PDF file as a simple font.
// Info must be an OpenType font with CFF outlines.
// If info is CID-keyed, the function will attempt to convert it to a simple font.
// If the conversion fails (because more than one private dictionary is used
// after subsetting), an error is returned.
// Consider using [cff.NewSimple] instead of this function.
func NewCFFSimple(info *sfnt.Font, opt *font.Options) (font.Font, error) {
	if !info.IsCFF() {
		return nil, errors.New("wrong font type")
	}

	opt = font.MergeOptions(opt, defaultOptionsCFF)

	geometry := &font.Geometry{
		UnitsPerEm:   info.UnitsPerEm,
		GlyphExtents: info.GlyphBBoxes(),
		Widths:       info.Widths(),

		Ascent:             info.Ascent,
		Descent:            info.Descent,
		BaseLineDistance:   info.Ascent - info.Descent + info.LineGap,
		UnderlinePosition:  info.UnderlinePosition,
		UnderlineThickness: info.UnderlineThickness,
	}

	cmap, err := info.CMapTable.GetBest()
	if err != nil {
		return nil, err
	}

	res := &fontCFFSimple{
		sfnt:        info,
		cmap:        cmap,
		gsubLookups: info.Gsub.FindLookups(opt.Language, opt.GsubFeatures),
		gposLookups: info.Gpos.FindLookups(opt.Language, opt.GposFeatures),
		Geometry:    geometry,
	}
	return res, nil
}

// Embed implements the [font.Font] interface.
func (f *fontCFFSimple) Embed(w pdf.Putter, resName pdf.Name) (font.Layouter, error) {
	err := pdf.CheckVersion(w, "simple OpenType/CFF fonts", pdf.V1_6)
	if err != nil {
		return nil, err
	}
	res := &embeddedCFFSimple{
		fontCFFSimple: f,
		w:             w,
		Res:           graphics.Res{Ref: w.Alloc(), DefName: resName},
		SimpleEncoder: encoding.NewSimpleEncoder(),
	}
	w.AutoClose(res)
	return res, nil
}

// Layout implements the [font.Layouter] interface.
func (f *fontCFFSimple) Layout(s string) glyph.Seq {
	return f.sfnt.Layout(f.cmap, f.gsubLookups, f.gposLookups, s)
}

type embeddedCFFSimple struct {
	*fontCFFSimple
	w pdf.Putter
	graphics.Res

	*encoding.SimpleEncoder
	closed bool
}

func (f *embeddedCFFSimple) ForeachWidth(s pdf.String, yield func(width float64, is_space bool)) {
	for _, c := range s {
		gid := f.Encoding[c]
		width := float64(f.sfnt.GlyphWidth(gid)) * f.sfnt.FontMatrix[0]
		yield(width, c == ' ')
	}
}

func (f *embeddedCFFSimple) CodeAndWidth(s pdf.String, gid glyph.ID, rr []rune) (pdf.String, float64, bool) {
	width := float64(f.sfnt.GlyphWidth(gid)) * f.sfnt.FontMatrix[0]
	c := f.GIDToCode(gid, rr)
	return append(s, c), width, c == ' '
}

func (f *embeddedCFFSimple) CodeToWidth(c byte) float64 {
	gid := f.Encoding[c]
	return float64(f.sfnt.GlyphWidth(gid)) * f.sfnt.FontMatrix[0]
}

func (f *embeddedCFFSimple) Close() error {
	if f.closed {
		return nil
	}
	f.closed = true

	if f.SimpleEncoder.Overflow() {
		return fmt.Errorf("too many distinct glyphs used in font %q (%s)",
			f.DefName, f.sfnt.PostScriptName())
	}
	encoding := f.SimpleEncoder.Encoding

	// Make our encoding the built-in encoding of the font.
	origOTF := f.sfnt.Clone()
	outlines := origOTF.Outlines.(*cff.Outlines)
	outlines.Encoding = encoding
	outlines.ROS = nil
	outlines.GIDToCID = nil

	origOTF.CMapTable = nil
	origOTF.Gdef = nil
	origOTF.Gsub = nil
	origOTF.Gpos = nil

	// subset the font
	subsetGID := f.SimpleEncoder.Subset()
	subsetTag := subset.Tag(subsetGID, origOTF.NumGlyphs())
	subsetOTF, err := origOTF.Subset(subsetGID)
	if err != nil {
		return fmt.Errorf("OpenType/CFF font subset: %w", err)
	}

	// convert the font to a simple font, if needed
	subsetOTF.EnsureGlyphNames()
	subsetCFF := subsetOTF.AsCFF()
	if len(subsetCFF.Private) != 1 {
		return fmt.Errorf("need exactly one private dict for a simple font")
	}

	m := f.SimpleEncoder.ToUnicode()
	toUnicode := cmap.NewToUnicode(charcode.Simple, m)
	// TODO(voss): check whether a ToUnicode CMap is actually needed

	info := EmbedInfoCFFSimple{
		Font:      subsetOTF,
		SubsetTag: subsetTag,
		Encoding:  subsetCFF.Encoding,
		ToUnicode: toUnicode,
	}
	return info.Embed(f.w, f.Ref)
}

// EmbedInfoCFFSimple is the information needed to embed a simple OpenType/CFF font.
type EmbedInfoCFFSimple struct {
	// Font is the font to embed (already subsetted, if needed).
	Font *sfnt.Font

	// SubsetTag should be a unique tag for the font subset,
	// or the empty string if this is the full font.
	SubsetTag string

	// Encoding is the encoding vector used by the client (a slice of length 256).
	// Together with the font's built-in encoding, this is used to determine
	// the `Encoding` entry of the PDF font dictionary.
	Encoding []glyph.ID

	IsAllCap   bool
	IsSmallCap bool

	// ToUnicode (optional) is a map from character codes to unicode strings.
	ToUnicode *cmap.ToUnicode
}

// Embed adds a simple OpenType/CFF font to a PDF file.
// This is the reverse of [ExtractCFFSimple]
func (info *EmbedInfoCFFSimple) Embed(w pdf.Putter, fontDictRef pdf.Reference) error {
	err := pdf.CheckVersion(w, "simple OpenType/CFF fonts", pdf.V1_6)
	if err != nil {
		return err
	}

	otf := info.Font
	if !otf.IsCFF() {
		return fmt.Errorf("not an OpenType/CFF font")
	}
	cff, ok := otf.Outlines.(*cff.Outlines)
	if !ok ||
		len(cff.Encoding) != 256 ||
		len(cff.Private) != 1 ||
		len(cff.Glyphs) == 0 ||
		len(cff.Glyphs[0].Name) == 0 {
		return errors.New("not a simple OpenType/CFF font")
	}

	fontName := otf.PostScriptName()
	if info.SubsetTag != "" {
		fontName = info.SubsetTag + "+" + fontName
	}

	unitsPerEm := otf.UnitsPerEm

	ww := make([]float64, 256)
	q := 1000 / float64(unitsPerEm)
	for i := range ww {
		ww[i] = float64(cff.Glyphs[info.Encoding[i]].Width) * q
	}
	widthsInfo := font.EncodeWidthsSimple(ww)

	ascent := otf.Ascent.AsFloat(q)
	descent := otf.Descent.AsFloat(q)
	linegap := otf.LineGap.AsFloat(q)
	leading := ascent - descent + linegap

	clientEnc := make([]string, 256)
	builtinEnc := make([]string, 256)
	for i := 0; i < 256; i++ {
		clientEnc[i] = cff.Glyphs[info.Encoding[i]].Name
		builtinEnc[i] = cff.Glyphs[cff.Encoding[i]].Name
	}

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
	if enc := encoding.DescribeEncodingType1(clientEnc, builtinEnc); enc != nil {
		fontDict["Encoding"] = enc
	}
	var toUnicodeRef pdf.Reference
	if info.ToUnicode != nil {
		toUnicodeRef = w.Alloc()
		fontDict["ToUnicode"] = toUnicodeRef
	}

	isSymbolic := !font.IsStandardLatin(otf)

	fd := &font.Descriptor{
		FontName:     fontName,
		IsFixedPitch: otf.IsFixedPitch(),
		IsSerif:      otf.IsSerif,
		IsSymbolic:   isSymbolic,
		IsScript:     otf.IsScript,
		IsItalic:     otf.IsItalic,
		IsAllCap:     info.IsAllCap,
		IsSmallCap:   info.IsSmallCap,
		ForceBold:    cff.Private[0].ForceBold,
		FontBBox:     fontBBox,
		ItalicAngle:  otf.ItalicAngle,
		Ascent:       ascent,
		Descent:      descent,
		Leading:      leading,
		CapHeight:    otf.CapHeight.AsFloat(q),
		StemV:        cff.Private[0].StdVW * q,
		MissingWidth: widthsInfo.MissingWidth,
	}
	fontDescriptor := fd.AsDict()
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
	err = otf.WriteOpenTypeCFFPDF(fontFileStream)
	if err != nil {
		return fmt.Errorf("OpenType/CFF font program %q: %w", fontName, err)
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

// ExtractCFFSimple extracts information about a simple OpenType/CFF font.
// This is the inverse of [EmbedInfoCFFSimple.Embed].
func ExtractCFFSimple(r pdf.Getter, dicts *font.Dicts) (*EmbedInfoCFFSimple, error) {
	if err := dicts.Type.MustBe(font.OpenTypeCFFSimple); err != nil {
		return nil, err
	}

	res := &EmbedInfoCFFSimple{}

	baseFont, _ := pdf.GetName(r, dicts.FontDict["BaseFont"])
	if m := subset.TagRegexp.FindStringSubmatch(string(baseFont)); m != nil {
		res.SubsetTag = m[1]
	}

	if dicts.FontProgram != nil {
		stm, err := pdf.DecodeStream(r, dicts.FontProgram, 0)
		if err != nil {
			return nil, pdf.Wrap(err, "OpenType/CFF font stream")
		}
		otf, err := sfnt.Read(stm)
		if err != nil {
			return nil, pdf.Wrap(err, "decoding OpenType/CFF font")
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
		nameEncoding, err := encoding.UndescribeEncodingType1(
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
		ascent := dicts.FontDescriptor.Ascent
		res.Font.Ascent = funit.Int16(math.Round(ascent / q))
		descent := dicts.FontDescriptor.Descent
		res.Font.Descent = funit.Int16(math.Round(descent / q))
		capHeight := dicts.FontDescriptor.CapHeight
		res.Font.CapHeight = funit.Int16(math.Round(capHeight / q))
		xHeight := dicts.FontDescriptor.XHeight // optional
		res.Font.XHeight = funit.Int16(math.Round(xHeight / q))
		leading := dicts.FontDescriptor.Leading
		if x := math.Round((leading - ascent + descent) / q); x > 0 {
			res.Font.LineGap = funit.Int16(x)
		}
	}

	res.IsAllCap = dicts.FontDescriptor.IsAllCap
	res.IsSmallCap = dicts.FontDescriptor.IsSmallCap

	if info, _ := cmap.ExtractToUnicode(r, dicts.FontDict["ToUnicode"], charcode.Simple); info != nil {
		res.ToUnicode = info
	}

	return res, nil
}
