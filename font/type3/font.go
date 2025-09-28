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
	"math"
	"strconv"

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
	"seehuhn.de/go/pdf/graphics"
)

// Font represents a Type 3 font with user-defined glyph procedures.
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

// Glyph represents a single glyph in a Type 3 font.
type Glyph struct {
	// Name is the PostScript name of the glyph.
	Name string

	// Width is the glyph's advance width in glyph coordinate units.
	Width float64

	// BBox is the glyph's bounding box.
	BBox rect.Rect

	// Color indicates whether the glyph specifies color, or only describes a
	// shape.
	Color bool

	// Draw is the function that renders the glyph.
	Draw func(*graphics.Writer) error
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

// ToTextSpace converts a value from glyph space to text space.
func (f *instance) ToTextSpace(x float64) float64 {
	return x * f.Font.FontMatrix[0]
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
			Advance: f.Font.Glyphs[gid].Width * q,
		})
	}
	return seq
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
func (f *instance) Encode(gid glyph.ID, width float64, text string) (charcode.Code, bool) {
	if c, ok := f.Simple.GetCode(gid, text); ok {
		return charcode.Code(c), true
	}

	// Allocate new code
	glyphName := f.Font.Glyphs[gid].Name
	if width <= 0 {
		width = math.Round(f.Font.Glyphs[gid].Width)
	}

	c, err := f.Simple.Encode(gid, glyphName, text, width)
	return charcode.Code(c), err == nil
}

// Embed implements the pdf.Embedder interface for Type 3 fonts.
func (f *instance) Embed(rm *pdf.EmbedHelper) (pdf.Native, pdf.Unused, error) {
	if len(f.Font.Glyphs) == 0 || f.Font.Glyphs[0].Name != "" {
		return nil, pdf.Unused{}, errors.New("invalid glyph 0")
	}

	ref := rm.Alloc()
	rm.Defer(func(eh *pdf.EmbedHelper) error {
		dict, err := f.makeFontDict(eh)
		if err != nil {
			return err
		}
		_, _, err = pdf.EmbedHelperEmbedAt(eh, ref, dict)
		return err
	})
	return ref, pdf.Unused{}, nil
}

// makeFontDict creates the Type 3 font dictionary for embedding.
func (f *instance) makeFontDict(rm *pdf.EmbedHelper) (*dict.Type3, error) {
	if err := f.Simple.Error(); err != nil {
		return nil, fmt.Errorf("Type3 font: %w", err)
	}

	glyphs := f.Simple.Glyphs()

	// Write the glyphs first, so that we can construct the resources
	// dictionary. Here we use a common builder for all glyphs, so that a
	// common resources dictionary for the whole font can be accumulated.
	//
	// TODO(voss):
	//   - consider the discussion at
	//     https://pdf-issues.pdfa.org/32000-2-2020/clause07.html#H7.8.3
	//   - check where different PDF versions put the Resources dictionary
	//   - make it configurable whether to use per-glyph resource dictionaries?
	page := graphics.NewWriter(nil, rm.GetRM())
	charProcs := make(map[pdf.Name]pdf.Reference)
	for _, gid := range glyphs {
		g := f.Font.Glyphs[gid]
		if g.Name == "" {
			continue
		}
		gRef := rm.Alloc()

		charProcs[pdf.Name(g.Name)] = gRef

		stm, err := rm.Out().OpenStream(gRef, nil, pdf.FilterCompress{})
		if err != nil {
			return nil, err
		}
		page.NewStream(stm)

		// TODO(voss): move "d0" and "d1" to the graphics package, and restrict
		// the list of allowed operators depending on the choice.
		if g.Color {
			fmt.Fprintf(stm, "%s 0 d0\n", format(g.Width))
		} else {
			fmt.Fprintf(stm,
				"%s 0 %s %s %s %s d1\n",
				format(g.Width),
				format(g.BBox.LLx),
				format(g.BBox.LLy),
				format(g.BBox.URx),
				format(g.BBox.URy))
		}
		if g.Draw != nil {
			err = g.Draw(page)
			if err != nil {
				return nil, err
			}
		}
		if page.Err != nil {
			return nil, page.Err
		}
		err = stm.Close()
		if err != nil {
			return nil, err
		}
	}
	resources := page.Resources

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
	dict := &dict.Type3{
		Descriptor: fd,
		Encoding:   f.Simple.Encoding(),
		CharProcs:  charProcs,
		// FontBBox:   &pdf.Rectangle{},
		FontMatrix: f.Font.FontMatrix,
		Resources:  resources,
		ToUnicode:  f.Simple.ToUnicode(f.Font.PostScriptName),
	}
	for c, info := range f.Simple.MappedCodes() {
		dict.Width[c] = info.Width
	}

	return dict, nil
}

func format(x float64) string {
	return strconv.FormatFloat(x, 'f', -1, 64)
}
