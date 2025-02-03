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

	"seehuhn.de/go/sfnt/cff"
	"seehuhn.de/go/sfnt/glyph"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/encoding"
	"seehuhn.de/go/pdf/font/pdfenc"
	"seehuhn.de/go/pdf/font/simple"
	"seehuhn.de/go/pdf/font/subset"
)

type embeddedSimple struct {
	embedded

	*encoding.SimpleEncoder
}

func (f *embeddedSimple) DecodeWidth(s pdf.String) (float64, int) {
	if len(s) == 0 {
		return 0, 0
	}
	gid := f.Encoding[s[0]]
	return math.Round(f.sfnt.GlyphWidthPDF(gid)) / 1000, 1
}

func (f *embeddedSimple) AppendEncoded(s pdf.String, gid glyph.ID, rr []rune) (pdf.String, float64) {
	width := f.sfnt.GlyphWidthPDF(gid) / 1000
	c := f.SimpleEncoder.GIDToCode(gid, rr)
	return append(s, c), math.Round(width*1000) / 1000
}

func (f *embeddedSimple) Finish(rm *pdf.ResourceManager) error {
	if f.closed {
		return nil
	}
	f.closed = true

	if f.SimpleEncoder.Overflow() {
		return fmt.Errorf("too many distinct glyphs used in font %q",
			f.sfnt.PostScriptName())
	}

	origSfnt := f.sfnt.Clone()
	origSfnt.CMapTable = nil
	origSfnt.Gdef = nil
	origSfnt.Gsub = nil
	origSfnt.Gpos = nil

	// Make our encoding the built-in encoding of the font.
	outlines := origSfnt.Outlines.(*cff.Outlines)
	outlines.Encoding = f.SimpleEncoder.Encoding
	outlines.ROS = nil
	outlines.GIDToCID = nil

	// subset the font
	subsetGID := f.SimpleEncoder.Subset()
	subsetTag := subset.Tag(subsetGID, origSfnt.NumGlyphs())
	subsetSfnt, err := origSfnt.Subset(subsetGID)
	if err != nil {
		return fmt.Errorf("CFF font subset: %w", err)
	}

	// convert the font to a simple font, if needed
	subsetSfnt.EnsureGlyphNames()
	subsetCFF := subsetSfnt.AsCFF()
	if len(subsetCFF.Private) != 1 {
		return fmt.Errorf("need exactly one private dict for a simple font")
	}

	isSymbolic := false
	for _, g := range subsetCFF.Glyphs {
		name := g.Name
		if name == ".notdef" {
			continue
		}
		if !pdfenc.StandardLatin.Has[name] {
			isSymbolic = true
			break
		}
	}

	qh := subsetCFF.FontMatrix[0] * 1000
	qv := subsetCFF.FontMatrix[3] * 1000
	ascent := subsetSfnt.Ascent.AsFloat(qv)
	descent := subsetSfnt.Descent.AsFloat(qv)
	lineGap := subsetSfnt.LineGap.AsFloat(qv)
	var leading float64
	if lineGap > 0 {
		leading = ascent - descent + lineGap
	}
	fd := &font.Descriptor{
		FontName:     subset.Join(subsetTag, subsetCFF.FontName),
		FontFamily:   subsetSfnt.FamilyName,
		FontStretch:  subsetSfnt.Width,
		FontWeight:   subsetSfnt.Weight,
		IsFixedPitch: subsetSfnt.IsFixedPitch(),
		IsSerif:      subsetSfnt.IsSerif,
		IsSymbolic:   isSymbolic,
		IsScript:     subsetSfnt.IsScript,
		IsItalic:     subsetSfnt.IsItalic,
		ForceBold:    subsetCFF.Private[0].ForceBold,
		FontBBox:     subsetSfnt.FontBBoxPDF().Rounded(),
		ItalicAngle:  subsetSfnt.ItalicAngle,
		Ascent:       math.Round(ascent),
		Descent:      math.Round(descent),
		Leading:      math.Round(leading),
		CapHeight:    math.Round(subsetSfnt.CapHeight.AsFloat(qv)),
		XHeight:      math.Round(subsetSfnt.XHeight.AsFloat(qv)),
		StemV:        math.Round(subsetCFF.Private[0].StdVW * qh),
		StemH:        math.Round(subsetCFF.Private[0].StdHW * qv),
	}
	res := &simple.Type1Dict{
		Ref:            f.ref,
		PostScriptName: subsetCFF.FontName,
		SubsetTag:      subsetTag,
		Descriptor:     fd,
		Encoding:       encoding.Builtin,
		GetFont:        func() (any, error) { return subsetCFF, nil },
	}

	ww := subsetCFF.WidthsPDF()
	fd.MissingWidth = math.Round(ww[0])
	for code := range 256 {
		res.Width[code] = math.Round(ww[subsetCFF.Encoding[code]])
	}

	tu := f.SimpleEncoder.ToUnicode()
	for code, text := range tu {
		res.Text[code[0]] = text
	}

	return res.WriteToPDF(rm)
}
