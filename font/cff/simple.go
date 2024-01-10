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
)

// fontCFFSimple is a CFF font for embedding into a PDF file as a simple font.
type fontCFFSimple struct {
	otf         *sfnt.Font
	cmap        sfntcmap.Subtable
	gsubLookups []gtab.LookupIndex
	gposLookups []gtab.LookupIndex
	*font.Geometry
}

var defaultOptionsCFF = &font.Options{
	Language:     language.Und,
	GsubFeatures: gtab.GsubDefaultFeatures,
	GposFeatures: gtab.GposDefaultFeatures,
}

// NewSimple allocates a new CFF font for embedding in a PDF file as a simple font.
// Info must be an OpenType font with CFF outlines.
// If info is CID-keyed, the function will attempt to convert it to a simple font.
// If the conversion fails (because more than one private dictionary is used
// after subsetting), an error is returned.
func NewSimple(info *sfnt.Font, opt *font.Options) (font.Font, error) {
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
		otf:         info,
		cmap:        cmap,
		gsubLookups: info.Gsub.FindLookups(opt.Language, opt.GsubFeatures),
		gposLookups: info.Gpos.FindLookups(opt.Language, opt.GposFeatures),
		Geometry:    geometry,
	}
	return res, nil
}

// Embed implements the [font.Font] interface.
func (f *fontCFFSimple) Embed(w pdf.Putter, resName pdf.Name) (font.Embedded, error) {
	err := pdf.CheckVersion(w, "simple CFF fonts", pdf.V1_2)
	if err != nil {
		return nil, err
	}
	res := &embeddedSimple{
		fontCFFSimple: f,
		w:             w,
		Res:           Res{Data: w.Alloc(), DefName: resName},
		SimpleEncoder: encoding.NewSimpleEncoder(),
	}
	w.AutoClose(res)
	return res, nil
}

// Layout implements the [font.Font] interface.
func (f *fontCFFSimple) Layout(s string, ptSize float64) glyph.Seq {
	return f.otf.Layout(f.cmap, f.gsubLookups, f.gposLookups, s)
}

type embeddedSimple struct {
	*fontCFFSimple
	w pdf.Putter
	Res

	*encoding.SimpleEncoder
	closed bool
}

func (f *embeddedSimple) AllWidths(s pdf.String) func(yield func(w float64, isSpace bool) bool) bool {
	return func(yield func(w float64, isSpace bool) bool) bool {
		q := 1000 / float64(f.otf.UnitsPerEm)
		for _, c := range s {
			gid := f.Encoding[c]
			w := f.otf.GlyphWidth(gid).AsFloat(q)
			if !yield(w, c == 0x20) {
				return false
			}
		}
		return true
	}
}

func (f *embeddedSimple) Close() error {
	if f.closed {
		return nil
	}
	f.closed = true

	if f.SimpleEncoder.Overflow() {
		return fmt.Errorf("too many distinct glyphs used in font %q (%s)",
			f.DefName, f.otf.PostscriptName())
	}
	encoding := f.SimpleEncoder.Encoding

	// Make our encoding the built-in encoding of the font.
	origOTF := f.otf.Clone()
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
		return fmt.Errorf("CFF font subset: %w", err)
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
		Font:      subsetCFF,
		SubsetTag: subsetTag,
		Encoding:  subsetCFF.Encoding, // we use the built-in encoding
		ToUnicode: toUnicode,

		UnitsPerEm: subsetOTF.UnitsPerEm,
		Ascent:     subsetOTF.Ascent,
		Descent:    subsetOTF.Descent,
		CapHeight:  subsetOTF.CapHeight,
		IsSerif:    subsetOTF.IsScript,
		IsScript:   subsetOTF.IsScript,
	}
	return info.Embed(f.w, f.Data)
}

// EmbedInfoCFFSimple is the information needed to embed a simple CFF font.
type EmbedInfoCFFSimple struct {
	// Font is the font to embed (already subsetted, if needed).
	Font *cff.Font

	// SubsetTag should be a unique tag for the font subset,
	// or the empty string if this is the full font.
	SubsetTag string

	// Encoding is the encoding vector used by the client (a slice of length 256).
	// Together with the font's built-in encoding, this is used to determine
	// the `Encoding` entry of the PDF font dictionary.
	Encoding []glyph.ID

	UnitsPerEm uint16 // TODO(voss): get this from the font matrix instead

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

// WritingMode implements the [font.NewFont] interface.
func (info *EmbedInfoCFFSimple) WritingMode() int {
	return 0
}

// AllWidths implements the [font.NewFont] interface.
func (info *EmbedInfoCFFSimple) AllWidths(s pdf.String) func(yield func(w float64, isSpace bool) bool) bool {
	return func(yield func(w float64, isSpace bool) bool) bool {
		q := 1 / float64(info.UnitsPerEm)
		for _, c := range s {
			gid := info.Encoding[c]
			w := info.Font.Glyphs[gid].Width.AsFloat(q)
			if !yield(w, c == 0x20) {
				return false
			}
		}
		return true
	}
}

// Embed adds the font to a PDF file.
// This is the reverse of [ExtractSimple]
func (info *EmbedInfoCFFSimple) Embed(w pdf.Putter, fontDictRef pdf.Reference) error {
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

	unitsPerEm := info.UnitsPerEm

	ww := make([]funit.Int16, 256)
	for i := range ww {
		ww[i] = cff.Glyphs[info.Encoding[i]].Width
	}
	widthsInfo := font.EncodeWidthsSimple(ww, unitsPerEm)

	clientEnc := make([]string, 256)
	builtinEnc := make([]string, 256)
	for i := 0; i < 256; i++ {
		clientEnc[i] = cff.Glyphs[info.Encoding[i]].Name
		builtinEnc[i] = cff.Glyphs[cff.Encoding[i]].Name
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

// ExtractSimple extracts all information about a simple CFF font from a PDF file.
// This is the inverse of [EmbedInfoCFFSimple.Embed].
func ExtractSimple(r pdf.Getter, dicts *font.Dicts) (*EmbedInfoCFFSimple, error) {
	if err := dicts.Type.MustBe(font.CFFSimple); err != nil {
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

		res.Font = cff
	}

	var unitsPerEm uint16 = 1000 // updated below, in case we have font data
	var builtin []string
	if res.Font != nil {
		cff := res.Font
		builtin = make([]string, 256)
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

		if cff.FontMatrix[0] != 0 {
			unitsPerEm = uint16(math.Round(1 / cff.FontMatrix[0]))
		}
	}
	// If the font is non-symbolic and not embedded, we could still get
	// the glyphnames for the encoding, but not the GIDs.

	res.UnitsPerEm = unitsPerEm

	q := 1000 / float64(unitsPerEm)
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
