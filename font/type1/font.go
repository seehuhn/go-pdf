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
	"seehuhn.de/go/pdf/internal/fontname"
)

// Instance is a Type 1 font instance which can be embedded into a PDF file.
//
// Use [New] to create new font instances.
type Instance struct {
	// Font is the font data to embed.  This must not be nil.
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

	// Name is the PDF resource-dictionary key under which this font is
	// referenced in content streams.  If non-empty, the builder uses this
	// value as the /Font subdictionary key; the spec requires the two to
	// match (PDF 2.0 Table 109).  Required in PDF 1.0; optional in PDF
	// 1.1–1.7; deprecated (forbidden by this library's writer) in PDF 2.0.
	Name pdf.Name
}

var _ font.Layouter = (*Instance)(nil)

// ResourceName returns the preferred resource-dictionary key for this font.
// See [font.Instance.ResourceName].
func (f *Instance) ResourceName() pdf.Name {
	return f.Name
}

// New creates a new Type 1 PDF font from a Type 1 PostScript font.
// The argument psFont must be present, metrics is optional.
func New(psFont *type1.Font, metrics *afm.Metrics) (*Instance, error) {
	if psFont.MM != nil {
		return nil, errors.New("instantiate multiple master font before embedding")
	}
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

	// The metrics may name glyphs the font program does not have: the two come
	// from separate files and need not agree, and a caller may have restricted
	// the glyph set.  Such entries are skipped, since an unknown name would
	// otherwise resolve to glyph ID 0 and quietly attach the ligature or the
	// kern to ".notdef".
	lig := make(map[glyph.Pair]glyph.ID)
	kern := make(map[glyph.Pair]funit.Int16)
	if metrics != nil {
		for left, name := range glyphNames {
			gi := metrics.Glyphs[name]
			if gi == nil {
				continue
			}
			for right, repl := range gi.Ligatures {
				rightGid, rightOK := nameGid[right]
				replGid, replOK := nameGid[repl]
				if !rightOK || !replOK {
					continue
				}
				lig[glyph.Pair{Left: glyph.ID(left), Right: rightGid}] = replGid
			}
		}
		for _, k := range metrics.Kern {
			left, leftOK := nameGid[k.Left]
			right, rightOK := nameGid[k.Right]
			if !leftOK || !rightOK {
				continue
			}
			kern[glyph.Pair{Left: left, Right: right}] = k.Adjust
		}
	}

	// The glyph list a name is looked up in is chosen by the name of the font,
	// which a program taken out of a PDF file gives with a subset tag in front.
	// The tag names the subset rather than the font, so it is dropped here.
	_, psName := subset.Split(psFont.FontName)

	cmap := make(map[rune]glyph.ID)
	for gid, name := range glyphNames {
		rr := []rune(names.ToUnicode(name, psName))
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
		Font:       psFont,
		Metrics:    metrics,
		GlyphNames: glyphNames,
		Geometry:   geometry,
		lig:        lig,
		kern:       kern,
		cmap:       cmap,
		Simple:     newEncoder(fontname.ForType1(psFont), widths[0]),
	}, nil
}

// newEncoder returns the encoding state of a fresh instance.  Type 1 fonts are
// always simple fonts.  fontName is the name the font program gives itself, and
// notdefWidth is the width of the ".notdef" glyph in text space units.
func newEncoder(fontName string, notdefWidth float64) *simpleenc.Simple {
	return simpleenc.NewSimple(
		math.Round(notdefWidth*1000),
		fontName,
		&pdfenc.WinAnsi,
	)
}

// Clone returns an instance which draws on the same font data but has an
// encoding state of its own, for use in a different document.
//
// An instance allocates character codes as text is laid out, which is what ties
// it to a single document.  Everything those codes are allocated from — the
// font program, the metrics, and the widths, extents and glyph tables derived
// from them — is read-only and is shared with the clone rather than built
// again.  A caller which needs the same font in several documents can therefore
// build it once and clone it for each, provided it treats the shared data as
// read-only: a change to [Instance.Font] or [Instance.Metrics] reaches every
// clone.
func (f *Instance) Clone() *Instance {
	other := *f
	other.Simple = newEncoder(f.PostScriptName(), f.Widths[0])
	return &other
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

// PostScriptName returns the name by which the PDF file refers to this font.
// This is the name of the font program, which is what the font dictionary
// describes, repaired where the program names itself in a way a PostScript name
// cannot be written.  The metrics are not consulted: they come from a separate
// file and may name the font differently.
func (f *Instance) PostScriptName() string {
	return fontname.ForType1(f.Font)
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
		return nil, pdf.Errorf("font %q: %w", f.PostScriptName(), err)
	}

	fontData := f.Font
	metricsData := f.Metrics

	// The dictionary describes the program it carries, so both the name and the
	// glyph count come from the font data: the glyph IDs a subset is made of
	// index the program's glyph list, and it is the program which is renamed to
	// match the dictionary.
	numGlyphs := fontData.NumGlyphs()
	srcTag, postScriptName := subset.Split(f.PostScriptName())

	// A program which arrived as a subset was embedded on purpose, so it is
	// kept even where one of the standard fonts has the same metrics: the tag
	// says the file carried glyph outlines of its own, which need not look
	// like the standard font they are metrically compatible with.
	omitFontData := srcTag == "" && isStandard(postScriptName, f.Simple)

	glyphs := f.Simple.Glyphs()
	subsetTag := subset.Retag(subset.Tag(glyphs, numGlyphs), srcTag)
	if omitFontData {
		// only subset the font, if the font is embedded
		subsetTag = ""
	}

	// The program names itself the same as the /BaseFont and /FontName entries,
	// tag and all, and by the repaired name where the one it arrived with
	// cannot be written.  Both it and TagFontInfo copy, so the font data this
	// instance shares with its clones is not renamed along with it.
	fontSubset := clone(fontData)
	fontSubset.FontInfo = subset.TagFontInfo(fontData.FontInfo, subsetTag, postScriptName)

	metricsSubset := metricsData
	if subsetTag != "" {
		fontSubset.Outlines = clone(fontData.Outlines)
		fontSubset.Glyphs = make(map[string]*type1.Glyph)
		for _, gid := range glyphs {
			glyphName := f.GlyphNames[gid]
			if g, ok := fontData.Glyphs[glyphName]; ok {
				fontSubset.Glyphs[glyphName] = g
			}
		}
		fontSubset.Encoding = psenc.StandardEncoding[:]

		if metricsData != nil {
			metricsSubset = clone(metricsData)
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
		FontName:     subset.Join(subsetTag, postScriptName),
		FontFamily:   fontSubset.FamilyName,
		FontWeight:   os2.WeightFromString(fontSubset.Weight),
		IsFixedPitch: fontSubset.IsFixedPitch,
		IsSerif:      f.IsSerif,
		IsSymbolic:   f.Simple.IsSymbolic(),
		IsItalic:     fontSubset.ItalicAngle != 0,
		ForceBold:    fontSubset.Private.ForceBold,
		FontBBox:     fontSubset.FontBBoxPDF().Rounded(),
		ItalicAngle:  fontSubset.ItalicAngle,
		StemV:        fontSubset.Private.StdVW,
		StemH:        fontSubset.Private.StdHW,
	}

	// the metrics describe the same font more precisely, where they are known
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
		ToUnicode:      f.Simple.ToUnicode(),
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
