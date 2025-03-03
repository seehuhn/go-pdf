// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2024  Jochen Voss <voss@seehuhn.de>
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

package main

import (
	"bytes"
	"errors"
	"fmt"
	"iter"
	"math"
	"slices"

	"golang.org/x/exp/maps"
	"golang.org/x/image/font/gofont/gomono"

	"seehuhn.de/go/sfnt"
	sfntcmap "seehuhn.de/go/sfnt/cmap"
	"seehuhn.de/go/sfnt/glyf"
	"seehuhn.de/go/sfnt/glyph"
	"seehuhn.de/go/sfnt/post"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/pdfenc"
	"seehuhn.de/go/postscript/funit"
	"seehuhn.de/go/postscript/type1/names"
)

type fontBuilder struct {
	base *sfnt.Font
	cmap sfntcmap.Subtable
	num  int
}

func NewFontBuilder() (*fontBuilder, error) {
	base, err := sfnt.Read(bytes.NewReader(gomono.TTF))
	if err != nil {
		return nil, fmt.Errorf("gofont: %w", err)
	}

	cmap, err := base.CMapTable.GetBest()
	if err != nil {
		return nil, fmt.Errorf("gofont: cmap: %w", err)
	}

	base.CMapTable = nil
	base.Gdef = nil
	base.Gpos = nil
	base.Gsub = nil

	res := &fontBuilder{
		base: base,
		cmap: cmap,
	}
	return res, nil
}

// design:
// We make sure that the sets of IDs and codes used for the different
// mechanisms are as disjoint as possible.
//
// 1-11: glyph IDs for the digits and "X"
// 48-57: ASCII codes for the digits
// 88: ASCII code for "X"
// 130: WinAnsi encoding for "quotesinglbase"
// 131: WinAnsi encoding for "florin"
// 132: WinAnsi encoding for "quotedblbase"
// 196: MacOS Roman encoding for "florin"
// 226: MacOS Roman encoding for "quotesinglbase"
// 227: MacOS Roman encoding for "quotedblbase"
// 402: Unicode for "florin"
// 8218: Unicode for "quotesinglbase"
// 8222: Unicode for "quotedblbase"

var markerString = []byte{130, 131, 132}

// encInfo specifies which glyphs should be shown for the input `markerString`,
// if the viewer uses a given method to specify the encoding.
type encInfo struct {
	useEncoding bool
	useSymbolic bool
	base        uint16

	cmap_1_0     string
	cmap_1_0_enc string
	cmap_3_0     string
	cmap_3_1     string
	post         string
}

