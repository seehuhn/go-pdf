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
	"fmt"
	"math"
	"slices"

	"seehuhn.de/go/postscript/afm"
	"seehuhn.de/go/postscript/funit"
	"seehuhn.de/go/postscript/type1"
	"seehuhn.de/go/postscript/type1/names"

	"seehuhn.de/go/sfnt/glyph"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/encoding"
	"seehuhn.de/go/pdf/font/subset"
)

//go:generate go run ./generate/

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

	*font.Geometry

	lig  map[glyph.Pair]glyph.ID
	kern map[glyph.Pair]funit.Int16
	cmap map[rune]glyph.ID

	// Opt controls some aspects of font embedding.
	Opt *font.Options
}

// New creates a new Type 1 PDF font from a Type 1 PostScript font.
func New(psFont *type1.Font, metrics *afm.Metrics, opt *font.Options) (*Instance, error) {
	if !isConsistent(psFont, metrics) {
		return nil, errors.New("inconsistent Type 1 font metrics")
	}

	if opt == nil {
		opt = &font.Options{}
	}
	if opt.Composite {
		return nil, errors.New("Type 1 fonts do not support composite embedding")
	}

	glyphNames := psFont.GlyphList()

	geometry := &font.Geometry{}
	widths := make([]float64, len(glyphNames))
	extents := make([]pdf.Rectangle, len(glyphNames))
	for i, name := range glyphNames {
		g := psFont.Glyphs[name]
		widths[i] = float64(g.WidthX) * psFont.FontMatrix[0]
		extents[i] = glyphBoxtoPDF(g.BBox(), psFont.FontMatrix[:])
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
	isDingbats := psFont.FontName == "ZapfDingbats"
	for gid, name := range glyphNames {
		rr := names.ToUnicode(name, isDingbats)
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
		Opt:      opt,
	}, nil
}

// IsConsistent checks whether the font metrics are compatible with the
// given font.
func isConsistent(F *type1.Font, M *afm.Metrics) bool {
	if M == nil {
		return true
	}
	for name, glyph := range F.Glyphs {
		metrics, ok := M.Glyphs[name]
		if !ok {
			return false
		}
		if math.Abs(glyph.WidthX-metrics.WidthX) > 0.5 {
			return false
		}
	}
	return true
}

// PostScriptName returns the PostScript name of the font.
func (f *Instance) PostScriptName() string {
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
				seq.Seq[len(seq.Seq)-1].Advance += float64(adj) * ptSize / 1000
			}
		}
		prev = g.GID
	}

	return seq
}

// Embed adds the font to a PDF file.
//
// This implements the [font.Font] interface.
func (f *Instance) Embed(rm *pdf.ResourceManager) (pdf.Native, font.Embedded, error) {
	psFont := f.Font
	metrics := f.Metrics

	glyphNames := psFont.GlyphList()

	w := rm.Out
	ref := w.Alloc()
	res := &embeddedSimple{
		ref: ref,
		w:   w,

		psFont:     psFont,
		metrics:    metrics,
		glyphNames: glyphNames,
		widths:     f.Widths,

		SimpleEncoder: encoding.NewSimpleEncoder(),
	}
	return ref, res, nil
}

