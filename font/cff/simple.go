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

	"seehuhn.de/go/postscript/funit"

	"seehuhn.de/go/sfnt/cff"
	"seehuhn.de/go/sfnt/glyph"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/encoding"
	"seehuhn.de/go/pdf/font/subset"
	"seehuhn.de/go/pdf/font/widths"
)

type embeddedSimple struct {
	embedded

	*encoding.SimpleEncoder
}

func (f *embeddedSimple) ForeachWidth(s pdf.String, yield func(width float64, is_space bool)) {
	for _, c := range s {
		gid := f.Encoding[c]
		yield(f.sfnt.GlyphWidthPDF(gid), c == ' ')
	}
}

func (f *embeddedSimple) CodeAndWidth(s pdf.String, gid glyph.ID, rr []rune) (pdf.String, float64, bool) {
	width := f.sfnt.GlyphWidthPDF(gid)
	c := f.SimpleEncoder.GIDToCode(gid, rr)
	return append(s, c), width, c == ' '
}

func (f *embeddedSimple) Close() error {
	if f.closed {
		return nil
	}
	f.closed = true

	if f.SimpleEncoder.Overflow() {
		return fmt.Errorf("too many distinct glyphs used in font %q (%s)",
			f.DefName, f.sfnt.PostScriptName())
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
		return fmt.Errorf("CFF font subset: %w", err)
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

	info := EmbedInfoSimple{
		Font:      subsetCFF,
		SubsetTag: subsetTag,
		Encoding:  subsetCFF.Encoding, // we use the built-in encoding
		ToUnicode: toUnicode,

		Ascent:    subsetSfnt.Ascent,
		Descent:   subsetSfnt.Descent,
		CapHeight: subsetSfnt.CapHeight,
		IsSerif:   subsetSfnt.IsScript,
		IsScript:  subsetSfnt.IsScript,
	}
	return info.Embed(f.w, f.Ref.(pdf.Reference))
}

// EmbedInfoSimple is the information needed to embed a simple CFF font.
type EmbedInfoSimple struct {
	// Font is the font to embed (already subsetted, if needed).
	Font *cff.Font

	// SubsetTag should be a unique tag for the font subset,
	// or the empty string if this is the full font.
	SubsetTag string

	// Encoding is the encoding vector used by the client (a slice of length 256).
	// This may be different from the font's built-in encoding.
	Encoding []glyph.ID

	// TODO(voss): use PDF text space units
	Ascent    funit.Int16
	Descent   funit.Int16
	CapHeight funit.Int16

	IsSerif  bool
	IsScript bool

	IsAllCap   bool
	IsSmallCap bool

	// ToUnicode (optional) is a map from character codes to unicode strings.
	ToUnicode *cmap.ToUnicode
}

// ExtractSimple extracts information about a simple CFF font from a PDF file.
// This is the inverse of [EmbedInfoSimple.Embed].
func ExtractSimple(r pdf.Getter, dicts *font.Dicts) (*EmbedInfoSimple, error) {
	if err := dicts.Type.MustBe(font.CFFSimple); err != nil {
		return nil, err
	}

	res := &EmbedInfoSimple{}

	baseFont, _ := pdf.GetName(r, dicts.FontDict["BaseFont"])
	if m := subset.TagRegexp.FindStringSubmatch(string(baseFont)); m != nil {
		res.SubsetTag = m[1]
	}

	if dicts.FontProgram != nil {
		stm, err := pdf.DecodeStream(r, dicts.FontProgram, 0)
		if err != nil {
			return nil, pdf.Wrap(err, "CFF font stream")
		}
		data, err := io.ReadAll(stm)
		if err != nil {
			return nil, pdf.Wrap(err, "reading CFF font stream")
		}
		cff, err := cff.Read(bytes.NewReader(data))
		if err != nil {
			return nil, pdf.Wrap(err, "decoding CFF font")
		}

		// TODO(voss): use the glyph widths from the font dictionaries.

		res.Font = cff
	}

	if cff := res.Font; cff != nil {
		nameEncoding, err := encoding.UndescribeEncodingType1(
			r, dicts.FontDict["Encoding"], cff.BuiltinEncoding())
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
	}

	q := 1.0
	if res.Font != nil {
		cff := res.Font
		if cff.FontMatrix[0] != 0 {
			q = 1000 * cff.FontMatrix[0]
		}
	}
	// If the font is non-symbolic and not embedded, we could still get
	// the glyphnames for the encoding, but not the GIDs.

	res.Ascent = funit.Int16(math.Round(dicts.FontDescriptor.Ascent / q))
	res.Descent = funit.Int16(math.Round(dicts.FontDescriptor.Descent / q))
	res.CapHeight = funit.Int16(math.Round(dicts.FontDescriptor.CapHeight / q))
	res.IsSerif = dicts.FontDescriptor.IsSerif
	res.IsScript = dicts.FontDescriptor.IsScript
	res.IsAllCap = dicts.FontDescriptor.IsAllCap
	res.IsSmallCap = dicts.FontDescriptor.IsSmallCap

	if info, _ := cmap.ExtractToUnicode(r, dicts.FontDict["ToUnicode"], charcode.Simple); info != nil {
		res.ToUnicode = info
	}

	return res, nil
}

// Embed adds the font to a PDF file.
// This is the reverse of [ExtractSimple]
func (info *EmbedInfoSimple) Embed(w pdf.Putter, fontDictRef pdf.Reference) error {
	err := pdf.CheckVersion(w, "simple CFF fonts", pdf.V1_2)
	if err != nil {
		return err
	}

	cff := info.Font
	if len(cff.Encoding) != 256 ||
		len(cff.Private) != 1 ||
		len(cff.Glyphs) == 0 ||
		cff.Glyphs[0].Name == "" {
		return errors.New("not a simple CFF font")
	}

	fontName := cff.FontInfo.FontName
	if info.SubsetTag != "" {
		fontName = info.SubsetTag + "+" + fontName
	}

	ww := make([]float64, 256)
	q := 1000 * cff.FontMatrix[0]
	for i := range ww {
		ww[i] = float64(cff.Glyphs[info.Encoding[i]].Width) * q
	}
	widthsInfo := widths.EncodeSimple(ww)

	clientEnc := make([]string, 256)
	builtinEnc := make([]string, 256)
	for i := 0; i < 256; i++ {
		clientEnc[i] = cff.Glyphs[info.Encoding[i]].Name
		builtinEnc[i] = cff.Glyphs[cff.Encoding[i]].Name
	}

	bbox := cff.BBox()
	// TODO(voss): use the full font matrix
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

	isSymbolic := true // TODO(voss): try to set this correctly?

	fd := &font.Descriptor{
		FontName:     fontName,
		IsFixedPitch: cff.IsFixedPitch,
		IsSerif:      info.IsSerif,
		IsSymbolic:   isSymbolic,
		IsScript:     info.IsScript,
		IsItalic:     cff.ItalicAngle != 0,
		IsAllCap:     info.IsAllCap,
		IsSmallCap:   info.IsSmallCap,
		ForceBold:    cff.Private[0].ForceBold,
		FontBBox:     fontBBox,
		ItalicAngle:  cff.ItalicAngle,
		Ascent:       info.Ascent.AsFloat(q),
		Descent:      info.Descent.AsFloat(q),
		CapHeight:    info.CapHeight.AsFloat(q),
		StemV:        cff.Private[0].StdVW * q,
		MissingWidth: widthsInfo.MissingWidth,
	}
	fontDescriptor := fd.AsDict()
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
	err = cff.Write(fontFileStream)
	if err != nil {
		return fmt.Errorf("CFF font program %q: %w", fontName, err)
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