// BuildFont constructs a new TrueType font which uses a selection of methods
// to map codes to glyphs.  To make it possible to tell apart which method
// a viewer uses, different mappings map the same codes to different
// glyphs.  The glyphs are chosen to spell the strings given in enc.
func (fb *fontBuilder) BuildFont(enc *encInfo) (font.Font, error) {
	// provisionally construct the subset of glyphs that are used
	runeUsed := make(map[rune]struct{})
	yes := struct{}{}
	for _, s := range []string{enc.cmap_1_0, enc.cmap_1_0_enc, enc.cmap_3_0, enc.cmap_3_1, enc.post} {
		for _, r := range s {
			runeUsed[r] = yes
		}
	}
	runeUsed['X'] = yes
	runes := maps.Keys(runeUsed)
	slices.Sort(runes)

	glyphsUsed := make(map[glyph.ID]rune)
	glyphsUsed[0] = 0xFFFD
	for _, r := range runes {
		gid := fb.cmap.Lookup(r)
		if gid == 0 {
			return nil, fmt.Errorf("gofont: no glyph for rune %q", r)
		}
		glyphsUsed[gid] = r
	}
	glyphs := maps.Keys(glyphsUsed)
	slices.Sort(glyphs)

	findGlyph := make(map[rune]glyph.ID, len(glyphs)-1)
	for subsetGID, origGID := range glyphs {
		if subsetGID == 0 {
			continue
		}
		findGlyph[glyphsUsed[origGID]] = glyph.ID(subsetGID)
	}

	// set up the different mappings

	var encoding pdf.Object
	if enc.useEncoding {
		encoding = pdf.Name("WinAnsiEncoding")
	}

	cmap_1_0 := &sfntcmap.Format0{}
	for i := range cmap_1_0.Data {
		cmap_1_0.Data[i] = byte(findGlyph['X'])
	}
	useCmap_1_0 := false

	cmap_3_0 := sfntcmap.Format4{}
	for c := range 256 {
		cmap_3_0[uint16(c)+enc.base] = findGlyph['X']
	}
	useCmap_3_0 := false

	cmap_3_1 := sfntcmap.Format4{}
	useCmap_3_1 := false

	var postTable *post.Info

	// method A: in a (1,0) "cmap" look up `c` to get the GID
	if enc.cmap_1_0 != "" {
		rr := []rune(enc.cmap_1_0)
		cmap_1_0.Data[markerString[0]] = byte(findGlyph[rr[0]])
		cmap_1_0.Data[markerString[1]] = byte(findGlyph[rr[1]])
		cmap_1_0.Data[markerString[2]] = byte(findGlyph[rr[2]])
		useCmap_1_0 = true
	}

	// method B: in a (3,0) "cmap" subtable look up `c+base` to get the GID.
	if enc.cmap_3_0 != "" {
		rr := []rune(enc.cmap_3_0)
		cmap_3_0[uint16(markerString[0])+enc.base] = findGlyph[rr[0]]
		cmap_3_0[uint16(markerString[1])+enc.base] = findGlyph[rr[1]]
		cmap_3_0[uint16(markerString[2])+enc.base] = findGlyph[rr[2]]
		useCmap_3_0 = true
	}

	// method C: use the encoding to map `c` to a name, use MacOS Roman to map
	// the name to a code, and then look up this code in a (1, 0) "cmap" to get
	// the GID.
	if enc.cmap_1_0_enc != "" {
		if !enc.useEncoding {
			return nil, errIncompatibleContraints
		}

		rr := []rune(enc.cmap_1_0_enc)
		cmap_1_0.Data[macOSRomanInv[pdfenc.WinAnsi.Encoding[markerString[0]]]] = byte(findGlyph[rr[0]])
		cmap_1_0.Data[macOSRomanInv[pdfenc.WinAnsi.Encoding[markerString[1]]]] = byte(findGlyph[rr[1]])
		cmap_1_0.Data[macOSRomanInv[pdfenc.WinAnsi.Encoding[markerString[2]]]] = byte(findGlyph[rr[2]])
		useCmap_1_0 = true
	}

	// method D: Use the encoding to map `c` to a name, use the Adobe Glyph
	// List to map the name to unicode, and use a (3,1) "cmap" subtable to map
	// this character to a glyph.
	if enc.cmap_3_1 != "" {
		if !enc.useEncoding {
			return nil, errIncompatibleContraints
		}

		rr := []rune(enc.cmap_3_1)
		for i := range 3 {
			code := markerString[i]
			name := pdfenc.WinAnsi.Encoding[code]
			uni := names.ToUnicode(name, false)
			if len(uni) != 1 {
				panic(fmt.Sprintf("expected 1 rune for %s, got %d", name, len(uni)))
			}
			cmap_3_1[uint16(uni[0])] = findGlyph[rr[i]]
		}
		useCmap_3_1 = true
	}

	// method E: Use the encoding to map `c` to a name, and use the "post"
	// table to look up the glyph.
	if enc.post != "" {
		if !enc.useEncoding {
			return nil, errIncompatibleContraints
		}

		names := make([]string, len(glyphs))
		names[0] = ".notdef"
		for i := 1; i < len(names); i++ {
			names[i] = fmt.Sprintf("glyph%02d", i)
		}

		seen := make(map[rune]bool)

		rr := []rune(enc.post)
		for i, r := range rr {
			name := pdfenc.WinAnsi.Encoding[markerString[i]]
			if !seen[r] {
				names[findGlyph[r]] = name
			} else {
				// we need a duplicate glyph, since we have two different
				// names for the character
				glyphs = append(glyphs, glyphs[findGlyph[r]])
				names = append(names, name)
			}
			seen[r] = true
		}
		postTable = &post.Info{
			IsFixedPitch: true,
			Names:        names,
		}
	}

	// build the subset	font

	newTTF := fb.base.Subset(glyphs)
	newTTF.CMapTable = make(sfntcmap.Table)
	newTTF.FamilyName = fmt.Sprintf("Test%03d", fb.num)
	fb.num++

	q := 1000 / float64(newTTF.UnitsPerEm)

	bbox := newTTF.FontBBox()
	pdfBBox := &pdf.Rectangle{
		LLx: bbox.LLx.AsFloat(q),
		LLy: bbox.LLy.AsFloat(q),
		URx: bbox.URx.AsFloat(q),
		URy: bbox.URy.AsFloat(q),
	}

	// Since we are not sure which glyphs are going to be shown,
	// make sure that all glyphs have the same width.
	outlines := newTTF.Outlines.(*glyf.Outlines)
	var w funit.Int16
	for _, wi := range outlines.Widths[1:] {
		if wi > w {
			w = wi
		}
	}
	for i := range outlines.Widths {
		outlines.Widths[i] = w
	}
	pdfWidth := math.Round(w.AsFloat(q))

	if useCmap_1_0 {
		newTTF.CMapTable[sfntcmap.Key{PlatformID: 1, EncodingID: 0}] = cmap_1_0.Encode(0)
	}
	if useCmap_3_0 {
		newTTF.CMapTable[sfntcmap.Key{PlatformID: 3, EncodingID: 0}] = cmap_3_0.Encode(0)
	}
	if useCmap_3_1 {
		newTTF.CMapTable[sfntcmap.Key{PlatformID: 3, EncodingID: 1}] = cmap_3_1.Encode(0)
	}

	res := &testFont{
		ttf:       newTTF,
		post:      postTable,
		width:     pdfWidth,
		bbox:      pdfBBox,
		ascent:    newTTF.Ascent.AsFloat(q),
		descent:   newTTF.Descent.AsFloat(q),
		capHeight: newTTF.CapHeight.AsFloat(q),

		encoding: encoding,
		symbolic: enc.useSymbolic,
	}

	return res, nil
}

