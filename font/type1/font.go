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

package type1

import (
	"errors"
	"math"

	"seehuhn.de/go/geom/rect"
	"seehuhn.de/go/postscript/afm"
	"seehuhn.de/go/postscript/funit"
	"seehuhn.de/go/postscript/psenc"
	"seehuhn.de/go/postscript/type1"
	"seehuhn.de/go/postscript/type1/names"

	"seehuhn.de/go/sfnt/glyph"
	"seehuhn.de/go/sfnt/os2"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/dict"
	"seehuhn.de/go/pdf/font/encoding/simpleenc"
	"seehuhn.de/go/pdf/font/glyphdata/type1glyphs"
	"seehuhn.de/go/pdf/font/pdfenc"
	"seehuhn.de/go/pdf/font/subset"
)

// Instance is a Type 1 font instance which can be embedded into a PDF file.
//
// Use [New] to create new font instances.
type Instance struct {
	// Font is the font data to embed.
	*type1.Font

	// Metrics (optional) provides additional information which helps
	// with using the font for typesetting text.  This includes information
	// about kerning and ligatures.
	*afm.Metrics

	// GlyphNames establishes the assignment between GIDs and glyph
	// names.  The slice starts with ".notdef".
	GlyphNames []string

	IsSerif    bool
	IsScript   bool
	IsAllCap   bool
	IsSmallCap bool

	*font.Geometry

	lig  map[glyph.Pair]glyph.ID
	kern map[glyph.Pair]funit.Int16
	cmap map[rune]glyph.ID

	*simpleenc.Simple

	// Name is deprecated and should be left empty.
	// Only used in PDF 1.0 where it was the name used to reference the font
	// from within content streams.
	Name pdf.Name
}

var _ font.Layouter = (*Instance)(nil)

// New creates a new Type 1 PDF font from a Type 1 PostScript font.
// The argument psFont must be present, metrics is optional.
func New(psFont *type1.Font, metrics *afm.Metrics) (*Instance, error) {
	if !isConsistent(psFont, metrics) {
		return nil, errors.New("inconsistent Type 1 font metrics")
	}

	glyphNames := psFont.GlyphList()

	geometry := &font.Geometry{}
	widths := make([]float64, len(glyphNames))
	extents := make([]rect.Rect, len(glyphNames))
	for i, name := range glyphNames {
		// Use metrics for width if available, to match GlyphWidthPDF behavior
		if metrics != nil {
			widths[i] = metrics.GlyphWidthPDF(name) / 1000
		} else {
			widths[i] = psFont.GlyphWidthPDF(name) / 1000
		}
		// GlyphBBoxPDF returns 1000-scale glyph space; convert to text space
		b := psFont.GlyphBBoxPDF(name)
		extents[i] = rect.Rect{
			LLx: b.LLx / 1000,
			LLy: b.LLy / 1000,
			URx: b.URx / 1000,
			URy: b.URy / 1000,
		}
	}
	geometry.UnderlinePosition = float64(psFont.FontInfo.UnderlinePosition) * psFont.FontMatrix[3]
	geometry.UnderlineThickness = float64(psFont.FontInfo.UnderlineThickness) * psFont.FontMatrix[3]
	geometry.Widths = widths
	geometry.GlyphExtents = extents
	if metrics != nil {
		geometry.Ascent = metrics.Ascent / 1000
		geometry.Descent = metrics.Descent / 1000
	} else {
		bbox := psFont.FontBBoxPDF()
		geometry.Ascent = bbox.URy / 1000
		geometry.Descent = bbox.LLy / 1000
	}

	nameGid := make(map[string]glyph.ID, len(glyphNames))
	for i, name := range glyphNames {
		nameGid[name] = glyph.ID(i)
	}

	lig := make(map[glyph.Pair]glyph.ID)
	kern := make(map[glyph.Pair]funit.Int16)
	if metrics != nil {
		for left, name := range glyphNames {
			gi := metrics.Glyphs[name]
			for right, repl := range gi.Ligatures {
				lig[glyph.Pair{Left: glyph.ID(left), Right: nameGid[right]}] = nameGid[repl]
			}
		}
		for _, k := range metrics.Kern {
			left, right := nameGid[k.Left], nameGid[k.Right]
			kern[glyph.Pair{Left: left, Right: right}] = k.Adjust
		}
	}

	cmap := make(map[rune]glyph.ID)
	for gid, name := range glyphNames {
		rr := []rune(names.ToUnicode(name, psFont.FontName))
		if len(rr) != 1 {
			continue
		}
		r := rr[0]

		if _, exists := cmap[r]; exists {
			continue
		}
		cmap[r] = glyph.ID(gid)
	}

	// Initialize encoding state - Type1 fonts are always simple fonts
	notdefWidth := math.Round(widths[0] * 1000)
	simple := simpleenc.NewSimple(
		notdefWidth,
		psFont.FontName,
		&pdfenc.WinAnsi,
	)

	return &Instance{
		Font:       psFont,
		Metrics:    metrics,
		GlyphNames: glyphNames,
		Geometry:   geometry,
		lig:        lig,
		kern:       kern,
		cmap:       cmap,
		Simple:     simple,
	}, nil
}

// IsConsistent checks whether the font metrics are compatible with the
// given font.
func isConsistent(F *type1.Font, M *afm.Metrics) bool {
	if M == nil {
		return true
	}
	qh := F.FontMatrix[0] * 1000
	for name, glyph := range F.Glyphs {
		metrics, ok := M.Glyphs[name]
		if !ok {
			return false
		}
		if math.Abs(glyph.WidthX*qh-metrics.WidthX) > 0.5 {
			return false
		}
	}
	return true
}

