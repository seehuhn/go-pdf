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
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/subset"
	"seehuhn.de/go/pdf/font/tounicode"
	"seehuhn.de/go/postscript/funit"
	"seehuhn.de/go/postscript/type1"
	"seehuhn.de/go/sfnt"
	"seehuhn.de/go/sfnt/glyph"
	"seehuhn.de/go/sfnt/opentype/gtab"
)

type CIDFontCFF struct {
	info        *sfnt.Font
	gsubLookups []gtab.LookupIndex
	gposLookups []gtab.LookupIndex
	*font.Geometry
}

func NewCompositeCFF(info *sfnt.Font, loc language.Tag) (*CIDFontCFF, error) {
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

	res := &CIDFontCFF{
		info:        info,
		gsubLookups: info.Gsub.FindLookups(loc, gtab.GsubDefaultFeatures),
		gposLookups: info.Gpos.FindLookups(loc, gtab.GposDefaultFeatures),
		Geometry:    geometry,
	}
	return res, nil
}

func (f *CIDFontCFF) Embed(w pdf.Putter, resName pdf.Name) (font.Embedded, error) {
	err := pdf.CheckVersion(w, "use of OpenType fonts", pdf.V1_6)
	if err != nil {
		return nil, err
	}
	res := &embeddedCIDCFF{
		CIDFontCFF: f,
		w:          w,
		Resource:   pdf.Resource{Ref: w.Alloc(), Name: resName},
		CIDEncoder: cmap.NewCIDEncoder(),
	}
	return res, nil
}

func (f *CIDFontCFF) Layout(s string, ptSize float64) glyph.Seq {
	rr := []rune(s)
	return f.info.Layout(rr, f.gsubLookups, f.gposLookups)
}

type embeddedCIDCFF struct {
	*CIDFontCFF
	w pdf.Putter
	pdf.Resource

	cmap.CIDEncoder
	closed bool
}

func (f *embeddedCIDCFF) Close() error {
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
	subsetInfo, err := subset.CID(f.info, ss, CIDSystemInfo)
	if err != nil {
		return fmt.Errorf("font subset: %w", err)
	}
	subsetTag := subset.Tag(ss, f.info.NumGlyphs())

	cmap := make(map[charcode.CharCode]type1.CID)
	for _, s := range ss {
		cmap[charcode.CharCode(s.CID)] = s.CID
	}

	toUnicode := make(map[charcode.CharCode][]rune)
	for _, e := range encoding {
		toUnicode[charcode.CharCode(e.CID)] = e.Text
	}

	info := EmbedInfoCIDCFF{
		Font:      subsetInfo,
		SubsetTag: subsetTag,
		CS:        charcode.UCS2,
		ROS:       CIDSystemInfo,
		CMap:      cmap,
		ToUnicode: toUnicode,
	}
	return info.Embed(f.w, f.Ref)
}

type EmbedInfoCIDCFF struct {
	Font      *sfnt.Font
	SubsetTag string

	CS   charcode.CodeSpaceRange
	ROS  *type1.CIDSystemInfo
	CMap map[charcode.CharCode]type1.CID

	ToUnicode map[charcode.CharCode][]rune

	IsAllCap   bool
	IsSmallCap bool
}

func (info *EmbedInfoCIDCFF) Embed(w pdf.Putter, fontDictRef pdf.Reference) error {
	err := pdf.CheckVersion(w, "OpenType fonts", pdf.V1_6)
	if err != nil {
		return err
	}

	otf := info.Font
	if !otf.IsCFF() {
		return fmt.Errorf("not a CFF font")
	}
	cff := otf.AsCFF()

	// CidFontName shall be the value of the CIDFontName entry in the CIDFont program.
	// The name may have a subset prefix if appropriate.
	cidFontName := otf.PostscriptName()
	if info.SubsetTag != "" {
		cidFontName = info.SubsetTag + "+" + cidFontName
	}

	// make a CMap
	cmapInfo := cmap.New(info.ROS, info.CS, info.CMap)
	var encoding pdf.Object
	if cmap.IsPredefined(cmapInfo) {
		encoding = pdf.Name(cmapInfo.Name)
	} else {
		encoding = w.Alloc()
	}

	unitsPerEm := otf.UnitsPerEm

	var ww []font.CIDWidth
	widths := otf.Widths()
	if cff.Gid2Cid != nil { // CID-keyed CFF font
		for gid, w := range widths {
			ww = append(ww, font.CIDWidth{CID: cff.Gid2Cid[gid], GlyphWidth: w})
		}
	} else { // simple CFF font
		for gid, w := range widths {
			ww = append(ww, font.CIDWidth{CID: type1.CID(gid), GlyphWidth: w})
		}
	}
	DW, W := font.EncodeCIDWidths(ww, otf.UnitsPerEm)

	q := 1000 / float64(unitsPerEm)
	bbox := otf.BBox()
	fontBBox := &pdf.Rectangle{
		LLx: bbox.LLx.AsFloat(q),
		LLy: bbox.LLy.AsFloat(q),
		URx: bbox.URx.AsFloat(q),
		URy: bbox.URy.AsFloat(q),
	}

	isSymbolic := true

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
		"Registry":   pdf.String(info.ROS.Registry),
		"Ordering":   pdf.String(info.ROS.Ordering),
		"Supplement": pdf.Integer(info.ROS.Supplement),
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
		// TODO(voss): use an indirect object?
		cidFontDict["W"] = W
	}

	fd := &font.Descriptor{
		FontName:     cidFontName,
		IsFixedPitch: cff.IsFixedPitch,
		IsSerif:      otf.IsSerif,
		IsScript:     otf.IsScript,
		IsItalic:     otf.ItalicAngle != 0,
		IsAllCap:     info.IsAllCap,
		IsSmallCap:   info.IsSmallCap,
		ForceBold:    cff.Private[0].ForceBold,
		FontBBox:     fontBBox,
		ItalicAngle:  otf.ItalicAngle,
		Ascent:       otf.Ascent.AsFloat(q),
		Descent:      otf.Descent.AsFloat(q),
		CapHeight:    otf.CapHeight.AsFloat(q),
	}
	fontDescriptor := fd.AsDict(isSymbolic)
	fontDescriptor["FontFile3"] = fontFileRef

	compressedRefs := []pdf.Reference{fontDictRef, cidFontRef, fontDescriptorRef}
	compressedObjects := []pdf.Object{fontDict, cidFontDict, fontDescriptor}
	err = w.WriteCompressed(compressedRefs, compressedObjects...)
	if err != nil {
		return err
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
		return fmt.Errorf("embedding CFF-based OpenType CIDFont %q: %w", cidFontName, err)
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

	return nil
}

