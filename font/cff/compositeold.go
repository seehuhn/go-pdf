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
	"math"

	"seehuhn.de/go/geom/matrix"

	pscid "seehuhn.de/go/postscript/cid"

	"seehuhn.de/go/sfnt/cff"
	"seehuhn.de/go/sfnt/glyph"
	"seehuhn.de/go/sfnt/os2"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/dict"
	"seehuhn.de/go/pdf/font/glyphdata"
	"seehuhn.de/go/pdf/font/glyphdata/cffglyphs"
	"seehuhn.de/go/pdf/font/subset"
)

type embedded struct {
	w   *pdf.Writer
	ref pdf.Reference

	Font *cff.Font

	Stretch  os2.Width
	Weight   os2.Weight
	IsSerif  bool
	IsScript bool

	Ascent    float64 // PDF glyph space units
	Descent   float64 // PDF glyph space units
	Leading   float64 // PDF glyph space units
	CapHeight float64 // PDF glyph space units
	XHeight   float64 // PDF glyph space units

	closed bool
}

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
		width := f.Font.GlyphWidthPDF(gid)
		return math.Round(width) / 1000, len(code)
	}
	return 0, 0
}

func (f *embeddedCompositeOld) AppendEncoded(s pdf.String, gid glyph.ID, text string) (pdf.String, float64) {
	width := f.Font.GlyphWidthPDF(gid)
	s = f.CIDEncoder.AppendEncoded(s, gid, text)
	return s, math.Round(width) / 1000
}

func (f *embeddedCompositeOld) Finish(rm *pdf.ResourceManager) error {
	if f.closed {
		return nil
	}
	f.closed = true

	fontInfo := f.Font.FontInfo
	outlines := f.Font.Outlines

	// subset the font
	glyphs := f.CIDEncoder.Subset()
	subsetOutlines := outlines.Subset(glyphs)
	subsetTag := subset.Tag(glyphs, f.Font.NumGlyphs())

	origGIDToCID := f.GIDToCID.GIDToCID(outlines.NumGlyphs())
	gidToCID := make([]pscid.CID, subsetOutlines.NumGlyphs())
	for i, gid := range glyphs {
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

	mustUseCID := len(subsetOutlines.Private) > 1
	if isIdentity && !mustUseCID { // convert to simple font
		subsetOutlines.Encoding = cff.StandardEncoding(subsetOutlines.Glyphs)
		subsetOutlines.ROS = nil
		subsetOutlines.GIDToCID = nil
	} else { // Make the font CID-keyed.
		subsetOutlines.Encoding = nil
		var sup int32
		if ros.Supplement > 0 && ros.Supplement < 0x1000_0000 {
			sup = int32(ros.Supplement)
		}
		subsetOutlines.ROS = &pscid.SystemInfo{
			Registry:   ros.Registry,
			Ordering:   ros.Ordering,
			Supplement: sup,
		}
		subsetOutlines.GIDToCID = gidToCID
		for len(subsetOutlines.FontMatrices) < len(subsetOutlines.Private) {
			// TODO(voss): I think it would be more normal to have the identity
			// matrix in the top dict, and the real font matrix here?
			subsetOutlines.FontMatrices = append(subsetOutlines.FontMatrices, matrix.Identity)
		}
	}

	subsetCFF := &cff.Font{
		FontInfo: fontInfo,
		Outlines: subsetOutlines,
	}

	postScriptName := f.Font.FontName // TODO(voss): try to set this correctly

	ww := make(map[cmap.CID]float64)
	for gid, cid := range gidToCID {
		ww[cid] = math.Round(subsetCFF.GlyphWidthPDF(glyph.ID(gid)))
	}
	dw := math.Round(subsetCFF.GlyphWidthPDF(0))

	isSymbolic := false // TODO(voss): set this correctly

	qh := subsetCFF.FontMatrix[0] * 1000
	qv := subsetCFF.FontMatrix[3] * 1000
	fd := &font.Descriptor{
		FontName:     subset.Join(subsetTag, postScriptName),
		FontFamily:   subsetCFF.FamilyName,
		FontStretch:  f.Stretch,
		FontWeight:   f.Weight,
		IsFixedPitch: subsetCFF.IsFixedPitch,
		IsSerif:      f.IsSerif,
		IsSymbolic:   isSymbolic,
		IsScript:     f.IsScript,
		IsItalic:     subsetCFF.ItalicAngle != 0,
		ForceBold:    subsetCFF.Private[0].ForceBold,
		FontBBox:     subsetCFF.FontBBoxPDF().Rounded(),
		ItalicAngle:  subsetCFF.ItalicAngle,
		Ascent:       math.Round(f.Ascent),
		Descent:      math.Round(f.Descent),
		Leading:      math.Round(f.Leading),
		CapHeight:    math.Round(f.CapHeight),
		XHeight:      math.Round(f.XHeight),
		StemV:        math.Round(subsetOutlines.Private[0].StdVW * qh),
		StemH:        math.Round(subsetOutlines.Private[0].StdHW * qv),
	}

	fontType := glyphdata.CFF
	fontRef := rm.Out.Alloc()
	err := cffglyphs.Embed(rm.Out, fontType, fontRef, subsetCFF)
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
