// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2026  Jochen Voss <voss@seehuhn.de>
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

// Package fontdesc derives [font.Descriptor] values from font programs.
//
// The font backends share this code so that a descriptor describes the font
// as designed, whichever glyphs a document ends up embedding.
package fontdesc

import (
	"math"

	"seehuhn.de/go/geom/rect"

	"seehuhn.de/go/postscript/afm"
	"seehuhn.de/go/postscript/type1"

	"seehuhn.de/go/sfnt"
	"seehuhn.de/go/sfnt/cff"
	"seehuhn.de/go/sfnt/os2"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
)

// FromSFNT returns the design part of the font descriptor of an OpenType or
// TrueType font.  Dimensions are in PDF glyph space units, as is fontBBox,
// the union of the glyph bounding boxes, which the caller has from the pass
// over the outlines that supplied the font geometry.
//
// The entries which describe how a document uses the font, rather than the
// font itself, are left at their zero value for the caller to fill in:
// FontName, IsSymbolic and MissingWidth.
//
// The font program must be the complete font.  Deriving any of these entries
// from a subset would make the descriptor depend on which glyphs a document
// happens to show.
func FromSFNT(info *sfnt.Font, fontBBox rect.Rect) *font.Descriptor {
	q := 1000 / float64(info.UnitsPerEm)

	// The angle is rounded so that the value written to the file stays short.
	// A slanted design without the italic bit set still counts as italic.
	italicAngle := pdf.Round(info.ItalicAngle, 1)

	fd := &font.Descriptor{
		FontFamily:   info.FamilyName,
		FontStretch:  info.Width,
		FontWeight:   info.Weight,
		IsFixedPitch: info.IsFixedPitch(),
		IsSerif:      info.IsSerif,
		IsScript:     info.IsScript,
		IsItalic:     info.IsItalic || italicAngle != 0,
		FontBBox:     fontBBox.Rounded(),
		ItalicAngle:  italicAngle,
		Ascent:       math.Round(float64(info.Ascent) * q),
		Descent:      math.Round(float64(info.Descent) * q),
		Leading:      math.Round(float64(info.Ascent-info.Descent+info.LineGap) * q),
		CapHeight:    math.Round(float64(info.CapHeight) * q),
		XHeight:      math.Round(float64(info.XHeight) * q),
	}

	// Stem widths and the ForceBold flag live in the CFF private dictionary.
	// A CID-keyed font has one per font DICT; FD 0 stands for the font as a
	// whole, and its own font matrix converts the stems to PDF glyph space.
	if outlines, ok := info.Outlines.(*cff.Outlines); ok && len(outlines.Private) > 0 {
		private := outlines.Private[0]
		m := outlines.FDMatrix(0, info.FontMatrix)
		fd.ForceBold = private.ForceBold
		fd.StemV = math.Round(private.StdVW * m[0] * 1000)
		fd.StemH = math.Round(private.StdHW * m[3] * 1000)
	}

	return fd
}

// FromType1 returns the design part of the font descriptor of a Type 1 font.
// Dimensions are in PDF glyph space units, as is fontBBox, the union of the
// glyph bounding boxes, which the caller has from its pass over the glyphs.
// The metrics, where given, describe the same font more precisely and take
// precedence, including for the bounding box.  The italic angle is the
// exception: the font program states the angle its outlines are actually
// slanted by, where metrics files have been seen to round it.
//
// The entries which describe how a document uses the font, rather than the
// font itself, are left at their zero value for the caller to fill in:
// FontName, IsSymbolic and MissingWidth.  So is IsSerif, which nothing in a
// Type 1 font program records.
func FromType1(psFont *type1.Font, metrics *afm.Metrics, fontBBox rect.Rect) *font.Descriptor {
	// the angle is rounded so that the value written to the file stays short
	italicAngle := pdf.Round(psFont.ItalicAngle, 1)
	isFixedPitch := psFont.IsFixedPitch
	if metrics != nil {
		fontBBox = metrics.FontBBoxPDF()
		isFixedPitch = metrics.IsFixedPitch
	}
	ascent, descent, capHeight, xHeight := Type1Heights(psFont, metrics, fontBBox)

	// the stem widths are in character space, and the font matrix converts
	// them to PDF glyph space
	return &font.Descriptor{
		FontFamily:   psFont.FamilyName,
		FontWeight:   os2.WeightFromString(psFont.Weight),
		IsFixedPitch: isFixedPitch,
		IsItalic:     italicAngle != 0,
		ForceBold:    psFont.Private.ForceBold,
		FontBBox:     fontBBox.Rounded(),
		ItalicAngle:  italicAngle,
		Ascent:       math.Round(ascent),
		Descent:      math.Round(descent),
		CapHeight:    math.Round(capHeight),
		XHeight:      math.Round(xHeight),
		StemV:        math.Round(psFont.Private.StdVW * psFont.FontMatrix[0] * 1000),
		StemH:        math.Round(psFont.Private.StdHW * psFont.FontMatrix[3] * 1000),
	}
}

// Type1Heights returns the ascent, descent, cap height and x-height of a
// Type 1 font, in PDF glyph space units and unrounded.  Each is taken from the
// metrics where these record it, since an AFM file may omit any of them, and
// derived otherwise: the ascent and descent from the font bounding box, the
// cap height and x-height by measuring the glyphs.  A height the glyphs
// cannot supply, such as the cap height of a font with no "H", is 0.
//
// The argument fontBBox is as for [FromType1]: the union of the glyph
// bounding boxes, superseded by the bounding box of the metrics where given.
func Type1Heights(psFont *type1.Font, metrics *afm.Metrics, fontBBox rect.Rect) (ascent, descent, capHeight, xHeight float64) {
	if metrics != nil {
		fontBBox = metrics.FontBBoxPDF()
		ascent, descent = metrics.Ascent, metrics.Descent
		capHeight, xHeight = metrics.CapHeight, metrics.XHeight
	}
	if ascent == 0 {
		ascent = fontBBox.URy
	}
	if descent == 0 {
		descent = fontBBox.LLy
	}
	if capHeight <= 0 {
		capHeight = psFont.CapHeightPDF()
	}
	if xHeight <= 0 {
		xHeight = psFont.XHeightPDF()
	}
	return ascent, descent, capHeight, xHeight
}
