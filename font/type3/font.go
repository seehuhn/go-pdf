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
	// Use [content/builder.Builder] to construct content streams.
	Content content.Operators

	// Resources (optional) holds named resources used by this glyph's content
	// stream. If set, the resources are embedded in the glyph's stream
	// dictionary. If nil, resources are looked up from the font's resource
	// dictionary or inherited from the page.
	Resources *content.Resources
}

// Width returns the glyph's advance width in glyph coordinate units.
// The width is extracted from the first operator (d0 or d1).
func (g *Glyph) Width() float64 {
	if len(g.Content) == 0 || len(g.Content[0].Args) < 1 {
		return 0
	}
	wx, _ := getNumber(g.Content[0].Args[0])
	return wx
}

// BBox returns the glyph's bounding box in glyph coordinate units.
// For d1 glyphs, the bounding box is taken from the operator arguments.
// For d0 glyphs, the bounding box is computed from path control points.
func (g *Glyph) BBox() rect.Rect {
	if len(g.Content) == 0 {
		return rect.Rect{}
	}
	op := g.Content[0]

	if op.Name == content.OpType3UncoloredGlyph { // d1
		if len(op.Args) >= 6 {
			llx, _ := getNumber(op.Args[2])
			lly, _ := getNumber(op.Args[3])
			urx, _ := getNumber(op.Args[4])
			ury, _ := getNumber(op.Args[5])
			return rect.Rect{LLx: llx, LLy: lly, URx: urx, URy: ury}
		}
		return rect.Rect{}
	}

	// d0: compute from path operators
	return computeBBoxFromPath(g.Content[1:])
}

// validateContent checks that the glyph content stream is valid.
func (g *Glyph) validateContent() error {
	if len(g.Content) == 0 {
		return nil // empty content is allowed (e.g., for .notdef)
	}

	firstOp := g.Content[0].Name
	if firstOp != content.OpType3ColoredGlyph && firstOp != content.OpType3UncoloredGlyph {
		return fmt.Errorf("content must start with d0 or d1, got %s", firstOp)
	}

	if firstOp == content.OpType3ColoredGlyph {
		if len(g.Content[0].Args) < 2 {
			return errors.New("d0 requires 2 arguments (wx, wy)")
		}
	} else {
		if len(g.Content[0].Args) < 6 {
			return errors.New("d1 requires 6 arguments (wx, wy, llx, lly, urx, ury)")
		}
	}

	// Per PDF spec Table 111: "The number wy shall be 0"
	if len(g.Content[0].Args) >= 2 {
		wy, _ := getNumber(g.Content[0].Args[1])
		if wy != 0 {
			return errors.New("wy must be 0")
		}
	}

	return nil
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

// computeBBoxFromPath computes a bounding box from path control points.
func computeBBoxFromPath(stream content.Operators) rect.Rect {
	bbox := rect.Rect{
		LLx: math.Inf(+1),
		LLy: math.Inf(+1),
		URx: math.Inf(-1),
		URy: math.Inf(-1),
	}
	hasPoints := false

	addPoint := func(x, y float64) {
		hasPoints = true
		bbox.LLx = min(bbox.LLx, x)
		bbox.LLy = min(bbox.LLy, y)
		bbox.URx = max(bbox.URx, x)
		bbox.URy = max(bbox.URy, y)
	}

	for _, op := range stream {
		switch op.Name {
		case content.OpMoveTo, content.OpLineTo: // m, l: x y
			if len(op.Args) >= 2 {
				x, _ := getNumber(op.Args[0])
				y, _ := getNumber(op.Args[1])
				addPoint(x, y)
			}
		case content.OpCurveTo: // c: x1 y1 x2 y2 x3 y3
			for i := 0; i+1 < len(op.Args) && i < 6; i += 2 {
				x, _ := getNumber(op.Args[i])
				y, _ := getNumber(op.Args[i+1])
				addPoint(x, y)
			}
		case content.OpCurveToV, content.OpCurveToY: // v, y: 4 args
			for i := 0; i+1 < len(op.Args) && i < 4; i += 2 {
				x, _ := getNumber(op.Args[i])
				y, _ := getNumber(op.Args[i+1])
				addPoint(x, y)
			}
		case content.OpRectangle: // re: x y w h
			if len(op.Args) >= 4 {
				x, _ := getNumber(op.Args[0])
				y, _ := getNumber(op.Args[1])
				w, _ := getNumber(op.Args[2])
				h, _ := getNumber(op.Args[3])
				addPoint(x, y)
				addPoint(x+w, y+h)
			}
		}
	}

	if !hasPoints {
		return rect.Rect{}
	}
	return bbox
}

var _ font.Layouter = (*instance)(nil)

// instance represents a Type 3 font instance ready for embedding.
type instance struct {
	// Font is the underlying Type 3 font definition.
	Font *Font

	// CMap maps Unicode code points to glyph IDs.
	CMap map[rune]glyph.ID

	*font.Geometry

	*simpleenc.Simple
}

// New creates a new Type 3 font instance from a font definition.
func (f *Font) New() (font.Layouter, error) {
	if len(f.Glyphs) == 0 || f.Glyphs[0].Name != "" {
		return nil, errors.New("invalid glyph 0")
	}

	// Validate all glyphs have valid content streams
	for i, g := range f.Glyphs {
		if err := g.validateContent(); err != nil {
			return nil, fmt.Errorf("glyph %d (%q): %w", i, g.Name, err)
		}
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
	for i, g := range f.Glyphs {
		// transform bounding box from glyph space to text space
		bbox := g.BBox()
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
		ww[i] = g.Width() * qh
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
		Font:     f,
		CMap:     cmap,
		Geometry: geom,
		Simple:   simple,
	}
	return res, nil
}

// PostScriptName returns the PostScript name of the font.
func (f *instance) PostScriptName() string {
	return f.Font.PostScriptName
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
			Advance: f.Font.Glyphs[gid].Width() * q,
		})
	}
	return seq
}

func (f *instance) Codes(s pdf.String) iter.Seq[*font.Code] {
	return func(yield func(*font.Code) bool) {
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
	width := math.Round(f.Font.Glyphs[gid].Width())

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
