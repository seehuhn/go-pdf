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
	"seehuhn.de/go/sfnt/cmap"
	"seehuhn.de/go/sfnt/glyph"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
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
	font.Embedded
	pdf.Finisher
} = (*embeddedSimple)(nil)

type embeddedSimple struct {
	Ref  pdf.Reference
	Font *sfnt.Font

	*simpleenc.Simple

	finished bool
}

func newEmbeddedSimple(ref pdf.Reference, font *sfnt.Font) *embeddedSimple {
	e := &embeddedSimple{
		Ref:  ref,
		Font: font,
		Simple: simpleenc.NewSimple(
			math.Round(font.GlyphWidthPDF(0)),
			font.PostScriptName() == "ZapfDingbats",
			&pdfenc.WinAnsi,
		),
	}

	return e
}

func (e *embeddedSimple) AppendEncoded(s pdf.String, gid glyph.ID, text string) (pdf.String, float64) {
	c, ok := e.Simple.GetCode(gid, text)
	if !ok {
		if e.finished {
			return s, 0
		}

		glyphName := e.Font.GlyphName(gid)
		width := math.Round(e.Font.GlyphWidthPDF(gid))
		var err error
		c, err = e.Simple.AllocateCode(gid, glyphName, text, width)
		if err != nil {
			return s, 0
		}
	}

	w := e.Simple.Width(c)
	return append(s, c), w / 1000
}

func (e *embeddedSimple) Finish(rm *pdf.ResourceManager) error {
	if e.finished {
		return nil
	}
	e.finished = true

	if err := e.Simple.Error(); err != nil {
		return pdf.Wrap(err, fmt.Sprintf("font %q", e.Font.PostScriptName()))
	}

	// subset the font
	origFont := e.Font.Clone()
	origFont.CMapTable = nil
	origFont.Gdef = nil
	origFont.Gsub = nil
	origFont.Gpos = nil

	glyphs := e.Simple.Glyphs()
	subsetTag := subset.Tag(glyphs, origFont.NumGlyphs())
	var subsetFont *sfnt.Font
	if subsetTag != "" {
		subsetFont = origFont.Subset(glyphs)
	} else {
		subsetFont = origFont
	}

	// Follow the advice of section 9.6.5.4 of ISO 32000-2:2020:
	// Only make the font as non-symbolic, if it can be encoded either
	// using "MacRomanEncoding" or "WinAnsiEncoding".
	var dictEnc encoding.Type1
	canMacRoman := true
	canWinAnsi := true
	for code := range 256 {
		gid := e.Simple.GID(byte(code))
		if gid == 0 {
			continue
		}

		glyphName := e.Simple.GlyphName(gid)
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

		subtable := cmap.Format4{}
		for code := range 256 {
			gid := e.Simple.GID(byte(code))
			if gid == 0 {
				continue
			}
			subtable[uint16(code)] = gid
		}
		subsetFont.CMapTable = cmap.Table{
			{PlatformID: 1, EncodingID: 0}: subtable.Encode(0),
		}
	} else {
		// Use the encoding to map codes to names, use the Adobe Glyph List to
		// map the names to unicode, and use a (3,1) "cmap" subtable to map
		// unicode to GIDs.

		dictEnc = e.Simple.Encoding()

		var needsFormat12 bool
		for _, origGid := range glyphs {
			glyphName := e.Simple.GlyphName(origGid)
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
			subtable := cmap.Format4{}
			for gid, origGid := range glyphs {
				glyphName := e.Simple.GlyphName(origGid)
				rr := names.ToUnicode(glyphName, subsetFont.PostScriptName() == "ZapfDingbats")
				if len(rr) != 1 {
					continue
				}
				subtable[uint16(rr[0])] = glyph.ID(gid)
			}
			subsetFont.CMapTable = cmap.Table{
				{PlatformID: 3, EncodingID: 1}: subtable.Encode(0),
			}
		} else {
			subtable := cmap.Format12{}
			for gid, origGid := range glyphs {
				glyphName := e.Simple.GlyphName(origGid)
				rr := names.ToUnicode(glyphName, subsetFont.PostScriptName() == "ZapfDingbats")
				if len(rr) != 1 {
					continue
				}
				subtable[uint32(rr[0])] = glyph.ID(gid)
			}
			subsetFont.CMapTable = cmap.Table{
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
		MissingWidth: e.Simple.DefaultWidth(),
	}

	dict := &dict.TrueType{
		PostScriptName: postScriptName,
		SubsetTag:      subsetTag,
		Descriptor:     fd,
		Encoding:       dictEnc,
		FontType:       glyphdata.TrueType,
		FontRef:        rm.Out.Alloc(),
	}
	for c, info := range e.Simple.MappedCodes() {
		dict.Width[c] = info.Width
		dict.Text[c] = info.Text
	}

	err := dict.WriteToPDF(rm, e.Ref)
	if err != nil {
		return err
	}

	err = opentypeglyphs.Embed(rm.Out, dict.FontType, dict.FontRef, subsetFont)
	if err != nil {
		return err
	}

	return nil
}
