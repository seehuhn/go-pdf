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
	sfntcmap "seehuhn.de/go/sfnt/cmap"
	"seehuhn.de/go/sfnt/glyph"
	"seehuhn.de/go/sfnt/opentype/gtab"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/subset"
	"seehuhn.de/go/pdf/graphics"
)

// fontCFFComposite is an OpenType/CFF font for embedding in a PDF file as a composite font.
type fontCFFComposite struct {
	otf         *sfnt.Font
	cmap        sfntcmap.Subtable
	gsubLookups []gtab.LookupIndex
	gposLookups []gtab.LookupIndex
	*font.Geometry

	makeGIDToCID func() cmap.GIDToCID
	makeEncoder  func(cmap.GIDToCID) cmap.CIDEncoder
}

var defaultOptionsCFF = &font.Options{
	Language:     language.Und,
	MakeGIDToCID: cmap.NewIdentityGIDToCID,
	MakeEncoder:  cmap.NewCIDEncoderIdentity,
	GsubFeatures: gtab.GsubDefaultFeatures,
	GposFeatures: gtab.GposDefaultFeatures,
}

// NewCFFComposite creates a new OpenType/CFF font for embedding in a PDF file as a composite font.
// Info must be an OpenType font with CFF outlines.
// The font info is allowed but not required to be CID-keyed.
// Consider using [cff.NewComposite] instead of this function.
func NewCFFComposite(info *sfnt.Font, opt *font.Options) (font.Font, error) {
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

	res := &fontCFFComposite{
		otf:          info,
		cmap:         cmap,
		gsubLookups:  info.Gsub.FindLookups(opt.Language, opt.GsubFeatures),
		gposLookups:  info.Gpos.FindLookups(opt.Language, opt.GposFeatures),
		Geometry:     geometry,
		makeGIDToCID: opt.MakeGIDToCID,
		makeEncoder:  opt.MakeEncoder,
	}
	return res, nil
}

// Embed implements the [font.Font] interface.
func (f *fontCFFComposite) Embed(w pdf.Putter, resName pdf.Name) (font.Embedded, error) {
	err := pdf.CheckVersion(w, "composite OpenType/CFF fonts", pdf.V1_6)
	if err != nil {
		return nil, err
	}
	gidToCID := f.makeGIDToCID()
	res := &embeddedCFFComposite{
		fontCFFComposite: f,
		w:                w,
		Res:              graphics.Res{Data: w.Alloc(), DefName: resName},
		GIDToCID:         gidToCID,
		CIDEncoder:       f.makeEncoder(gidToCID),
	}
	w.AutoClose(res)
	return res, nil
}

// Layout implements the [font.Font] interface.
func (f *fontCFFComposite) Layout(s string, ptSize float64) glyph.Seq {
	return f.otf.Layout(f.cmap, f.gsubLookups, f.gposLookups, s)
}

type embeddedCFFComposite struct {
	*fontCFFComposite
	w pdf.Putter
	graphics.Res

	cmap.GIDToCID
	cmap.CIDEncoder

	closed bool
}

func (f *embeddedCFFComposite) WritingMode() int {
	return 0 // TODO(voss): implement vertical writing mode
}

func (f *embeddedCFFComposite) AllWidths(s pdf.String) func(yield func(w float64, isSpace bool) bool) bool {
	return func(yield func(w float64, isSpace bool) bool) bool {
		cs := f.CS()
		q := 1000 / float64(f.otf.UnitsPerEm)
		return cs.AllCodes(s)(func(c pdf.String, valid bool) bool {
			if !valid {
				notdefWidth := f.otf.GlyphWidth(0).AsFloat(q)
				return yield(notdefWidth, false)
			}
			code, k := cs.Decode(c)
			if k != len(c) {
				panic("internal error")
			}

			// If code is invalid, CID 0 is used.
			cid, _ := f.Lookup(code)
			gid := f.GID(cid)
			width := f.otf.GlyphWidth(gid).AsFloat(q)

			return yield(width, len(c) == 1 && c[0] == 0x20)
		})
	}
}

