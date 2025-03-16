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
	"seehuhn.de/go/postscript/type1"
	"seehuhn.de/go/postscript/type1/names"

	"seehuhn.de/go/sfnt/glyph"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
)

type Options struct {
}

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

	IsSerif    bool
	IsScript   bool
	IsAllCap   bool
	IsSmallCap bool

	*font.Geometry

	lig  map[glyph.Pair]glyph.ID
	kern map[glyph.Pair]funit.Int16
	cmap map[rune]glyph.ID
}

// New creates a new Type 1 PDF font from a Type 1 PostScript font.
func New(psFont *type1.Font, metrics *afm.Metrics) (*Instance, error) {
	if !isConsistent(psFont, metrics) {
		return nil, errors.New("inconsistent Type 1 font metrics")
	}

	var glyphNames []string
	if psFont != nil {
		glyphNames = psFont.GlyphList()
	} else {
		glyphNames = metrics.GlyphList()
	}

	geometry := &font.Geometry{}
	widths := make([]float64, len(glyphNames))
	extents := make([]rect.Rect, len(glyphNames))
	for i, name := range glyphNames {
		g := psFont.Glyphs[name]
		widths[i] = g.WidthX * psFont.FontMatrix[0]
		extents[i] = psFont.GlyphBBoxPDF(name)
	}
	geometry.UnderlinePosition = float64(psFont.FontInfo.UnderlinePosition) * psFont.FontMatrix[3]
	geometry.UnderlineThickness = float64(psFont.FontInfo.UnderlineThickness) * psFont.FontMatrix[3]
	geometry.Widths = widths
	geometry.GlyphExtents = extents
	if metrics != nil {
		geometry.Ascent = metrics.Ascent / 1000
		geometry.Descent = metrics.Descent / 1000
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

	return &Instance{
		Font:     psFont,
		Metrics:  metrics,
		Geometry: geometry,
		lig:      lig,
		kern:     kern,
		cmap:     cmap,
	}, nil
}

// IsConsistent checks whether the font metrics are compatible with the
// given font.
func isConsistent(F *type1.Font, M *afm.Metrics) bool {
	if F == nil || M == nil {
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

// PostScriptName returns the PostScript name of the font.
func (f *Instance) PostScriptName() string {
	if f.Metrics != nil {
		return f.Metrics.FontName
	}
	return f.Font.FontInfo.FontName
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
				seq.Seq[len(seq.Seq)-1].Text = append(seq.Seq[len(seq.Seq)-1].Text, r)
				seq.Seq[len(seq.Seq)-1].Advance = f.Widths[repl] * ptSize
				prev = repl
				continue
			}
		}
		seq.Seq = append(seq.Seq, font.Glyph{
			GID:     gid,
			Text:    []rune{r},
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

func (f *Instance) GlyphWidthPDF(glyphName string) float64 {
	if f.Metrics != nil {
		return f.Metrics.GlyphWidthPDF(glyphName)
	} else {
		return f.Font.GlyphWidthPDF(glyphName)
	}
}

// Embed adds the font to a PDF file.
//
// This implements the [font.Font] interface.
func (f *Instance) Embed(rm *pdf.ResourceManager) (pdf.Native, font.Embedded, error) {
	ref := rm.Out.Alloc()
	res := newEmbeddedSimple(ref, f)
	return ref, res, nil
}

func clone[T any](x *T) *T {
	y := *x
	return &y
}
