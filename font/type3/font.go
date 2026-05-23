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
	"fmt"
	"iter"
	"math"

	"seehuhn.de/go/postscript/type1/names"

	"seehuhn.de/go/sfnt/glyph"
	"seehuhn.de/go/sfnt/os2"

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/geom/rect"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/dict"
	"seehuhn.de/go/pdf/font/encoding/simpleenc"
	"seehuhn.de/go/pdf/font/glyphdata"
	"seehuhn.de/go/pdf/font/pdfenc"
	"seehuhn.de/go/pdf/graphics/content"
)

// Font represents a Type 3 font with user-defined glyph procedures.
type Font struct {
	// Glyphs is a list of glyphs in the font.
	// An empty glyph without a name must be included at index 0,
	// to replace the ".notdef" glyph.
	Glyphs []*Glyph

	// Resources (optional) holds named resources shared by all glyph content
	// streams that don't have their own resource dictionary. This is embedded
	// in the Type 3 font dictionary.
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
	// Name is the PostScript name of the glyph.
	Name string

	// Content is the glyph's content stream.
	// It must start with either the d0 or d1 operator.
	// Use [content/builder.Builder] to construct content streams; its
	// [content/builder.Builder.Build] / [content/builder.Builder.Harvest]
	// methods return a [*content.Operators] that satisfies [content.Stream].
	Content content.Stream

	// Resources (optional) holds named resources used by this glyph's content
	// stream. If set, the resources are embedded in the glyph's stream
	// dictionary. If nil, resources are looked up from the font's resource
	// dictionary or inherited from the page.
	Resources *content.Resources
}

// extract iterates the glyph's content stream once and returns its
// advance width, bounding box, and any validation error.  For glyphs
// loaded from PDF files, Content may be backed by a file stream;
// this method ensures the stream is read at most once per glyph.
func (g *Glyph) extract() (width float64, bbox rect.Rect, err error) {
	if g.Content == nil {
		return 0, rect.Rect{}, nil
	}

	it := g.Content.NewIter()
	first := true
	var firstName content.OpName
	// d0 path-bbox accumulator
	pathBBox := rect.Rect{
		LLx: math.Inf(+1),
		LLy: math.Inf(+1),
		URx: math.Inf(-1),
		URy: math.Inf(-1),
	}
	hasPoints := false
	addPoint := func(x, y float64) {
		hasPoints = true
		pathBBox.LLx = min(pathBBox.LLx, x)
		pathBBox.LLy = min(pathBBox.LLy, y)
		pathBBox.URx = max(pathBBox.URx, x)
		pathBBox.URy = max(pathBBox.URy, y)
	}
	for name, args := range it.All() {
		if first {
			first = false
			firstName = name
			width, bbox, err = parseGlyphMetricsOp(name, args)
			if err != nil {
				return 0, rect.Rect{}, err
			}
			if firstName == content.OpType3UncoloredGlyph {
				// d1: bbox came from the args; the remaining content
				// has no metric significance, so stop reading.
				return width, bbox, nil
			}
			continue
		}
		switch name {
		case content.OpMoveTo, content.OpLineTo:
			if len(args) >= 2 {
				x, _ := getNumber(args[0])
				y, _ := getNumber(args[1])
				addPoint(x, y)
			}
		case content.OpCurveTo:
			for i := 0; i+1 < len(args) && i < 6; i += 2 {
				x, _ := getNumber(args[i])
				y, _ := getNumber(args[i+1])
				addPoint(x, y)
			}
		case content.OpCurveToV, content.OpCurveToY:
			for i := 0; i+1 < len(args) && i < 4; i += 2 {
				x, _ := getNumber(args[i])
				y, _ := getNumber(args[i+1])
				addPoint(x, y)
			}
		case content.OpRectangle:
			if len(args) >= 4 {
				x, _ := getNumber(args[0])
				y, _ := getNumber(args[1])
				w, _ := getNumber(args[2])
				h, _ := getNumber(args[3])
				addPoint(x, y)
				addPoint(x+w, y+h)
			}
		}
	}
	if first {
		return 0, rect.Rect{}, nil
	}
	if hasPoints {
		bbox = pathBBox
	}
	return width, bbox, nil
}

