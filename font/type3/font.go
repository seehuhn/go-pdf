// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package type3

import (
	"errors"
	"fmt"
	"iter"
	"math"

	"seehuhn.de/go/postscript/type1/names"

	"seehuhn.de/go/sfnt/glyph"
	"seehuhn.de/go/sfnt/os2"

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/geom/rect"
	pstype1 "seehuhn.de/go/postscript/type1"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/dict"
	"seehuhn.de/go/pdf/font/encoding/simpleenc"
	"seehuhn.de/go/pdf/font/internal/fontgeom"
	"seehuhn.de/go/pdf/font/pdfenc"
	"seehuhn.de/go/pdf/graphics/content"
)

// Font represents a Type 3 font with user-defined glyph procedures.
type Font struct {
	// Glyphs is a list of glyphs in the font.
	// An empty glyph without a name must be included at index 0,
	// to replace the ".notdef" glyph.  The other glyphs must have distinct
	// names which contain no NUL byte.  Any other name is allowed, including
	// the empty name and ".notdef".
	Glyphs []*Glyph

	// Resources (optional; PDF 1.2) holds named resources shared by all glyph
	// content streams that don't have their own resource dictionary. This is
	// embedded in the Type 3 font dictionary.
	Resources *content.Resources

	// FontMatrix transforms glyph space units to text space units.
	FontMatrix matrix.Matrix

	// PostScriptName (optional) is the PostScript name of the font.
	PostScriptName string

	// Name is the name used to reference the font from within content
	// streams in PDF 1.0.  From PDF 1.1 onward it is optional and exists
	// only as the font's human-readable name (Type 3 has no BaseFont);
	// see https://github.com/pdf-association/pdf-issues/issues/11#issuecomment-753665847
	Name pdf.Name

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

// Glyph represents a single glyph in a Type 3 font.
type Glyph struct {
	// Name is the name of the glyph, which keys its procedure in the
	// font's CharProcs dictionary.  See [Font.Glyphs] for the names allowed.
	Name string

	// Content is the glyph's content stream.
	// It must start with either the d0 or d1 operator.  Only glyph 0 may
	// have a nil or empty content stream.
	// Use [content/builder.Builder] to construct content streams; its
	// [content/builder.Builder.Build] / [content/builder.Builder.Harvest]
	// methods return a [*content.Operators] that satisfies [content.Stream].
	Content content.Stream

	// Resources (optional; PDF 2.0) holds named resources used by this glyph's
	// content stream. If set, the resources are embedded in the glyph's stream
	// dictionary. If nil, resources are looked up from the font's resource
	// dictionary or inherited from the page.
	Resources *content.Resources
}

// extract iterates the glyph's content stream once and returns its
// advance width, the rectangle its ink lies in, and any validation error.
// res is the resource dictionary the content stream uses; nil stands for
// resources inherited from the page.  An empty content stream is an error
// unless isNotdef is set.  For glyphs loaded from PDF files,
// Content may be backed by a file stream; this method ensures the stream is
// read at most once per glyph.
//
// The rectangle of a d1 glyph is the box its d1 operator gives, which
// confines the glyph.  For other glyphs, including d1 glyphs whose box has
// no interior, it contains all ink the content stream can paint, whatever
// the graphics state it is invoked with; it is infinite where the ink has no
// bound, and zero where there is none.
func (g *Glyph) extract(res *content.Resources, isNotdef bool) (width float64, bbox rect.Rect, err error) {
	m, ink, err := analyseGlyph(g.Content, res, true)
	if err != nil {
		return 0, rect.Rect{}, err
	}
	switch {
	case ink == nil && !m.HasBBox():
		return 0, rect.Rect{}, errEmpty(isNotdef)
	case m.HasBBox():
		return m.Width, m.BBox, nil
	}
	return m.Width, ink.result(), nil
}

// analyseGlyph scans the glyph procedure proc.  It returns the metrics of
// the leading d0 or d1 operator and the ink bounds of the operators from the
// metrics operator onwards.  The ink bounds are nil for a nil or empty
// procedure.  If stopAtBox is set, scanning ends at a d1 operator whose box
// has an interior, and the ink bounds are nil.  An error is returned for a
// procedure which does not start with a valid d0 or d1 operator and for
// iteration errors.
func analyseGlyph(proc content.Stream, res *content.Resources, stopAtBox bool) (m content.GlyphMetrics, ink *inkBounds, err error) {
	if proc == nil {
		return m, nil, nil
	}
	it := proc.NewIter()
	for name, args := range it.All() {
		if ink != nil {
			ink.apply(name, args)
			if ink.done() {
				break
			}
			continue
		}
		if name == content.OpRawContent {
			continue
		}
		var ok bool
		m, ok = content.ParseGlyphMetrics(name, args)
		if !ok {
			return m, nil, errors.New("glyph must start with a valid d0 or d1 operator")
		}
		if stopAtBox && m.HasBBox() {
			// the remaining content has no metric significance
			return m, nil, nil
		}
		ink = newInkBounds(res)
		ink.apply(name, args)
	}
	if err := it.Err(); err != nil {
		return m, nil, err
	}
	return m, ink, nil
}

// maxReach is the distance from the origin, in text space units, beyond
// which a glyph extent is treated as unbounded.  Ornaments and swashes
// reach a few em, while a box far larger than this would only spoil the
// layout of the text around it.
const maxReach = 100

// withinReach reports whether r lies within [maxReach] of the origin.  This
// fails for a box which is not finite, as huge box coordinates or font
// matrix entries can give.
func withinReach(r rect.Rect) bool {
	for _, x := range []float64{r.LLx, r.LLy, r.URx, r.URy} {
		if !(math.Abs(x) <= maxReach) {
			return false
		}
	}
	return true
}

// errEmpty returns the error for a glyph without any operators.
// Only glyph 0 may be empty.
func errEmpty(isNotdef bool) error {
	if isNotdef {
		return nil
	}
	return errors.New("empty glyph procedure")
}

// Width returns the glyph's advance width in glyph coordinate units.
// The width is extracted from the first operator (d0 or d1).
func (g *Glyph) Width() float64 {
	m, _ := content.ReadGlyphMetrics(g.Content)
	return m.Width
}

// getNumber extracts a float64 from a pdf.Object.
func getNumber(obj pdf.Object) (float64, bool) {
	switch v := obj.(type) {
	case pdf.Integer:
		return float64(v), true
	case pdf.Real:
		return float64(v), true
	case pdf.Number:
		return float64(v), true
	}
	return 0, false
}

var _ font.Layouter = (*instance)(nil)

// instance represents a Type 3 font instance ready for embedding.
type instance struct {
	// Font is the underlying Type 3 font definition.
	Font *Font

	// CMap maps Unicode code points to glyph IDs.
	CMap map[rune]glyph.ID

	// rawWidths caches each glyph's advance width in glyph-space units.
	// Computed once in [Font.New] so layout, encoding, and embedding do
	// not have to re-iterate the glyph content stream.
	rawWidths []float64

	// charProcs holds the glyph procedure of every named glyph, indexed by
	// glyph name.
	charProcs map[pdf.Name]*dict.CharProc

	*font.Geometry

	*simpleenc.Simple
}

// New creates a new Type 3 font instance from a font definition.
//
// The glyph extents in the instance's geometry are the bounding boxes of
// the d1 operators.  For a glyph which starts with d0, or with a d1 operator
// whose box has no interior, the extent is a rectangle which contains all
// ink the glyph can paint, whatever graphics state the text is shown with.
// It is infinite where the ink has no bound, for example for a shading
// painted without a clipping path, or a stroke whose line width the glyph
// does not set.  An extent which reaches far beyond the glyph's em square
// is infinite, too, since no layout can make use of it.
func (f *Font) New() (font.Layouter, error) {
	if len(f.Glyphs) == 0 || f.Glyphs[0].Name != "" {
		return nil, errors.New("invalid glyph 0")
	}

	// extract per-glyph width and bbox once, then derive everything else
	// from the cached values; this avoids re-iterating Content streams
	// that may be backed by file streams.
	rawWidths := make([]float64, len(f.Glyphs))
	rawBBoxes := make([]rect.Rect, len(f.Glyphs))
	charProcs := make(map[pdf.Name]*dict.CharProc, len(f.Glyphs))
	for i, g := range f.Glyphs {
		if i > 0 {
			name := pdf.Name(g.Name)
			if err := name.Validate(); err != nil {
				return nil, fmt.Errorf("glyph %d: %w", i, err)
			}
			if _, dup := charProcs[name]; dup {
				return nil, fmt.Errorf("glyph %d: duplicate name %q", i, g.Name)
			}
			charProcs[name] = &dict.CharProc{
				Content:   g.Content,
				Resources: g.Resources,
			}
		}

		res := g.Resources
		if res == nil {
			res = f.Resources
		}
		w, bbox, err := g.extract(res, i == 0)
		if err != nil {
			return nil, fmt.Errorf("glyph %d (%q): %w", i, g.Name, err)
		}
		rawWidths[i] = w
		rawBBoxes[i] = bbox
	}

	cmap := make(map[rune]glyph.ID)
	for i, g := range f.Glyphs {
		rr := []rune(names.ToUnicode(g.Name, f.PostScriptName))
		if len(rr) == 1 {
			cmap[rr[0]] = glyph.ID(i)
		}
	}

	qv := f.FontMatrix[3]
	qh := f.FontMatrix[0]
	ee := make([]rect.Rect, len(f.Glyphs))
	ww := make([]float64, len(f.Glyphs))
	for i := range f.Glyphs {
		// transform bounding box from glyph space to text space
		switch bbox := rawBBoxes[i]; {
		case isInfinite(bbox):
			ee[i] = infiniteRect
		case !bbox.IsZero():
			ee[i] = bbox.Transform(f.FontMatrix)
			if !withinReach(ee[i]) {
				ee[i] = infiniteRect
			}
		}
		ww[i] = rawWidths[i] * qh
	}
	// the font need not set the cap height and x-height, so fall back to
	// measuring the glyphs; anything still missing is estimated
	capHeight := f.CapHeight * qv
	xHeight := f.XHeight * qv
	if capHeight <= 0 || xHeight <= 0 {
		byName := make(map[string]int, len(f.Glyphs))
		for i, g := range f.Glyphs {
			if g.Name != "" {
				byName[g.Name] = i
			}
		}
		if capHeight <= 0 {
			capHeight = measureHeight(byName, ee, pstype1.CapHeightChars)
		}
		if xHeight <= 0 {
			xHeight = measureHeight(byName, ee, pstype1.XHeightChars)
		}
	}
	geom := &font.Geometry{
		Ascent:             f.Ascent * qv,
		Descent:            f.Descent * qv,
		Leading:            f.Leading * qv,
		CapHeight:          capHeight,
		XHeight:            xHeight,
		UnderlinePosition:  pdf.Round(f.UnderlinePosition*qv, 6),
		UnderlineThickness: pdf.Round(f.UnderlineThickness*qv, 6),
		GlyphExtents:       ee,
		Widths:             ww,
	}
	fontgeom.FillHeights(geom)

	// Type 3 fonts are always simple fonts.  The encoder records widths in
	// the units of the Widths array, which for Type 3 fonts is glyph space.
	notdefWidth := math.Round(rawWidths[0])
	simple := simpleenc.NewSimpleType3(
		notdefWidth,
		f.PostScriptName,
		&pdfenc.WinAnsi,
	)

	res := &instance{
		Font:      f,
		CMap:      cmap,
		rawWidths: rawWidths,
		charProcs: charProcs,
		Geometry:  geom,
		Simple:    simple,
	}
	return res, nil
}

// PostScriptName returns the PostScript name of the font.
func (f *instance) PostScriptName() string {
	return f.Font.PostScriptName
}

// ResourceName returns the preferred resource-dictionary key for this font.
// See [font.Instance.ResourceName].
func (f *instance) ResourceName() pdf.Name {
	return f.Font.Name
}

// Layout converts a string to a sequence of positioned glyphs.
func (f *instance) Layout(seq *font.GlyphSeq, ptSize float64, s string) *font.GlyphSeq {
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
			Text:    string(r),
			Advance: f.rawWidths[gid] * q,
		})
	}
	return seq
}

