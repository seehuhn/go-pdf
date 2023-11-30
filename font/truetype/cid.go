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
	"errors"
	"fmt"
	"io"
	"math"

	"golang.org/x/text/language"

	"seehuhn.de/go/postscript/funit"
	"seehuhn.de/go/postscript/type1"

	"seehuhn.de/go/sfnt"
	sfntcmap "seehuhn.de/go/sfnt/cmap"
	"seehuhn.de/go/sfnt/glyf"
	"seehuhn.de/go/sfnt/glyph"
	"seehuhn.de/go/sfnt/opentype/gtab"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/subset"
	"seehuhn.de/go/pdf/graphics"
)

// fontComposite is a composite TrueType font.
type fontComposite struct {
	ttf         *sfnt.Font
	cmap        sfntcmap.Subtable
	gsubLookups []gtab.LookupIndex
	gposLookups []gtab.LookupIndex
	*font.Geometry

	makeGIDToCID func() cmap.GIDToCID
	makeEncoder  func(cmap.GIDToCID) cmap.CIDEncoder
}

var defaultFontOptions = &font.Options{
	Language:     language.Und,
	MakeGIDToCID: cmap.NewSequentialGIDToCID,
	MakeEncoder:  cmap.NewCIDEncoderIdentity,
	GsubFeatures: gtab.GsubDefaultFeatures,
	GposFeatures: gtab.GposDefaultFeatures,
}

// NewComposite creates a new composite TrueType font.
func NewComposite(info *sfnt.Font, opt *font.Options) (font.Font, error) {
	if !info.IsGlyf() {
		return nil, errors.New("wrong font type")
	}

	opt = font.MergeOptions(opt, defaultFontOptions)

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

	res := &fontComposite{
		ttf:          info,
		cmap:         cmap,
		gsubLookups:  info.Gsub.FindLookups(opt.Language, opt.GsubFeatures),
		gposLookups:  info.Gpos.FindLookups(opt.Language, opt.GposFeatures),
		Geometry:     geometry,
		makeGIDToCID: opt.MakeGIDToCID,
		makeEncoder:  opt.MakeEncoder,
	}
	return res, nil
}

// Layout implements the [font.Font] interface.
func (f *fontComposite) Layout(s string, ptSize float64) glyph.Seq {
	return f.ttf.Layout(f.cmap, f.gsubLookups, f.gposLookups, s)
}

// Embed implements the [font.Font] interface.
func (f *fontComposite) Embed(w pdf.Putter, resName pdf.Name) (font.Embedded, error) {
	err := pdf.CheckVersion(w, "composite TrueType fonts", pdf.V1_3)
	if err != nil {
		return nil, err
	}
	gidToCID := f.makeGIDToCID()
	res := &embeddedCID{
		fontComposite: f,
		w:             w,
		Res:           graphics.Res{Ref: w.Alloc(), DefName: resName},
		GIDToCID:      gidToCID,
		CIDEncoder:    f.makeEncoder(gidToCID),
	}
	w.AutoClose(res, res.Ref)
	return res, nil
}

type embeddedCID struct {
	*fontComposite
	w pdf.Putter
	graphics.Res

	cmap.GIDToCID
	cmap.CIDEncoder

	closed bool
}

func (f *embeddedCID) Close() error {
	if f.closed {
		return nil
	}
	f.closed = true

	origTTF := f.ttf.Clone()
	origTTF.CMapTable = nil
	origTTF.Gdef = nil
	origTTF.Gsub = nil
	origTTF.Gpos = nil

	// subset the font
	subsetGID := f.CIDEncoder.Subset()
	subsetTag := subset.Tag(subsetGID, origTTF.NumGlyphs())
	subsetTTF, err := origTTF.Subset(subsetGID)
	if err != nil {
		return fmt.Errorf("TrueType font subset: %w", err)
	}

	toUnicode := f.ToUnicode()

	cmapInfo := f.CMap()

	//  The `CIDToGIDMap` entry in the CIDFont dictionary specifies the mapping
	//  from CIDs to glyphs.
	m := make(map[glyph.ID]type1.CID)
	origGIDToCID := f.GIDToCID.GIDToCID(origTTF.NumGlyphs())
	for origGID, cid := range origGIDToCID {
		m[glyph.ID(origGID)] = cid
	}
	var maxCID type1.CID
	for _, origGID := range subsetGID {
		cid := m[origGID]
		if cid > maxCID {
			maxCID = cid
		}
	}
	cidToGID := make([]glyph.ID, maxCID+1)
	for subsetGID, origGID := range subsetGID {
		cidToGID[m[origGID]] = glyph.ID(subsetGID)
	}

	info := EmbedInfoComposite{
		Font:      subsetTTF,
		SubsetTag: subsetTag,
		CMap:      cmapInfo,
		CID2GID:   cidToGID,
		ToUnicode: toUnicode,
	}
	return info.Embed(f.w, f.Ref)
}

