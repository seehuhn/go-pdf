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
	"iter"
	"math"

	"seehuhn.de/go/postscript/type1/names"
	"seehuhn.de/go/sfnt"
	sfntcmap "seehuhn.de/go/sfnt/cmap"
	"seehuhn.de/go/sfnt/glyph"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/dict"
	"seehuhn.de/go/pdf/font/encoding"
	"seehuhn.de/go/pdf/font/encoding/simpleenc"
	"seehuhn.de/go/pdf/font/glyphdata"
	"seehuhn.de/go/pdf/font/glyphdata/opentypeglyphs"
	"seehuhn.de/go/pdf/font/pdfenc"
	"seehuhn.de/go/pdf/font/subset"
)

var _ interface {
	font.EmbeddedLayouter
	font.Scanner
	pdf.Finisher
} = (*embeddedSimple)(nil)

type embeddedSimple struct {
	Ref  pdf.Reference
	Font *sfnt.Font

	gd *simpleenc.Table

	finished bool
}

func newEmbeddedSimple(ref pdf.Reference, font *sfnt.Font) *embeddedSimple {
	e := &embeddedSimple{
		Ref:  ref,
		Font: font,
		gd: simpleenc.NewTable(
			font.GlyphWidthPDF(0),
			font.PostScriptName() == "ZapfDingbats",
			&pdfenc.WinAnsi,
		),
	}

	return e
}

// WritingMode implements the [font.Embedded] interface.
func (*embeddedSimple) WritingMode() cmap.WritingMode {
	return cmap.Horizontal
}

// Codes iterates over the character codes in a PDF string.
func (e *embeddedSimple) Codes(s pdf.String) iter.Seq[*font.Code] {
	return e.gd.Codes(s)
}

func (e *embeddedSimple) DecodeWidth(s pdf.String) (float64, int) {
	if len(s) == 0 {
		return 0, 0
	}
	w := e.gd.Width(s[0])
	return w / 1000, 1
}

func (e *embeddedSimple) AppendEncoded(s pdf.String, gid glyph.ID, text string) (pdf.String, float64) {
	c, ok := e.gd.Code(gid, text)
	if !ok {
		if e.finished {
			return s, 0
		}

		width := math.Round(e.Font.GlyphWidthPDF(gid))
		var err error
		c, err = e.gd.NewCode(gid, e.Font.GlyphName(gid), text, width)
		if err != nil {
			return s, 0
		}
	}

	w := e.gd.Width(c)
	return append(s, c), w / 1000
}

