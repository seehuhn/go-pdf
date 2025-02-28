// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2025  Jochen Voss <voss@seehuhn.de>
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

package type1

import (
	"math"

	"seehuhn.de/go/postscript/afm"
	"seehuhn.de/go/postscript/psenc"
	"seehuhn.de/go/postscript/type1"

	"seehuhn.de/go/sfnt/glyph"
	"seehuhn.de/go/sfnt/os2"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/dict"
	"seehuhn.de/go/pdf/font/encoding/simpleenc"
	"seehuhn.de/go/pdf/font/glyphdata"
	"seehuhn.de/go/pdf/font/glyphdata/type1glyphs"
	"seehuhn.de/go/pdf/font/pdfenc"
	"seehuhn.de/go/pdf/font/subset"
)

var _ interface {
	font.EmbeddedLayouter
	font.Scanner
	pdf.Finisher
} = (*embeddedSimple)(nil)

// embeddedSimple represents an [Instance] which has been embedded in a PDF
// file.
type embeddedSimple struct {
	Ref       pdf.Reference
	Font      *type1.Font
	Metrics   *afm.Metrics
	GlyphList []string

	IsSerif    bool
	IsScript   bool
	IsAllCap   bool
	IsSmallCap bool

	*simpleenc.Simple

	finished bool
}

func newEmbeddedSimple(ref pdf.Reference, f *Instance) *embeddedSimple {
	var glyphList []string
	if f.Metrics != nil {
		glyphList = f.Metrics.GlyphList()
	} else {
		glyphList = f.Font.GlyphList()
	}

	e := &embeddedSimple{
		Ref:       ref,
		Font:      f.Font,
		Metrics:   f.Metrics,
		GlyphList: glyphList,

		IsSerif:    f.IsSerif,
		IsScript:   f.IsScript,
		IsAllCap:   f.IsAllCap,
		IsSmallCap: f.IsSmallCap,

		Simple: simpleenc.NewSimple(
			f.GlyphWidthPDF(".notdef"),
			f.PostScriptName() == "ZapfDingbats",
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

		glyphName := e.GlyphList[gid]
		var width float64
		if e.Metrics != nil {
			width = e.Metrics.GlyphWidthPDF(glyphName)
		} else {
			width = e.Font.GlyphWidthPDF(glyphName)
		}
		width = math.Round(width)

		var err error
		c, err = e.Simple.AllocateCode(gid, glyphName, text, width)
		if err != nil {
			return s, 0
		}
	}

	w := e.Simple.Width(c)
	return append(s, c), w / 1000
}

// Finish is called when the resource manager is closed.
// At this point the subset of glyphs to be embedded is known.
func (e *embeddedSimple) Finish(rm *pdf.ResourceManager) error {
	if e.finished {
		return nil
	}
	e.finished = true

	if err := e.Simple.Error(); err != nil {
		return pdf.Errorf("font %q: %w", e.Font.FontName, err)
	}

	fontData := e.Font
	metricsData := e.Metrics

	var numGlyphs int
	var postScriptName string
	if metricsData != nil {
		numGlyphs = metricsData.NumGlyphs()
		postScriptName = metricsData.FontName
	} else {
		numGlyphs = fontData.NumGlyphs()
		postScriptName = fontData.FontName
	}

	omitFontData := isStandard(postScriptName, e.Simple)

	glyphs := e.Simple.Glyphs()
	subsetTag := subset.Tag(glyphs, numGlyphs)

	fontSubset := fontData
	metricsSubset := metricsData
	if omitFontData {
		// only subset the font, if the font is embedded
		subsetTag = ""
	} else if subsetTag != "" {
		if fontData != nil {
			fontSubset = clone(fontData)
			fontSubset.Glyphs = make(map[string]*type1.Glyph)
			for _, gid := range glyphs {
				glyphName := e.GlyphList[gid]
				if g, ok := fontData.Glyphs[glyphName]; ok {
					fontSubset.Glyphs[glyphName] = g
				}
			}
			fontSubset.Encoding = psenc.StandardEncoding[:]
		}

		if metricsData != nil {
			metricsSubset := clone(metricsData)
			metricsSubset.Glyphs = make(map[string]*afm.GlyphInfo)
			for _, gid := range glyphs {
				glyphName := e.GlyphList[gid]
				if g, ok := metricsData.Glyphs[glyphName]; ok {
					metricsSubset.Glyphs[glyphName] = g
				}
			}
			metricsSubset.Encoding = psenc.StandardEncoding[:]
		}
	}

	fd := &font.Descriptor{
		FontName:   subset.Join(subsetTag, postScriptName),
		IsSerif:    e.IsSerif,
		IsSymbolic: e.Simple.IsSymbolic(),
	}
	if fontSubset != nil {
		fd.FontFamily = fontSubset.FamilyName
		fd.FontWeight = os2.WeightFromString(fontSubset.Weight)
		fd.FontBBox = fontSubset.FontBBoxPDF().Rounded()
		fd.IsItalic = fontSubset.ItalicAngle != 0
		fd.ItalicAngle = fontSubset.ItalicAngle
		fd.IsFixedPitch = fontSubset.IsFixedPitch
		fd.ForceBold = fontSubset.Private.ForceBold
		fd.StemV = fontSubset.Private.StdVW
		fd.StemH = fontSubset.Private.StdHW
	}
	if metricsSubset != nil {
		fd.FontBBox = metricsSubset.FontBBoxPDF().Rounded()
		fd.CapHeight = math.Round(metricsSubset.CapHeight)
		fd.XHeight = math.Round(metricsSubset.XHeight)
		fd.Ascent = math.Round(metricsSubset.Ascent)
		fd.Descent = math.Round(metricsSubset.Descent)
		fd.IsItalic = metricsSubset.ItalicAngle != 0
		fd.ItalicAngle = metricsSubset.ItalicAngle
		fd.IsFixedPitch = metricsSubset.IsFixedPitch
	}
	dict := &dict.Type1{
		Ref:            e.Ref,
		PostScriptName: postScriptName,
		SubsetTag:      subsetTag,
		Descriptor:     fd,
		Encoding:       e.Simple.Encoding(),
	}
	for c, info := range e.Simple.MappedCodes() {
		dict.Width[c] = info.Width
		dict.Text[c] = info.Text
	}
	if omitFontData {
		dict.FontType = glyphdata.None
	} else {
		dict.FontType = glyphdata.Type1
		dict.FontRef = rm.Out.Alloc()
	}

	err := dict.WriteToPDF(rm)
	if err != nil {
		return err
	}

	if dict.FontType == glyphdata.Type1 {
		err := type1glyphs.Embed(rm.Out, dict.FontType, dict.FontRef, fontSubset)
		if err != nil {
			return err
		}
	}

	return nil
}
