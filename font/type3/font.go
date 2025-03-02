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

package type3

import (
	"errors"

	"seehuhn.de/go/postscript/type1/names"

	"seehuhn.de/go/sfnt/glyph"
	"seehuhn.de/go/sfnt/os2"

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/geom/rect"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/graphics"
)

type Font struct {
	// Glyphs is a list of glyphs in the font.
	// An empty glyph without a name must be included at index 0,
	// to replace the ".notdef" glyph.
	Glyphs []*Glyph

	// FontMatrix transforms glyph space units to text space units.
	FontMatrix matrix.Matrix

	// PostScriptName (optional) is the PostScript name of the font.
	PostScriptName string

	// FontFamily (optional) is the name of the font family.
	FontFamily string

	// FontStretch (optional) is the font stretch value.
	FontStretch os2.Width

	// FontWeight (optional) is the font weight value.
	FontWeight os2.Weight

	IsFixedPitch bool
	IsSerif      bool
	IsScript     bool
	IsAllCap     bool
	IsSmallCap   bool

	ItalicAngle float64

	Ascent    float64 // Type 3 glyph space units
	Descent   float64 // Type 3 glyph space units
	Leading   float64 // Type 3 glyph space units
	CapHeight float64 // Type 3 glyph space units
	XHeight   float64 // Type 3 glyph space units

	UnderlinePosition  float64
	UnderlineThickness float64
}

type Glyph struct {
	Name  string
	Width float64
	BBox  rect.Rect
	Color bool
	Draw  func(*graphics.Writer)
}

var _ interface {
	font.Layouter
} = (*Instance)(nil)

type Instance struct {
	Font *Font
	CMap map[rune]glyph.ID

	*font.Geometry
}

func New(f *Font) (*Instance, error) {
	if len(f.Glyphs) == 0 || f.Glyphs[0].Name != "" {
		return nil, errors.New("invalid glyph 0")
	}

	cmap := make(map[rune]glyph.ID)
	for i, g := range f.Glyphs {
		rr := names.ToUnicode(g.Name, f.PostScriptName == "ZapfDingbats")
		if len(rr) == 1 {
			cmap[rr[0]] = glyph.ID(i)
		}
	}

	qv := f.FontMatrix[3]
	qh := f.FontMatrix[0]
	ee := make([]rect.Rect, len(f.Glyphs))
	ww := make([]float64, len(f.Glyphs))
	for i, g := range f.Glyphs {
		ee[i] = g.BBox
		ww[i] = g.Width * qh
	}
	geom := &font.Geometry{
		Ascent:             f.Ascent * qv,
		Descent:            f.Descent * qv,
		Leading:            f.Leading * qv,
		UnderlinePosition:  f.UnderlinePosition * qv,
		UnderlineThickness: f.UnderlineThickness * qv,
		GlyphExtents:       ee,
		Widths:             ww,
	}

	res := &Instance{
		Font:     f,
		CMap:     cmap,
		Geometry: geom,
	}
	return res, nil
}

func (f *Instance) PostScriptName() string {
	return f.Font.PostScriptName
}

func (f *Instance) Layout(seq *font.GlyphSeq, ptSize float64, s string) *font.GlyphSeq {
	if seq == nil {
		seq = &font.GlyphSeq{}
	}

	q := f.Font.FontMatrix[0] * ptSize

	for _, r := range s {
		gid, ok := f.CMap[r]
		if !ok {
			continue
		}
		seq.Seq = append(seq.Seq, font.Glyph{
			GID:     gid,
			Text:    []rune{r},
			Advance: f.Font.Glyphs[gid].Width * q,
		})
	}
	return seq
}

func (f *Instance) Embed(rm *pdf.ResourceManager) (pdf.Native, font.Embedded, error) {
	if len(f.Font.Glyphs) == 0 || f.Font.Glyphs[0].Name != "" {
		return nil, nil, errors.New("invalid glyph 0")
	}

	ref := rm.Out.Alloc()
	res := newEmbeddedSimple(ref, f.Font)
	return ref, res, nil
}