func ExtractEmbedInfoCFF(r pdf.Getter, ref pdf.Reference) (*EmbedInfoCIDCFF, error) {
	fontDict, err := pdf.GetDict(r, ref)
	if err != nil {
		return nil, err
	}
	err = pdf.CheckDictType(r, fontDict, "Font")
	if err != nil {
		return nil, err
	}
	subType, err := pdf.GetName(r, fontDict["Subtype"])
	if err != nil || (subType != "Type0" && subType != "") {
		return nil, fmt.Errorf("invalid font subtype: %v", fontDict["Subtype"])
	}

	cmap, err := cmap.Extract(r, fontDict["Encoding"])
	if err != nil {
		return nil, err
	}

	descendantFonts, err := pdf.GetArray(r, fontDict["DescendantFonts"])
	if err != nil {
		return nil, err
	} else if len(descendantFonts) != 1 {
		return nil, fmt.Errorf("invalid descendant fonts: %v", descendantFonts)
	}

	var toUnicode map[charcode.CharCode][]rune
	if info, _ := tounicode.Extract(r, fontDict["ToUnicode"]); info != nil {
		// TODO(voss): check that the codespace ranges are compatible with the cmap.
		toUnicode = info.GetMapping()
	}

	cidFontDict, err := pdf.GetDict(r, descendantFonts[0])
	if err != nil {
		return nil, err
	}
	err = pdf.CheckDictType(r, cidFontDict, "Font")
	if err != nil {
		return nil, err
	}

	postScriptName, _ := pdf.GetName(r, cidFontDict["BaseFont"])
	var subsetTag string
	if m := subset.TagRegexp.FindStringSubmatch(string(postScriptName)); m != nil {
		subsetTag = m[1]
	}
	var ROS *type1.CIDSystemInfo
	if rosDict, _ := pdf.GetDict(r, cidFontDict["CIDSystemInfo"]); rosDict != nil {
		registry, _ := pdf.GetString(r, rosDict["Registry"])
		ordering, _ := pdf.GetString(r, rosDict["Ordering"])
		supplement, _ := pdf.GetInteger(r, rosDict["Supplement"])
		ROS = &type1.CIDSystemInfo{
			Registry:   string(registry),
			Ordering:   string(ordering),
			Supplement: int32(supplement),
		}
	}
	if ROS == nil {
		ROS = cmap.ROS
	}

	fontDescriptor, err := pdf.GetDict(r, cidFontDict["FontDescriptor"])
	if err != nil {
		return nil, err
	}
	err = pdf.CheckDictType(r, fontDescriptor, "FontDescriptor")
	if err != nil {
		return nil, err
	}
	ascent, _ := pdf.GetNumber(r, fontDescriptor["Ascent"])
	descent, _ := pdf.GetNumber(r, fontDescriptor["Descent"])
	capHeight, _ := pdf.GetNumber(r, fontDescriptor["CapHeight"])
	flagsInt, _ := pdf.GetInteger(r, fontDescriptor["Flags"])
	flags := font.Flags(flagsInt)

	fontProgramStm, err := pdf.GetStream(r, fontDescriptor["FontFile3"])
	if err != nil {
		return nil, err
	}
	subType, err = pdf.GetName(r, fontProgramStm.Dict["Subtype"])
	if err != nil {
		return nil, err
	} else if subType != "OpenType" {
		return nil, fmt.Errorf("invalid font program subtype: %v", subType)
	}
	stm, err := pdf.DecodeStream(r, fontProgramStm, 0)
	if err != nil {
		return nil, err
	}
	otf, err := sfnt.Read(stm)
	if err != nil {
		return nil, err
	}

	// Most OpenType tables will be missing, so we fill in information from
	// the font descriptor instead.
	otf.IsSerif = flags&font.FlagSerif != 0
	otf.IsScript = flags&font.FlagScript != 0
	q := 1000 / float64(otf.UnitsPerEm)
	otf.Ascent = funit.Int16(math.Round(float64(ascent) / q))
	otf.Descent = funit.Int16(math.Round(float64(descent) / q))
	otf.CapHeight = funit.Int16(math.Round(float64(capHeight) / q))

	res := &EmbedInfoCIDCFF{
		Font:      otf,
		SubsetTag: subsetTag,
		CS:        cmap.CS,
		ROS:       ROS,
		CMap:      cmap.GetMapping(),
		ToUnicode: toUnicode,

		IsAllCap:   flags&font.FlagAllCap != 0,
		IsSmallCap: flags&font.FlagSmallCap != 0,
	}
	return res, nil
}
