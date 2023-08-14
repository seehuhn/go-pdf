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

	"golang.org/x/text/language"

	"seehuhn.de/go/postscript/type1"

	"seehuhn.de/go/sfnt"
	"seehuhn.de/go/sfnt/glyf"
	"seehuhn.de/go/sfnt/glyph"
	"seehuhn.de/go/sfnt/opentype/gtab"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/subset"
	"seehuhn.de/go/pdf/font/tounicode"
)

type CompositeFont struct {
	ttf         *sfnt.Font
	gsubLookups []gtab.LookupIndex
	gposLookups []gtab.LookupIndex
	*font.Geometry
}

func NewComposite(info *sfnt.Font, loc language.Tag) (*CompositeFont, error) {
	if !info.IsGlyf() {
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

	res := &CompositeFont{
		ttf:         info,
		gsubLookups: info.Gsub.FindLookups(loc, gtab.GsubDefaultFeatures),
		gposLookups: info.Gpos.FindLookups(loc, gtab.GposDefaultFeatures),
		Geometry:    geometry,
	}
	return res, nil
}

func (f *CompositeFont) Embed(w pdf.Putter, resName pdf.Name) (font.Embedded, error) {
	err := pdf.CheckVersion(w, "composite TrueType fonts", pdf.V1_3)
	if err != nil {
		return nil, err
	}
	res := &embeddedCID{
		CompositeFont: f,
		w:             w,
		Resource:      pdf.Resource{Ref: w.Alloc(), Name: resName},
		CIDEncoder:    cmap.NewCIDEncoder(),
	}
	w.AutoClose(res)
	return res, nil
}

func (f *CompositeFont) Layout(s string, ptSize float64) glyph.Seq {
	rr := []rune(s)
	return f.ttf.Layout(rr, f.gsubLookups, f.gposLookups)
}

type embeddedCID struct {
	*CompositeFont
	w pdf.Putter
	pdf.Resource

	cmap.CIDEncoder
	closed bool
}

func (f *embeddedCID) Close() error {
	if f.closed {
		return nil
	}
	f.closed = true

	// subset the font
	encoding := f.CIDEncoder.Encoding()
	CIDSystemInfo := f.CIDEncoder.CIDSystemInfo()
	var ss []subset.Glyph
	ss = append(ss, subset.Glyph{OrigGID: 0, CID: 0})
	for _, p := range encoding {
		ss = append(ss, subset.Glyph{OrigGID: p.GID, CID: p.CID})
	}
	ttfSubset, err := subset.CID(f.ttf, ss, CIDSystemInfo)
	if err != nil {
		return fmt.Errorf("font subset: %w", err)
	}
	subsetTag := subset.Tag(ss, f.ttf.NumGlyphs())

	cmap := make(map[charcode.CharCode]type1.CID)
	for _, s := range ss {
		cmap[charcode.CharCode(s.CID)] = s.CID
	}

	CID2GID := make([]glyph.ID, f.ttf.NumGlyphs())
	for subsetGID, s := range ss {
		CID2GID[s.CID] = glyph.ID(subsetGID)
	}

	toUnicode := make(map[charcode.CharCode][]rune)
	for _, e := range encoding {
		toUnicode[charcode.CharCode(e.CID)] = e.Text
	}

	info := EmbedInfoComposite{
		Font:      ttfSubset,
		SubsetTag: subsetTag,
		CS:        charcode.UCS2,
		ROS:       CIDSystemInfo,
		CMap:      cmap,
		CID2GID:   CID2GID,
		ToUnicode: toUnicode,
	}
	return info.Embed(f.w, f.Ref)
}

type EmbedInfoComposite struct {
	Font      *sfnt.Font
	SubsetTag string

	CS   charcode.CodeSpaceRange
	ROS  *type1.CIDSystemInfo
	CMap map[charcode.CharCode]type1.CID

	CID2GID   []glyph.ID
	ToUnicode map[charcode.CharCode][]rune

	IsAllCap   bool
	IsSmallCap bool
	ForceBold  bool
}

func (info *EmbedInfoComposite) Embed(w pdf.Putter, fontDictRef pdf.Reference) error {
	err := pdf.CheckVersion(w, "composite TrueType fonts", pdf.V1_3)
	if err != nil {
		return err
	}

	ttf := info.Font
	if !ttf.IsGlyf() {
		return fmt.Errorf("not a TrueType font")
	}
	outlines := ttf.Outlines.(*glyf.Outlines)

	fontName := ttf.PostscriptName()
	if info.SubsetTag != "" {
		fontName = info.SubsetTag + "+" + fontName
	}

	// make a CMap
	cmapInfo := cmap.New(info.ROS, info.CS, info.CMap)
	var encoding pdf.Object
	if cmap.IsPredefined(cmapInfo) {
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
	DW, W := font.EncodeCIDWidths(ww, ttf.UnitsPerEm)

	isSymbolic := true

	var CIDToGIDMap pdf.Object
	isIdentity := true
	for cid, gid := range info.CID2GID {
		if cid != int(gid) {
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
		"Registry":   pdf.String(info.ROS.Registry),
		"Ordering":   pdf.String(info.ROS.Ordering),
		"Supplement": pdf.Integer(info.ROS.Supplement),
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
		// TODO(voss): use an indirect object?
		cidFontDict["W"] = W
	}

	fd := &font.Descriptor{
		FontName:     fontName,
		IsFixedPitch: ttf.IsFixedPitch(),
		IsSerif:      ttf.IsSerif,
		IsScript:     ttf.IsScript,
		IsItalic:     ttf.ItalicAngle != 0,
		IsAllCap:     info.IsAllCap,
		IsSmallCap:   info.IsSmallCap,
		ForceBold:    info.ForceBold,
		FontBBox:     fontBBox,
		ItalicAngle:  ttf.ItalicAngle,
		Ascent:       ttf.Ascent.AsFloat(q),
		Descent:      ttf.Descent.AsFloat(q),
		CapHeight:    ttf.CapHeight.AsFloat(q),
	}
	fontDescriptor := fd.AsDict(isSymbolic)
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
	n, err := ttf.WriteTrueTypePDF(fontFileStream, nil)
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
		err = tounicode.Embed(w, toUnicodeRef, charcode.Simple, info.ToUnicode)
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