func (f *instance) Codes(s pdf.String) iter.Seq[font.Code] {
	return func(yield func(font.Code) bool) {
		q := 1000 * f.Font.FontMatrix[0]
		for code := range f.Simple.Codes(s) {
			code.Width *= q
			if !yield(code) {
				return
			}
		}
	}
}

// FontInfo returns a [*dict.FontInfoType3] describing the font.  CharProcs
// holds every named glyph of the font, while Encoding covers the character
// codes assigned so far.  The CharProcs map is shared between calls and must
// not be modified.
func (f *instance) FontInfo() any {
	return &dict.FontInfoType3{
		CharProcs:  f.charProcs,
		FontMatrix: f.Font.FontMatrix,
		Resources:  f.Font.Resources,
		Encoding:   f.Simple.Encoding(),
	}
}

// Encode converts a glyph ID to a character code.
func (f *instance) Encode(gid glyph.ID, text string) (charcode.Code, bool) {
	if c, ok := f.Simple.GetCode(gid, text); ok {
		return charcode.Code(c), true
	}

	// Allocate new code
	glyphName := f.Font.Glyphs[gid].Name
	width := math.Round(f.rawWidths[gid])

	c, err := f.Simple.Encode(gid, glyphName, text, width)
	return charcode.Code(c), err == nil
}

