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
	"regexp"

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
	"seehuhn.de/go/sfnt/cff"
	"seehuhn.de/go/sfnt/glyph"
	"seehuhn.de/go/sfnt/opentype/gtab"
)

type CIDFontCFF struct {
	info        *sfnt.Font
	gsubLookups []gtab.LookupIndex
	gposLookups []gtab.LookupIndex
	*font.Geometry
}

func NewComposite(info *sfnt.Font, loc language.Tag) (*CIDFontCFF, error) {
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
	w.AutoClose(res)
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

	info := EmbedInfoComposite{
		Font:       subsetInfo.AsCFF(),
		SubsetTag:  subsetTag,
		CS:         charcode.UCS2,
		ROS:        CIDSystemInfo,
		CMap:       cmap,
		ToUnicode:  toUnicode,
		UnitsPerEm: f.info.UnitsPerEm,
		Ascent:     f.info.Ascent,
		Descent:    f.info.Descent,
		CapHeight:  f.info.CapHeight,
		IsSerif:    f.info.IsSerif,
		IsScript:   f.info.IsScript,
	}
	return info.Embed(f.w, f.Ref)
}

type EmbedInfoComposite struct {
	Font      *cff.Font
	SubsetTag string

	CS   charcode.CodeSpaceRange
	ROS  *type1.CIDSystemInfo
	CMap map[charcode.CharCode]type1.CID

	ToUnicode map[charcode.CharCode][]rune

	UnitsPerEm uint16 // TODO(voss): get this from the font matrix instead?
	Ascent     funit.Int16
	Descent    funit.Int16
	CapHeight  funit.Int16

	IsSerif    bool
	IsScript   bool
	IsAllCap   bool
	IsSmallCap bool
}

// Section 9.7.4.2 of ISO-32000-2 ("Glyph selection in CIDFonts"):
//
// If the "CFF" font program has a Top DICT that does not use CIDFont
// operators: The CIDs shall be used directly as GID values, and the glyph
// procedure shall be retrieved using the CharStrings INDEX.
//
// If the "CFF" font program has a Top DICT that uses CIDFont operators:
// The CIDs shall be used to determine the GID value for the glyph
// procedure using the charset table in the CFF program. The GID value
// shall then be used to look up the glyph procedure using the CharStrings
// INDEX table.

func (info *EmbedInfoComposite) Embed(w pdf.Putter, fontDictRef pdf.Reference) error {
	err := pdf.CheckVersion(w, "CFF CIDFonts", pdf.V1_3)
	if err != nil {
		return err
	}

	cffFont := info.Font

	// CidFontName shall be the value of the CIDFontName entry in the CIDFont program.
	// The name may have a subset prefix if appropriate.
	var cidFontName string
	if info.SubsetTag == "" {
		cidFontName = cffFont.FontInfo.FontName
	} else {
		cidFontName = info.SubsetTag + "+" + cffFont.FontInfo.FontName
	}

	// make a CMap
	cmapInfo := cmap.New(info.ROS, info.CS, info.CMap)
	var encoding pdf.Object
	if cmap.IsPredefined(cmapInfo) {
		encoding = pdf.Name(cmapInfo.Name)
	} else {
		encoding = w.Alloc()
	}

	unitsPerEm := info.UnitsPerEm

	var ww []font.CIDWidth
	widths := cffFont.Widths()
	if cffFont.Gid2Cid != nil { // CID-keyed CFF font
		for gid, w := range widths {
			ww = append(ww, font.CIDWidth{CID: cffFont.Gid2Cid[gid], GlyphWidth: w})
		}
	} else { // simple CFF font
		for gid, w := range widths {
			ww = append(ww, font.CIDWidth{CID: type1.CID(gid), GlyphWidth: w})
		}
	}
	DW, W := font.EncodeCIDWidths(ww, info.UnitsPerEm)

	q := 1000 / float64(unitsPerEm)
	bbox := cffFont.BBox()
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
		IsFixedPitch: cffFont.IsFixedPitch,
		IsSerif:      info.IsSerif,
		IsScript:     info.IsScript,
		IsItalic:     cffFont.ItalicAngle != 0,
		IsAllCap:     info.IsAllCap,
		IsSmallCap:   info.IsSmallCap,
		ForceBold:    cffFont.Private[0].ForceBold,
		FontBBox:     fontBBox,
		ItalicAngle:  cffFont.ItalicAngle,
		Ascent:       info.Ascent.AsFloat(q),
		Descent:      info.Descent.AsFloat(q),
		CapHeight:    info.CapHeight.AsFloat(q),
	}
	fontDescriptor := fd.AsDict(isSymbolic)
	fontDescriptor["FontFile3"] = fontFileRef

	compressedRefs := []pdf.Reference{fontDictRef, cidFontRef, fontDescriptorRef}
	compressedObjects := []pdf.Object{fontDict, cidFontDict, fontDescriptor}
	err = w.WriteCompressed(compressedRefs, compressedObjects...)
	if err != nil {
		return pdf.Wrap(err, "composite CFF font dicts")
	}

	// See section 9.9 of PDF 32000-1:2008 for details.
	fontFileDict := pdf.Dict{
		"Subtype": pdf.Name("CIDFontType0C"),
	}
	fontFileStream, err := w.OpenStream(fontFileRef, fontFileDict, pdf.FilterCompress{})
	if err != nil {
		return err
	}
	err = cffFont.Encode(fontFileStream)
	if err != nil {
		return fmt.Errorf("embedding CFF CIDFont %q: %w", cidFontName, err)
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

func ExtractCIDInfo(r pdf.Getter, ref pdf.Reference) (*EmbedInfoComposite, error) {
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
	if m := subsetTagRegexp.FindStringSubmatch(string(postScriptName)); m != nil {
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
	} else if subType != "CIDFontType0C" {
		return nil, fmt.Errorf("invalid font program subtype: %v", subType)
	}
	stm, err := pdf.DecodeStream(r, fontProgramStm, 0)
	if err != nil {
		return nil, err
	}
	data, err := io.ReadAll(stm)
	if err != nil {
		return nil, err
	}
	cffFont, err := cff.Read(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	// TODO(voss): be more robust here
	unitsPerEm := uint16(math.Round(1 / float64(cffFont.FontMatrix[0])))
	q := 1000 / float64(unitsPerEm)

	res := &EmbedInfoComposite{
		Font:      cffFont,
		SubsetTag: subsetTag,
		CS:        cmap.CS,
		ROS:       ROS,
		CMap:      cmap.GetMapping(),
		ToUnicode: toUnicode,

		UnitsPerEm: unitsPerEm,
		Ascent:     funit.Int16(math.Round(float64(ascent) / q)),
		Descent:    funit.Int16(math.Round(float64(descent) / q)),
		CapHeight:  funit.Int16(math.Round(float64(capHeight) / q)),
		IsSerif:    flags&font.FlagSerif != 0,
		IsScript:   flags&font.FlagScript != 0,
		IsAllCap:   flags&font.FlagAllCap != 0,
		IsSmallCap: flags&font.FlagSmallCap != 0,
	}
	return res, nil
}

var subsetTagRegexp = regexp.MustCompile(`^([A-Z]{6})\+(.*)$`)
