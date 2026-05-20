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

package cff

import (
	"errors"
	"fmt"
	"math"
	"slices"

	"golang.org/x/text/language"

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/geom/rect"

	"seehuhn.de/go/sfnt"
	"seehuhn.de/go/sfnt/cff"
	"seehuhn.de/go/sfnt/glyph"
	"seehuhn.de/go/sfnt/os2"

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

type OptionsSimple struct {
	Language     language.Tag
	GsubFeatures map[string]bool
	GposFeatures map[string]bool
}

// Simple represents a CFF font which can be embedded in a PDF file
// as a simple font.
type Simple struct {
	*cff.Font

	// unitsPerEm caches the source sfnt.Font's UnitsPerEm so Layout can scale
	// layouter output (which is in these units) without holding the sfnt.Font.
	unitsPerEm uint16

	Stretch  os2.Width
	Weight   os2.Weight
	IsSerif  bool
	IsScript bool

	Ascent    float64 // PDF glyph space units
	Descent   float64 // PDF glyph space units
	Leading   float64 // PDF glyph space units
	CapHeight float64 // PDF glyph space units
	XHeight   float64 // PDF glyph space units

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

var _ font.Layouter = (*Simple)(nil)

// ResourceName returns the preferred resource-dictionary key for this font.
// See [font.Instance.ResourceName].
func (f *Simple) ResourceName() pdf.Name {
	return f.Name
}

// NewSimple turns a sfnt.Font into a PDF CFF font.
//
// The font can be embedded as a simple font or as a composite font,
// depending on the options used.
//
// The sfnt.Font info must be an OpenType font with CFF outlines.
func NewSimple(info *sfnt.Font, opt *OptionsSimple) (*Simple, error) {
	if opt == nil {
		opt = &OptionsSimple{}
	}

	cffFont := info.AsCFF()
	if cffFont == nil {
		return nil, errors.New("no CFF outlines in font")
	}

	qv := 1000 / float64(info.UnitsPerEm)
	ascent := math.Round(float64(info.Ascent) * qv)
	descent := math.Round(float64(info.Descent) * qv)
	leading := math.Round(float64(info.Ascent-info.Descent+info.LineGap) * qv)
	capHeight := math.Round(float64(info.CapHeight) * qv)
	xHeight := math.Round(float64(info.XHeight) * qv)
	glyphExtents := make([]rect.Rect, len(cffFont.Glyphs))
	for gid := range cffFont.Glyphs {
		// GlyphBBoxPDF returns 1000-scale glyph space; convert to text space
		b := cffFont.GlyphBBoxPDF(cffFont.FontMatrix, glyph.ID(gid))
		glyphExtents[gid] = rect.Rect{
			LLx: b.LLx / 1000,
			LLy: b.LLy / 1000,
			URx: b.URx / 1000,
			URy: b.URy / 1000,
		}
	}
	geom := &font.Geometry{
		Ascent:             ascent / 1000,
		Descent:            descent / 1000,
		Leading:            leading / 1000,
		UnderlinePosition:  float64(info.UnderlinePosition) * qv / 1000,
		UnderlineThickness: float64(info.UnderlineThickness) * qv / 1000,

		GlyphExtents: glyphExtents,
		Widths:       info.WidthsPDF(),
	}

	layouter, err := info.NewLayouter(opt.Language, opt.GsubFeatures, opt.GposFeatures)
	if err != nil {
		return nil, err
	}

	notdefWidth := math.Round(info.GlyphWidthPDF(0))

	f := &Simple{
		Font:       cffFont,
		unitsPerEm: info.UnitsPerEm,

		Stretch:  info.Width,
		Weight:   info.Weight,
		IsSerif:  info.IsSerif,
		IsScript: info.IsScript,

		Ascent:    ascent,
		Descent:   descent,
		Leading:   leading,
		CapHeight: capHeight,
		XHeight:   xHeight,

		Geometry: geom,
		layouter: layouter,

		Simple: simpleenc.NewSimple(notdefWidth, cffFont.FontName, &pdfenc.WinAnsi),
	}

	return f, nil
}

// FontInfo returns information required to load the font file and to
// extract the the glyph corresponding to a character identifier. The
// result is a pointer to one of the FontInfo* types defined in the
// font/dict package.
func (f *Simple) FontInfo() any {
	dict, err := f.makeFontDict()
	if err != nil {
		return nil
	}
	return dict.FontInfo()
}

// Encode converts a glyph ID to a character code (for use with the
// instance's codec).  The arguments width and text are hints for choosing
// an appropriate advance width and text representation for the character
// code, in case a new code is allocated.
//
// The function returns the character code, and a boolean indicating
// whether the encoding was successful.  If the function returns false, the
// glyph ID cannot be encoded with this font instance.
//
// Use the Codec to append the character code to PDF strings.
//
// Encode converts a glyph ID to a character code.
func (f *Simple) Encode(gid glyph.ID, text string) (charcode.Code, bool) {
	if c, ok := f.Simple.GetCode(gid, text); ok {
		return charcode.Code(c), true
	}

	// Allocate new code
	width := math.Round(f.GlyphWidthPDF(gid))
	c, err := f.Simple.Encode(gid, f.Font.Glyphs[gid].Name, text, width)
	return charcode.Code(c), err == nil
}

// Layout appends a string to a glyph sequence.  The string is typeset at
// the given point size and the resulting GlyphSeq is returned.
//
// If seq is nil, a new glyph sequence is allocated.  If seq is not
// nil, the return value is guaranteed to be equal to seq.
func (f *Simple) Layout(seq *font.GlyphSeq, ptSize float64, s string) *font.GlyphSeq {
	if seq == nil {
		seq = &font.GlyphSeq{}
	}

	// Layouter advances/offsets are in UnitsPerEm; scale uniformly to points.
	q := ptSize / float64(f.unitsPerEm)

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

// Embed converts the Go representation of the object into a PDF object,
// corresponding to the PDF version of the output file.
//
// The first return value is the PDF representation of the object.
// If the object is embedded in the PDF file, this may be a reference.
//
// The second return value is a Go representation of the embedded object.
// In most cases, this value is not used and T can be set to [Unused].
func (f *Simple) Embed(e *pdf.EmbedHelper) (pdf.Native, error) {
	if err := pdf.CheckVersion(e.Out(), "simple CFF fonts", pdf.V1_2); err != nil {
		return nil, err
	}

	ref := e.Alloc()
	e.Defer(func(eh *pdf.EmbedHelper) error {
		dict, err := f.makeFontDict()
		if err != nil {
			return err
		}
		_, err = eh.EmbedAt(ref, dict)
		return err
	})

	return ref, nil
}

func (f *Simple) makeFontDict() (*dict.Type1, error) {
	if err := f.Simple.Error(); err != nil {
		return nil, pdf.Errorf("font %q: %w", f.Font.FontName, err)
	}

	fontInfo := f.Font.FontInfo
	outlines := f.Font.Outlines

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
		return nil, fmt.Errorf("need exactly one private dict for a simple font")
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
	isSymbolic := false
	for _, gid := range glyphs {
		if gid == 0 {
			continue
		}
		if !pdfenc.StandardLatin.Has[f.Simple.GlyphName(gid)] {
			isSymbolic = true
			break
		}
	}

	// StemV/StemH come from the CFF Private dict in CFF coordinates; the
	// per-FD matrix (if any) has already been composed into subsetCFF.FontMatrix
	// above.
	qh := subsetCFF.FontMatrix[0] * 1000
	qv := subsetCFF.FontMatrix[3] * 1000

	fd := &font.Descriptor{
		FontName:     subset.Join(subsetTag, f.Font.FontName),
		FontFamily:   subsetCFF.FamilyName,
		FontStretch:  f.Stretch,
		FontWeight:   f.Weight,
		IsFixedPitch: subsetCFF.IsFixedPitch,
		IsSerif:      f.IsSerif,
		IsSymbolic:   isSymbolic,
		IsScript:     f.IsScript,
		IsItalic:     subsetCFF.ItalicAngle != 0,
		ForceBold:    subsetCFF.Private[0].ForceBold,
		FontBBox:     subsetCFF.FontBBoxPDF().Rounded(),
		ItalicAngle:  subsetCFF.ItalicAngle,
		Ascent:       f.Ascent,
		Descent:      f.Descent,
		Leading:      f.Leading,
		CapHeight:    f.CapHeight,
		XHeight:      f.XHeight,
		StemV:        math.Round(subsetCFF.Private[0].StdVW * qh),
		StemH:        math.Round(subsetCFF.Private[0].StdHW * qv),
		MissingWidth: f.Simple.DefaultWidth(),
	}
	dict := &dict.Type1{
		PostScriptName: f.Font.FontName,
		SubsetTag:      subsetTag,
		Name:           f.Name,
		Descriptor:     fd,
		Encoding:       f.Simple.Encoding(),
		FontFile:       cffglyphs.ToStream(subsetCFF, glyphdata.CFFSimple),
		ToUnicode:      f.Simple.ToUnicode(f.Font.FontName),
	}
	for c, info := range f.Simple.MappedCodes() {
		dict.Width[c] = info.Width
	}

	return dict, nil
}
