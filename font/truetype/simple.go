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

package truetype

import (
	"fmt"
	"math"

	"seehuhn.de/go/postscript/type1/names"

	"seehuhn.de/go/sfnt"
	sfntcmap "seehuhn.de/go/sfnt/cmap"
	"seehuhn.de/go/sfnt/glyph"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/encoding"
	"seehuhn.de/go/pdf/font/pdfenc"
	"seehuhn.de/go/pdf/font/simple"
	"seehuhn.de/go/pdf/font/subset"
)

type embeddedSimple struct {
	w   *pdf.Writer
	ref pdf.Reference

	sfnt *sfnt.Font

	*encoding.SimpleEncoder

	closed bool
}

// WritingMode implements the [font.Embedded] interface.
func (f *embeddedSimple) WritingMode() cmap.WritingMode {
	return 0
}

func (f *embeddedSimple) DecodeWidth(s pdf.String) (float64, int) {
	if len(s) == 0 {
		return 0, 0
	}
	gid := f.Encoding[s[0]]
	return f.sfnt.GlyphWidthPDF(gid) / 1000, 1
}

func (f *embeddedSimple) AppendEncoded(s pdf.String, gid glyph.ID, rr []rune) (pdf.String, float64) {
	width := float64(f.sfnt.GlyphWidth(gid)) / float64(f.sfnt.UnitsPerEm)
	c := f.GIDToCode(gid, rr)
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
	enc := f.SimpleEncoder.Encoding

	origSfnt := f.sfnt.Clone()
	origSfnt.CMapTable = nil
	origSfnt.Gdef = nil
	origSfnt.Gsub = nil
	origSfnt.Gpos = nil

	// subset the font
	subsetGID := f.SimpleEncoder.Subset()
	subsetTag := subset.Tag(subsetGID, origSfnt.NumGlyphs())
	subsetSfnt, err := origSfnt.Subset(subsetGID)
	if err != nil {
		return fmt.Errorf("TrueType font subset: %w", err)
	}

	subsetGid := make(map[glyph.ID]glyph.ID)
	for gNew, gOld := range subsetGID {
		subsetGid[gOld] = glyph.ID(gNew)
	}
	subsetEncoding := make([]glyph.ID, 256)
	for i, gid := range enc {
		subsetEncoding[i] = subsetGid[gid]
	}

	// Mark the font as "symbolic", and use a (1, 0) "cmap" subtable to map
	// character codes to glyphs.
	//
	// TODO(voss): also try the two allowed encodings for "non-symbolic" fonts.
	//
	// TODO(voss): revisit this, once
	// https://github.com/pdf-association/pdf-issues/issues/316 is resolved.
	isSymbolic := true
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

	postScriptName := subsetSfnt.PostScriptName()

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
		Encoding:       encoding.Builtin,
		IsOpenType:     false,
		GetFont:        func() (any, error) { return subsetSfnt, nil },
	}
	for code := range 256 {
		gid := subsetEncoding[code]
		res.Width[code] = subsetSfnt.GlyphWidthPDF(gid)
	}
	f.SimpleEncoder.FillText(&res.Text)

	return res.WriteToPDF(rm)
}

// ExtractEncoding tries to extract an encoding vector from the given encoding
// dictionary.  See section 9.6.5.4 of ISO 32000-2:2020.
//
// TODO(voss): revisit this, once
// https://github.com/pdf-association/pdf-issues/issues/316 is resolved.
func ExtractEncoding(r pdf.Getter, encodingDict pdf.Object, ttf *sfnt.Font) []glyph.ID {
	if encodingEntry, _ := pdf.Resolve(r, encodingDict); encodingEntry != nil {
		encodingNames, _ := encoding.UndescribeEncodingType1(r, encodingEntry, pdfenc.Standard.Encoding[:])
		for i, name := range encodingNames {
			if name == ".notdef" {
				encodingNames[i] = pdfenc.Standard.Encoding[i]
			}
		}

		cmap, _ := ttf.CMapTable.GetNoLang(3, 1)
		if cmap != nil {
			encoding := make([]glyph.ID, 256)
			for code, name := range encodingNames {
				rr := names.ToUnicode(name, false)
				if len(rr) == 1 {
					encoding[code] = cmap.Lookup(rr[0])
				}
			}
			return encoding
		}
		// TODO(voss): also try to use a (1,0) subtable together with encodingNames
	}

	cmap, _ := ttf.CMapTable.GetNoLang(3, 0)
	if cmap != nil {
		encoding := make([]glyph.ID, 256)
		for code := rune(0); code < 256; code++ {
			for _, pfx := range []rune{0xF000, 0xF100, 0xF200, 0x0000} {
				if cmap.Lookup(pfx+code) != 0 {
					encoding[code] = cmap.Lookup(pfx | code)
					break
				}
			}
		}
		return encoding
	}

	cmap, _ = ttf.CMapTable.GetNoLang(1, 0)
	if cmap != nil {
		encoding := make([]glyph.ID, 256)
		for code := rune(0); code < 256; code++ {
			encoding[code] = cmap.Lookup(code)
		}
		return encoding
	}

	// encoding := make([]glyph.ID, 256)
	// for i := range encoding {
	// 	encoding[i] = glyph.ID(i)
	// }
	// return encoding

	return nil
}
