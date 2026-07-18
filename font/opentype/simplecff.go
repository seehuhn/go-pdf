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
	"errors"
	"math"
	"slices"

	"seehuhn.de/go/geom/matrix"

	"seehuhn.de/go/sfnt"
	"seehuhn.de/go/sfnt/cff"
	"seehuhn.de/go/sfnt/glyph"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/dict"
	"seehuhn.de/go/pdf/font/encoding/simpleenc"
	"seehuhn.de/go/pdf/font/glyphdata"
	"seehuhn.de/go/pdf/font/glyphdata/cffglyphs"
	"seehuhn.de/go/pdf/font/pdfenc"
	"seehuhn.de/go/pdf/font/subset"
)

// SimpleCFF represents a simple OpenType font with CFF outlines.
// This implements the font.Layouter interface.
type SimpleCFF struct {
	*sfnt.Font

	*font.Geometry
	layouter *sfnt.Layouter

	*simpleenc.Simple

	// Name is the PDF resource-dictionary key under which this font is
	// referenced in content streams.  If non-empty, the builder uses this
	// value as the /Font subdictionary key; the spec requires the two to
	// match (PDF 2.0 Table 109).  Required in PDF 1.0; optional in PDF
	// 1.1–1.7; deprecated (forbidden by this library's writer) in PDF 2.0.
	Name pdf.Name
}

var _ font.Layouter = (*SimpleCFF)(nil)

// ResourceName returns the preferred resource-dictionary key for this font.
// See [font.Instance.ResourceName].
func (f *SimpleCFF) ResourceName() pdf.Name {
	return f.Name
}

// newSimpleCFF creates a simple OpenType font with CFF outlines.
func newSimpleCFF(info *sfnt.Font, opt *OptionsSimple) (*SimpleCFF, error) {
	if !info.IsCFF() {
		return nil, errors.New("no CFF outlines in font")
	}

	glyphExtents := info.GlyphBBoxesPDF()
	for i := range glyphExtents {
		glyphExtents[i].Scale(1.0 / 1000)
	}

	q := 1 / float64(info.UnitsPerEm)
	geometry := &font.Geometry{
		GlyphExtents: glyphExtents,
		Widths:       info.WidthsPDF(),

		Ascent:             float64(info.Ascent) * q,
		Descent:            float64(info.Descent) * q,
		Leading:            float64(info.Ascent-info.Descent+info.LineGap) * q,
		UnderlinePosition:  float64(info.UnderlinePosition) * q,
		UnderlineThickness: float64(info.UnderlineThickness) * q,
	}

	layouter, err := info.NewLayouter(opt.Language, opt.GsubFeatures, opt.GposFeatures)
	if err != nil {
		return nil, err
	}

	f := &SimpleCFF{
		Font:     info,
		Geometry: geometry,
		layouter: layouter,
	}

	notdefWidth := math.Round(info.GlyphWidthPDF(0))
	f.Simple = simpleenc.NewSimple(
		notdefWidth,
		info.PostScriptName(),
		&pdfenc.WinAnsi,
	)

	return f, nil
}

// FontInfo returns information required to load the font file and to
// extract the the glyph corresponding to a character identifier.
func (f *SimpleCFF) FontInfo() any {
	return &dict.FontInfoSimple{
		PostScriptName: f.Font.PostScriptName(),
		FontFile:       &glyphdata.Stream{},
		Encoding:       f.Simple.Encoding(),
		IsSymbolic:     f.isSymbolic(),
	}
}

// Embed adds the font to a PDF file.
func (f *SimpleCFF) Embed(e *pdf.EmbedHelper) (pdf.Native, error) {
	if err := pdf.CheckVersion(e.Out(), "OpenType fonts", pdf.V1_6); err != nil {
		return nil, err
	}

	ref := e.Alloc()
	e.Defer(func(eh *pdf.EmbedHelper) error {
		dict, err := f.makeDict()
		if err != nil {
			return err
		}
		_, err = eh.EmbedAt(ref, dict)
		return err
	})

	return ref, nil
}

// Encode converts a glyph ID to a character code.
func (f *SimpleCFF) Encode(gid glyph.ID, text string) (charcode.Code, bool) {
	if c, ok := f.Simple.GetCode(gid, text); ok {
		return charcode.Code(c), true
	}

	width := math.Round(f.Font.GlyphWidthPDF(gid))
	c, err := f.Simple.Encode(gid, f.Font.GlyphName(gid), text, width)
	return charcode.Code(c), err == nil
}

// Layout appends a string to a glyph sequence.
func (f *SimpleCFF) Layout(seq *font.GlyphSeq, ptSize float64, s string) *font.GlyphSeq {
	if seq == nil {
		seq = &font.GlyphSeq{}
	}

	// Layouter advances/offsets are in UnitsPerEm; scale uniformly to points.
	q := ptSize / float64(f.Font.UnitsPerEm)

	buf := f.layouter.Layout(s)
	seq.Seq = slices.Grow(seq.Seq, len(buf))
	for _, g := range buf {
		xOffset := float64(g.XOffset) * q
		if len(seq.Seq) == 0 {
			seq.Skip += xOffset
		} else {
			seq.Seq[len(seq.Seq)-1].Advance += xOffset
		}
		seq.Seq = append(seq.Seq, font.Glyph{
			GID:     g.GID,
			Advance: float64(g.Advance) * q,
			Rise:    float64(g.YOffset) * q,
			Text:    string(g.Text),
		})
	}
	return seq
}

