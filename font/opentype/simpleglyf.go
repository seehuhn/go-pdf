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

	"seehuhn.de/go/postscript/type1/names"
	"seehuhn.de/go/sfnt"
	sfntcmap "seehuhn.de/go/sfnt/cmap"
	"seehuhn.de/go/sfnt/glyph"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/encoding"
	"seehuhn.de/go/pdf/font/pdfenc"
	"seehuhn.de/go/pdf/font/simple"
	"seehuhn.de/go/pdf/font/subset"
)

type embeddedGlyfSimple struct {
	w   *pdf.Writer
	ref pdf.Reference

	sfnt *sfnt.Font

	*encoding.TrueTypeEncoder

	closed bool
}

func (f *embeddedGlyfSimple) DecodeWidth(s pdf.String) (float64, int) {
	if len(s) == 0 {
		return 0, 0
	}
	gid := f.Encoding[s[0]]
	return f.sfnt.GlyphWidthPDF(gid) / 1000, 1
}

func (f *embeddedGlyfSimple) AppendEncoded(s pdf.String, gid glyph.ID, rr []rune) (pdf.String, float64) {
	width := float64(f.sfnt.GlyphWidth(gid)) / float64(f.sfnt.UnitsPerEm)
	c := f.GIDToCode(gid, rr)
	return append(s, c), width
}

func (f *embeddedGlyfSimple) Finish(rm *pdf.ResourceManager) error {
	if f.closed {
		return nil
	}
	f.closed = true

	if f.TrueTypeEncoder.Overflow() {
		return fmt.Errorf("too many distinct glyphs used in font %q",
			f.sfnt.PostScriptName())
	}
	enc := f.TrueTypeEncoder.Encoding

	origSfnt := f.sfnt.Clone()
	origSfnt.CMapTable = nil
	origSfnt.Gdef = nil
	origSfnt.Gsub = nil
	origSfnt.Gpos = nil

	// subset the font
	subsetGID := f.TrueTypeEncoder.Subset()
	subsetTag := subset.Tag(subsetGID, origSfnt.NumGlyphs())
	subsetSfnt, err := origSfnt.Subset(subsetGID)
	if err != nil {
		return fmt.Errorf("font subset: %w", err)
	}

	subsetGid := make(map[glyph.ID]glyph.ID)
	for gNew, gOld := range subsetGID {
		subsetGid[gOld] = glyph.ID(gNew)
	}
	subsetEncoding := make([]glyph.ID, 256)
	for i, gid := range enc {
		subsetEncoding[i] = subsetGid[gid]
	}

	postScriptName := subsetSfnt.PostScriptName()

	// Follow the advice of section 9.6.5.4 of ISO 32000-2:2020:
	// Only make the font as non-symbolic, if it can be encoded either
	// using "MacRomanEncoding" or "WinAnsiEncoding".
	var isSymbolic bool
	var dictEnc encoding.Type1
	canMacRoman := true
	canWinAnsi := true
	var needsFormat12 bool
	var text [256]string
	f.TrueTypeEncoder.FillText(&text)
	for code, s := range text {
		if !f.TrueTypeEncoder.CodeIsUsed(byte(code)) {
			continue
		}
		rr := []rune(s)
		if len(rr) != 1 {
			canMacRoman = false
			canWinAnsi = false
			break
		}
		r := rr[0]
		if r >= 0x1_0000 {
			needsFormat12 = true
		}
		glyphName := names.FromUnicode(r)
		if pdfenc.MacRoman.Encoding[code] != glyphName {
			canMacRoman = false
		}
		if pdfenc.WinAnsi.Encoding[code] != glyphName {
			canWinAnsi = false
		}
		if !(canMacRoman || canWinAnsi) {
			break
		}
	}
	if !(canMacRoman || canWinAnsi) {
		// Mark the font as "symbolic", and use a (1, 0) "cmap" subtable to map
		// character codes to glyphs.
		isSymbolic = true
		dictEnc = encoding.Builtin

		subtable := sfntcmap.Format4{}
		for code, gid := range subsetEncoding {
			if gid == 0 {
				continue
			}
			subtable[uint16(code)] = gid
		}
		subsetSfnt.CMapTable = sfntcmap.Table{
			{PlatformID: 1, EncodingID: 0}: subtable.Encode(0),
		}
	} else {
		isSymbolic = false
		dictEnc = func(code byte) string {
			if !f.TrueTypeEncoder.CodeIsUsed(byte(code)) {
				return ""
			}
			return names.FromUnicode([]rune(text[code])[0])
		}

		if needsFormat12 {
			subtable := sfntcmap.Format12{}
			for code, gid := range subsetEncoding {
				if !f.TrueTypeEncoder.CodeIsUsed(byte(code)) {
					continue
				}
				r := []rune(text[code])[0]
				subtable[uint32(r)] = gid
			}
			subsetSfnt.CMapTable = sfntcmap.Table{
				{PlatformID: 3, EncodingID: 1}: subtable.Encode(0),
			}
		} else {
			subtable := sfntcmap.Format4{}
			for code, gid := range subsetEncoding {
				if !f.TrueTypeEncoder.CodeIsUsed(byte(code)) {
					continue
				}
				r := []rune(text[code])[0]
				subtable[uint16(r)] = gid
			}
			subsetSfnt.CMapTable = sfntcmap.Table{
				{PlatformID: 3, EncodingID: 1}: subtable.Encode(0),
			}
		}
	}

	q := subsetSfnt.FontMatrix[3] * 1000

	ascent := subsetSfnt.Ascent
	descent := subsetSfnt.Descent
	lineGap := subsetSfnt.LineGap
	var leadingPDF float64
	if lineGap > 0 {
		leadingPDF = (ascent - descent + lineGap).AsFloat(q)
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
		Ascent:       math.Round(ascent.AsFloat(q)),
		Descent:      math.Round(descent.AsFloat(q)),
		Leading:      math.Round(leadingPDF),
		CapHeight:    math.Round(subsetSfnt.CapHeight.AsFloat(q)),
		XHeight:      math.Round(subsetSfnt.XHeight.AsFloat(q)),
		MissingWidth: subsetSfnt.GlyphWidthPDF(0),
	}
	res := &simple.TrueTypeDict{
		Ref:            f.ref,
		PostScriptName: postScriptName,
		SubsetTag:      subsetTag,
		Descriptor:     fd,
		Encoding:       dictEnc,
		IsOpenType:     true,
		GetFont:        func() (any, error) { return subsetSfnt, nil },
	}
	for code := range 256 {
		gid := subsetEncoding[code]
		res.Width[code] = subsetSfnt.GlyphWidthPDF(gid)
	}
	f.TrueTypeEncoder.FillText(&res.Text)

	return res.WriteToPDF(rm)
}
