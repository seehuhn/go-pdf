// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"bytes"
	"fmt"
	"math"
	"time"

	"seehuhn.de/go/postscript/funit"
	"seehuhn.de/go/postscript/type1/names"

	"seehuhn.de/go/sfnt"
	"seehuhn.de/go/sfnt/cff"
	"seehuhn.de/go/sfnt/cmap"
	"seehuhn.de/go/sfnt/glyph"
	"seehuhn.de/go/sfnt/head"
	"seehuhn.de/go/sfnt/os2"
	"seehuhn.de/go/sfnt/parser"

	"seehuhn.de/go/pdf/font/pdfenc"
)

// pdfiumNotice is added to the copyright field of the Foxit fonts.
const pdfiumNotice = "Copyright 2014 PDFium Authors. All rights reserved."

// symbolSpec describes one of the Foxit fonts.
type symbolSpec struct {
	src      source
	family   string // e.g. "Quire Symbol"
	fontName string // e.g. "QuireSymbol"
	nameSet  string // glyph name set for names.ToUnicode
	encoding *pdfenc.Encoding
	desc     string
}

var (
	symbolSpecs = []symbolSpec{
		{foxitSymbolSource, "Quire Symbol", "QuireSymbol", "Symbol", &pdfenc.Symbol,
			"Derived from Foxit Symbol, as distributed with PDFium (renamed and wrapped as OpenType)."},
		{foxitDingbatsSource, "Quire Dingbats", "QuireDingbats", "ZapfDingbats", &pdfenc.ZapfDingbats,
			"Derived from Foxit Dingbats, as distributed with PDFium (renamed and wrapped as OpenType)."},
	}
)

// makeQuireSymbol wraps a bare CFF font as an OpenType/CFF font.
// The cmap has a Unicode subtable, built from the glyph names, and a
// (3,0) symbol subtable for the built-in encoding.
func makeQuireSymbol(spec symbolSpec, data []byte) ([]byte, error) {
	cf, err := cff.Read(bytes.NewReader(data), parser.NewBudget(int64(len(data))))
	if err != nil {
		return nil, err
	}
	outlines := cf.Outlines
	if outlines.IsCIDKeyed() {
		return nil, fmt.Errorf("%s: unexpected CID-keyed font", spec.fontName)
	}
	info := cf.FontInfo

	name2gid := make(map[string]glyph.ID, len(outlines.Glyphs))
	for gid, g := range outlines.Glyphs {
		name2gid[g.Name] = glyph.ID(gid)
	}

	// Unicode subtable, the first glyph wins
	uni := cmap.Format4{}
	for gid, g := range outlines.Glyphs {
		rr := []rune(names.ToUnicode(g.Name, spec.nameSet))
		if len(rr) != 1 || rr[0] > 0xFFFF {
			continue
		}
		if _, exists := uni[uint16(rr[0])]; !exists {
			uni[uint16(rr[0])] = glyph.ID(gid)
		}
	}

	// built-in encoding, both in the CFF data and as a symbol subtable
	encoding := make([]glyph.ID, 256)
	sym := cmap.Format4{}
	for code, name := range spec.encoding.Encoding {
		if name == "" {
			continue
		}
		gid, ok := name2gid[name]
		if !ok {
			return nil, fmt.Errorf("%s: glyph %q of the built-in encoding missing", spec.fontName, name)
		}
		encoding[code] = gid
		sym[uint16(0xF000+code)] = gid
	}
	outlines.Encoding = encoding

	if info.FontMatrix[0] <= 0 {
		return nil, fmt.Errorf("%s: invalid font matrix", spec.fontName)
	}
	unitsPerEm := math.Round(1 / info.FontMatrix[0])

	var capHeight, xHeight float64
	if gid, ok := name2gid["H"]; ok {
		capHeight = float64(outlines.Glyphs[gid].Extent().URy)
	}
	if gid, ok := name2gid["x"]; ok {
		xHeight = float64(outlines.Glyphs[gid].Extent().URy)
	}

	if outlines.Glyphs[0].Name != ".notdef" {
		return nil, fmt.Errorf("%s: glyph 0 is %q", spec.fontName, outlines.Glyphs[0].Name)
	}
	setNotdefCFF(outlines, unitsPerEm, capHeight)

	// metrics from the glyph boxes
	var ascent, descent float64
	for _, g := range outlines.Glyphs {
		if g.Name == "" {
			continue
		}
		bb := g.Extent()
		ascent = max(ascent, float64(bb.URy))
		descent = min(descent, float64(bb.LLy))
	}

	font := &sfnt.Font{
		FontName:   spec.fontName,
		FamilyName: spec.family,
		Subfamily:  "Regular",
		FullName:   spec.family,
		Width:      os2.WidthNormal,
		Weight:     os2.WeightNormal,
		IsRegular:  true,
		Version:    head.Version(0x00010000),
		// fixed timestamps: the PDFium fonts carry no date, and the output must be reproducible
		CreationTime:       time.Date(2014, 1, 1, 0, 0, 0, 0, time.UTC),
		ModificationTime:   time.Date(2014, 1, 1, 0, 0, 0, 0, time.UTC),
		Description:        spec.desc,
		Copyright:          joinNotices(info.Copyright, pdfiumNotice),
		License:            "BSD 3-Clause License, see Licenses.txt in go-pdf font/loader/builtin.",
		LicenseURL:         "https://opensource.org/license/bsd-3-clause",
		UnitsPerEm:         uint16(unitsPerEm),
		Ascent:             funit.Int16(math.Round(ascent)),
		Descent:            funit.Int16(math.Round(descent)),
		CapHeight:          funit.Int16(math.Round(capHeight)),
		XHeight:            funit.Int16(math.Round(xHeight)),
		ItalicAngle:        info.ItalicAngle,
		UnderlinePosition:  info.UnderlinePosition,
		UnderlineThickness: info.UnderlineThickness,
		FontMatrix:         info.FontMatrix,
		Outlines:           outlines,
	}
	font.CodePageRange.Set(os2.CPSymbol)
	font.CMapTable = cmap.Table{
		{PlatformID: 0, EncodingID: 3}: uni.Encode(0),
		{PlatformID: 3, EncodingID: 1}: uni.Encode(0),
		{PlatformID: 3, EncodingID: 0}: sym.Encode(0),
	}

	buf := &bytes.Buffer{}
	if _, err := font.Write(buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func joinNotices(a, b string) string {
	if a == "" {
		return b
	}
	return a + " " + b
}