// Finish is called when the resource manager is closed.
// At this point the subset of glyphs to be embedded is known.
func (e *embeddedSimple) Finish(rm *pdf.ResourceManager) error {
	if e.finished {
		return nil
	}
	e.finished = true

	if e.gd.Overflow() {
		return fmt.Errorf("too many distinct glyphs used in font %q",
			e.Font.PostScriptName())
	}

	// subset the font
	origSfnt := e.Font.Clone()
	origSfnt.CMapTable = nil
	origSfnt.Gdef = nil
	origSfnt.Gsub = nil
	origSfnt.Gpos = nil

	glyphs := e.gd.Subset()
	subsetTag := subset.Tag(glyphs, origSfnt.NumGlyphs())
	var subsetFont *sfnt.Font
	if subsetTag != "" {
		subsetFont = origSfnt.Subset(glyphs)
	} else {
		subsetFont = origSfnt
	}

	// Follow the advice of section 9.6.5.4 of ISO 32000-2:2020:
	// Only make the font as non-symbolic, if it can be encoded either
	// using "MacRomanEncoding" or "WinAnsiEncoding".
	var dictEnc encoding.Type1
	canMacRoman := true
	canWinAnsi := true
	for code := range 256 {
		gid := e.gd.GID(byte(code))
		if gid == 0 {
			continue
		}

		glyphName := e.gd.GlyphName(gid)
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

	isSymbolic := !(canMacRoman || canWinAnsi)

	if isSymbolic {
		// Use the built-in encoding, defined by a (1,0) "cmap" subtable which
		// maps codes to a GIDs.

		dictEnc = encoding.Builtin

		subtable := sfntcmap.Format4{}
		for code := range 256 {
			gid := e.gd.GID(byte(code))
			if gid == 0 {
				continue
			}
			subtable[uint16(code)] = gid
		}
		subsetFont.CMapTable = sfntcmap.Table{
			{PlatformID: 1, EncodingID: 0}: subtable.Encode(0),
		}
	} else {
		// Use the encoding to map codes to names, use the Adobe Glyph List to
		// map the names to unicode, and use a (3,1) "cmap" subtable to map
		// unicode to GIDs.

		dictEnc = e.gd.Encoding()

		var needsFormat12 bool
		for _, origGid := range glyphs {
			glyphName := e.gd.GlyphName(origGid)
			rr := names.ToUnicode(glyphName, subsetFont.PostScriptName() == "ZapfDingbats")
			if len(rr) != 1 {
				continue
			}
			if rr[0] > 0xFFFF {
				needsFormat12 = true
				break
			}
		}

		if !needsFormat12 {
			subtable := sfntcmap.Format4{}
			for gid, origGid := range glyphs {
				glyphName := e.gd.GlyphName(origGid)
				rr := names.ToUnicode(glyphName, subsetFont.PostScriptName() == "ZapfDingbats")
				if len(rr) != 1 {
					continue
				}
				subtable[uint16(rr[0])] = glyph.ID(gid)
			}
			subsetFont.CMapTable = sfntcmap.Table{
				{PlatformID: 3, EncodingID: 1}: subtable.Encode(0),
			}
		} else {
			subtable := sfntcmap.Format12{}
			for gid, origGid := range glyphs {
				glyphName := e.gd.GlyphName(origGid)
				rr := names.ToUnicode(glyphName, subsetFont.PostScriptName() == "ZapfDingbats")
				if len(rr) != 1 {
					continue
				}
				subtable[uint32(rr[0])] = glyph.ID(gid)
			}
			subsetFont.CMapTable = sfntcmap.Table{
				{PlatformID: 3, EncodingID: 1}: subtable.Encode(0),
			}
		}
	}

	qv := subsetFont.FontMatrix[3] * 1000
	ascent := math.Round(float64(subsetFont.Ascent) * qv)
	descent := math.Round(float64(subsetFont.Descent) * qv)
	leading := math.Round(float64(subsetFont.Ascent-subsetFont.Descent+subsetFont.LineGap) * qv)
	capHeight := math.Round(float64(subsetFont.CapHeight) * qv)
	xHeight := math.Round(float64(subsetFont.XHeight) * qv)

	italicAngle := math.Round(subsetFont.ItalicAngle*10) / 10

	postScriptName := subsetFont.PostScriptName()
	fd := &font.Descriptor{
		FontName:     subset.Join(subsetTag, postScriptName),
		FontFamily:   subsetFont.FamilyName,
		FontStretch:  subsetFont.Width,
		FontWeight:   subsetFont.Weight,
		IsFixedPitch: subsetFont.IsFixedPitch(),
		IsSerif:      subsetFont.IsSerif,
		IsSymbolic:   isSymbolic,
		IsScript:     subsetFont.IsScript,
		IsItalic:     subsetFont.IsItalic,
		FontBBox:     subsetFont.FontBBoxPDF().Rounded(),
		ItalicAngle:  italicAngle,
		Ascent:       ascent,
		Descent:      descent,
		Leading:      leading,
		CapHeight:    capHeight,
		XHeight:      xHeight,
		MissingWidth: e.gd.DefaultWidth(),
	}

	dict := &dict.TrueType{
		Ref:            e.Ref,
		PostScriptName: postScriptName,
		SubsetTag:      subsetTag,
		Descriptor:     fd,
		Encoding:       dictEnc,
		FontType:       glyphdata.TrueType,
		FontRef:        rm.Out.Alloc(),
	}
	for c := range 256 {
		dict.Width[c] = e.gd.Width(byte(c))
		dict.Text[c] = e.gd.Text(byte(c))
	}

	err := dict.WriteToPDF(rm)
	if err != nil {
		return err
	}

	err = opentypeglyphs.Embed(rm.Out, dict.FontType, dict.FontRef, subsetFont)
	if err != nil {
		return err
	}

	return nil
}
