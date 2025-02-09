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
	"fmt"
	"math"

	"seehuhn.de/go/geom/matrix"
	pscid "seehuhn.de/go/postscript/cid"

	"seehuhn.de/go/sfnt/cff"
	"seehuhn.de/go/sfnt/glyph"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/dict"
	"seehuhn.de/go/pdf/font/glyphdata"
	"seehuhn.de/go/pdf/font/glyphdata/cffglyphs"
	"seehuhn.de/go/pdf/font/subset"
)

type embeddedCompositeOld struct {
	embedded

	cmap.GIDToCID
	cmap.CIDEncoder
}

func (f *embeddedCompositeOld) WritingMode() cmap.WritingMode {
	return 0 // TODO(voss): implement
}

func (f *embeddedCompositeOld) DecodeWidth(s pdf.String) (float64, int) {
	for code, cid := range f.AllCIDs(s) {
		gid := f.GID(cid)
		// TODO(voss): deal with different Font Matrices for different private dicts.
		width := float64(f.sfnt.GlyphWidth(gid)) * f.sfnt.FontMatrix[0]
		return math.Round(width*1000) / 1000, len(code)
	}
	return 0, 0
}

func (f *embeddedCompositeOld) AppendEncoded(s pdf.String, gid glyph.ID, text string) (pdf.String, float64) {
	// TODO(voss): deal with different Font Matrices for different private dicts.
	width := float64(f.sfnt.GlyphWidth(gid)) * f.sfnt.FontMatrix[0]
	s = f.CIDEncoder.AppendEncoded(s, gid, text)
	return s, math.Round(width*1000) / 1000
}

func (f *embeddedCompositeOld) Finish(rm *pdf.ResourceManager) error {
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
	cmapInfo := f.CMap()
	toUnicode := f.ToUnicode()

	// If the CFF font is CID-keyed, then the `charset` table in the CFF font
	// describes the mapping from CIDs to glyphs.  Otherwise, the CID is used
	// as the glyph index directly.
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
		subsetCFF.ROS = &pscid.SystemInfo{
			Registry:   ros.Registry,
			Ordering:   ros.Ordering,
			Supplement: sup,
		}
		subsetCFF.GIDToCID = gidToCID
		for len(subsetCFF.FontMatrices) < len(subsetCFF.Private) {
			// TODO(voss): I think it would be more normal to have the identity
			// matrix in the top dict, and the real font matrix here?
			subsetCFF.FontMatrices = append(subsetCFF.FontMatrices, matrix.Identity)
		}
	}

	postScriptName := subsetCFF.FontInfo.FontName // TODO(voss): try to set this correctly

	ww := make(map[cmap.CID]float64)
	for gid, cid := range gidToCID {
		ww[cid] = math.Round(subsetCFF.GlyphWidthPDF(glyph.ID(gid)))
	}
	dw := math.Round(subsetCFF.GlyphWidthPDF(0))

	isSymbolic := false // TODO(voss): set this correctly

	qh := subsetCFF.FontMatrix[0] * 1000
	qv := subsetCFF.FontMatrix[3] * 1000
	ascent := subsetOTF.Ascent.AsFloat(qv)
	descent := subsetOTF.Descent.AsFloat(qv)
	lineGap := subsetOTF.LineGap.AsFloat(qv)
	var leading float64
	if lineGap > 0 {
		leading = ascent - descent + lineGap
	}
	fd := &font.Descriptor{
		FontName:     subset.Join(subsetTag, postScriptName),
		FontFamily:   subsetOTF.FamilyName,
		FontStretch:  subsetOTF.Width,
		FontWeight:   subsetOTF.Weight,
		IsFixedPitch: subsetOTF.IsFixedPitch(),
		IsSerif:      subsetOTF.IsSerif,
		IsSymbolic:   isSymbolic,
		IsScript:     subsetOTF.IsScript,
		IsItalic:     subsetOTF.IsItalic,
		ForceBold:    subsetCFF.Private[0].ForceBold,
		FontBBox:     subsetOTF.FontBBoxPDF().Rounded(),
		ItalicAngle:  subsetOTF.ItalicAngle,
		Ascent:       math.Round(ascent),
		Descent:      math.Round(descent),
		Leading:      math.Round(leading),
		CapHeight:    math.Round(subsetOTF.CapHeight.AsFloat(qv)),
		XHeight:      math.Round(subsetOTF.XHeight.AsFloat(qv)),
		StemV:        math.Round(subsetCFF.Private[0].StdVW * qh),
		StemH:        math.Round(subsetCFF.Private[0].StdHW * qv),
	}

	fontType := glyphdata.CFF
	fontRef := rm.Out.Alloc()
	err = cffglyphs.Embed(rm.Out, fontType, fontRef, subsetCFF)
	if err != nil {
		return err
	}

	info := &dict.CIDFontType0{
		Ref:            f.ref,
		PostScriptName: postScriptName,
		SubsetTag:      subsetTag,
		Descriptor:     fd,
		ROS:            ros,
		Encoding:       cmapInfo,
		Width:          ww,
		DefaultWidth:   dw,
		Text:           toUnicode,
		FontType:       fontType,
		FontRef:        fontRef,
	}
	return info.WriteToPDF(rm)
}
