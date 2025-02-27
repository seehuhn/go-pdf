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
	"math"

	pscid "seehuhn.de/go/postscript/cid"

	"seehuhn.de/go/sfnt"
	"seehuhn.de/go/sfnt/glyf"
	"seehuhn.de/go/sfnt/glyph"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/dict"
	"seehuhn.de/go/pdf/font/glyphdata"
	"seehuhn.de/go/pdf/font/glyphdata/opentypeglyphs"
	"seehuhn.de/go/pdf/font/subset"
)

type embeddedGlyfComposite struct {
	w   *pdf.Writer
	ref pdf.Reference
	*font.Geometry

	sfnt *sfnt.Font

	cmap.GIDToCID
	cmap.CIDEncoder

	closed bool
}

// WritingMode implements the [font.Embedded] interface.
func (f *embeddedGlyfComposite) WritingMode() font.WritingMode {
	// TODO(voss): implement this
	return 0
}

func (f *embeddedGlyfComposite) DecodeWidth(s pdf.String) (float64, int) {
	for code, cid := range f.AllCIDs(s) {
		gid := f.GID(cid)
		width := f.sfnt.GlyphWidth(gid) / float64(f.sfnt.UnitsPerEm)
		return width, len(code)
	}
	return 0, 0
}

func (f *embeddedGlyfComposite) AppendEncoded(s pdf.String, gid glyph.ID, text string) (pdf.String, float64) {
	width := f.sfnt.GlyphWidth(gid) / float64(f.sfnt.UnitsPerEm)
	s = f.CIDEncoder.AppendEncoded(s, gid, text)
	return s, width
}

func (f *embeddedGlyfComposite) Finish(rm *pdf.ResourceManager) error {
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
	subsetSfnt := origOTF.Subset(subsetGID)

	toUnicode := f.ToUnicode()
	cmapInfo := f.CMap()

	postScriptName := subsetSfnt.PostScriptName()

	// TODO(voss): set this correctly
	isSymbolic := true

	//  The `CIDToGIDMap` entry in the CIDFont dictionary specifies the mapping
	//  from CIDs to glyphs.
	m := make(map[glyph.ID]pscid.CID)
	origGIDToCID := f.GIDToCID.GIDToCID(origOTF.NumGlyphs())
	for origGID, cid := range origGIDToCID {
		m[glyph.ID(origGID)] = cid
	}
	var maxCID pscid.CID
	isIdentity := false
	for _, origGID := range subsetGID {
		cid := m[origGID]
		if cid != pscid.CID(origGID) {
			isIdentity = false
		}
		if cid > maxCID {
			maxCID = cid
		}
	}
	cidToGID := make([]glyph.ID, maxCID+1)
	for subsetGID, origGID := range subsetGID {
		cidToGID[m[origGID]] = glyph.ID(subsetGID)
	}

	qh := subsetSfnt.FontMatrix[0] * 1000
	qv := subsetSfnt.FontMatrix[3] * 1000

	outlines := subsetSfnt.Outlines.(*glyf.Outlines)
	glyphWidths := outlines.Widths
	ww := make(map[cmap.CID]float64, len(glyphWidths))
	for cid, gid := range cidToGID {
		ww[cmap.CID(cid)] = glyphWidths[gid].AsFloat(qh)
	}

	ascent := subsetSfnt.Ascent
	descent := subsetSfnt.Descent
	lineGap := subsetSfnt.LineGap
	var leadingPDF float64
	if lineGap > 0 {
		leadingPDF = (ascent - descent + lineGap).AsFloat(qv)
	}

	fd := &font.Descriptor{
		FontName:     subset.Join(subsetTag, postScriptName),
		FontFamily:   subsetSfnt.FamilyName,
		FontStretch:  subsetSfnt.Width,
		FontWeight:   subsetSfnt.Weight,
		IsFixedPitch: subsetSfnt.IsFixedPitch(),
		IsSerif:      subsetSfnt.IsSerif,
		IsSymbolic:   isSymbolic,
		IsScript:     subsetSfnt.IsScript,
		IsItalic:     subsetSfnt.IsItalic,
		FontBBox:     subsetSfnt.FontBBoxPDF().Rounded(),
		ItalicAngle:  subsetSfnt.ItalicAngle,
		Ascent:       math.Round(ascent.AsFloat(qv)),
		Descent:      math.Round(descent.AsFloat(qv)),
		Leading:      math.Round(leadingPDF),
		CapHeight:    math.Round(subsetSfnt.CapHeight.AsFloat(qv)),
		XHeight:      math.Round(subsetSfnt.XHeight.AsFloat(qv)),
	}

	fontType := glyphdata.OpenTypeGlyf
	fontRef := rm.Out.Alloc()
	err := opentypeglyphs.Embed(f.w, fontType, fontRef, subsetSfnt)
	if err != nil {
		return err
	}

	d := &dict.CIDFontType2{
		Ref:            f.ref,
		PostScriptName: postScriptName,
		SubsetTag:      subsetTag,
		Descriptor:     fd,
		ROS:            cmapInfo.ROS, // TODO(voss): deal with Identity-H and Identity-V
		Encoding:       cmapInfo,
		Width:          ww,
		DefaultWidth:   subsetSfnt.GlyphWidthPDF(0),
		Text:           toUnicode,
		FontType:       fontType,
		FontRef:        fontRef,
	}
	if !isIdentity {
		d.CIDToGID = cidToGID
	}

	return d.WriteToPDF(rm)
}
