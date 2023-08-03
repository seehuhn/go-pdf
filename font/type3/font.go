// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2021  Jochen Voss <voss@seehuhn.de>
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

// Package type3 provides support for embedding type 3 fonts into PDF documents.
package type3

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"strings"
	"unicode/utf16"

	"seehuhn.de/go/postscript/funit"
	"seehuhn.de/go/postscript/psenc"
	"seehuhn.de/go/postscript/type1"
	"seehuhn.de/go/postscript/type1/names"

	"seehuhn.de/go/sfnt/glyph"
	"seehuhn.de/go/sfnt/os2"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/tounicodeold"
	type11 "seehuhn.de/go/pdf/font/type1"
	"seehuhn.de/go/pdf/graphics"
)

// TODO(voss): The spec says "Type 3 fonts do not support the concept of a
// default glyph name." Does this refer to the ".notdef" glyph?

// TODO(voss): do type3 fonts without a .notdef character work properly
// everywhere?

// A Builder is used to construct a type 3 font for inclusion in PDF file.
type Builder struct {
	// Required, if the PDF file is "tagged", Otherwise, these fields are ignored.
	FontName    pdf.Name // PostScript name of the font
	FontFamily  string
	Width       os2.Width
	Weight      os2.Weight
	Flags       font.Flags
	ItalicAngle float64 // Italic angle (degrees counterclockwise from vertical)

	// The following fields are simply copied into the font's [font.Geometry]
	// struct.  They are not otherwise used.
	Ascent             funit.Int16
	Descent            funit.Int16 // negative
	BaseLineSkip       funit.Int16
	UnderlinePosition  funit.Float64
	UnderlineThickness funit.Float64

	names        []pdf.Name
	glyphs       [][]byte
	widths       []funit.Int16
	glyphExtents []funit.Rect16
	idx          map[pdf.Name]int
	unitsPerEm   uint16

	made bool
}

// New creates a new Builder for drawing the glyphs of a type 3 font.
// A typical value for unitsPerEm is 1000.
func New(unitsPerEm uint16) *Builder {
	b := &Builder{
		idx:        make(map[pdf.Name]int),
		unitsPerEm: unitsPerEm,
	}
	return b
}

// AddGlyph adds a new glyph to the type 3 font.
//
// Glyph IDs are allocated in the order the glyphs are added.
//
// If shapeOnly is true, a call to the "d1" operator is added at the start of
// the glyph description.  In this case, the glyph description may only specify
// the shape of the glyph, but not its color.  Otherwise, a call to the "d0"
// operator is added at the start of the glyph description.  In this case, the
// glyph description may specify both the shape and the color of the glyph.
func (b *Builder) AddGlyph(name pdf.Name, width funit.Int16, bbox funit.Rect16, shapeOnly bool) (*Glyph, error) {
	if b.made {
		return nil, errors.New("font already made")
	}
	if _, exists := b.idx[name]; exists {
		return nil, errors.New("glyph already present")
	}

	if len(b.glyphs) == 0 && name != ".notdef" {
		b.names = append(b.names, "")
		b.glyphs = append(b.glyphs, nil)
		b.widths = append(b.widths, 0)
		b.glyphExtents = append(b.glyphExtents, funit.Rect16{})
	}

	gid := len(b.glyphs)
	b.names = append(b.names, name)
	b.glyphs = append(b.glyphs, nil)
	b.widths = append(b.widths, width)
	b.glyphExtents = append(b.glyphExtents, bbox)

	glyph := &Glyph{
		Page: graphics.NewPage(&bytes.Buffer{}),
		b:    b,
		gid:  gid,
	}

	glyph.Page.ForgetGraphicsState()

	if shapeOnly {
		fmt.Fprintf(glyph.Content,
			"%d 0 %d %d %d %d d1\n", width, bbox.LLx, bbox.LLy, bbox.URx, bbox.URy)
	} else {
		fmt.Fprintf(glyph.Content, "%d 0 d0\n", width)
	}

	return glyph, nil
}

func (b *Builder) EmbedFont(w pdf.Putter, resName pdf.Name) (font.Embedded, error) {
	F, err := b.MakeFont()
	if err != nil {
		return nil, err
	}
	E, err := F.Embed(w, resName)
	if err != nil {
		return nil, err
	}
	return E, nil
}

