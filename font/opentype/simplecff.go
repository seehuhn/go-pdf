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

	"seehuhn.de/go/geom/rect"
	"seehuhn.de/go/postscript/funit"

	"seehuhn.de/go/sfnt"
	"seehuhn.de/go/sfnt/cff"
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

type embeddedCFFSimple struct {
	w   *pdf.Writer
	ref pdf.Reference
	*font.Geometry

	sfnt *sfnt.Font

	*encoding.SimpleEncoder

	closed bool
}

func (f *embeddedCFFSimple) DecodeWidth(s pdf.String) (float64, int) {
	if len(s) == 0 {
		return 0, 0
	}
	gid := f.Encoding[s[0]]
	return f.sfnt.GlyphWidthPDF(gid), 1
}

func (f *embeddedCFFSimple) AppendEncoded(s pdf.String, gid glyph.ID, rr []rune) (pdf.String, float64) {
	width := f.sfnt.GlyphWidthPDF(gid)
	c := f.SimpleEncoder.GIDToCode(gid, rr)
	return append(s, c), width
}

func (f *embeddedCFFSimple) Finish(*pdf.ResourceManager) error {
	if f.closed {
		return nil
	}
	f.closed = true

	if f.SimpleEncoder.Overflow() {
		return fmt.Errorf("too many distinct glyphs used in font %q",
			f.sfnt.PostScriptName())
	}

	origSfnt := f.sfnt.Clone()
	origSfnt.CMapTable = nil
	origSfnt.Gdef = nil
	origSfnt.Gsub = nil
	origSfnt.Gpos = nil

	// Make our encoding the built-in encoding of the font.
	outlines := origSfnt.Outlines.(*cff.Outlines)
	outlines.Encoding = f.SimpleEncoder.Encoding
	outlines.ROS = nil
	outlines.GIDToCID = nil

	// subset the font
	subsetGID := f.SimpleEncoder.Subset()
	subsetTag := subset.Tag(subsetGID, origSfnt.NumGlyphs())
	subsetSfnt, err := origSfnt.Subset(subsetGID)
	if err != nil {
		return fmt.Errorf("OpenType/CFF font subset: %w", err)
	}

	// convert the font to a simple font, if needed
	subsetSfnt.EnsureGlyphNames()
	subsetCFF := subsetSfnt.AsCFF()
	if len(subsetCFF.Private) != 1 {
		return fmt.Errorf("need exactly one private dict for a simple font")
	}

	m := f.SimpleEncoder.ToUnicode()
	toUnicode := cmap.NewToUnicode(charcode.Simple, m)
	// TODO(voss): check whether a ToUnicode CMap is actually needed

	info := FontDictCFFSimple{
		Font:      subsetSfnt,
		SubsetTag: subsetTag,
		Encoding:  subsetCFF.Encoding,
		ToUnicode: toUnicode,
	}
	return info.Embed(f.w, f.ref)
}

// FontDictCFFSimple is the information needed to embed a simple OpenType/CFF font.
type FontDictCFFSimple struct {
	// Font is the font to embed (already subsetted, if needed).
	Font *sfnt.Font

	// SubsetTag should be a unique tag for the font subset,
	// or the empty string if this is the full font.
	SubsetTag string

	// Encoding is the encoding vector used by the client (a slice of length 256).
	// This may be different from the font's built-in encoding.
	Encoding []glyph.ID

	IsAllCap   bool
	IsSmallCap bool

	// ToUnicode (optional) is a map from character codes to unicode strings.
	ToUnicode *cmap.ToUnicode
}

// ExtractCFFSimple extracts information about a simple OpenType/CFF font.
// This is the inverse of [FontDictCFFSimple.Embed].
func ExtractCFFSimple(r pdf.Getter, dicts *font.Dicts) (*FontDictCFFSimple, error) {
	if err := dicts.FontTypeOld.MustBe(font.OpenTypeCFFSimple); err != nil {
		return nil, err
	}

	res := &FontDictCFFSimple{}

	baseFont, _ := pdf.GetName(r, dicts.FontDict["BaseFont"])
	if m := subset.TagRegexp.FindStringSubmatch(string(baseFont)); m != nil {
		res.SubsetTag = m[1]
	}

	if dicts.FontData != nil {
		stm, err := pdf.DecodeStream(r, dicts.FontData, 0)
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

// Embed adds the font to a PDF file.
// This implements the [font.Dict] interface.
// This is the reverse of [ExtractCFFSimple]
func (info *FontDictCFFSimple) Embed(w *pdf.Writer, fontDictRef pdf.Reference) error {
	err := pdf.CheckVersion(w, "simple OpenType/CFF fonts", pdf.V1_6)
	if err != nil {
		return err
	}

	sfnt := info.Font
	if !sfnt.IsCFF() {
		return fmt.Errorf("not an OpenType/CFF font")
	}
	cff, ok := sfnt.Outlines.(*cff.Outlines)
	if !ok ||
		len(cff.Encoding) != 256 ||
		len(cff.Private) != 1 ||
		len(cff.Glyphs) == 0 ||
		len(cff.Glyphs[0].Name) == 0 {
		return errors.New("not a simple OpenType/CFF font")
	}

	fontName := sfnt.PostScriptName()
	if info.SubsetTag != "" {
		fontName = info.SubsetTag + "+" + fontName
	}

	ww := make([]float64, 256)
	q := 1000 * sfnt.FontMatrix[0]
	for i := range ww {
		ww[i] = float64(cff.Glyphs[info.Encoding[i]].Width) * q
	}
	widthsInfo := widths.EncodeSimple(ww)

	ascent := sfnt.Ascent.AsFloat(q)
	descent := sfnt.Descent.AsFloat(q)
	linegap := sfnt.LineGap.AsFloat(q)
	leading := ascent - descent + linegap

	clientEnc := make([]string, 256)
	builtinEnc := make([]string, 256)
	for i := 0; i < 256; i++ {
		clientEnc[i] = cff.Glyphs[info.Encoding[i]].Name
		builtinEnc[i] = cff.Glyphs[cff.Encoding[i]].Name
	}

	bbox := cff.BBox()
	// TODO(voss): use the full font matrix
	fontBBox := rect.Rect{
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

	isSymbolic := !pdfenc.IsNonSymbolic(sfnt.MakeGlyphNames())

	fd := &font.Descriptor{
		FontName:     fontName,
		IsFixedPitch: sfnt.IsFixedPitch(),
		IsSerif:      sfnt.IsSerif,
		IsSymbolic:   isSymbolic,
		IsScript:     sfnt.IsScript,
		IsItalic:     sfnt.IsItalic,
		IsAllCap:     info.IsAllCap,
		IsSmallCap:   info.IsSmallCap,
		ForceBold:    cff.Private[0].ForceBold,
		FontBBox:     fontBBox,
		ItalicAngle:  sfnt.ItalicAngle,
		Ascent:       ascent,
		Descent:      descent,
		Leading:      leading,
		CapHeight:    sfnt.CapHeight.AsFloat(q),
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
	err = sfnt.WriteOpenTypeCFFPDF(fontFileStream)
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