func (f *embeddedCFFComposite) Close() error {
	if f.closed {
		return nil
	}
	f.closed = true

	origOTF := f.otf.Clone()
	origOTF.CMapTable = nil
	origOTF.Gdef = nil
	origOTF.Gsub = nil
	origOTF.Gpos = nil

	// subset the font
	subsetGID := f.CIDEncoder.Subset()
	subsetTag := subset.Tag(subsetGID, origOTF.NumGlyphs())
	subsetOTF, err := origOTF.Subset(subsetGID)
	if err != nil {
		return fmt.Errorf("OpenType/CFF font subset: %w", err)
	}

	origGIDToCID := f.GIDToCID.GIDToCID(origOTF.NumGlyphs())
	gidToCID := make([]type1.CID, len(subsetGID))
	for i, gid := range subsetGID {
		gidToCID[i] = origGIDToCID[gid]
	}

	ros := f.ROS()
	toUnicode := f.ToUnicode()

	cmapInfo := f.CMap()

	// If the CFF font is CID-keyed, *i.e.* if it contain a `ROS` operator,
	// then the `charset` table in the CFF font describes the mapping from CIDs
	// to glyphs.  Otherwise, the CID is used as the glyph index directly.
	isIdentity := true
	for gid, cid := range gidToCID {
		if cid != 0 && cid != type1.CID(gid) {
			isIdentity = false
			break
		}
	}
	outlines := subsetOTF.Outlines.(*cff.Outlines)
	mustUseCID := len(outlines.Private) > 1
	if isIdentity && !mustUseCID { // Make the font non-CID-keyed.
		outlines.Encoding = cff.StandardEncoding(outlines.Glyphs)
		outlines.ROS = nil
		outlines.GIDToCID = nil
	} else { // Make the font CID-keyed.
		outlines.Encoding = nil
		outlines.ROS = ros
		outlines.GIDToCID = gidToCID
	}

	info := EmbedInfoCFFComposite{
		Font:      subsetOTF,
		SubsetTag: subsetTag,
		CMap:      cmapInfo,
		ToUnicode: toUnicode,
	}
	return info.Embed(f.w, f.Data)
}

// EmbedInfoCFFComposite contains the information needed to embed an OpenType/CFF font in a PDF file as a composite font.
type EmbedInfoCFFComposite struct {
	// Font is the font to embed (already subsetted, if needed).
	Font *sfnt.Font

	// SubsetTag should be a unique tag for the font subset,
	// or the empty string if this is the full font.
	SubsetTag string

	CMap *cmap.Info

	IsAllCap   bool
	IsSmallCap bool

	// ToUnicode (optional) is a map from character codes to unicode strings.
	ToUnicode *cmap.ToUnicode
}

