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

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/builtin"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/tounicode"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/sfnt/funit"
	"seehuhn.de/go/sfnt/glyph"
	"seehuhn.de/go/sfnt/os2"
	"seehuhn.de/go/sfnt/type1"
	"seehuhn.de/go/sfnt/type1/names"
)

// A Builder is used to construct a type 3 font for inclusion in PDF file.
type Builder struct {
	// The following fields must be set before the font is made,
	// if the PDF file is "tagged".  Otherwise, they are ignored.
	FontName    pdf.Name // The PostScript name of the font.
	FontFamily  string
	Width       os2.Width
	Weight      os2.Weight
	Flags       font.Flags
	ItalicAngle float64 // Italic angle (degrees counterclockwise from vertical)

	names        []pdf.Name
	glyphs       [][]byte
	widths       []funit.Int16
	glyphExtents []funit.Rect
	idx          map[pdf.Name]int
	unitsPerEm   uint16

	cmap      map[rune]int
	glyphRefs []*pdf.Reference
}

// New creates a new Builder for embedding a type 3 font into the PDF file w.
// A typical value for unitsPerEm is 1000.
func New(unitsPerEm uint16) (*Builder, error) {
	b := &Builder{
		idx:        make(map[pdf.Name]int),
		unitsPerEm: unitsPerEm,
	}
	return b, nil
}

