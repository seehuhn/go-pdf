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

package opentype

import (
	"math"
	"slices"

	"golang.org/x/text/language"

	"seehuhn.de/go/geom/rect"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/encoding/cidenc"
	"seehuhn.de/go/postscript/funit"
	"seehuhn.de/go/sfnt"
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

// Instance is an OpenType font instance.
type Instance struct {
	*sfnt.Font
	Opt *Options

	*font.Geometry
	layouter *sfnt.Layouter
}

// New makes a PDF font from an OpenType/TrueType font.
//
// The font options control whether the font will be embedded as a simple font
// or as a composite font.
//
// If the font has CFF outlines, it is often more efficient to embed the CFF
// glyph data without the OpenType wrapper. Consider using
// [seehuhn.de/go/pdf/font/cff.New] instead of this function.
//
// If the font has TrueType outlines, it is often more efficient to embed the
// font as a TrueType font instead of an OpenType font.  Consider using
// [seehuhn.de/go/pdf/font/truetype.New] instead of this function.
func New(info *sfnt.Font, opt *Options) (*Instance, error) {
	if opt == nil {
		opt = &Options{}
	}

	layouter, err := info.NewLayouter(opt.Language, opt.GsubFeatures, opt.GposFeatures)
	if err != nil {
		return nil, err
	}

	var geometry *font.Geometry
	if info.IsCFF() {
		geometry = &font.Geometry{
			GlyphExtents: scaleBoxesCFF(info.GlyphBBoxes(), info.FontMatrix[:]),
			Widths:       info.WidthsPDF(),

			Ascent:             float64(info.Ascent) * info.FontMatrix[3],
			Descent:            float64(info.Descent) * info.FontMatrix[3],
			Leading:            float64(info.Ascent-info.Descent+info.LineGap) * info.FontMatrix[3],
			UnderlinePosition:  float64(info.UnderlinePosition) * info.FontMatrix[3],
			UnderlineThickness: float64(info.UnderlineThickness) * info.FontMatrix[3],
		}
	} else { // glyf outlines
		geometry = &font.Geometry{
			GlyphExtents: scaleBoxesGlyf(info.GlyphBBoxes(), info.UnitsPerEm),
			Widths:       info.WidthsPDF(),

			Ascent:             float64(info.Ascent) / float64(info.UnitsPerEm),
			Descent:            float64(info.Descent) / float64(info.UnitsPerEm),
			Leading:            float64(info.Ascent-info.Descent+info.LineGap) / float64(info.UnitsPerEm),
			UnderlinePosition:  float64(info.UnderlinePosition) / float64(info.UnitsPerEm),
			UnderlineThickness: float64(info.UnderlineThickness) / float64(info.UnitsPerEm),
		}
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
			Text:    string(g.Text),
		})
	}
	return seq
}

// Embed adds the font to a PDF file.
//
// This implements the [font.Font] interface.
func (f *Instance) Embed(rm *pdf.ResourceManager) (pdf.Native, font.Embedded, error) {
	w := rm.Out
	ref := w.Alloc()

	err := pdf.CheckVersion(w, "OpenType fonts", pdf.V1_6)
	if err != nil {
		return nil, nil, err
	}

	opt := f.Opt
	if opt == nil {
		opt = &Options{}
	}

	var embedded font.Embedded
	if f.Font.IsCFF() {
		if !opt.Composite {
			embedded = newEmbeddedCFFSimple(ref, f.Font)
		} else {
			embedded = newEmbeddedCFFComposite(ref, f)
		}
	} else {
		if !opt.Composite {
			embedded = newEmbeddedGlyfSimple(ref, f.Font)
		} else {
			embedded = newEmbeddedGlyfComposite(ref, f)
		}
	}

	return ref, embedded, nil
}

func scaleBoxesGlyf(bboxes []funit.Rect16, unitsPerEm uint16) []rect.Rect {
	res := make([]rect.Rect, len(bboxes))
	for i, b := range bboxes {
		res[i] = rect.Rect{
			LLx: float64(b.LLx) / float64(unitsPerEm),
			LLy: float64(b.LLy) / float64(unitsPerEm),
			URx: float64(b.URx) / float64(unitsPerEm),
			URy: float64(b.URy) / float64(unitsPerEm),
		}
	}
	return res
}

func scaleBoxesCFF(bboxes []funit.Rect16, M []float64) []rect.Rect {
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
			x, y := M[0]*xf+M[2]*yf+M[4], M[1]*xf+M[3]*yf+M[5]
			bPDF.LLx = min(bPDF.LLx, x)
			bPDF.LLy = min(bPDF.LLy, y)
			bPDF.URx = max(bPDF.URx, x)
			bPDF.URy = max(bPDF.URy, y)
		}
		res[i] = bPDF
	}
	return res
}

func clone[T any](obj *T) *T {
	new := *obj
	return &new
}
