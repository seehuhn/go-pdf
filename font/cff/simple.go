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
	"seehuhn.de/go/pdf/font/subset"
	"seehuhn.de/go/pdf/font/type1"
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
	return f.sfnt.GlyphWidthPDF(gid), 1
}

func (f *embeddedSimple) AppendEncoded(s pdf.String, gid glyph.ID, rr []rune) (pdf.String, float64) {
	width := f.sfnt.GlyphWidthPDF(gid)
	c := f.SimpleEncoder.GIDToCode(gid, rr)
	return append(s, c), width
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

	q := subsetCFF.FontMatrix[3] * 1000

	ascent := subsetSfnt.Ascent
	descent := subsetSfnt.Descent
	lineGap := subsetSfnt.LineGap
	var leading float64
	if lineGap > 0 {
		leading = (ascent - descent + lineGap).AsFloat(q)
	}
	fd := &font.Descriptor{
		FontName:     subsetTag + "+" + subsetCFF.FontName,
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
		Ascent:       math.Round(ascent.AsFloat(q)),
		Descent:      math.Round(descent.AsFloat(q)),
		Leading:      math.Round(leading),
		CapHeight:    math.Round(subsetSfnt.CapHeight.AsFloat(q)),
		XHeight:      math.Round(subsetSfnt.XHeight.AsFloat(q)),
		StemV:        subsetCFF.Private[0].StdVW,
		StemH:        subsetCFF.Private[0].StdHW,
	}
	res := &type1.FontDict{
		Ref:            f.ref,
		PostScriptName: subsetCFF.FontName,
		SubsetTag:      subsetTag,
		Descriptor:     fd,
		Encoding:       encoding.Builtin,
		GetFont:        func() (type1.FontData, error) { return subsetCFF, nil },
	}

	ww := subsetCFF.WidthsPDF()
	fd.MissingWidth = ww[0]
	for code := range 256 {
		res.Width[code] = ww[subsetCFF.Encoding[code]]
	}

	tu := f.SimpleEncoder.ToUnicodeNew()
	for code, text := range tu {
		res.Text[code[0]] = string(text)
	}

	return res.Finish(rm)
}
