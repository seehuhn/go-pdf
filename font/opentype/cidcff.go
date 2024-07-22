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
	"fmt"
	"math"

	pscid "seehuhn.de/go/postscript/cid"
	"seehuhn.de/go/postscript/funit"

	"seehuhn.de/go/sfnt"
	"seehuhn.de/go/sfnt/cff"
	"seehuhn.de/go/sfnt/glyph"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/pdfenc"
	"seehuhn.de/go/pdf/font/subset"
	"seehuhn.de/go/pdf/font/widths"
)

type embeddedCFFComposite struct {
	w pdf.Putter
	pdf.Res

	sfnt *sfnt.Font

	cmap.GIDToCID
	cmap.CIDEncoder

	closed bool
}

func (f *embeddedCFFComposite) WritingMode() font.WritingMode {
	return 0 // TODO(voss): implement vertical writing mode
}

func (f *embeddedCFFComposite) ForeachWidth(s pdf.String, yield func(width float64, isSpace bool)) {
	f.AllCIDs(s)(func(code []byte, cid pscid.CID) bool {
		gid := f.GID(cid)
		// TODO(voss): deal with different Font Matrices for different private dicts.
		width := float64(f.sfnt.GlyphWidth(gid)) * f.sfnt.FontMatrix[0]
		yield(width, len(code) == 1 && code[0] == ' ')
		return true
	})
}

func (f *embeddedCFFComposite) CodeAndWidth(s pdf.String, gid glyph.ID, rr []rune) (pdf.String, float64, bool) {
	// TODO(voss): deal with different Font Matrices for different private dicts.
	width := float64(f.sfnt.GlyphWidth(gid)) * f.sfnt.FontMatrix[0]
	k := len(s)
	s = f.CIDEncoder.AppendEncoded(s, gid, rr)
	return s, width, len(s) == k+1 && s[k] == ' '
}

func (f *embeddedCFFComposite) Close() error {
	if f.closed {
		return nil
	}
	f.closed = true

	origSfnt := f.sfnt.Clone()
	origSfnt.CMapTable = nil
	origSfnt.Gdef = nil
	origSfnt.Gsub = nil
	origSfnt.Gpos = nil

	// subset the font
	subsetGID := f.CIDEncoder.Subset()
	subsetTag := subset.Tag(subsetGID, origSfnt.NumGlyphs())
	subsetOTF, err := origSfnt.Subset(subsetGID)
	if err != nil {
		return fmt.Errorf("OpenType/CFF font subset: %w", err)
	}

	origGIDToCID := f.GIDToCID.GIDToCID(origSfnt.NumGlyphs())
	gidToCID := make([]pscid.CID, len(subsetGID))
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

	info := FontDictCFFComposite{
		Font:      subsetOTF,
		SubsetTag: subsetTag,
		CMap:      cmapInfo,
		ToUnicode: toUnicode,
	}
	return info.Embed(f.w, f.Data.(pdf.Reference))
}

// FontDictCFFComposite is the information needed to embed a composite OpenType/CFF font.
type FontDictCFFComposite struct {
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

// ExtractCFFComposite extracts information about a composite OpenType/CFF font from a PDF file.
// This is the inverse of [FontDictCFFComposite.Embed].
func ExtractCFFComposite(r pdf.Getter, dicts *font.Dicts) (*FontDictCFFComposite, error) {
	if err := dicts.Type.MustBe(font.OpenTypeCFFComposite); err != nil {
		return nil, err
	}

	res := &FontDictCFFComposite{}

	stmObj, err := pdf.GetStream(r, dicts.FontProgram)
	if err != nil {
		return nil, err
	}
	stmData, err := pdf.DecodeStream(r, stmObj, 0)
	if err != nil {
		return nil, err
	}
	otf, err := sfnt.Read(stmData)
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

// Embed adds a composite OpenType/CFF font to a PDF file.
// This implements the [font.Dict] interface.
// This is the reverse of [ExtractCFFComposite]
func (info *FontDictCFFComposite) Embed(w pdf.Putter, fontDictRef pdf.Reference) error {
	err := pdf.CheckVersion(w, "composite OpenType/CFF fonts", pdf.V1_6)
	if err != nil {
		return err
	}

	sfnt := info.Font
	if !sfnt.IsCFF() {
		return fmt.Errorf("not an OpenType/CFF font")
	}
	cff := sfnt.AsCFF()

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

	glyphwidths := sfnt.Widths()
	ww := make(map[pscid.CID]float64, len(glyphwidths))
	if cff.GIDToCID != nil {
		for gid, w := range glyphwidths {
			ww[cff.GIDToCID[gid]] = float64(w) * sfnt.FontMatrix[0] * 1000
		}
	} else {
		for gid, w := range glyphwidths {
			ww[pscid.CID(gid)] = float64(w) * sfnt.FontMatrix[0] * 1000
		}
	}
	DW, W := widths.EncodeComposite(ww, pdf.GetVersion(w))

	bbox := sfnt.BBox()

	q := 1000 / float64(sfnt.UnitsPerEm)
	fontBBox := &pdf.Rectangle{
		LLx: bbox.LLx.AsFloat(q),
		LLy: bbox.LLy.AsFloat(q),
		URx: bbox.URx.AsFloat(q),
		URy: bbox.URy.AsFloat(q),
	}

	// isSymbolic := !font.IsNonSymbolic(sfnt)
	isSymbolic := !pdfenc.IsNonSymbolic(sfnt.MakeGlyphNames())

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
		cidFontDict["DW"] = pdf.Number(DW)
	}
	if W != nil {
		cidFontDict["W"] = W
	}

	fd := &font.Descriptor{
		FontName:     cidFontName,
		IsFixedPitch: cff.IsFixedPitch,
		IsSerif:      sfnt.IsSerif,
		IsSymbolic:   isSymbolic,
		IsScript:     sfnt.IsScript,
		IsItalic:     sfnt.IsItalic,
		IsAllCap:     info.IsAllCap,
		IsSmallCap:   info.IsSmallCap,
		ForceBold:    cff.Private[0].ForceBold,
		FontBBox:     fontBBox,
		ItalicAngle:  sfnt.ItalicAngle,
		Ascent:       sfnt.Ascent.AsFloat(q),
		Descent:      sfnt.Descent.AsFloat(q),
		CapHeight:    sfnt.CapHeight.AsFloat(q),
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
	err = sfnt.WriteOpenTypeCFFPDF(fontFileStream)
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