func glyphBoxtoPDF(b funit.Rect16, fMat []float64) pdf.Rectangle {
	bPDF := pdf.Rectangle{
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
	return bPDF
}

type embeddedSimple struct {
	w   *pdf.Writer
	ref pdf.Reference

	psFont     *type1.Font
	metrics    *afm.Metrics
	glyphNames []string
	widths     []float64

	*encoding.SimpleEncoder

	closed bool
}

func (f *embeddedSimple) DecodeWidth(s pdf.String) (float64, int) {
	if len(s) == 0 {
		return 0, 0
	}
	gid := f.Encoding[s[0]]
	return f.widths[gid], 1
}

func (f *embeddedSimple) CodeAndWidth(s pdf.String, gid glyph.ID, rr []rune) (pdf.String, float64, bool) {
	c := f.GIDToCode(gid, rr)
	return append(s, c), f.widths[gid], c == ' '
}

func (f *embeddedSimple) Close() error {
	if f.closed {
		return nil
	}
	f.closed = true

	if f.SimpleEncoder.Overflow() {
		fontName := f.psFont.FontInfo.FontName
		return fmt.Errorf("too many distinct glyphs used in font %q",
			fontName)
	}

	origFontName := f.psFont.FontName

	encoding := make([]string, 256)
	ww := make([]float64, 256)
	for c, gid := range f.Encoding {
		name := f.glyphNames[gid]
		if name == ".notdef" && f.CodeIsUsed(byte(c)) {
			name = notdefForce
		}
		encoding[c] = name
		ww[c] = f.widths[gid] * 1000
	}

	fontData := f.psFont
	metricsData := f.metrics
	var subsetTag string

	omitFontData := pdf.GetVersion(f.w) < pdf.V2_0 && isStandard(f.psFont.FontInfo.FontName, encoding, ww)
	if !omitFontData { // only subset the font, if the font is embedded
		psFull := f.psFont

		// make sure notdefForce is listed in the Differences array
		e2 := slices.Clone(encoding)
		for i, name := range e2 {
			if name == notdefForce {
				e2[i] = ".notdef"
			}
		}

		psSubset := clone(psFull)
		psSubset.Glyphs = make(map[string]*type1.Glyph)
		if _, ok := psFull.Glyphs[".notdef"]; ok {
			psSubset.Glyphs[".notdef"] = psFull.Glyphs[".notdef"]
		}
		for _, name := range encoding {
			if _, ok := psFull.Glyphs[name]; ok {
				psSubset.Glyphs[name] = psFull.Glyphs[name]
			}
		}
		psSubset.Encoding = e2
		fontData = psSubset

		if metricsData != nil {
			metricsSubset := clone(metricsData)
			metricsSubset.Glyphs = make(map[string]*afm.GlyphInfo)

			if _, ok := metricsData.Glyphs[".notdef"]; ok {
				metricsSubset.Glyphs[".notdef"] = metricsData.Glyphs[".notdef"]
			}
			for _, name := range encoding {
				if _, ok := metricsData.Glyphs[name]; ok {
					metricsSubset.Glyphs[name] = metricsData.Glyphs[name]
				}
			}
			metricsSubset.Encoding = e2
			metricsData = metricsSubset
		}

		var ss []glyph.ID
		for origGid, name := range f.glyphNames {
			if _, ok := psSubset.Glyphs[name]; ok {
				ss = append(ss, glyph.ID(origGid))
			}
		}
		subsetTag = subset.Tag(ss, psFull.NumGlyphs())
	}

	var toUnicode *cmap.ToUnicode
	toUniMap := f.ToUnicodeNew()
	for c, name := range encoding {
		got := names.ToUnicode(name, origFontName == "ZapfDingbats")
		want := toUniMap[string(rune(c))]
		if !slices.Equal(got, want) {
			toUnicode = cmap.NewToUnicodeNew(charcode.Simple, toUniMap)
			break
		}
	}

	info := &FontDict{
		Font:      fontData,
		Metrics:   metricsData,
		SubsetTag: subsetTag,
		Encoding:  encoding,
		ToUnicode: toUnicode,
	}
	return info.Embed(f.w, f.ref)
}

// IsStandard returns true if the font is one of the standard 14 PDF fonts.
// This is determined by the font name, the set of glyphs used, and the glyph
// widths.
//
// ww must be the widths of the 256 encoded characters, given in PDF text space
// units times 1000.
func isStandard(fontName string, enc []string, ww []float64) bool {
	m, ok := builtinMetrics[fontName]
	if !ok {
		return false
	}

	for i, glyphName := range enc {
		if glyphName == ".notdef" || glyphName == notdefForce {
			continue
		}
		w, ok := m.Widths[glyphName]
		if !ok {
			return false
		}
		if math.Abs(ww[i]-w) > 0.5 {
			return false
		}
	}
	return true
}

func clone[T any](x *T) *T {
	y := *x
	return &y
}

// NotdefForce is a glyph name which is unlikely to be used by any real-world
// font. We map code points to this glyph name, when the user requests to
// typeset the .notdef glyph.
const notdefForce = ".notdef.force"