var errIncompatibleContraints = errors.New("incompatible constraints")

type testFont struct {
	ttf       *sfnt.Font
	post      *post.Info
	width     float64
	bbox      *pdf.Rectangle
	ascent    float64
	descent   float64
	capHeight float64

	encoding pdf.Object
	symbolic bool
}

func (f *testFont) PostScriptName() string {
	return f.ttf.FamilyName
}

func (f *testFont) Embed(rm *pdf.ResourceManager) (pdf.Native, font.Embedded, error) {
	fontDictRef := rm.Out.Alloc()
	fontDescriptorRef := rm.Out.Alloc()
	fontFileRef := rm.Out.Alloc()

	fontDict := pdf.Dict{
		"Type":           pdf.Name("Font"),
		"Subtype":        pdf.Name("TrueType"),
		"BaseFont":       pdf.Name(f.PostScriptName()),
		"FirstChar":      pdf.Integer(0),
		"LastChar":       pdf.Integer(0),
		"Widths":         pdf.Array{pdf.Number(f.width)},
		"FontDescriptor": fontDescriptorRef,
		"Encoding":       f.encoding,
	}
	err := rm.Out.Put(fontDictRef, fontDict)
	if err != nil {
		return nil, nil, err
	}

	var flags pdf.Integer
	if f.symbolic {
		flags |= 1 << 2
	} else {
		flags |= 1 << 5
	}
	fontDescriptor := pdf.Dict{
		"Type":         pdf.Name("FontDescriptor"),
		"FontName":     pdf.Name(f.PostScriptName()),
		"Flags":        flags,
		"FontBBox":     f.bbox,
		"ItalicAngle":  pdf.Number(0),
		"Ascent":       pdf.Number(f.ascent),
		"Descent":      pdf.Number(f.descent),
		"CapHeight":    pdf.Number(f.capHeight),
		"StemV":        pdf.Number(0),
		"MissingWidth": pdf.Number(f.width),
		"FontFile2":    fontFileRef,
	}
	err = rm.Out.Put(fontDescriptorRef, fontDescriptor)
	if err != nil {
		return nil, nil, err
	}

	length1 := pdf.NewPlaceholder(rm.Out, 10)
	fontFileDict := pdf.Dict{
		"Length1": length1,
	}
	fontFileStream, err := rm.Out.OpenStream(fontFileRef, fontFileDict, pdf.FilterCompress{})
	if err != nil {
		return nil, nil, err
	}
	var extra []any
	if f.post != nil {
		extra = append(extra, "post", f.post.Encode())
	}
	n, err := f.ttf.WriteTrueTypePDF(fontFileStream, extra...)
	if err != nil {
		return nil, nil, err
	}
	err = length1.Set(pdf.Integer(n))
	if err != nil {
		return nil, nil, err
	}
	err = fontFileStream.Close()
	if err != nil {
		return nil, nil, err
	}

	return fontDictRef, f, nil
}

func (f *testFont) WritingMode() font.WritingMode {
	return font.Horizontal
}

func (f *testFont) Codes(s pdf.String) iter.Seq[*font.Code] {
	return func(yield func(*font.Code) bool) {
		// TODO(voss): implement
	}
}

var (
	macOSRomanInv map[string]int
)

func init() {
	macOSRomanInv = make(map[string]int)
	for c, name := range pdfenc.MacRomanAlt.Encoding {
		if name == ".notdef" {
			continue
		}
		if _, ok := macOSRomanInv[name]; !ok {
			macOSRomanInv[name] = c
		}
	}
}
