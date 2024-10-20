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
	"fmt"
	"math"

	"seehuhn.de/go/geom/rect"
	pscid "seehuhn.de/go/postscript/cid"
	"seehuhn.de/go/postscript/funit"

	"seehuhn.de/go/sfnt/cff"
	"seehuhn.de/go/sfnt/glyph"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/subset"
	"seehuhn.de/go/pdf/font/widths"
)

type embeddedComposite struct {
	embedded

	cmap.GIDToCID
	cmap.CIDEncoder
}

func (f *embeddedComposite) WritingMode() cmap.WritingMode {
	return 0 // TODO(voss): implement
}

func (f *embeddedComposite) DecodeWidth(s pdf.String) (float64, int) {
	for code, cid := range f.AllCIDs(s) {
		gid := f.GID(cid)
		// TODO(voss): deal with different Font Matrices for different private dicts.
		width := float64(f.sfnt.GlyphWidth(gid)) * f.sfnt.FontMatrix[0]
		return width, len(code)
	}
	return 0, 0
}

func (f *embeddedComposite) AppendEncoded(s pdf.String, gid glyph.ID, rr []rune) (pdf.String, float64) {
	// TODO(voss): deal with different Font Matrices for different private dicts.
	width := float64(f.sfnt.GlyphWidth(gid)) * f.sfnt.FontMatrix[0]
	s = f.CIDEncoder.AppendEncoded(s, gid, rr)
	return s, width
}

func (f *embeddedComposite) Finish(*pdf.ResourceManager) error {
	if f.closed {
		return nil
	}
	f.closed = true

	origOTF := f.sfnt.Clone()
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
	gidToCID := make([]pscid.CID, subsetOTF.NumGlyphs())
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
		if cid != 0 && cid != pscid.CID(gid) {
			isIdentity = false
			break
		}
	}
	subsetCFF := subsetOTF.AsCFF().Clone()
	mustUseCID := len(subsetCFF.Private) > 1
	if isIdentity && !mustUseCID { // Make the font non-CID-keyed.
		subsetCFF.Encoding = cff.StandardEncoding(subsetCFF.Glyphs)
		subsetCFF.ROS = nil
		subsetCFF.GIDToCID = nil
	} else { // Make the font CID-keyed.
		subsetCFF.Encoding = nil
		var sup int32
		if ros.Supplement > 0 && ros.Supplement < 0x1000_0000 {
			sup = int32(ros.Supplement)
		}
		subsetCFF.ROS = &cff.CIDSystemInfo{
			Registry:   ros.Registry,
			Ordering:   ros.Ordering,
			Supplement: sup,
		}
		subsetCFF.GIDToCID = gidToCID
	}

	info := FontDictComposite{
		Font:      subsetCFF,
		SubsetTag: subsetTag,
		CMap:      cmapInfo,
		ToUnicode: toUnicode,

		UnitsPerEm: subsetOTF.UnitsPerEm,
		Ascent:     subsetOTF.Ascent,
		Descent:    subsetOTF.Descent,
		CapHeight:  subsetOTF.CapHeight,
		IsSerif:    subsetOTF.IsSerif,
		IsScript:   subsetOTF.IsScript,
	}
	return info.Embed(f.w, f.ref)
}

// FontDictComposite is the information needed to embed a CFF font as a composite PDF font.
type FontDictComposite struct {
	// Font is the font to embed (already subsetted, if needed).
	Font *cff.Font

	// SubsetTag should be a unique tag for the font subset,
	// or the empty string if this is the full font.
	SubsetTag string

	CMap *cmap.Info

	UnitsPerEm uint16 // TODO(voss): get this from the font matrix instead?

	Ascent    funit.Int16
	Descent   funit.Int16
	CapHeight funit.Int16
	IsSerif   bool
	IsScript  bool

	IsAllCap   bool
	IsSmallCap bool

	// ToUnicode (optional) is a map from character codes to unicode strings.
	ToUnicode *cmap.ToUnicode
}