// Embed adds a composite OpenType/CFF font to a PDF file.
// This is the reverse of [ExtractCFFComposite]
func (info *EmbedInfoCFFComposite) Embed(w pdf.Putter, fontDictRef pdf.Reference) error {
	err := pdf.CheckVersion(w, "composite OpenType/CFF fonts", pdf.V1_6)
	if err != nil {
		return err
	}

	otf := info.Font
	if !otf.IsCFF() {
		return fmt.Errorf("not an OpenType/CFF font")
	}
	cff := otf.AsCFF()

	cidFontName := cff.FontInfo.FontName
	if info.SubsetTag != "" {
		cidFontName = info.SubsetTag + "+" + cidFontName
	}

	// make a PDF CMap
	cmapInfo := info.CMap
	var encoding pdf.Object
	if cmapInfo.IsPredefined() {
		encoding = pdf.Name(cmapInfo.Name)
	} else {
		encoding = w.Alloc()
	}

	unitsPerEm := otf.UnitsPerEm

	var ww []font.CIDWidth
	widths := otf.Widths()
	if cff.GIDToCID != nil {
		for gid, w := range widths {
			ww = append(ww, font.CIDWidth{CID: cff.GIDToCID[gid], GlyphWidth: w})
		}
	} else {
		for gid, w := range widths {
			ww = append(ww, font.CIDWidth{CID: type1.CID(gid), GlyphWidth: w})
		}
	}
	DW, W := font.EncodeWidthsComposite(ww, otf.UnitsPerEm)

	q := 1000 / float64(unitsPerEm)
	bbox := otf.BBox()
	fontBBox := &pdf.Rectangle{
		LLx: bbox.LLx.AsFloat(q),
		LLy: bbox.LLy.AsFloat(q),
		URx: bbox.URx.AsFloat(q),
		URy: bbox.URy.AsFloat(q),
	}

	isSymbolic := !font.IsStandardLatin(otf)

	cidFontRef := w.Alloc()
	var toUnicodeRef pdf.Reference
	fontDescriptorRef := w.Alloc()
	fontFileRef := w.Alloc()

	fontDict := pdf.Dict{
		"Type":            pdf.Name("Font"),
		"Subtype":         pdf.Name("Type0"),
		"BaseFont":        pdf.Name(cidFontName + "-" + cmapInfo.Name),
		"Encoding":        encoding,
		"DescendantFonts": pdf.Array{cidFontRef},
	}
	if info.ToUnicode != nil {
		toUnicodeRef = w.Alloc()
		fontDict["ToUnicode"] = toUnicodeRef
	}

	ROS := pdf.Dict{
		"Registry":   pdf.String(info.CMap.ROS.Registry),
		"Ordering":   pdf.String(info.CMap.ROS.Ordering),
		"Supplement": pdf.Integer(info.CMap.ROS.Supplement),
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
		IsFixedPitch: cff.IsFixedPitch,
		IsSerif:      otf.IsSerif,
		IsSymbolic:   isSymbolic,
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
	}
	fontDescriptor := fd.AsDict()
	fontDescriptor["FontFile3"] = fontFileRef

	compressedRefs := []pdf.Reference{fontDictRef, cidFontRef, fontDescriptorRef}
	compressedObjects := []pdf.Object{fontDict, cidFontDict, fontDescriptor}
	err = w.WriteCompressed(compressedRefs, compressedObjects...)
	if err != nil {
		return pdf.Wrap(err, "composite OpenType/CFF font dicts")
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
		return fmt.Errorf("OpenType/CFF font program %q: %w", cidFontName, err)
	}
	err = fontFileStream.Close()
	if err != nil {
		return err
	}

	if ref, ok := encoding.(pdf.Reference); ok {
		err = cmapInfo.Embed(w, ref, nil)
		if err != nil {
			return err
		}
	}

	if toUnicodeRef != 0 {
		err = info.ToUnicode.Embed(w, toUnicodeRef)
		if err != nil {
			return err
		}
	}

	return nil
}

// ExtractCFFComposite extracts information about a composite OpenType/CFF font from a PDF file.
// This is the inverse of [EmbedInfoCFFComposite.Embed].
func ExtractCFFComposite(r pdf.Getter, dicts *font.Dicts) (*EmbedInfoCFFComposite, error) {
	if err := dicts.Type.MustBe(font.OpenTypeCFFComposite); err != nil {
		return nil, err
	}

	res := &EmbedInfoCFFComposite{}

	stm, err := pdf.DecodeStream(r, dicts.FontProgram, 0)
	if err != nil {
		return nil, err
	}
	otf, err := sfnt.Read(stm)
	if err != nil {
		return nil, err
	}
	if _, ok := otf.Outlines.(*cff.Outlines); !ok {
		return nil, fmt.Errorf("expected CFF outlines, got %T", otf.Outlines)
	}
	// Most OpenType tables will be missing, so we fill in information from
	// the font descriptor instead.
	otf.IsSerif = dicts.FontDescriptor.IsSerif
	otf.IsScript = dicts.FontDescriptor.IsScript
	q := 1000 / float64(otf.UnitsPerEm)
	otf.Ascent = funit.Int16(math.Round(float64(dicts.FontDescriptor.Ascent) / q))
	otf.Descent = funit.Int16(math.Round(float64(dicts.FontDescriptor.Descent) / q))
	otf.CapHeight = funit.Int16(math.Round(float64(dicts.FontDescriptor.CapHeight) / q))
	res.Font = otf

	postScriptName, _ := pdf.GetName(r, dicts.CIDFontDict["BaseFont"])
	if m := subset.TagRegexp.FindStringSubmatch(string(postScriptName)); m != nil {
		res.SubsetTag = m[1]
	}

	cmapInfo, err := cmap.Extract(r, dicts.FontDict["Encoding"])
	if err != nil {
		return nil, err
	}
	res.CMap = cmapInfo

	res.IsAllCap = dicts.FontDescriptor.IsAllCap
	res.IsSmallCap = dicts.FontDescriptor.IsSmallCap

	if info, _ := cmap.ExtractToUnicode(r, dicts.FontDict["ToUnicode"], cmapInfo.CodeSpaceRange); info != nil {
		res.ToUnicode = info
	}

	return res, nil
}