// AddGlyph adds a new glyph to the type 3 font.
//
// Glyph IDs are allocated in the order the glyphs are added.  The first glyph
// should be the ".notdef" glyph.
//
// If shapeOnly is true, a call to the "d1" operator is added at the start of
// the glyph description.  In this case, the glyph description may only specify
// the shape of the glyph, but not its color.  Otherwise, a call to the "d0"
// operator is added at the start of the glyph description.  In this case, the
// glyph description may specify both the shape and the color of the glyph.
func (b *Builder) AddGlyph(name pdf.Name, width funit.Int16, bbox funit.Rect, shapeOnly bool) (*Glyph, error) {
	if b.cmap != nil {
		return nil, errors.New("font already made")
	}
	if _, exists := b.idx[name]; exists {
		return nil, errors.New("glyph already present")
	}

	if len(b.glyphs) == 0 && name != ".notdef" {
		b.names = append(b.names, "")
		b.glyphs = append(b.glyphs, nil)
		b.widths = append(b.widths, 0)
		b.glyphExtents = append(b.glyphExtents, funit.Rect{})
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

	if shapeOnly {
		fmt.Fprintf(glyph.Content,
			"%d 0 %d %d %d %d d1\n", width, bbox.LLx, bbox.LLy, bbox.URx, bbox.URy)
	} else {
		fmt.Fprintf(glyph.Content, "%d 0 d0\n", width)
	}

	return glyph, nil
}

func (b *Builder) MakeFont(resourceName pdf.Name) *font.NewFont {
	if b.cmap == nil {
		b.cmap = make(map[rune]int)
		for i, name := range b.names {
			rr := names.ToUnicode(string(name), false)
			if len(rr) != 1 {
				continue
			}
			r := rr[0]
			if j, exists := b.cmap[r]; exists && b.names[j] < name {
				// In case two names map to the same rune, use the
				// one with the lexicographically earlier name.
				continue
			}
			b.cmap[r] = i
		}
		b.glyphRefs = make([]*pdf.Reference, len(b.names))
	}

	res := &font.NewFont{
		UnitsPerEm:         b.unitsPerEm,
		Ascent:             0,
		Descent:            0,
		BaseLineSkip:       0,
		UnderlinePosition:  0,
		UnderlineThickness: 0,
		GlyphExtents:       b.glyphExtents,
		Widths:             b.widths,
		Layout:             b.layout,
		ResourceName:       resourceName,
		GetDict:            b.getDict,
	}
	return res
}

func (b *Builder) layout(s string, ptSize float64) glyph.Seq {
	rr := []rune(s)
	res := make(glyph.Seq, len(rr))
	for i, r := range rr {
		gid := b.cmap[r]
		res[i].Gid = glyph.ID(gid)
		res[i].Advance = b.widths[gid]
		res[i].Text = []rune{r}
	}
	return res
}

func (b *Builder) getDict(w *pdf.Writer, resName pdf.Name) (font.Dict, error) {
	fd := &t3dict{
		w:       w,
		ref:     w.Alloc(),
		resName: resName,
		b:       b,
		enc:     cmap.NewSimpleEncoder(),
	}
	return fd, nil
}

type t3dict struct {
	w       *pdf.Writer
	ref     *pdf.Reference
	resName pdf.Name
	b       *Builder
	enc     cmap.SimpleEncoder
}

func (fd *t3dict) Reference() *pdf.Reference {
	return fd.ref
}

func (fd *t3dict) ResourceName() pdf.Name {
	return fd.resName
}

func (fd *t3dict) Typeset(s string, ptSize float64) glyph.Seq {
	return fd.b.layout(s, ptSize)
}

func (fd *t3dict) AppendEncoded(s pdf.String, gid glyph.ID, rr []rune) pdf.String {
	return append(s, fd.enc.Encode(gid, rr))
}

func (fd *t3dict) GetUnitsPerEm() uint16 {
	return fd.b.unitsPerEm
}

func (fd *t3dict) GetWidths() []funit.Int16 {
	return fd.b.widths
}

func (fd *t3dict) Close() error {
	w := fd.w
	compress := &pdf.FilterInfo{Name: pdf.Name("LZWDecode")}
	if w.Version >= pdf.V1_2 {
		compress.Name = "FlateDecode"
	}

	encoding := fd.enc.Encoding()
	var firstChar cmap.CID
	for int(firstChar) < len(encoding) && encoding[firstChar] == 0 {
		firstChar++
	}
	lastChar := cmap.CID(len(encoding) - 1)
	for lastChar > firstChar && encoding[lastChar] == 0 {
		lastChar--
	}

	q := 1000 / float64(fd.b.unitsPerEm)

	FontDictRef := fd.ref
	CharProcsRef := w.Alloc()
	EncodingRef := w.Alloc()
	WidthsRef := w.Alloc()

	FontDict := pdf.Dict{ // See section 9.6.5 of PDF 32000-1:2008.
		"Type":     pdf.Name("Font"),
		"Subtype":  pdf.Name("Type3"),
		"FontBBox": &pdf.Rectangle{}, // [0,0,0,0] is always valid
		"FontMatrix": pdf.Array{
			pdf.Real(1 / float64(fd.b.unitsPerEm)), pdf.Integer(0), pdf.Integer(0),
			pdf.Real(1 / float64(fd.b.unitsPerEm)), pdf.Integer(0), pdf.Integer(0)},
		"CharProcs": CharProcsRef,
		"Encoding":  EncodingRef,
		"FirstChar": pdf.Integer(firstChar),
		"LastChar":  pdf.Integer(lastChar),
		"Widths":    WidthsRef,
		// Resources
	}

	CharProcs := pdf.Dict{}
	for _, gid := range encoding {
		if gid == 0 && fd.b.names[0] == "" {
			continue
		}
		if fd.b.glyphRefs[gid] == nil {
			stream, ref, err := w.OpenStream(nil, nil, compress)
			if err != nil {
				return err
			}
			_, err = stream.Write(fd.b.glyphs[gid])
			if err != nil {
				return err
			}
			err = stream.Close()
			if err != nil {
				return err
			}
			fd.b.glyphRefs[gid] = ref
		}
		name := fd.b.names[gid]
		CharProcs[name] = fd.b.glyphRefs[gid]
	}

	var Differences pdf.Array
	var prevIdx cmap.CID = lastChar + 1
	for idx := firstChar; idx <= lastChar; idx++ {
		gid := encoding[idx]
		if gid == 0 {
			continue
		}

		if idx != prevIdx+1 {
			Differences = append(Differences, pdf.Integer(idx))
		}
		Differences = append(Differences, fd.b.names[gid])
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
			width = pdf.Integer(math.Round(fd.b.widths[gid].AsFloat(q)))
		}
		Widths = append(Widths, width)
	}

	compressedRefs := []*pdf.Reference{FontDictRef, CharProcsRef, EncodingRef, WidthsRef}
	compressedObjects := []pdf.Object{FontDict, CharProcs, Encoding, Widths}

	if w.Tagged {
		if fd.b.FontName == "" || fd.b.Flags == 0 {
			return errors.New("FontName/Flags required for Type 3 fonts in tagged PDF files")
		}

		FontDescriptorRef := w.Alloc()
		FontDescriptor := pdf.Dict{ // See section 9.8.1 of PDF 32000-1:2008.
			"Type":        pdf.Name("FontDescriptor"),
			"FontName":    fd.b.FontName,
			"Flags":       pdf.Integer(fd.b.Flags),
			"ItalicAngle": pdf.Number(fd.b.ItalicAngle),
		}
		if fd.b.FontFamily != "" {
			FontDescriptor["FontFamily"] = pdf.String(fd.b.FontFamily)
		}
		if fd.b.Width != 0 {
			FontDescriptor["FontStretch"] = pdf.Name(strings.ReplaceAll(fd.b.Width.String(), " ", ""))
		}
		if fd.b.Weight != 0 {
			FontDescriptor["FontWeight"] = pdf.Integer(fd.b.Weight.Rounded())
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
		name := fd.b.names[gid]
		if name == ".notdef" {
			continue
		}
		if _, ok := type1.StandardEncoding[string(name)]; ok {
			continue
		}
		if inSymbolFont[name] {
			continue
		}
		needToUnicode = true
		break
	}
	var ToUnicodeRef *pdf.Reference
	if needToUnicode {
		ToUnicodeRef = w.Alloc()
		FontDict["ToUnicode"] = ToUnicodeRef
	}

	_, err := w.WriteCompressed(compressedRefs, compressedObjects...)
	if err != nil {
		return err
	}

	if needToUnicode {
		var mappings []tounicode.Single
		for code, gid := range encoding {
			if gid == 0 {
				continue
			}
			name := fd.b.names[gid]
			mappings = append(mappings, tounicode.Single{
				Code:  cmap.CID(code),
				UTF16: utf16.Encode(names.ToUnicode(string(name), false)),
			})
		}
		info := tounicode.FromMappings(mappings)
		_, err := info.Embed(w, ToUnicodeRef)
		if err != nil {
			return err
		}
	}

	return nil
}

func symbolNames() map[pdf.Name]bool {
	symbolAfm, err := builtin.Afm(builtin.Symbol)
	if err != nil {
		panic(err)
	}
	symbolNames := make(map[pdf.Name]bool)
	for _, name := range symbolAfm.GlyphName {
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
