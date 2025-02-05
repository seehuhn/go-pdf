// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2024  Jochen Voss <voss@seehuhn.de>
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

// Package cff implements CFF font data embedded into PDF files.
//
// CFF fonts can be embedded into a PDF file either as "simple fonts" or as
// "composite fonts".
package cff

import (
	"errors"
	"math"
	"slices"

	"seehuhn.de/go/geom/rect"
	"seehuhn.de/go/postscript/funit"

	"seehuhn.de/go/sfnt"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/encoding"
)

var _ font.Font = (*Instance)(nil)

// Instance represents a CFF font.
type Instance struct {
	*sfnt.Font
	*font.Geometry

	layouter *sfnt.Layouter

	Opt *font.Options
}

// New turns a sfnt.Font into a PDF CFF font.
//
// The font can be embedded as a simple font or as a composite font,
// depending on the options used.
//
// The sfnt.Font info must be an OpenType font with CFF outlines.
func New(info *sfnt.Font, opt *font.Options) (*Instance, error) {
	if !info.IsCFF() {
		return nil, errors.New("no CFF outlines in font")
	}

	if opt == nil {
		opt = &font.Options{}
	}

	layouter, err := info.NewLayouter(opt.Language, opt.GsubFeatures, opt.GposFeatures)
	if err != nil {
		return nil, err
	}

	geometry := &font.Geometry{
		Ascent:             float64(info.Ascent) * info.FontMatrix[3],
		Descent:            float64(info.Descent) * info.FontMatrix[3],
		BaseLineDistance:   float64(info.Ascent-info.Descent+info.LineGap) * info.FontMatrix[3],
		UnderlinePosition:  float64(info.UnderlinePosition) * info.FontMatrix[3],
		UnderlineThickness: float64(info.UnderlineThickness) * info.FontMatrix[3],

		GlyphExtents: scaleBoxesCFF(info.GlyphBBoxes(), info.FontMatrix[:]),
		Widths:       info.WidthsPDF(),
	}

	F := &Instance{
		Font:     info,
		Geometry: geometry,
		layouter: layouter,
		Opt:      opt,
	}

	return F, nil
}

// Layout implements the [font.Layouter] interface.
func (f *Instance) Layout(seq *font.GlyphSeq, ptSize float64, s string) *font.GlyphSeq {
	if seq == nil {
		seq = &font.GlyphSeq{}
	}

	buf := f.layouter.Layout(s)
	seq.Seq = slices.Grow(seq.Seq, len(buf))
	for _, g := range buf {
		xOffset := float64(g.XOffset) * ptSize * f.Font.FontMatrix[0]
		if len(seq.Seq) == 0 {
			seq.Skip += xOffset
		} else {
			seq.Seq[len(seq.Seq)-1].Advance += xOffset
		}
		seq.Seq = append(seq.Seq, font.Glyph{
			GID:     g.GID,
			Advance: float64(g.Advance) * ptSize * f.Font.FontMatrix[0],
			Rise:    float64(g.YOffset) * ptSize * f.Font.FontMatrix[3],
			Text:    g.Text,
		})
	}
	return seq
}

// Embed adds the font to a PDF file.
// This implements the [font.Font] interface.
func (f *Instance) Embed(rm *pdf.ResourceManager) (pdf.Native, font.Embedded, error) {
	opt := f.Opt

	info := f.Font

	w := rm.Out
	ref := w.Alloc()

	e := embedded{
		w:   w,
		ref: ref,

		sfnt: info,
	}

	var res font.Embedded
	if opt.Composite {
		err := pdf.CheckVersion(w, "composite CFF fonts", pdf.V1_3)
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

		res = &embeddedComposite{
			embedded:   e,
			GIDToCID:   gidToCID,
			CIDEncoder: cidEncoder,
		}
	} else {
		err := pdf.CheckVersion(w, "simple CFF fonts", pdf.V1_2)
		if err != nil {
			return nil, nil, err
		}
		res = &embeddedSimple{
			embedded:      e,
			SimpleEncoder: encoding.NewSimpleEncoder(),
		}
	}

	return ref, res, nil
}

func scaleBoxesCFF(bboxes []funit.Rect16, fMat []float64) []rect.Rect {
	res := make([]rect.Rect, len(bboxes))
	for i, b := range bboxes {
		bPDF := rect.Rect{
			LLx: math.Inf(+1),
			LLy: math.Inf(+1),
			URx: math.Inf(-1),
			URy: math.Inf(-1),
		}
		corners := []struct{ x, y funit.Int16 }{
			{b.LLx, b.LLy},
			{b.LLx, b.URy},
			{b.URx, b.LLy},
			{b.URx, b.URy},
		}
		for _, c := range corners {
			xf := float64(c.x)
			yf := float64(c.y)
			x, y := fMat[0]*xf+fMat[2]*yf+fMat[4], fMat[1]*xf+fMat[3]*yf+fMat[5]
			bPDF.LLx = min(bPDF.LLx, x)
			bPDF.LLy = min(bPDF.LLy, y)
			bPDF.URx = max(bPDF.URx, x)
			bPDF.URy = max(bPDF.URy, y)
		}
		res[i] = bPDF
	}
	return res
}

type embedded struct {
	w   *pdf.Writer
	ref pdf.Reference

	sfnt *sfnt.Font

	closed bool
}