func (f *SimpleCFF) isSymbolic() bool {
	// Follow the advice of section 9.6.5.4 of ISO 32000-2:2020, we
	// only make the font as non-symbolic, if it can be encoded either
	// using "MacRomanEncoding" or "WinAnsiEncoding".
	canMacRoman := true
	canWinAnsi := true
	for code := range 256 {
		gid := f.Simple.GID(byte(code))
		if gid == 0 {
			continue
		}
		glyphName := f.Simple.GlyphName(gid)
		if !pdfenc.MacRoman.Has[glyphName] {
			canMacRoman = false
		}
		if !pdfenc.WinAnsi.Has[glyphName] {
			canWinAnsi = false
		}
	}
	return !canMacRoman && !canWinAnsi
}

// makeDict creates the PDF font dictionary for this font.
func (f *SimpleCFF) makeDict() (*dict.Type1, error) {
	if err := f.Simple.Error(); err != nil {
		return nil, pdf.Errorf("font %q: %w", f.Font.PostScriptName(), err)
	}

	// get the CFF font data
	cffFont := f.Font.AsCFF()
	if cffFont == nil {
		return nil, errors.New("no CFF outlines in font")
	}

	fontInfo := cffFont.FontInfo
	outlines := cffFont.Outlines

	// subset the font, if needed
	glyphs := f.Simple.Glyphs()
	subsetTag := subset.Tag(glyphs, outlines.NumGlyphs())

	var subsetOutlines *cff.Outlines
	if subsetTag != "" {
		subsetOutlines = outlines.Subset(glyphs)
	} else {
		subsetOutlines = clone(outlines)
	}

	// convert to a simple font, if needed:
	if len(subsetOutlines.Private) != 1 {
		return nil, errors.New("font must be embedded as a composite font")
	}
	subsetOutlines.ROS = nil
	subsetOutlines.GIDToCID = nil
	if len(subsetOutlines.FontMatrices) > 0 && subsetOutlines.FontMatrices[0] != matrix.Identity {
		fontInfo = clone(fontInfo)
		fontInfo.FontMatrix = subsetOutlines.FontMatrices[0].Mul(fontInfo.FontMatrix)
	}
	subsetOutlines.FontMatrices = nil
	for gid, origGID := range glyphs { // fill in the glyph names
		g := subsetOutlines.Glyphs[gid]
		glyphName := f.Simple.GlyphName(origGID)
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

	subsetCFF := &cff.Font{
		FontInfo: fontInfo,
		Outlines: subsetOutlines,
	}

	// construct the font dictionary and font descriptor
	isSymbolic := f.isSymbolic()

	var widths [256]float64
	for code, info := range f.Simple.MappedCodes() {
		widths[code] = info.Width
	}

	// Ascent, Descent, etc. come from the OpenType OS/2/hhea tables and are
	// measured in UnitsPerEm.  PDF descriptor entries use 1/1000 em.
	qv := 1000 / float64(f.Font.UnitsPerEm)

	// StemV/StemH come from the CFF Private dict in CFF coordinates; the
	// per-FD matrix (if any) has already been composed into subsetCFF.FontMatrix
	// above.
	qhStem := subsetCFF.FontMatrix[0] * 1000
	qvStem := subsetCFF.FontMatrix[3] * 1000

	fd := &font.Descriptor{
		FontName:     subset.Join(subsetTag, f.Font.PostScriptName()),
		FontFamily:   subsetCFF.FamilyName,
		FontStretch:  f.Font.Width,
		FontWeight:   f.Font.Weight,
		IsFixedPitch: f.Font.IsFixedPitch(),
		IsSerif:      f.Font.IsSerif,
		IsSymbolic:   isSymbolic,
		IsScript:     f.Font.IsScript,
		IsItalic:     f.Font.IsItalic,
		FontBBox:     subsetCFF.FontBBoxPDF().Rounded(),
		ItalicAngle:  subsetCFF.ItalicAngle,
		Ascent:       math.Round(float64(f.Font.Ascent) * qv),
		Descent:      math.Round(float64(f.Font.Descent) * qv),
		Leading:      math.Round(float64(f.Font.Ascent-f.Font.Descent+f.Font.LineGap) * qv),
		CapHeight:    math.Round(float64(f.Font.CapHeight) * qv),
		XHeight:      math.Round(float64(f.Font.XHeight) * qv),
		StemV:        math.Round(subsetCFF.Private[0].StdVW * qhStem),
		StemH:        math.Round(subsetCFF.Private[0].StdHW * qvStem),
		AvgWidth:     0, // not specified
		MaxWidth:     0, // not specified
		MissingWidth: f.Simple.DefaultWidth(),
	}

	fontDict := &dict.Type1{
		PostScriptName: f.Font.PostScriptName(),
		SubsetTag:      subsetTag,
		Name:           f.Name,
		Descriptor:     fd,
		Encoding:       f.Simple.Encoding(),
		Width:          widths,
		ToUnicode:      f.Simple.ToUnicode(f.Font.PostScriptName()),
		FontFile:       cffglyphs.ToStream(subsetCFF, glyphdata.CFFSimple),
	}

	return fontDict, nil
}

func clone[T any](obj *T) *T {
	new := *obj
	return &new
}