// GetName returns the PostScript name of the font.
func (f *Instance) PostScriptName() string {
	if f.Metrics != nil {
		return f.Metrics.FontName
	}
	return f.Font.FontInfo.FontName
}

// FontInfo returns information about the font file.
func (f *Instance) FontInfo() any {
	dict, err := f.makeFontDict()
	if err != nil {
		return nil
	}
	return dict.FontInfo()
}

// Encode converts a glyph ID to a character code.
func (f *Instance) Encode(gid glyph.ID, text string) (charcode.Code, bool) {
	if c, ok := f.Simple.GetCode(gid, text); ok {
		return charcode.Code(c), true
	}

	// Allocate new code
	glyphName := f.GlyphNames[gid]
	width := math.Round(f.GlyphWidthPDF(glyphName))

	c, err := f.Simple.Encode(gid, glyphName, text, width)
	return charcode.Code(c), err == nil
}

// Layout implements the [font.Layouter] interface.
func (f *Instance) Layout(seq *font.GlyphSeq, ptSize float64, s string) *font.GlyphSeq {
	if seq == nil {
		seq = &font.GlyphSeq{}
	}

	base := len(seq.Seq)
	var prev glyph.ID
	for i, r := range s {
		gid := f.cmap[r]
		if i > 0 {
			if repl, ok := f.lig[glyph.Pair{Left: prev, Right: gid}]; ok {
				seq.Seq[len(seq.Seq)-1].GID = repl
				seq.Seq[len(seq.Seq)-1].Text = seq.Seq[len(seq.Seq)-1].Text + string(r)
				seq.Seq[len(seq.Seq)-1].Advance = f.Widths[repl] * ptSize
				prev = repl
				continue
			}
		}
		seq.Seq = append(seq.Seq, font.Glyph{
			GID:     gid,
			Text:    string(r),
			Advance: f.Widths[gid] * ptSize,
		})
		prev = gid
	}

	for i := base; i < len(seq.Seq); i++ {
		g := seq.Seq[i]
		if i > base {
			if adj, ok := f.kern[glyph.Pair{Left: prev, Right: g.GID}]; ok {
				seq.Seq[i-1].Advance += float64(adj) * ptSize / 1000
			}
		}
		prev = g.GID
	}

	return seq
}

// GlyphWidthPDF returns the width of the given glyph in PDF glyph space units.
func (f *Instance) GlyphWidthPDF(glyphName string) float64 {
	if f.Metrics != nil {
		return f.Metrics.GlyphWidthPDF(glyphName)
	} else {
		return f.Font.GlyphWidthPDF(glyphName)
	}
}

// Embed adds the font to a PDF file.
//
// This implements the [font.Instance] interface.
func (f *Instance) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) {
	ref := rm.Alloc()
	rm.Defer(func(eh *pdf.EmbedHelper) error {
		dict, err := f.makeFontDict()
		if err != nil {
			return err
		}
		_, err = eh.EmbedAt(ref, dict)
		return err
	})
	return ref, nil
}

func (f *Instance) makeFontDict() (*dict.Type1, error) {
	if err := f.Simple.Error(); err != nil {
		return nil, pdf.Errorf("font %q: %w", f.Font.FontName, err)
	}

	fontData := f.Font
	metricsData := f.Metrics

	var numGlyphs int
	var postScriptName string
	if metricsData != nil {
		numGlyphs = metricsData.NumGlyphs()
		postScriptName = metricsData.FontName
	} else {
		numGlyphs = fontData.NumGlyphs()
		postScriptName = fontData.FontName
	}

	omitFontData := isStandard(postScriptName, f.Simple)

	glyphs := f.Simple.Glyphs()
	subsetTag := subset.Tag(glyphs, numGlyphs)

	fontSubset := fontData
	metricsSubset := metricsData
	if omitFontData {
		// only subset the font, if the font is embedded
		subsetTag = ""
	} else if subsetTag != "" {
		if fontData != nil {
			fontSubset = clone(fontData)
			fontSubset.Outlines = clone(fontData.Outlines)
			fontSubset.Glyphs = make(map[string]*type1.Glyph)
			for _, gid := range glyphs {
				glyphName := f.GlyphNames[gid]
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
				glyphName := f.GlyphNames[gid]
				if g, ok := metricsData.Glyphs[glyphName]; ok {
					metricsSubset.Glyphs[glyphName] = g
				}
			}
			metricsSubset.Encoding = psenc.StandardEncoding[:]
		}
	}

	fd := &font.Descriptor{
		FontName:   subset.Join(subsetTag, postScriptName),
		IsSerif:    f.IsSerif,
		IsSymbolic: f.Simple.IsSymbolic(),
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
		PostScriptName: postScriptName,
		SubsetTag:      subsetTag,
		Descriptor:     fd,
		Encoding:       f.Simple.Encoding(),
		ToUnicode:      f.Simple.ToUnicode(postScriptName),
		Name:           f.Name,
	}
	for c, info := range f.Simple.MappedCodes() {
		dict.Width[c] = info.Width
	}
	if !omitFontData {
		dict.FontFile = type1glyphs.ToStream(fontSubset)
	}
	return dict, nil
}

func clone[T any](x *T) *T {
	y := *x
	return &y
}