// ExtractComposite extracts information about a composite CFF font from a PDF file.
// This is the reverse of [FontDictComposite.Embed].
func ExtractComposite(r pdf.Getter, dicts *font.Dicts) (*FontDictComposite, error) {
	if err := dicts.FontTypeOld.MustBe(font.CFFComposite); err != nil {
		return nil, err
	}
	res := &FontDictComposite{}

	stmObj, err := pdf.GetStream(r, dicts.FontData)
	if err != nil {
		return nil, err
	}
	data, err := pdf.ReadAll(r, stmObj)
	if err != nil {
		return nil, err
	}
	cff, err := cff.Read(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	res.Font = cff

	postScriptName, _ := pdf.GetName(r, dicts.CIDFontDict["BaseFont"])
	if m := subset.TagRegexp.FindStringSubmatch(string(postScriptName)); m != nil {
		res.SubsetTag = m[1]
	}

	cmapInfo, err := cmap.Extract(r, dicts.FontDict["Encoding"])
	if err != nil {
		return nil, err
	}
	res.CMap = cmapInfo

	// TODO(voss): be more robust here
	unitsPerEm := uint16(math.Round(1 / float64(cff.FontMatrix[0])))
	q := 1000 / float64(unitsPerEm)

	res.UnitsPerEm = unitsPerEm
	res.Ascent = funit.Int16(math.Round(dicts.FontDescriptor.Ascent / q))
	res.Descent = funit.Int16(math.Round(dicts.FontDescriptor.Descent / q))
	res.CapHeight = funit.Int16(math.Round(dicts.FontDescriptor.CapHeight / q))
	res.IsSerif = dicts.FontDescriptor.IsSerif
	res.IsScript = dicts.FontDescriptor.IsScript
	res.IsAllCap = dicts.FontDescriptor.IsAllCap
	res.IsSmallCap = dicts.FontDescriptor.IsSmallCap

	if info, _ := cmap.ExtractToUnicode(r, dicts.FontDict["ToUnicode"], cmapInfo.CodeSpaceRange); info != nil {
		res.ToUnicode = info
	}

	return res, nil
}

// Embed adds a composite CFF font to a PDF file.
// This implements the [font.Dict] interface.
// This is the reverse of [ExtractComposite]
func (info *FontDictComposite) Embed(w *pdf.Writer, fontDictRef pdf.Reference) error {
	err := pdf.CheckVersion(w, "composite CFF fonts", pdf.V1_3)
	if err != nil {
		return err
	}

	cff := info.Font

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

	unitsPerEm := info.UnitsPerEm

	q := 1000 / float64(unitsPerEm)

	ww := make(map[cmap.CID]float64)
	glyphWidths := cff.Widths()
	if cff.GIDToCID != nil {
		for gid, w := range glyphWidths {
			ww[cmap.CID(cff.GIDToCID[gid])] = w * q
		}
	} else {
		for gid, w := range glyphWidths {
			ww[cmap.CID(gid)] = w * q
		}
	}
	W, DW := widths.EncodeComposite(ww, pdf.GetVersion(w))

	bbox := cff.BBox()
	fontBBox := rect.Rect{
		LLx: bbox.LLx.AsFloat(q),
		LLy: bbox.LLy.AsFloat(q),
		URx: bbox.URx.AsFloat(q),
		URy: bbox.URy.AsFloat(q),
	}

	isSymbolic := true // TODO(voss): try to set this correctly

	cidFontRef := w.Alloc()
	fontDescriptorRef := w.Alloc()
	fontFileRef := w.Alloc()

	fontDict := pdf.Dict{
		"Type":            pdf.Name("Font"),
		"Subtype":         pdf.Name("Type0"),
		"BaseFont":        pdf.Name(cidFontName + "-" + cmapInfo.Name),
		"Encoding":        encoding,
		"DescendantFonts": pdf.Array{cidFontRef},
	}
	var toUnicodeRef pdf.Reference
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
		cidFontDict["DW"] = pdf.Number(DW)
	}
	if W != nil {
		cidFontDict["W"] = W
	}

	fd := &font.Descriptor{
		FontName:     cidFontName,
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
	}
	fontDescriptor := fd.AsDict()
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
	err = cff.Write(fontFileStream)
	if err != nil {
		return fmt.Errorf("CFF font program %q: %w", cidFontName, err)
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
