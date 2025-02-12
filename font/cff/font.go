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
	"slices"

	"seehuhn.de/go/geom/rect"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/postscript/type1"
	"seehuhn.de/go/sfnt"
	"seehuhn.de/go/sfnt/cff"
	"seehuhn.de/go/sfnt/glyph"
	"seehuhn.de/go/sfnt/os2"
)

var _ interface {
	font.Layouter
} = (*Instance)(nil)

// Instance represents a CFF font which can be embedded in a PDF file.
type Instance struct {
	Font *cff.Font
	Opt  *font.Options

	Stretch  os2.Width
	Weight   os2.Weight
	IsSerif  bool
	IsScript bool

	Ascent    float64 // PDF glyph space units
	Descent   float64 // PDF glyph space units
	Leading   float64 // PDF glyph space units
	CapHeight float64 // PDF glyph space units
	XHeight   float64 // PDF glyph space units

	layouter *sfnt.Layouter
}

// New turns a sfnt.Font into a PDF CFF font.
//
// The font can be embedded as a simple font or as a composite font,
// depending on the options used.
//
// The sfnt.Font info must be an OpenType font with CFF outlines.
func New(info *sfnt.Font, opt *font.Options) (*Instance, error) {
	if opt == nil {
		opt = &font.Options{}
	}

	fontInfo := &type1.FontInfo{
		FontName:           info.PostScriptName(),
		Version:            info.Version.String(),
		Notice:             info.Trademark,
		Copyright:          info.Copyright,
		FullName:           info.FullName(),
		FamilyName:         info.FamilyName,
		Weight:             info.Weight.String(),
		ItalicAngle:        info.ItalicAngle,
		IsFixedPitch:       info.IsFixedPitch(),
		UnderlinePosition:  info.UnderlinePosition,
		UnderlineThickness: info.UnderlineThickness,
		FontMatrix:         info.FontMatrix,
	}
	outlines, ok := info.Outlines.(*cff.Outlines)
	if !ok {
		return nil, errors.New("no CFF outlines in font")
	}
	cffFont := &cff.Font{
		FontInfo: fontInfo,
		Outlines: outlines,
	}

	qv := info.FontMatrix[3] * 1000
	ascent := float64(info.Ascent) * qv
	descent := float64(info.Descent) * qv
	leading := float64(info.Ascent-info.Descent+info.LineGap) * qv
	capHeight := float64(info.CapHeight) * qv
	xHeight := float64(info.XHeight) * qv

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

		layouter: layouter,
	}
	return f, nil
}

// PostScriptName returns the PostScript name of the font.
//
// This implements the [font.Font] interface.
func (f *Instance) PostScriptName() string {
	return f.Font.FontName
}

// GetGeometry returns font metrics required for typesetting.
//
// This implements the [font.Layouter] interface.
func (f *Instance) GetGeometry() *font.Geometry {
	glyphExtents := make([]rect.Rect, len(f.Font.Glyphs))
	for gid := range f.Font.Glyphs {
		glyphExtents[gid] = f.Font.GlyphBBoxPDF(f.Font.FontMatrix, glyph.ID(gid))
	}

	qv := f.Font.FontMatrix[3]
	return &font.Geometry{
		Ascent:             f.Ascent / 1000,
		Descent:            f.Descent / 1000,
		Leading:            f.Leading / 1000,
		UnderlinePosition:  float64(f.Font.UnderlinePosition) * qv,
		UnderlineThickness: float64(f.Font.UnderlineThickness) * qv,

		GlyphExtents: glyphExtents,
		Widths:       f.Font.WidthsPDF(),
	}
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
// This is usually called by [pdf.ResourceManagerEmbed], instead of being called directly.
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

		var gidToCID cmap.GIDToCID
		if opt.MakeGIDToCID != nil {
			gidToCID = opt.MakeGIDToCID()
		} else {
			gidToCID = cmap.NewGIDToCIDIdentity()
		}

		var cidEncoder cmap.CIDEncoder
		if opt.MakeEncoder != nil {
			cidEncoder = opt.MakeEncoder(gidToCID)
		} else {
			cidEncoder = cmap.NewCIDEncoderIdentity(gidToCID)
		}

		e := embedded{
			w:    rm.Out,
			ref:  ref,
			Font: f.Font,

			Stretch:  f.Stretch,
			Weight:   f.Weight,
			IsSerif:  f.IsSerif,
			IsScript: f.IsScript,

			Ascent:    f.Ascent,
			Descent:   f.Descent,
			Leading:   f.Leading,
			CapHeight: f.CapHeight,
			XHeight:   f.XHeight,
		}
		res = &embeddedCompositeOld{
			embedded:   e,
			GIDToCID:   gidToCID,
			CIDEncoder: cidEncoder,
		}
	} else {
		err := pdf.CheckVersion(rm.Out, "simple CFF fonts", pdf.V1_2)
		if err != nil {
			return nil, nil, err
		}

		res = newEmbeddedSimple(ref, f)
	}

	return ref, res, nil
}