// Embed implements the [pdf.Embedder] interface for Type 3 fonts.
func (f *instance) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) {
	if len(f.Font.Glyphs) == 0 || f.Font.Glyphs[0].Name != "" {
		return nil, errors.New("invalid glyph 0")
	}

	ref := rm.Alloc()
	rm.Defer(func(eh *pdf.EmbedHelper) error {
		dict, err := f.makeFontDict(eh)
		if err != nil {
			return err
		}
		_, err = eh.EmbedAt(ref, dict)
		return err
	})
	return ref, nil
}

// makeFontDict creates the Type 3 font dictionary for embedding.
func (f *instance) makeFontDict(_ *pdf.EmbedHelper) (*dict.Type3, error) {
	if err := f.Simple.Error(); err != nil {
		return nil, fmt.Errorf("Type3 font: %w", err)
	}

	italicAngle := math.Round(f.Font.ItalicAngle*10) / 10

	fd := &font.Descriptor{
		FontName:     f.Font.PostScriptName,
		FontFamily:   f.Font.FontFamily,
		FontStretch:  f.Font.FontStretch,
		FontWeight:   f.Font.FontWeight,
		IsFixedPitch: f.Font.IsFixedPitch,
		IsSerif:      f.Font.IsSerif,
		IsSymbolic:   f.Simple.IsSymbolic(),
		IsScript:     f.Font.IsScript,
		IsItalic:     italicAngle != 0,
		IsAllCap:     f.Font.IsAllCap,
		IsSmallCap:   f.Font.IsSmallCap,
		ItalicAngle:  italicAngle,
		Ascent:       f.Font.Ascent,
		Descent:      f.Font.Descent,
		Leading:      f.Font.Leading,
		CapHeight:    f.Font.CapHeight,
		XHeight:      f.Font.XHeight,
		StemV:        -1,
		MissingWidth: f.Simple.DefaultWidth(),
	}
	toUnicode, err := f.Simple.ToUnicode()
	if err != nil {
		return nil, err
	}

	// only the glyphs which have been assigned codes are embedded
	charProcs := make(map[pdf.Name]*dict.CharProc)
	for _, gid := range f.Simple.Glyphs() {
		if gid != 0 {
			name := pdf.Name(f.Font.Glyphs[gid].Name)
			charProcs[name] = f.charProcs[name]
		}
	}

	d := &dict.Type3{
		Name:       f.Font.Name,
		Descriptor: fd,
		Encoding:   f.Simple.Encoding(),
		CharProcs:  charProcs,
		FontMatrix: f.Font.FontMatrix,
		Resources:  f.Font.Resources,
		ToUnicode:  toUnicode,
	}
	for c, info := range f.Simple.MappedCodes() {
		d.Width[c] = info.Width
	}

	return d, nil
}

// measureHeight returns the height above the baseline of the first of the
// given characters the font has a non-blank glyph for, in text space units.
// It is used to derive a cap height or x-height for fonts which set neither;
// see [pstype1.CapHeightChars].  The byName map indexes extents by glyph name.
func measureHeight(byName map[string]int, extents []rect.Rect, chars string) float64 {
	for _, r := range chars {
		i, ok := byName[string(r)]
		if !ok {
			continue
		}
		// an extent without bound says nothing about the height
		if ext := extents[i]; ext.URy > 0 && !isInfinite(ext) { // the test also rejects NaN
			return ext.URy
		}
	}
	return 0
}
