// Copyright 2023 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package type1

import (
	"errors"
	"fmt"
	"math"
	"slices"

	"seehuhn.de/go/geom/path"
	"seehuhn.de/go/geom/rect"
	"seehuhn.de/go/geom/vec"
	"seehuhn.de/go/postscript/afm"
	"seehuhn.de/go/postscript/psenc"
	"seehuhn.de/go/postscript/type1"
	"seehuhn.de/go/postscript/type1/names"

	"seehuhn.de/go/sfnt/glyph"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/dict"
	"seehuhn.de/go/pdf/font/encoding"
	"seehuhn.de/go/pdf/font/encoding/simpleenc"
	"seehuhn.de/go/pdf/font/glyphdata/type1glyphs"
	"seehuhn.de/go/pdf/font/internal/fontdesc"
	"seehuhn.de/go/pdf/font/internal/fontgeom"
	"seehuhn.de/go/pdf/font/pdfenc"
	"seehuhn.de/go/pdf/font/subset"
	"seehuhn.de/go/pdf/internal/fontname"
)

// Instance is a Type 1 font instance which can be embedded into a PDF file.
//
// Use [New] to create new font instances.
type Instance struct {
	// psFont is the font program to embed (nil for a metrics-only instance),
	// and metrics the matching AFM metrics, which add kerning and ligatures.
	// The metrics are optional if psFont is present.  Everything below is
	// derived from the two, so neither may change once the instance exists.
	psFont  *type1.Font
	metrics *afm.Metrics

	// GlyphNames establishes the assignment between GIDs and glyph
	// names.  The slice starts with ".notdef".
	GlyphNames []string

	// Descriptor describes the font as designed, in PDF glyph space units.
	// It is filled in from the font program and metrics and may be adjusted
	// before the font is embedded; entries which depend on the document
	// instead of the font (FontName, IsSymbolic and MissingWidth) are
	// ignored and filled in at embedding time.
	Descriptor *font.Descriptor

	*font.Geometry

	lig  map[glyph.Pair]glyph.ID
	kern map[glyph.Pair]float64
	cmap map[rune]glyph.ID

	*simpleenc.Simple

	// builtin is the built-in encoding of a metrics-only instance whose
	// metrics give an encoding other than StandardEncoding, and nil otherwise.
	// Such a font is symbolic, and since it is not embedded, the PDF file can
	// only refer to its glyphs through this encoding (PDF 2.0, Table 112).
	builtin *pdfenc.Encoding

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

// New creates a new Type 1 PDF font from a Type 1 PostScript font and/or
// AFM metrics.  At least one of the arguments must be present.
//
// If psFont is nil, the instance is built from the metrics alone.  Such an
// instance is never written with font data, so the PDF file relies on the
// viewer to supply the glyphs, and every glyph has an empty outline.
// Otherwise the font program is embedded (as a subset) and metrics is
// optional.
func New(psFont *type1.Font, metrics *afm.Metrics) (*Instance, error) {
	if psFont == nil {
		if metrics == nil {
			return nil, errors.New("missing font program and metrics")
		}
		return newMetricsOnly(metrics), nil
	}
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
	var fontBBox rect.Rect
	for i, name := range glyphNames {
		// prefer the metrics for widths, to match GlyphWidthPDF
		if metrics != nil {
			widths[i] = metrics.GlyphWidthPDF(name) / 1000
		} else {
			widths[i] = psFont.GlyphWidthPDF(name) / 1000
		}
		// GlyphBBoxPDF returns 1000-scale glyph space; convert to text space
		b := psFont.GlyphBBoxPDF(name)
		if !b.IsZero() {
			if fontBBox.IsZero() {
				fontBBox = b
			} else {
				fontBBox.Extend(b)
			}
		}
		extents[i] = rect.Rect{
			LLx: b.LLx / 1000,
			LLy: b.LLy / 1000,
			URx: b.URx / 1000,
			URy: b.URy / 1000,
		}
	}
	geometry.UnderlinePosition = pdf.Round(float64(psFont.FontInfo.UnderlinePosition)*psFont.FontMatrix[3], 6)
	geometry.UnderlineThickness = pdf.Round(float64(psFont.FontInfo.UnderlineThickness)*psFont.FontMatrix[3], 6)
	geometry.Widths = widths
	geometry.GlyphExtents = extents
	// The geometry keeps the heights unrounded, where the descriptor rounds
	// them to whole glyph space units.  Nothing records a leading, and heights
	// the glyphs cannot supply are estimated.
	descriptor := fontdesc.FromType1(psFont, metrics, fontBBox)
	ascent, descent, capHeight, xHeight := fontdesc.Type1Heights(psFont, metrics, fontBBox)
	geometry.Ascent = ascent / 1000
	geometry.Descent = descent / 1000
	geometry.CapHeight = capHeight / 1000
	geometry.XHeight = xHeight / 1000
	fontgeom.FillHeights(geometry)

	lig, kern := ligaturesAndKerns(metrics, glyphNames)

	// The glyph list a name is looked up in is chosen by the name of the font,
	// which a program taken out of a PDF file gives with a subset tag in front.
	// The tag names the subset rather than the font, so it is dropped here.
	_, psName := subset.Split(psFont.FontName)

	cmap := buildCmap(glyphNames, psName)

	return &Instance{
		psFont:     psFont,
		metrics:    metrics,
		GlyphNames: glyphNames,
		Descriptor: descriptor,
		Geometry:   geometry,
		lig:        lig,
		kern:       kern,
		cmap:       cmap,
		Simple:     newEncoder(fontname.ForType1(psFont), widths[0], nil),
	}, nil
}

// buildCmap maps each rune to the first glyph whose name stands for exactly
// that rune.  psName is the font name, which selects the glyph list used to
// look up the names.
func buildCmap(glyphNames []string, psName string) map[rune]glyph.ID {
	cmap := make(map[rune]glyph.ID)
	for gid, name := range glyphNames {
		rr := []rune(names.ToUnicode(name, psName))
		if len(rr) != 1 {
			continue
		}
		if _, exists := cmap[rr[0]]; !exists {
			cmap[rr[0]] = glyph.ID(gid)
		}
	}
	return cmap
}

// ligaturesAndKerns returns the ligature and kerning tables of the metrics,
// indexed by position in glyphNames (metrics may be nil).
//
// The metrics may name glyphs the font program does not have: the two come
// from separate files and need not agree, and a caller may have restricted
// the glyph set.  Such entries are skipped, since an unknown name would
// otherwise resolve to glyph ID 0 and quietly attach the ligature or the
// kern to ".notdef".
func ligaturesAndKerns(metrics *afm.Metrics, glyphNames []string) (map[glyph.Pair]glyph.ID, map[glyph.Pair]float64) {
	nameGid := make(map[string]glyph.ID, len(glyphNames))
	for i, name := range glyphNames {
		nameGid[name] = glyph.ID(i)
	}

	lig := make(map[glyph.Pair]glyph.ID)
	kern := make(map[glyph.Pair]float64)
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
	return lig, kern
}

// newMetricsOnly creates an instance which has no font program.
func newMetricsOnly(metrics *afm.Metrics) *Instance {
	// glyph ID 0 must be ".notdef", where unmapped characters end up
	glyphNames := metrics.GlyphList()
	if len(glyphNames) == 0 || glyphNames[0] != ".notdef" {
		glyphNames = slices.Insert(glyphNames, 0, ".notdef")
	}

	geometry := &font.Geometry{}
	widths := make([]float64, len(glyphNames))
	extents := make([]rect.Rect, len(glyphNames))
	for i, name := range glyphNames {
		widths[i] = metrics.GlyphWidthPDF(name) / 1000
		if gi := metrics.Glyphs[name]; gi != nil {
			b := gi.BBox
			extents[i] = rect.Rect{
				LLx: b.LLx / 1000,
				LLy: b.LLy / 1000,
				URx: b.URx / 1000,
				URy: b.URy / 1000,
			}
		}
	}
	geometry.UnderlinePosition = pdf.Round(metrics.UnderlinePosition/1000, 6)
	geometry.UnderlineThickness = pdf.Round(metrics.UnderlineThickness/1000, 6)
	geometry.Widths = widths
	geometry.GlyphExtents = extents
	ascent, descent, capHeight, xHeight := fontdesc.AFMHeights(metrics)
	geometry.Ascent = ascent / 1000
	geometry.Descent = descent / 1000
	geometry.CapHeight = capHeight / 1000
	geometry.XHeight = xHeight / 1000
	fontgeom.FillHeights(geometry)

	lig, kern := ligaturesAndKerns(metrics, glyphNames)

	_, psName := subset.Split(metrics.FontName)
	cmap := buildCmap(glyphNames, psName)

	builtin := fontSpecificEncoding(metrics)

	return &Instance{
		metrics:    metrics,
		GlyphNames: glyphNames,
		Descriptor: fontdesc.FromAFM(metrics),
		Geometry:   geometry,
		lig:        lig,
		kern:       kern,
		cmap:       cmap,
		Simple:     newEncoder(fontname.ForAFM(metrics), widths[0], builtin),
		builtin:    builtin,
	}
}

// fontSpecificEncoding returns the encoding given in the metrics, or nil if
// this is StandardEncoding.
func fontSpecificEncoding(metrics *afm.Metrics) *pdfenc.Encoding {
	res := &pdfenc.Encoding{Has: make(map[string]bool)}
	isStandard := true
	for code := range 256 {
		name := ".notdef"
		if code < len(metrics.Encoding) && metrics.Encoding[code] != "" {
			name = metrics.Encoding[code]
		}
		if name != psenc.StandardEncoding[code] {
			isStandard = false
		}
		res.Encoding[code] = name
		if name != ".notdef" {
			res.Has[name] = true
		}
	}
	if isStandard {
		return nil
	}
	return res
}

// newEncoder returns the encoding state of a fresh instance.  Type 1 fonts are
// always simple fonts.  fontName is the name the font program gives itself, and
// notdefWidth is the width of the ".notdef" glyph in text space units.  Codes
// are preferably allocated where base has the glyph, and base defaults to
// WinAnsiEncoding if nil.
func newEncoder(fontName string, notdefWidth float64, base *pdfenc.Encoding) *simpleenc.Simple {
	if base == nil {
		base = &pdfenc.WinAnsi
	}
	return simpleenc.NewSimple(
		math.Round(notdefWidth*1000),
		fontName,
		base,
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
// build it once and clone it for each.  The descriptor is copied, so that each
// clone can be adjusted on its own.
func (f *Instance) Clone() *Instance {
	other := *f
	descriptor := *f.Descriptor
	other.Descriptor = &descriptor
	other.Simple = newEncoder(f.PostScriptName(), f.Widths[0], f.builtin)
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
	if f.psFont == nil {
		return fontname.ForAFM(f.metrics)
	}
	return fontname.ForType1(f.psFont)
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

	// allocate a new code
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
				seq.Seq[i-1].Advance += adj * ptSize / 1000
			}
		}
		prev = g.GID
	}

	return seq
}

