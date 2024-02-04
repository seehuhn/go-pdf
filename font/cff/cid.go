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
	"io"
	"math"

	pscid "seehuhn.de/go/postscript/cid"
	"seehuhn.de/go/postscript/funit"

	"seehuhn.de/go/sfnt"
	"seehuhn.de/go/sfnt/cff"
	sfntcmap "seehuhn.de/go/sfnt/cmap"
	"seehuhn.de/go/sfnt/glyph"
	"seehuhn.de/go/sfnt/opentype/gtab"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/subset"
	"seehuhn.de/go/pdf/font/widths"
)

type embeddedComposite struct {
	w pdf.Putter
	font.Res
	*font.Geometry

	sfnt        *sfnt.Font
	cmap        sfntcmap.Subtable
	gsubLookups []gtab.LookupIndex
	gposLookups []gtab.LookupIndex

	cmap.GIDToCID
	cmap.CIDEncoder

	closed bool
}

// Layout implements the [font.Layouter] interface.
func (f *embeddedComposite) Layout(ptSize float64, s string) *font.GlyphSeq {
	gg := f.sfnt.Layout(f.cmap, f.gsubLookups, f.gposLookups, s)
	res := &font.GlyphSeq{
		Seq: make([]font.Glyph, len(gg)),
	}
	for i, g := range gg {
		xOffset := float64(g.XOffset) * ptSize * f.sfnt.FontMatrix[0]
		if i == 0 {
			res.Skip += xOffset
		} else {
			res.Seq[i-1].Advance += xOffset
		}
		res.Seq[i] = font.Glyph{
			GID:     g.GID,
			Advance: float64(g.Advance) * ptSize * f.sfnt.FontMatrix[0],
			Rise:    float64(g.YOffset) * ptSize * f.sfnt.FontMatrix[3],
			Text:    g.Text,
		}
	}
	return res
}

func (f *embeddedComposite) WritingMode() int {
	return 0 // TODO(voss): implement
}

func (f *embeddedComposite) ForeachWidth(s pdf.String, yield func(float64, bool)) {
	f.AllCIDs(s)(func(code []byte, cid pscid.CID) bool {
		gid := f.GID(cid)
		// TODO(voss): deal with different Font Matrices for different private dicts.
		width := float64(f.sfnt.GlyphWidth(gid)) * f.sfnt.FontMatrix[0]
		yield(width, len(code) == 1 && code[0] == ' ')
		return true
	})
}

func (f *embeddedComposite) CodeAndWidth(s pdf.String, gid glyph.ID, rr []rune) (pdf.String, float64, bool) {
	// TODO(voss): deal with different Font Matrices for different private dicts.
	width := float64(f.sfnt.GlyphWidth(gid)) * f.sfnt.FontMatrix[0]
	k := len(s)
	s = f.CIDEncoder.AppendEncoded(s, gid, rr)
	return s, width, len(s) == k+1 && s[k] == ' '
}

func (f *embeddedComposite) Close() error {
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
		subsetCFF.ROS = ros
		subsetCFF.GIDToCID = gidToCID
	}

	info := EmbedInfoComposite{
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
	return info.Embed(f.w, f.Ref.(pdf.Reference))
}

// EmbedInfoComposite is the information needed to embed a CFF font as a composite PDF font.
type EmbedInfoComposite struct {
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
// This is the reverse of [EmbedInfoComposite.Embed].
func ExtractComposite(r pdf.Getter, dicts *font.Dicts) (*EmbedInfoComposite, error) {
	if err := dicts.Type.MustBe(font.CFFComposite); err != nil {
		return nil, err
	}
	res := &EmbedInfoComposite{}

	stmObj, err := pdf.GetStream(r, dicts.FontProgram)
	if err != nil {
		return nil, err
	}
	stm, err := pdf.DecodeStream(r, stmObj, 0)
	if err != nil {
		return nil, err
	}
	data, err := io.ReadAll(stm)
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
// This is the reverse of [ExtractComposite]
func (info *EmbedInfoComposite) Embed(w pdf.Putter, fontDictRef pdf.Reference) error {
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

	ww := make(map[pscid.CID]float64)
	glyphWidths := cff.Widths()
	if cff.GIDToCID != nil {
		for gid, w := range glyphWidths {
			ww[cff.GIDToCID[gid]] = w.AsFloat(q)
		}
	} else {
		for gid, w := range glyphWidths {
			ww[pscid.CID(gid)] = w.AsFloat(q)
		}
	}
	DW, W := widths.EncodeComposite(ww, pdf.GetVersion(w))

	bbox := cff.BBox()
	fontBBox := &pdf.Rectangle{
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