// EmbedInfoComposite is the information needed to embed a TrueType font as a composite PDF font.
type EmbedInfoComposite struct {
	// Font is the font to embed (already subsetted, if needed).
	Font *sfnt.Font

	// SubsetTag should be a unique tag for the font subset,
	// or the empty string if this is the full font.
	SubsetTag string

	CMap *cmap.Info

	CID2GID []glyph.ID

	ForceBold bool

	IsAllCap   bool
	IsSmallCap bool

	// ToUnicode (optional) is a map from character codes to unicode strings.
	ToUnicode *cmap.ToUnicode
}

// Embed adds a composite TrueType font to a PDF file.
// This is the reverse of [ExtractComposite]
func (info *EmbedInfoComposite) Embed(w pdf.Putter, fontDictRef pdf.Reference) error {
	err := pdf.CheckVersion(w, "composite TrueType fonts", pdf.V1_3)
	if err != nil {
		return err
	}

	ttf := info.Font.Clone()
	if !ttf.IsGlyf() {
		return fmt.Errorf("not a TrueType font")
	}
	outlines := ttf.Outlines.(*glyf.Outlines)

	fontName := ttf.PostscriptName()
	if info.SubsetTag != "" {
		fontName = info.SubsetTag + "+" + fontName
	}

	// make a CMap
	cmapInfo := info.CMap
	var encoding pdf.Object
	if cmapInfo.IsPredefined() {
		encoding = pdf.Name(cmapInfo.Name)
	} else {
		encoding = w.Alloc()
	}

	unitsPerEm := ttf.UnitsPerEm

	var ww []font.CIDWidth
	widths := outlines.Widths
	for cid, gid := range info.CID2GID {
		ww = append(ww, font.CIDWidth{CID: type1.CID(cid), GlyphWidth: widths[gid]})
	}
	DW, W := font.EncodeWidthsComposite(ww, unitsPerEm)

	var CIDToGIDMap pdf.Object
	isIdentity := true
	for cid, gid := range info.CID2GID {
		if int(gid) != cid && gid != 0 {
			isIdentity = false
			break
		}
	}
	if isIdentity {
		CIDToGIDMap = pdf.Name("Identity")
	} else {
		CIDToGIDMap = w.Alloc()
	}

	q := 1000 / float64(unitsPerEm)
	bbox := ttf.BBox()
	fontBBox := &pdf.Rectangle{
		LLx: bbox.LLx.AsFloat(q),
		LLy: bbox.LLy.AsFloat(q),
		URx: bbox.URx.AsFloat(q),
		URy: bbox.URy.AsFloat(q),
	}

	isSymbolic := !font.IsStandardLatin(ttf)

	cidFontRef := w.Alloc()
	var toUnicodeRef pdf.Reference
	fontDescriptorRef := w.Alloc()
	fontFileRef := w.Alloc()

	fontDict := pdf.Dict{
		"Type":            pdf.Name("Font"),
		"Subtype":         pdf.Name("Type0"),
		"BaseFont":        pdf.Name(fontName),
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
		"Subtype":        pdf.Name("CIDFontType2"),
		"BaseFont":       pdf.Name(fontName),
		"CIDSystemInfo":  ROS,
		"FontDescriptor": fontDescriptorRef,
		"CIDToGIDMap":    CIDToGIDMap,
	}
	if DW != 1000 {
		cidFontDict["DW"] = DW
	}
	if W != nil {
		cidFontDict["W"] = W
	}

	fd := &font.Descriptor{
		FontName:     fontName,
		IsFixedPitch: ttf.IsFixedPitch(),
		IsSerif:      ttf.IsSerif,
		IsSymbolic:   isSymbolic,
		IsScript:     ttf.IsScript,
		IsItalic:     ttf.IsItalic,
		IsAllCap:     info.IsAllCap,
		IsSmallCap:   info.IsSmallCap,
		ForceBold:    info.ForceBold,
		FontBBox:     fontBBox,
		ItalicAngle:  ttf.ItalicAngle,
		Ascent:       ttf.Ascent.AsFloat(q),
		Descent:      ttf.Descent.AsFloat(q),
		CapHeight:    ttf.CapHeight.AsFloat(q),
	}
	fontDescriptor := fd.AsDict()
	fontDescriptor["FontFile2"] = fontFileRef

	compressedRefs := []pdf.Reference{fontDictRef, cidFontRef, fontDescriptorRef}
	compressedObjects := []pdf.Object{fontDict, cidFontDict, fontDescriptor}
	err = w.WriteCompressed(compressedRefs, compressedObjects...)
	if err != nil {
		return pdf.Wrap(err, "composite TrueType font dicts")
	}

	// See section 9.9 of PDF 32000-1:2008 for details.
	length1 := pdf.NewPlaceholder(w, 10)
	fontFileDict := pdf.Dict{
		"Subtype": pdf.Name("TrueType"),
		"Length1": length1,
	}
	fontFileStream, err := w.OpenStream(fontFileRef, fontFileDict, pdf.FilterCompress{})
	if err != nil {
		return err
	}
	n, err := ttf.WriteTrueTypePDF(fontFileStream)
	if err != nil {
		return fmt.Errorf("composite TrueType Font %q: %w", fontName, err)
	}
	err = length1.Set(pdf.Integer(n))
	if err != nil {
		return err
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

	if ref, ok := CIDToGIDMap.(pdf.Reference); ok {
		cid2gidStream, err := w.OpenStream(ref, nil,
			pdf.FilterCompress{
				"Predictor": pdf.Integer(12),
				"Columns":   pdf.Integer(2),
			})
		if err != nil {
			return err
		}
		cid2gid := make([]byte, 2*len(info.CID2GID))
		for cid, gid := range info.CID2GID {
			cid2gid[2*cid] = byte(gid >> 8)
			cid2gid[2*cid+1] = byte(gid)
		}
		_, err = cid2gidStream.Write(cid2gid)
		if err != nil {
			return err
		}
		err = cid2gidStream.Close()
		if err != nil {
			return err
		}
	}

	return nil
}

// ExtractComposite extracts information about a composite TrueType font.
// This is the reverse of [EmbedInfoComposite.Embed].
func ExtractComposite(r pdf.Getter, dicts *font.Dicts) (*EmbedInfoComposite, error) {
	if err := dicts.Type.MustBe(font.TrueTypeComposite); err != nil {
		return nil, err
	}
	res := &EmbedInfoComposite{}

	stm, err := pdf.DecodeStream(r, dicts.FontProgram, 0)
	if err != nil {
		return nil, err
	}
	ttf, err := sfnt.Read(stm)
	if err != nil {
		return nil, err
	}
	_, ok := ttf.Outlines.(*glyf.Outlines)
	if !ok {
		return nil, fmt.Errorf("expected glyf outlines, got %T", ttf.Outlines)
	}
	// Most TrueType tables will be missing, so we fill in information from
	// the font descriptor instead.
	if ttf.FamilyName == "" {
		ttf.FamilyName = dicts.FontDescriptor.FontFamily
	}
	if ttf.Width == 0 {
		ttf.Width = dicts.FontDescriptor.FontStretch
	}
	if ttf.Weight == 0 {
		ttf.Weight = dicts.FontDescriptor.FontWeight
	}
	ttf.IsItalic = dicts.FontDescriptor.IsItalic
	ttf.IsSerif = dicts.FontDescriptor.IsSerif
	ttf.IsScript = dicts.FontDescriptor.IsScript
	q := 1000 / float64(ttf.UnitsPerEm)
	if ttf.CapHeight == 0 {
		capHeight := dicts.FontDescriptor.CapHeight
		ttf.CapHeight = funit.Int16(math.Round(float64(capHeight) / q))
	}
	if ttf.XHeight == 0 {
		xHeight := dicts.FontDescriptor.XHeight
		ttf.XHeight = funit.Int16(math.Round(float64(xHeight) / q))
	}
	res.Font = ttf

	postScriptName, _ := pdf.GetName(r, dicts.FontDict["BaseFont"])
	if m := subset.TagRegexp.FindStringSubmatch(string(postScriptName)); m != nil {
		res.SubsetTag = m[1]
	}

	cmapInfo, err := cmap.Extract(r, dicts.FontDict["Encoding"])
	if err != nil {
		return nil, err
	}
	res.CMap = cmapInfo

	CID2GIDObj, err := pdf.Resolve(r, dicts.CIDFontDict["CIDToGIDMap"])
	if err != nil {
		return nil, err
	}
	switch CID2GID := CID2GIDObj.(type) {
	case pdf.Name:
		if CID2GID != "Identity" {
			return nil, fmt.Errorf("unsupported CIDToGIDMap: %q", CID2GID)
		}
		res.CID2GID = make([]glyph.ID, ttf.NumGlyphs())
		for i := range res.CID2GID {
			res.CID2GID[i] = glyph.ID(i)
		}
	case *pdf.Stream:
		in, err := pdf.DecodeStream(r, CID2GID, 0)
		if err != nil {
			return nil, err
		}
		cid2gidData, err := io.ReadAll(in)
		if err == nil && len(cid2gidData)%2 != 0 {
			err = fmt.Errorf("odd length CIDToGIDMap")
		}
		if err != nil {
			return nil, err
		}
		res.CID2GID = make([]glyph.ID, len(cid2gidData)/2)
		for i := range res.CID2GID {
			res.CID2GID[i] = glyph.ID(cid2gidData[2*i])<<8 | glyph.ID(cid2gidData[2*i+1])
		}
	}

	if info, _ := cmap.ExtractToUnicode(r, dicts.FontDict["ToUnicode"], cmapInfo.CS); info != nil {
		res.ToUnicode = info
	}

	res.IsAllCap = dicts.FontDescriptor.IsAllCap
	res.IsSmallCap = dicts.FontDescriptor.IsSmallCap
	res.ForceBold = dicts.FontDescriptor.ForceBold

	return res, nil
}