// parseGlyphMetricsOp reads the leading d0/d1 operator and returns the
// glyph's advance width and (for d1) the explicit bounding box.
func parseGlyphMetricsOp(name content.OpName, args []pdf.Object) (float64, rect.Rect, error) {
	switch name {
	case content.OpType3ColoredGlyph: // d0
		if len(args) < 2 {
			return 0, rect.Rect{}, errors.New("d0 requires 2 arguments (wx, wy)")
		}
		wy, _ := getNumber(args[1])
		if wy != 0 {
			return 0, rect.Rect{}, errors.New("wy must be 0")
		}
		wx, _ := getNumber(args[0])
		return wx, rect.Rect{}, nil
	case content.OpType3UncoloredGlyph: // d1
		if len(args) < 6 {
			return 0, rect.Rect{}, errors.New("d1 requires 6 arguments (wx, wy, llx, lly, urx, ury)")
		}
		wy, _ := getNumber(args[1])
		if wy != 0 {
			return 0, rect.Rect{}, errors.New("wy must be 0")
		}
		wx, _ := getNumber(args[0])
		llx, _ := getNumber(args[2])
		lly, _ := getNumber(args[3])
		urx, _ := getNumber(args[4])
		ury, _ := getNumber(args[5])
		return wx, rect.Rect{LLx: llx, LLy: lly, URx: urx, URy: ury}, nil
	default:
		return 0, rect.Rect{}, fmt.Errorf("content must start with d0 or d1, got %s", name)
	}
}

// Width returns the glyph's advance width in glyph coordinate units.
// The width is extracted from the first operator (d0 or d1).  This
// iterates the content stream — type3.Font.New caches per-glyph
// widths so callers should not need to invoke this on a hot path.
func (g *Glyph) Width() float64 {
	w, _, _ := g.extract()
	return w
}

// BBox returns the glyph's bounding box in glyph coordinate units.
// For d1 glyphs, the bounding box is taken from the operator arguments.
// For d0 glyphs, the bounding box is computed from path control points.
// This iterates the content stream — type3.Font.New caches per-glyph
// bounding boxes so callers should not need to invoke this on a hot path.
func (g *Glyph) BBox() rect.Rect {
	_, b, _ := g.extract()
	return b
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

	*font.Geometry

	*simpleenc.Simple
}

// New creates a new Type 3 font instance from a font definition.
func (f *Font) New() (font.Layouter, error) {
	if len(f.Glyphs) == 0 || f.Glyphs[0].Name != "" {
		return nil, errors.New("invalid glyph 0")
	}

	// extract per-glyph width and bbox once, then derive everything else
	// from the cached values; this avoids re-iterating Content streams
	// that may be backed by file streams.
	rawWidths := make([]float64, len(f.Glyphs))
	rawBBoxes := make([]rect.Rect, len(f.Glyphs))
	for i, g := range f.Glyphs {
		w, bbox, err := g.extract()
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
		bbox := rawBBoxes[i]
		if !bbox.IsZero() {
			corners := []struct{ x, y float64 }{
				{bbox.LLx, bbox.LLy},
				{bbox.LLx, bbox.URy},
				{bbox.URx, bbox.LLy},
				{bbox.URx, bbox.URy},
			}
			M := f.FontMatrix
			ee[i] = rect.Rect{
				LLx: math.Inf(+1),
				LLy: math.Inf(+1),
				URx: math.Inf(-1),
				URy: math.Inf(-1),
			}
			for _, c := range corners {
				x := M[0]*c.x + M[2]*c.y + M[4]
				y := M[1]*c.x + M[3]*c.y + M[5]
				ee[i].LLx = min(ee[i].LLx, x)
				ee[i].LLy = min(ee[i].LLy, y)
				ee[i].URx = max(ee[i].URx, x)
				ee[i].URy = max(ee[i].URy, y)
			}
		}
		ww[i] = rawWidths[i] * qh
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

	// Initialize encoding state - Type3 fonts are always simple fonts
	notdefWidth := math.Round(ww[0] * 1000)
	simple := simpleenc.NewSimple(
		notdefWidth,
		f.PostScriptName,
		&pdfenc.WinAnsi,
	)

	res := &instance{
		Font:      f,
		CMap:      cmap,
		rawWidths: rawWidths,
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

// FontInfo returns information about the font file.
func (f *instance) FontInfo() any {
	return &dict.FontInfoSimple{
		PostScriptName: f.Font.PostScriptName,
		FontFile:       &glyphdata.Stream{},
		Encoding:       f.Simple.Encoding(),
		IsSymbolic:     f.Simple.IsSymbolic(),
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

	glyphs := f.Simple.Glyphs()

	// Build CharProc entries from the font's glyphs.
	// The actual stream writing is handled by dict.Type3.Embed().
	charProcs := make(map[pdf.Name]*dict.CharProc)
	for _, gid := range glyphs {
		g := f.Font.Glyphs[gid]
		if g.Name == "" {
			continue
		}
		charProcs[pdf.Name(g.Name)] = &dict.CharProc{
			Content:   g.Content,
			Resources: g.Resources,
		}
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
	d := &dict.Type3{
		Name:       f.Font.Name,
		Descriptor: fd,
		Encoding:   f.Simple.Encoding(),
		CharProcs:  charProcs,
		// FontBBox:   &pdf.Rectangle{},
		FontMatrix: f.Font.FontMatrix,
		Resources:  f.Font.Resources,
		ToUnicode:  f.Simple.ToUnicode(f.Font.PostScriptName),
	}
	for c, info := range f.Simple.MappedCodes() {
		d.Width[c] = info.Width
	}

	return d, nil
}
