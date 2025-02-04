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

	"seehuhn.de/go/geom/matrix"
	pscid "seehuhn.de/go/postscript/cid"

	"seehuhn.de/go/sfnt"
	"seehuhn.de/go/sfnt/cff"
	"seehuhn.de/go/sfnt/glyph"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/dict"
	"seehuhn.de/go/pdf/font/glyphdata"
	"seehuhn.de/go/pdf/font/glyphdata/opentypeglyphs"
	"seehuhn.de/go/pdf/font/pdfenc"
	"seehuhn.de/go/pdf/font/subset"
)

type embeddedCFFComposite struct {
	w   *pdf.Writer
	ref pdf.Reference

	sfnt *sfnt.Font

	cmap.GIDToCID
	cmap.CIDEncoder

	closed bool
}

func (f *embeddedCFFComposite) WritingMode() cmap.WritingMode {
	return 0 // TODO(voss): implement vertical writing mode
}

func (f *embeddedCFFComposite) DecodeWidth(s pdf.String) (float64, int) {
	for code, cid := range f.AllCIDs(s) {
		gid := f.GID(cid)
		// TODO(voss): deal with different Font Matrices for different private dicts.
		width := float64(f.sfnt.GlyphWidth(gid)) * f.sfnt.FontMatrix[0]
		return width, len(code)
	}
	return 0, 0
}

func (f *embeddedCFFComposite) AppendEncoded(s pdf.String, gid glyph.ID, rr []rune) (pdf.String, float64) {
	// TODO(voss): deal with different Font Matrices for different private dicts.
	width := float64(f.sfnt.GlyphWidth(gid)) * f.sfnt.FontMatrix[0]
	s = f.CIDEncoder.AppendEncoded(s, gid, rr)
	return s, width
}

func (f *embeddedCFFComposite) Finish(rm *pdf.ResourceManager) error {
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
		var sup int32
		if sup32 := int32(ros.Supplement); ros.Supplement == pdf.Integer(sup32) {
			sup = sup32
		}
		outlines.ROS = &pscid.SystemInfo{
			Registry:   ros.Registry,
			Ordering:   ros.Ordering,
			Supplement: sup,
		}
		outlines.GIDToCID = gidToCID

		outlines.FontMatrices = make([]matrix.Matrix, len(outlines.Private))
		for i := range outlines.Private {
			outlines.FontMatrices[i] = matrix.Identity
		}
	}

	postScriptName := subsetOTF.PostScriptName()

	ww := make(map[cmap.CID]float64)
	for gid, cid := range gidToCID {
		ww[cid] = subsetOTF.GlyphWidthPDF(glyph.ID(gid))
	}
	dw := subsetOTF.GlyphWidthPDF(0)

	isSymbolic := false
	for _, g := range outlines.Glyphs {
		name := g.Name // TODO(voss): is this correct?
		if name == ".notdef" {
			continue
		}
		if !pdfenc.StandardLatin.Has[name] {
			isSymbolic = true
			break
		}
	}

	qh := subsetOTF.FontMatrix[0] * 1000 // TODO(voss): is this correct for CID-keyed fonts?
	qv := subsetOTF.FontMatrix[3] * 1000
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
		ForceBold:    outlines.Private[0].ForceBold,
		FontBBox:     subsetOTF.FontBBoxPDF().Rounded(),
		ItalicAngle:  subsetOTF.ItalicAngle,
		Ascent:       math.Round(ascent),
		Descent:      math.Round(descent),
		Leading:      math.Round(leading),
		CapHeight:    math.Round(subsetOTF.CapHeight.AsFloat(qv)),
		XHeight:      math.Round(subsetOTF.XHeight.AsFloat(qv)),
		StemV:        math.Round(outlines.Private[0].StdVW * qh),
		StemH:        math.Round(outlines.Private[0].StdHW * qv),
	}

	fontType := glyphdata.OpenTypeCFF
	fontRef := rm.Out.Alloc()
	err = opentypeglyphs.Embed(rm.Out, fontType, fontRef, subsetOTF)
	if err != nil {
		return err
	}

	info := &dict.CIDFontType0{
		Ref:            f.ref,
		PostScriptName: postScriptName,
		SubsetTag:      subsetTag,
		Descriptor:     fd,
		ROS:            ros,
		Encoding:       f.CMap(),
		Width:          ww,
		DefaultWidth:   dw,
		Text:           f.ToUnicode(),
		FontType:       fontType,
		FontRef:        fontRef,
	}
	return info.WriteToPDF(rm)
}
