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

	"seehuhn.de/go/sfnt"
	"seehuhn.de/go/sfnt/cff"
	"seehuhn.de/go/sfnt/glyph"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/dict"
	"seehuhn.de/go/pdf/font/encoding/simpleenc"
	"seehuhn.de/go/pdf/font/glyphdata"
	"seehuhn.de/go/pdf/font/glyphdata/sfntglyphs"
	"seehuhn.de/go/pdf/font/pdfenc"
	"seehuhn.de/go/pdf/font/subset"
)

var _ interface {
	font.EmbeddedLayouter
	font.Embedded
	pdf.Finisher
} = (*embeddedCFFSimple)(nil)

type embeddedCFFSimple struct {
	Ref  pdf.Reference
	Font *sfnt.Font

	*simpleenc.Simple

	finished bool
}

func newEmbeddedCFFSimple(ref pdf.Reference, font *sfnt.Font) *embeddedCFFSimple {
	e := &embeddedCFFSimple{
		Ref:  ref,
		Font: font,

		Simple: simpleenc.NewSimple(
			math.Round(font.GlyphWidthPDF(0)),
			font.PostScriptName(),
			&pdfenc.WinAnsi,
		),
	}
	return e
}

func (e *embeddedCFFSimple) AppendEncoded(s pdf.String, gid glyph.ID, text string) (pdf.String, float64) {
	c, ok := e.Simple.GetCode(gid, text)
	if !ok {
		if e.finished {
			return s, 0
		}

		glyphName := e.Font.GlyphName(gid)
		width := math.Round(e.Font.GlyphWidthPDF(gid))
		var err error
		c, err = e.Simple.Encode(gid, glyphName, text, width)
		if err != nil {
			return s, 0
		}
	}

	w := e.Simple.Width(c)
	return append(s, c), w / 1000
}

func (e *embeddedCFFSimple) Finish(rm *pdf.ResourceManager) error {
	if e.finished {
		return nil
	}
	e.finished = true

	if err := e.Simple.Error(); err != nil {
		return pdf.Errorf("font %q: %w", e.Font.PostScriptName(), err)
	}

	origFont := e.Font
	postScriptName := origFont.PostScriptName()

	origFont = origFont.Clone()
	origFont.CMapTable = nil
	origFont.Gdef = nil
	origFont.Gsub = nil
	origFont.Gpos = nil

	// subset the font, if needed
	glyphs := e.Simple.Glyphs()
	subsetTag := subset.Tag(glyphs, origFont.NumGlyphs())

	var subsetFont *sfnt.Font
	if subsetTag != "" {
		subsetFont = origFont.Subset(glyphs)
	} else {
		subsetFont = origFont
	}

	// convert to a simple font, if needed:
	subsetOutlines := subsetFont.Outlines.(*cff.Outlines)
	if len(subsetOutlines.Private) != 1 {
		return fmt.Errorf("need exactly one private dict for a simple font")
	}
	if len(subsetOutlines.FontMatrices) > 0 && subsetOutlines.FontMatrices[0] != matrix.Identity {
		subsetFont.FontMatrix = subsetOutlines.FontMatrices[0].Mul(subsetFont.FontMatrix)
	}

	subsetOutlines.ROS = nil
	subsetOutlines.GIDToCID = nil
	subsetOutlines.FontMatrices = nil
	for gid, origGID := range glyphs { // fill in the glyph names
		g := subsetOutlines.Glyphs[gid]
		glyphName := e.Simple.GlyphName(origGID)
		if g.Name == glyphName {
			continue
		}
		g = clone(g)
		g.Name = glyphName
		subsetOutlines.Glyphs[gid] = g
	}
	// The real encoding is set in the PDF font dictionary, so that readers can
	// know the meaning of codes without having to parse the font file. Here we
	// set the built-in encoding of the font to the standard encoding, to
	// minimise font size.
	subsetOutlines.Encoding = cff.StandardEncoding(subsetOutlines.Glyphs)

	subsetFont.Outlines = subsetOutlines

	qh := subsetFont.FontMatrix[0] * 1000
	qv := subsetFont.FontMatrix[3] * 1000
	ascent := math.Round(float64(subsetFont.Ascent) * qv)
	descent := math.Round(float64(subsetFont.Descent) * qv)
	leading := math.Round(float64(subsetFont.Ascent-subsetFont.Descent+subsetFont.LineGap) * qv)
	capHeight := math.Round(float64(subsetFont.CapHeight) * qv)
	xHeight := math.Round(float64(subsetFont.XHeight) * qv)

	italicAngle := math.Round(subsetFont.ItalicAngle*10) / 10

	fd := &font.Descriptor{
		FontName:     subset.Join(subsetTag, postScriptName),
		FontFamily:   subsetFont.FamilyName,
		FontStretch:  subsetFont.Width,
		FontWeight:   subsetFont.Weight,
		IsFixedPitch: subsetFont.IsFixedPitch(),
		IsSerif:      subsetFont.IsSerif,
		IsSymbolic:   e.Simple.IsSymbolic(),
		IsScript:     subsetFont.IsScript,
		IsItalic:     subsetFont.IsItalic,
		ForceBold:    subsetOutlines.Private[0].ForceBold,
		FontBBox:     subsetFont.FontBBoxPDF().Rounded(),
		ItalicAngle:  italicAngle,
		Ascent:       ascent,
		Descent:      descent,
		Leading:      leading,
		CapHeight:    capHeight,
		XHeight:      xHeight,
		StemV:        math.Round(subsetOutlines.Private[0].StdVW * qh),
		StemH:        math.Round(subsetOutlines.Private[0].StdHW * qv),
		MissingWidth: e.Simple.DefaultWidth(),
	}
	dict := &dict.Type1{
		PostScriptName: postScriptName,
		SubsetTag:      subsetTag,
		Descriptor:     fd,
		Encoding:       e.Simple.Encoding(),
		FontFile:       sfntglyphs.ToStream(subsetFont, glyphdata.OpenTypeCFFSimple),
		ToUnicode:      e.Simple.ToUnicode(postScriptName),
	}
	for c, info := range e.Simple.MappedCodes() {
		dict.Width[c] = info.Width
	}

	err := dict.WriteToPDF(rm, e.Ref)
	if err != nil {
		return err
	}

	return nil
}