func (b *Builder) MakeFont() (font.Font, error) {
	if len(b.glyphs) == 0 {
		return nil, errors.New("no glyphs in font")
	}
	b.made = true

	g := &font.Geometry{
		UnitsPerEm:   b.unitsPerEm,
		GlyphExtents: b.glyphExtents,
		Widths:       b.widths,

		Ascent:             b.Ascent,
		Descent:            b.Descent,
		BaseLineSkip:       b.BaseLineSkip,
		UnderlinePosition:  b.UnderlinePosition,
		UnderlineThickness: b.UnderlineThickness,
	}

	cmap := make(map[rune]int)
	for i, name := range b.names {
		rr := names.ToUnicode(string(name), false)
		if len(rr) != 1 {
			continue
		}
		r := rr[0]
		if j, exists := cmap[r]; exists && b.names[j] < name {
			// In case two names map to the same rune, use the
			// one with the lexicographically earlier name.
			continue
		}
		cmap[r] = i
	}

	return &type3{
		b:    b,
		g:    g,
		cmap: cmap,
	}, nil
}

type type3 struct {
	b    *Builder
	g    *font.Geometry
	cmap map[rune]int
}

func (t3 *type3) GetGeometry() *font.Geometry {
	return t3.g
}

func (t3 *type3) Layout(s string, ptSize float64) glyph.Seq {
	rr := []rune(s)
	gg := make(glyph.Seq, len(rr))
	for i, r := range rr {
		gid := t3.cmap[r]
		gg[i].Gid = glyph.ID(gid)
		gg[i].Advance = t3.g.Widths[gid]
		gg[i].Text = []rune{r}
	}
	return gg
}

func (t3 *type3) Embed(w pdf.Putter, resName pdf.Name) (font.Embedded, error) {
	res := &embedded{
		w:         w,
		ref:       w.Alloc(),
		resName:   resName,
		enc:       cmap.NewSimpleEncoder(),
		glyphRefs: make([]pdf.Reference, len(t3.b.names)),
		type3:     t3,
	}
	w.AutoClose(res)
	return res, nil
}

type embedded struct {
	w         pdf.Putter
	ref       pdf.Reference
	resName   pdf.Name
	enc       cmap.SimpleEncoder
	glyphRefs []pdf.Reference
	*type3
}

func (e3 *embedded) AppendEncoded(s pdf.String, gid glyph.ID, rr []rune) pdf.String {
	return append(s, e3.enc.Encode(gid, rr))
}

func (e3 *embedded) Reference() pdf.Reference {
	return e3.ref
}

func (e3 *embedded) ResourceName() pdf.Name {
	return e3.resName
}

