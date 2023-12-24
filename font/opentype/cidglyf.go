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

// fontGlyfComposite is a composite OpenType/glyf font.
type fontGlyfComposite struct {
	otf         *sfnt.Font
	cmap        sfntcmap.Subtable
	gsubLookups []gtab.LookupIndex
	gposLookups []gtab.LookupIndex
	*font.Geometry

	makeGIDToCID func() cmap.GIDToCID
	makeEncoder  func(cmap.GIDToCID) cmap.CIDEncoder
}

var defaultOptionsGlyf = &font.Options{
	Language:     language.Und,
	MakeGIDToCID: cmap.NewSequentialGIDToCID,
	MakeEncoder:  cmap.NewCIDEncoderIdentity,
	GsubFeatures: gtab.GsubDefaultFeatures,
	GposFeatures: gtab.GposDefaultFeatures,
}

// NewGlyfComposite creates a new composite OpenType/glyf font.
// Info must either be a TrueType font or an OpenType font with TrueType outlines.
// Consider using [truetype.NewComposite] instead of this function.
func NewGlyfComposite(info *sfnt.Font, opt *font.Options) (font.Font, error) {
	if !info.IsGlyf() {
		return nil, errors.New("wrong font type")
	}

	opt = font.MergeOptions(opt, defaultOptionsGlyf)

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

	res := &fontGlyfComposite{
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

// Layout implements the [font.Font] interface.
func (f *fontGlyfComposite) Layout(s string, ptSize float64) glyph.Seq {
	return f.otf.Layout(f.cmap, f.gsubLookups, f.gposLookups, s)
}

// Embed implements the [font.Font] interface.
func (f *fontGlyfComposite) Embed(w pdf.Putter, resName pdf.Name) (font.Embedded, error) {
	err := pdf.CheckVersion(w, "composite OpenType/glyf fonts", pdf.V1_6)
	if err != nil {
		return nil, err
	}
	gidToCID := f.makeGIDToCID()
	res := &embeddedGlyfComposite{
		fontGlyfComposite: f,
		w:                 w,
		Res:               graphics.Res{Data: w.Alloc(), DefName: resName},
		GIDToCID:          gidToCID,
		CIDEncoder:        f.makeEncoder(gidToCID),
	}
	w.AutoClose(res)
	return res, nil
}

type embeddedGlyfComposite struct {
	*fontGlyfComposite
	w pdf.Putter
	graphics.Res

	cmap.GIDToCID
	cmap.CIDEncoder

	closed bool
}

func (f *embeddedGlyfComposite) WritingMode() int {
	return 0 // TODO(voss): implement vertical writing mode
}

func (f *embeddedGlyfComposite) Close() error {
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
		return fmt.Errorf("OpenType/glyf font subset: %w", err)
	}

	toUnicode := f.ToUnicode()

	cmapInfo := f.CMap()

	//  The `CIDToGIDMap` entry in the CIDFont dictionary specifies the mapping
	//  from CIDs to glyphs.
	m := make(map[glyph.ID]type1.CID)
	origGIDToCID := f.GIDToCID.GIDToCID(origOTF.NumGlyphs())
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

	info := EmbedInfoGlyfComposite{
		Font:      subsetOTF,
		SubsetTag: subsetTag,
		CMap:      cmapInfo,
		CIDToGID:  cidToGID,
		ToUnicode: toUnicode,
	}
	return info.Embed(f.w, f.Data)
}

// EmbedInfoGlyfComposite is the information needed to embed a composite OpenType/glyf font.
type EmbedInfoGlyfComposite struct {
	// Font is the font to embed (already subsetted, if needed).
	Font *sfnt.Font

	// SubsetTag should be a unique tag for the font subset,
	// or the empty string if this is the full font.
	SubsetTag string

	CMap *cmap.Info

	CIDToGID []glyph.ID

	ForceBold bool

	IsAllCap   bool
	IsSmallCap bool

	// ToUnicode (optional) is a map from character codes to unicode strings.
	ToUnicode *cmap.ToUnicode
}

// Embed adds the font to the PDF file.
func (info *EmbedInfoGlyfComposite) Embed(w pdf.Putter, fontDictRef pdf.Reference) error {
	err := pdf.CheckVersion(w, "composite OpenType/glyf fonts", pdf.V1_6)
	if err != nil {
		return err
	}

	if !info.Font.IsGlyf() {
		return fmt.Errorf("not a glyf-based OpenType font")
	}
	otf := info.Font.Clone()
	otf.CMapTable = nil
	outlines := otf.Outlines.(*glyf.Outlines)

	// CidFontName shall be the value of the CIDFontName entry in the CIDFont program.
	// The name may have a subset prefix if appropriate.
	cidFontName := otf.PostscriptName()
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
	widths := outlines.Widths
	for cid, gid := range info.CIDToGID {
		ww = append(ww, font.CIDWidth{CID: type1.CID(cid), GlyphWidth: widths[gid]})
	}
	DW, W := font.EncodeWidthsComposite(ww, unitsPerEm)

	var CIDToGIDMap pdf.Object
	isIdentity := true
	for cid, gid := range info.CIDToGID {
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
		"BaseFont":        pdf.Name(cidFontName),
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
		"BaseFont":       pdf.Name(cidFontName),
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
		FontName:     cidFontName,
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
	}
	fontDescriptor := fd.AsDict()
	fontDescriptor["FontFile3"] = fontFileRef

	compressedRefs := []pdf.Reference{fontDictRef, cidFontRef, fontDescriptorRef}
	compressedObjects := []pdf.Object{fontDict, cidFontDict, fontDescriptor}
	err = w.WriteCompressed(compressedRefs, compressedObjects...)
	if err != nil {
		return pdf.Wrap(err, "composite OpenType/glyf font dicts")
	}

	// See section 9.9 of PDF 32000-1:2008 for details.
	fontFileDict := pdf.Dict{
		"Subtype": pdf.Name("OpenType"),
	}
	fontFileStream, err := w.OpenStream(fontFileRef, fontFileDict, pdf.FilterCompress{})
	if err != nil {
		return err
	}
	_, err = otf.WriteTrueTypePDF(fontFileStream)
	if err != nil {
		return fmt.Errorf("embedding OpenType/glyf CIDFont %q: %w", cidFontName, err)
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
		cid2gid := make([]byte, 2*len(info.CIDToGID))
		for cid, gid := range info.CIDToGID {
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

// ExtractGlyfComposite extracts information for a composite OpenType/glyf font.
func ExtractGlyfComposite(r pdf.Getter, dicts *font.Dicts) (*EmbedInfoGlyfComposite, error) {
	if err := dicts.Type.MustBe(font.OpenTypeGlyfComposite); err != nil {
		return nil, err
	}
	res := &EmbedInfoGlyfComposite{}

	stm, err := pdf.DecodeStream(r, dicts.FontProgram, 0)
	if err != nil {
		return nil, err
	}
	otf, err := sfnt.Read(stm)
	if err != nil {
		return nil, err
	}
	if _, ok := otf.Outlines.(*glyf.Outlines); !ok {
		return nil, fmt.Errorf("expected glyf outlines, got %T", otf.Outlines)
	}
	// Most OpenType tables will be missing, so we fill in information from
	// the font descriptor instead.
	if otf.FamilyName == "" {
		otf.FamilyName = dicts.FontDescriptor.FontFamily
	}
	if otf.Width == 0 {
		otf.Width = dicts.FontDescriptor.FontStretch
	}
	if otf.Weight == 0 {
		otf.Weight = dicts.FontDescriptor.FontWeight
	}
	otf.IsItalic = dicts.FontDescriptor.IsItalic
	otf.IsSerif = dicts.FontDescriptor.IsSerif
	otf.IsScript = dicts.FontDescriptor.IsScript
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

	postScriptName, _ := pdf.GetName(r, dicts.CIDFontDict["BaseFont"])
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
		numCIDs := int(cmapInfo.MaxCID()) + 1
		if numCIDs > otf.NumGlyphs() {
			numCIDs = otf.NumGlyphs()
		}
		res.CIDToGID = make([]glyph.ID, numCIDs)
		for i := range res.CIDToGID {
			res.CIDToGID[i] = glyph.ID(i)
		}
	case *pdf.Stream:
		in, err := pdf.DecodeStream(r, CID2GID, 0)
		if err != nil {
			return nil, err
		}
		cidToGIDData, err := io.ReadAll(in)
		if err == nil && len(cidToGIDData)%2 != 0 {
			err = fmt.Errorf("odd length CIDToGIDMap")
		}
		if err != nil {
			return nil, err
		}
		res.CIDToGID = make([]glyph.ID, len(cidToGIDData)/2)
		for i := range res.CIDToGID {
			res.CIDToGID[i] = glyph.ID(cidToGIDData[2*i])<<8 | glyph.ID(cidToGIDData[2*i+1])
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
