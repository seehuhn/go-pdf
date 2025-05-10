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
	"math"
	"slices"

	"golang.org/x/text/language"

	"seehuhn.de/go/geom/rect"

	"seehuhn.de/go/sfnt"
	"seehuhn.de/go/sfnt/cff"
	"seehuhn.de/go/sfnt/glyph"
	"seehuhn.de/go/sfnt/os2"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/encoding/cidenc"
)

type Options struct {
	Language     language.Tag
	GsubFeatures map[string]bool
	GposFeatures map[string]bool

	// options for composite fonts
	Composite    bool
	WritingMode  font.WritingMode
	MakeGIDToCID func() cmap.GIDToCID
	MakeEncoder  func(cid0Width float64, wMode font.WritingMode) cidenc.CIDEncoder
}

var _ interface {
	font.Layouter
} = (*Instance)(nil)

// Instance represents a CFF font which can be embedded in a PDF file.
type Instance struct {
	*cff.Font
	Opt *Options

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
}

// New turns a sfnt.Font into a PDF CFF font.
//
// The font can be embedded as a simple font or as a composite font,
// depending on the options used.
//
// The sfnt.Font info must be an OpenType font with CFF outlines.
func New(info *sfnt.Font, opt *Options) (*Instance, error) {
	if opt == nil {
		opt = &Options{}
	}

	cffFont := info.AsCFF()
	if cffFont == nil {
		return nil, errors.New("no CFF outlines in font")
	}

	qv := info.FontMatrix[3] * 1000
	ascent := math.Round(float64(info.Ascent) * qv)
	descent := math.Round(float64(info.Descent) * qv)
	leading := math.Round(float64(info.Ascent-info.Descent+info.LineGap) * qv)
	capHeight := math.Round(float64(info.CapHeight) * qv)
	xHeight := math.Round(float64(info.XHeight) * qv)
	glyphExtents := make([]rect.Rect, len(cffFont.Glyphs))
	for gid := range cffFont.Glyphs {
		glyphExtents[gid] = cffFont.GlyphBBoxPDF(cffFont.FontMatrix, glyph.ID(gid))
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

	f := &Instance{
		Font: cffFont,
		Opt:  opt,

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
	}
	return f, nil
}

// Layout appends a string to a glyph sequence, as a sequence of glyphs
// to be typeset at the given point size.
//
// This implements the [font.Layouter] interface.
func (f *Instance) Layout(seq *font.GlyphSeq, ptSize float64, s string) *font.GlyphSeq {
	if seq == nil {
		seq = &font.GlyphSeq{}
	}

	qh := ptSize * f.Font.FontMatrix[0]
	qv := ptSize * f.Font.FontMatrix[3]

	buf := f.layouter.Layout(s)
	seq.Seq = slices.Grow(seq.Seq, len(buf))
	for _, g := range buf {
		xOffset := float64(g.XOffset) * qh
		if len(seq.Seq) == 0 {
			seq.Skip += xOffset
		} else {
			seq.Seq[len(seq.Seq)-1].Advance += xOffset
		}
		seq.Seq = append(seq.Seq, font.Glyph{
			GID:     g.GID,
			Advance: float64(g.Advance) * qh,
			Rise:    float64(g.YOffset) * qv,
			Text:    g.Text,
		})
	}
	return seq
}

// Embed adds the font to a PDF file.
// The function is usually called by [pdf.ResourceManagerEmbed],
// instead of being called directly.
//
// This implements the [font.Font] interface.
func (f *Instance) Embed(rm *pdf.ResourceManager) (pdf.Native, font.Embedded, error) {
	ref := rm.Out.Alloc()

	opt := f.Opt

	var res font.Embedded
	if opt.Composite {
		err := pdf.CheckVersion(rm.Out, "composite CFF fonts", pdf.V1_3)
		if err != nil {
			return nil, nil, err
		}
		res = newEmbeddedComposite(ref, f)
	} else {
		err := pdf.CheckVersion(rm.Out, "simple CFF fonts", pdf.V1_2)
		if err != nil {
			return nil, nil, err
		}
		res = newEmbeddedSimple(ref, f)
	}

	return ref, res, nil
}

func clone[T any](obj *T) *T {
	new := *obj
	return &new
}