func (e3 *embedded) Close() error {
	if e3.enc.Overflow() {
		return fmt.Errorf("too many distinct glyphs used from type 3 font %q",
			e3.resName)
	}

	w := e3.w

	encoding := e3.enc.Encoding()
	var firstChar type1.CID
	for int(firstChar) < len(encoding) && encoding[firstChar] == 0 {
		firstChar++
	}
	lastChar := type1.CID(len(encoding) - 1)
	for lastChar > firstChar && encoding[lastChar] == 0 {
		lastChar--
	}

	q := 1000 / float64(e3.b.unitsPerEm)

	FontDictRef := e3.ref
	CharProcsRef := w.Alloc()
	EncodingRef := w.Alloc()
	WidthsRef := w.Alloc()

	FontDict := pdf.Dict{ // See section 9.6.5 of PDF 32000-1:2008.
		"Type":     pdf.Name("Font"),
		"Subtype":  pdf.Name("Type3"),
		"FontBBox": &pdf.Rectangle{}, // empty rectangle is allowed here
		"FontMatrix": pdf.Array{
			pdf.Real(1 / float64(e3.b.unitsPerEm)), pdf.Integer(0), pdf.Integer(0),
			pdf.Real(1 / float64(e3.b.unitsPerEm)), pdf.Integer(0), pdf.Integer(0)},
		"CharProcs": CharProcsRef,
		"Encoding":  EncodingRef,
		"FirstChar": pdf.Integer(firstChar),
		"LastChar":  pdf.Integer(lastChar),
		"Widths":    WidthsRef,
		// Resources
	}

	CharProcs := pdf.Dict{}
	for _, gid := range encoding {
		if gid == 0 && e3.b.names[0] == "" {
			continue
		}
		if e3.glyphRefs[gid] == 0 {
			ref := w.Alloc()
			stream, err := w.OpenStream(ref, nil, pdf.FilterCompress{})
			if err != nil {
				return err
			}
			_, err = stream.Write(e3.b.glyphs[gid])
			if err != nil {
				return err
			}
			err = stream.Close()
			if err != nil {
				return err
			}
			e3.glyphRefs[gid] = ref
		}
		name := e3.b.names[gid]
		CharProcs[name] = e3.glyphRefs[gid]
	}

	var Differences pdf.Array
	var prevIdx type1.CID = lastChar + 1
	for idx := firstChar; idx <= lastChar; idx++ {
		gid := encoding[idx]
		if gid == 0 {
			continue
		}

		if idx != prevIdx+1 {
			Differences = append(Differences, pdf.Integer(idx))
		}
		Differences = append(Differences, e3.b.names[gid])
		prevIdx = idx
	}
	Encoding := pdf.Dict{
		"Differences": Differences,
	}

	var Widths pdf.Array
	for i := firstChar; i <= lastChar; i++ {
		var width pdf.Integer
		gid := encoding[i]
		if gid != 0 {
			width = pdf.Integer(math.Round(e3.b.widths[gid].AsFloat(q)))
		}
		Widths = append(Widths, width)
	}

	compressedRefs := []pdf.Reference{FontDictRef, CharProcsRef, EncodingRef, WidthsRef}
	compressedObjects := []pdf.Object{FontDict, CharProcs, Encoding, Widths}

	if pdf.IsTagged(w) {
		if e3.b.FontName == "" || e3.b.Flags == 0 {
			return errors.New("FontName/Flags required for Type 3 fonts in tagged PDF files")
		}

		FontDescriptorRef := w.Alloc()
		FontDescriptor := pdf.Dict{ // See section 9.8.1 of PDF 32000-1:2008.
			"Type":        pdf.Name("FontDescriptor"),
			"FontName":    e3.b.FontName,
			"Flags":       pdf.Integer(e3.b.Flags),
			"ItalicAngle": pdf.Number(e3.b.ItalicAngle),
		}
		if e3.b.FontFamily != "" {
			FontDescriptor["FontFamily"] = pdf.String(e3.b.FontFamily)
		}
		if e3.b.Width != 0 {
			FontDescriptor["FontStretch"] = pdf.Name(strings.ReplaceAll(e3.b.Width.String(), " ", ""))
		}
		if e3.b.Weight != 0 {
			FontDescriptor["FontWeight"] = pdf.Integer(e3.b.Weight.Rounded())
		}

		FontDict["FontDescriptor"] = FontDescriptorRef
		compressedRefs = append(compressedRefs, FontDescriptorRef)
		compressedObjects = append(compressedObjects, FontDescriptor)
	}

	// If the following condition is violated, we need to include a
	// /ToUnicode entry: "the font includes only character names taken from the
	// Adobe standard Latin character set and the set of named characters in
	// the Symbol font".
	needToUnicode := false
	inSymbolFont := symbolNames()
	for _, gid := range encoding {
		name := e3.b.names[gid]
		if name == ".notdef" {
			continue
		}
		if _, ok := psenc.StandardEncodingRev[string(name)]; ok {
			continue
		}
		if inSymbolFont[name] {
			continue
		}
		needToUnicode = true
		break
	}
	var ToUnicodeRef pdf.Reference
	if needToUnicode {
		ToUnicodeRef = w.Alloc()
		FontDict["ToUnicode"] = ToUnicodeRef
	}

	err := w.WriteCompressed(compressedRefs, compressedObjects...)
	if err != nil {
		return err
	}

	if needToUnicode {
		var mappings []tounicodeold.Single
		for code, gid := range encoding {
			if gid == 0 {
				continue
			}
			name := e3.b.names[gid]
			mappings = append(mappings, tounicodeold.Single{
				Code:  type1.CID(code),
				UTF16: utf16.Encode(names.ToUnicode(string(name), false)),
			})
		}
		info := tounicodeold.FromMappings(mappings)
		err := info.Embed(ToUnicodeRef, w)
		if err != nil {
			return err
		}
	}

	return nil
}

func symbolNames() map[pdf.Name]bool {
	symbolAfm, err := type11.Symbol.Afm()
	if err != nil {
		panic(err)
	}
	symbolNames := make(map[pdf.Name]bool)
	for name := range symbolAfm.Outlines {
		symbolNames[pdf.Name(name)] = true
	}
	return symbolNames
}

// Glyph is used to write a glyph description as described in section
// 9.6.5 of PDF 32000-1:2008.  The .Close() method must be called after
// the description has been written.
type Glyph struct {
	*graphics.Page
	b   *Builder
	gid int
}

// Close most be called after the glyph description has been written.
func (g *Glyph) Close() error {
	w := g.Content.(*bytes.Buffer)
	g.b.glyphs[g.gid] = w.Bytes()
	return nil
}