// GlyphWidthPDF returns the width of the given glyph in PDF glyph space units.
func (f *Instance) GlyphWidthPDF(glyphName string) float64 {
	if f.metrics != nil {
		return f.metrics.GlyphWidthPDF(glyphName)
	}
	return f.psFont.GlyphWidthPDF(glyphName)
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
		return nil, fmt.Errorf("font %q: %w", f.PostScriptName(), err)
	}

	fontData := f.psFont
	if fontData == nil {
		return f.makeMetricsOnlyDict(), nil
	}

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
	}

	// the descriptor describes the design; only these entries depend on how
	// the document uses the font
	fd := *f.Descriptor
	fd.FontName = subset.Join(subsetTag, postScriptName)
	fd.IsSymbolic = f.Simple.IsSymbolic()
	fd.MissingWidth = f.Simple.DefaultWidth()

	dict := &dict.Type1{
		PostScriptName: postScriptName,
		SubsetTag:      subsetTag,
		Descriptor:     &fd,
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

// makeMetricsOnlyDict returns the font dictionary of an instance without
// font program.  It carries no font data and the name has no subset tag.
func (f *Instance) makeMetricsOnlyDict() *dict.Type1 {
	postScriptName := f.PostScriptName()

	fd := *f.Descriptor
	fd.FontName = postScriptName
	fd.IsSymbolic = f.builtin != nil || f.Simple.IsSymbolic()
	fd.MissingWidth = f.Simple.DefaultWidth()

	enc := f.Simple.Encoding()
	if f.builtin != nil {
		// codes which agree with the built-in encoding need no /Encoding entry
		allocated := enc
		enc = func(c byte) string {
			name := allocated(c)
			if name != "" && name == f.builtin.Encoding[c] {
				return encoding.UseBuiltin
			}
			return name
		}
	}

	dict := &dict.Type1{
		PostScriptName: postScriptName,
		Descriptor:     &fd,
		Encoding:       enc,
		ToUnicode:      f.Simple.ToUnicode(),
		Name:           f.Name,
	}
	for c, info := range f.Simple.MappedCodes() {
		dict.Width[c] = info.Width
	}
	return dict
}

func clone[T any](x *T) *T {
	y := *x
	return &y
}

var _ font.Outliner = (*Instance)(nil)

// Outline returns the outline of a glyph in text space.
// See [font.Outliner.Outline].
// A metrics-only instance has no outlines, so the result is empty.
func (f *Instance) Outline(gid glyph.ID) path.Path {
	if f.psFont == nil {
		return func(yield func(path.Command, []vec.Vec2) bool) {}
	}
	g := f.psFont.Glyphs[f.GlyphNames[gid]]
	return g.Path().Transform(f.psFont.FontMatrix)
}
