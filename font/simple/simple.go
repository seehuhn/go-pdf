// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2022  Jochen Voss <voss@seehuhn.de>
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

// Package simple provides support for embedding simple fonts into PDF documents.
package simple

import (
	"errors"
	"fmt"
	"math"
	"os"
	"sort"

	"golang.org/x/text/language"

	"seehuhn.de/go/sfnt"
	"seehuhn.de/go/sfnt/cff"
	sfntcmap "seehuhn.de/go/sfnt/cmap"
	"seehuhn.de/go/sfnt/funit"
	"seehuhn.de/go/sfnt/glyf"
	"seehuhn.de/go/sfnt/glyph"
	"seehuhn.de/go/sfnt/opentype/gtab"
	"seehuhn.de/go/sfnt/type1"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cmap"
)

// LoadFont loads a fonts from a file and embeds it into a PDF document as a
// simple font. At the moment, only TrueType and OpenType fonts are supported.
//
// Up to 256 distinct glyphs from the font file can be accessed via the
// returned font object.  In comparison, fonts embedded via cid.LoadFont() lead
// to larger PDF files but there is no limit on the number of distinct glyphs
// which can be accessed.
func LoadFont(fname string, resourceName pdf.Name, loc language.Tag) (*font.NewFont, error) {
	fd, err := os.Open(fname)
	if err != nil {
		return nil, err
	}
	defer fd.Close()

	info, err := sfnt.Read(fd)
	if err != nil {
		return nil, err
	}
	return Font(info, resourceName, loc)
}

// Embed embeds a font into a PDF document as a simple font.
// At the moment, only TrueType and OpenType fonts are supported.
//
// Up to 256 distinct glyphs from the font file can be accessed via the
// returned font object.  In comparison, fonts embedded via cid.NewFile() lead
// to larger PDF files but there is no limit on the number of distinct glyphs
// which can be accessed.
func Font(info *sfnt.Info, resourceName pdf.Name, loc language.Tag) (*font.NewFont, error) {
	gsubLookups := info.Gsub.FindLookups(loc, gtab.GsubDefaultFeatures)
	gposLookups := info.Gpos.FindLookups(loc, gtab.GposDefaultFeatures)

	defaultText := make(map[glyph.ID][]rune)
	low, high := info.CMap.CodeRange()
	for r := low; r <= high; r++ {
		gid := info.CMap.Lookup(r)
		if gid != 0 {
			defaultText[gid] = []rune{r}
		}
	}

	res := &font.NewFont{
		UnitsPerEm:         info.UnitsPerEm,
		Ascent:             info.Ascent,
		Descent:            info.Descent,
		BaseLineSkip:       info.Ascent - info.Descent + info.LineGap,
		UnderlinePosition:  info.UnderlinePosition,
		UnderlineThickness: info.UnderlineThickness,
		GlyphExtents:       info.Extents(),
		Widths:             info.Widths(),
		Layout: func(rr []rune) glyph.Seq {
			gg := info.Layout(rr, gsubLookups, gposLookups)
			for _, g := range gg {
				if g.Text != nil && defaultText[g.Gid] == nil {
					defaultText[g.Gid] = g.Text
				}
			}
			return gg
		},
		ResourceName: resourceName,
		GetDict: func(w *pdf.Writer) (font.Dict, error) {
			return getDict(info, defaultText, w)
		},
	}
	return res, nil
}

type fontDict struct {
	ref *pdf.Reference

	info *sfnt.Info

	defaultText map[glyph.ID][]rune
	enc         cmap.SimpleEncoder
}

func getDict(info *sfnt.Info, defaultText map[glyph.ID][]rune, w *pdf.Writer) (font.Dict, error) {
	if info.IsGlyf() {
		err := w.CheckVersion("use of TrueType glyph outlines", pdf.V1_1)
		if err != nil {
			return nil, err
		}
	} else if info.IsCFF() {
		err := w.CheckVersion("use of CFF glyph outlines", pdf.V1_2)
		if err != nil {
			return nil, err
		}
	} else {
		panic("unexpected glyph format")
	}

	return &fontDict{
		ref: w.Alloc(),

		info: info,

		defaultText: defaultText,
		enc:         cmap.NewSimpleEncoder(),
	}, nil
}

func (fd *fontDict) AppendEncoded(s pdf.String, gid glyph.ID, rr []rune) pdf.String {
	if rr == nil {
		rr = fd.defaultText[gid]
	}
	return append(s, fd.enc.Encode(gid, rr))
}

func (fd *fontDict) Reference() *pdf.Reference {
	return fd.ref
}

func (fd *fontDict) Write(w *pdf.Writer) error {
	if fd.enc.Overflow() {
		return fmt.Errorf("too many distinct glyphs for simple font %q",
			fd.info.FullName())
	}

	// Determine the subset of glyphs to include.
	encoding := fd.enc.Encoding()
	var firstChar cmap.CharCode
	for int(firstChar) < len(encoding) && encoding[firstChar] == 0 {
		firstChar++
	}
	if int(firstChar) >= len(encoding) {
		// no glyphs are encoded, so we don't need to write the font
		return nil
	}
	lastChar := cmap.CharCode(len(encoding) - 1)
	for encoding[lastChar] == 0 {
		lastChar--
	}

	subsetEncoding, subsetGlyphs := makeSubset(encoding)
	subsetTag := font.GetSubsetTag(subsetGlyphs, fd.info.NumGlyphs())

	// subset the font
	subsetInfo := &sfnt.Info{}
	*subsetInfo = *fd.info
	switch outlines := fd.info.Outlines.(type) {
	case *cff.Outlines:
		o2 := &cff.Outlines{}
		pIdxMap := make(map[int]int)
		for _, gid := range subsetGlyphs {
			o2.Glyphs = append(o2.Glyphs, outlines.Glyphs[gid])
			oldPIdx := outlines.FdSelect(gid)
			_, ok := pIdxMap[oldPIdx]
			if !ok {
				newPIdx := len(o2.Private)
				pIdxMap[oldPIdx] = newPIdx
				o2.Private = append(o2.Private, outlines.Private[oldPIdx])
			}
		}
		o2.FdSelect = func(gid glyph.ID) int {
			// TODO(voss): translate the gid values?
			return pIdxMap[outlines.FdSelect(gid)]
		}
		if len(o2.Private) > 1 || o2.Glyphs[0].Name == "" {
			// Embed as a CID-keyed CFF font.
			// TODO(voss): is this right?
			o2.ROS = &type1.ROS{
				Registry:   "Adobe",
				Ordering:   "Identity",
				Supplement: 0,
			}
			o2.Gid2cid = make([]int32, len(subsetGlyphs))
			for code, subsetGid := range subsetEncoding {
				o2.Gid2cid[subsetGid] = int32(code)
			}
		} else {
			// Embed as a simple CFF font.
			o2.Encoding = subsetEncoding
		}
		subsetInfo.Outlines = o2

	case *glyf.Outlines:
		newGid := make(map[glyph.ID]glyph.ID)
		todo := make(map[glyph.ID]bool)
		nextGid := glyph.ID(0)
		for _, gid := range subsetGlyphs {
			newGid[gid] = nextGid
			nextGid++

			for _, gid2 := range outlines.Glyphs[gid].Components() {
				if _, ok := newGid[gid2]; !ok {
					todo[gid2] = true
				}
			}
		}
		for len(todo) > 0 {
			gid := pop(todo)
			subsetGlyphs = append(subsetGlyphs, gid)
			newGid[gid] = nextGid
			nextGid++

			for _, gid2 := range outlines.Glyphs[gid].Components() {
				if _, ok := newGid[gid2]; !ok {
					todo[gid2] = true
				}
			}
		}

		o2 := &glyf.Outlines{
			Tables: outlines.Tables,
			Maxp:   outlines.Maxp,
		}
		for _, gid := range subsetGlyphs {
			g := outlines.Glyphs[gid]
			o2.Glyphs = append(o2.Glyphs, g.FixComponents(newGid))
			o2.Widths = append(o2.Widths, outlines.Widths[gid])
			// o2.Names = append(o2.Names, outlines.Names[gid])
		}
		subsetInfo.Outlines = o2

		encoding := sfntcmap.Format4{}
		for code, subsetGid := range subsetEncoding {
			encoding[uint16(code)] = subsetGid
		}
		subsetInfo.CMap = encoding
	}

	fontName := pdf.Name(subsetTag + "+" + subsetInfo.PostscriptName())

	q := 1000 / float64(subsetInfo.UnitsPerEm)

	var Widths pdf.Array
	for i := firstChar; i <= lastChar; i++ {
		var width pdf.Integer
		gid := encoding[i]
		if gid != 0 {
			width = pdf.Integer(math.Round(fd.info.GlyphWidth(gid).AsFloat(q)))
		}
		Widths = append(Widths, width)
	}

	FontDictRef := fd.ref
	FontDescriptorRef := w.Alloc()
	WidthsRef := w.Alloc()
	FontFileRef := w.Alloc()
	ToUnicodeRef := w.Alloc()

	FontDict := pdf.Dict{ // See section 9.6.2.1 of PDF 32000-1:2008.
		"Type":           pdf.Name("Font"),
		"BaseFont":       fontName,
		"FirstChar":      pdf.Integer(firstChar),
		"LastChar":       pdf.Integer(lastChar),
		"Widths":         WidthsRef,
		"FontDescriptor": FontDescriptorRef,
		"ToUnicode":      ToUnicodeRef,
	}

	rect := subsetInfo.BBox()
	fontBBox := &pdf.Rectangle{
		LLx: rect.LLx.AsFloat(q),
		LLy: rect.LLy.AsFloat(q),
		URx: rect.URx.AsFloat(q),
		URy: rect.URy.AsFloat(q),
	}

	FontDescriptor := pdf.Dict{ // See section 9.8.1 of PDF 32000-1:2008.
		"Type":        pdf.Name("FontDescriptor"),
		"FontName":    fontName,
		"Flags":       pdf.Integer(font.MakeFlags(subsetInfo, true)), // TODO(voss)
		"FontBBox":    fontBBox,
		"ItalicAngle": pdf.Number(subsetInfo.ItalicAngle),
		"Ascent":      pdf.Integer(math.Round(subsetInfo.Ascent.AsFloat(q))),
		"Descent":     pdf.Integer(math.Round(subsetInfo.Descent.AsFloat(q))),
		"CapHeight":   pdf.Integer(math.Round(subsetInfo.CapHeight.AsFloat(q))),
		"StemV":       pdf.Integer(70), // information not available in sfnt files
	}

	compressedRefs := []*pdf.Reference{FontDictRef, FontDescriptorRef, WidthsRef}
	compressedObjects := []pdf.Object{FontDict, FontDescriptor, Widths}

	switch outlines := subsetInfo.Outlines.(type) {
	case *cff.Outlines:
		FontDict["Subtype"] = pdf.Name("Type1")
		FontDescriptor["FontFile3"] = FontFileRef

		// Write the "font program".
		// See section 9.9 of PDF 32000-1:2008 for details.
		fontFileDict := pdf.Dict{
			"Subtype": pdf.Name("Type1C"),
		}
		fontFileStream, _, err := w.OpenStream(fontFileDict, FontFileRef,
			&pdf.FilterInfo{Name: "FlateDecode"})
		if err != nil {
			return err
		}
		fontFile := cff.Font{
			FontInfo: subsetInfo.GetFontInfo(),
			Outlines: outlines,
		}
		err = fontFile.Encode(fontFileStream)
		if err != nil {
			return err
		}
		err = fontFileStream.Close()
		if err != nil {
			return err
		}

	case *glyf.Outlines:
		FontDict["Subtype"] = pdf.Name("TrueType")
		FontDescriptor["FontFile2"] = FontFileRef

		// Write the "font program".
		// See section 9.9 of PDF 32000-1:2008 for details.
		size := w.NewPlaceholder(10)
		fontFileDict := pdf.Dict{
			"Length1": size,
		}
		compress := &pdf.FilterInfo{Name: pdf.Name("LZWDecode")}
		if w.Version >= pdf.V1_2 {
			compress = &pdf.FilterInfo{Name: pdf.Name("FlateDecode")}
		}
		fontFileStream, _, err := w.OpenStream(fontFileDict, FontFileRef, compress)
		if err != nil {
			return err
		}
		n, err := subsetInfo.PDFEmbedTrueType(fontFileStream)
		if err != nil {
			return err
		}
		err = size.Set(pdf.Integer(n))
		if err != nil {
			return err
		}
		err = fontFileStream.Close()
		if err != nil {
			return err
		}
	}

	_, err := w.WriteCompressed(compressedRefs, compressedObjects...)
	if err != nil {
		return err
	}

	var cc2text []font.SimpleMapping
	for code, gid := range encoding {
		if gid != 0 {
			continue
		}
		text := fd.defaultText[gid]
		cc2text = append(cc2text, font.SimpleMapping{CharCode: byte(code), Text: text})
	}
	err = font.WriteToUnicodeSimple(w, subsetTag, cc2text, ToUnicodeRef)
	if err != nil {
		return err
	}

	return nil
}

func makeSubset(origEncoding []glyph.ID) (subsetEncoding, glyphs []glyph.ID) {
	subsetEncoding = make([]glyph.ID, 256)
	glyphs = append(glyphs, 0) // always include the .notdef glyph
	for code, origGid := range origEncoding {
		if origGid == 0 {
			continue
		}
		newGid := glyph.ID(len(glyphs))
		subsetEncoding[code] = newGid
		glyphs = append(glyphs, origGid)
	}
	return subsetEncoding, glyphs
}

func pop(todo map[glyph.ID]bool) glyph.ID {
	for key := range todo {
		delete(todo, key)
		return key
	}
	panic("empty map")
}

// EmbedFile embeds the named font file into the PDF document.
func EmbedFile(w *pdf.Writer, fname string, instName pdf.Name, loc language.Tag) (*font.Font, error) {
	fd, err := os.Open(fname)
	if err != nil {
		return nil, err
	}
	defer fd.Close()

	fontInfo, err := sfnt.Read(fd)
	if err != nil {
		return nil, err
	}
	return Embed(w, fontInfo, instName, loc)
}

// Embed embeds a TrueType or OpenType font into a PDF document as a simple font.
// Up to 256 arbitrary glyphs from the font file can be accessed via the
// returned font object.
//
// In comparison, fonts embedded via cid.Embed() lead to larger PDF files, but
// there is no limit on the number of glyphs which can be accessed.
//
// This requires PDF version 1.1 or higher, and
// use of CFF-based OpenType fonts requires PDF version 1.2 or higher.
func Embed(w *pdf.Writer, info *sfnt.Info, instName pdf.Name, loc language.Tag) (*font.Font, error) {
	if info.IsGlyf() {
		err := w.CheckVersion("use of TrueType glyph outlines", pdf.V1_1)
		if err != nil {
			return nil, err
		}
	} else if info.IsCFF() {
		err := w.CheckVersion("use of CFF glyph outlines", pdf.V1_2)
		if err != nil {
			return nil, err
		}
	} else {
		return nil, errors.New("unsupported glyph format")
	}

	widths := info.Widths()
	if widths == nil {
		return nil, errors.New("no glyph widths found")
	}

	s := &fontHandler{
		FontRef: w.Alloc(),

		info:        info,
		widths:      widths,
		GsubLookups: info.Gsub.FindLookups(loc, gtab.GsubDefaultFeatures),
		GposLookups: info.Gpos.FindLookups(loc, gtab.GposDefaultFeatures),

		text: map[glyph.ID][]rune{},
		enc:  map[glyph.ID]byte{},
	}

	w.OnClose(s.WriteFont)

	res := &font.Font{
		InstName:           instName,
		Ref:                s.FontRef,
		Layout:             s.Layout,
		Enc:                s.Enc,
		UnitsPerEm:         info.UnitsPerEm,
		Ascent:             info.Ascent,
		Descent:            info.Descent,
		BaseLineSkip:       info.Ascent - info.Descent + info.LineGap,
		UnderlinePosition:  info.UnderlinePosition,
		UnderlineThickness: info.UnderlineThickness,
		GlyphExtents:       info.Extents(),
		Widths:             widths,
	}
	return res, nil
}

type fontHandler struct {
	FontRef *pdf.Reference

	info        *sfnt.Info
	widths      []funit.Int16
	GsubLookups []gtab.LookupIndex
	GposLookups []gtab.LookupIndex

	text         map[glyph.ID][]rune
	enc          map[glyph.ID]byte
	nextCharCode int
}

func (s *fontHandler) Layout(rr []rune) glyph.Seq {
	seq := s.info.Layout(rr, s.GsubLookups, s.GposLookups)

	for _, g := range seq {
		if _, seen := s.text[g.Gid]; !seen && len(g.Text) > 0 {
			// copy the slice, in case the caller modifies it later
			s.text[g.Gid] = append([]rune{}, g.Text...)
		}
	}

	return seq
}

func (s *fontHandler) Enc(enc pdf.String, gid glyph.ID) pdf.String {
	c, ok := s.enc[gid]
	if ok {
		return append(enc, c)
	}

	c = byte(s.nextCharCode)
	s.nextCharCode++
	s.enc[gid] = c
	return append(enc, c)
}

// cMapEntry describes the association between a character code and
// a glyph ID.
type cMapEntry struct {
	CharCode uint16
	GID      glyph.ID
}

func (s *fontHandler) WriteFont(w *pdf.Writer) error {
	if s.nextCharCode > 256 {
		return fmt.Errorf("too many different glyphs for simple font %q",
			s.info.FullName())
	}

	// Determine the subset of glyphs to include.
	mapping := make([]cMapEntry, 0, len(s.enc))
	for gid, c := range s.enc {
		mapping = append(mapping, cMapEntry{
			CharCode: uint16(c),
			GID:      gid,
		})
	}
	sort.Slice(mapping, func(i, j int) bool { return mapping[i].CharCode < mapping[j].CharCode })

	if len(mapping) == 0 {
		// no glyphs are encoded, so we don't need to write the font
		return nil
	}

	firstCharCode := mapping[0].CharCode
	lastCharCode := mapping[len(mapping)-1].CharCode

	includeGlyphs := make([]glyph.ID, 0, len(mapping)+1)
	includeGlyphs = append(includeGlyphs, 0) // always include the .notdef glyph
	for _, m := range mapping {
		if m.GID == 0 {
			continue
		}
		includeGlyphs = append(includeGlyphs, m.GID)
	}
	subsetTag := font.GetSubsetTag(includeGlyphs, s.info.NumGlyphs())

	// subset the font
	subsetInfo := &sfnt.Info{}
	*subsetInfo = *s.info
	switch outlines := s.info.Outlines.(type) {
	case *cff.Outlines:
		o2 := &cff.Outlines{}
		pIdxMap := make(map[int]int)
		for _, gid := range includeGlyphs {
			o2.Glyphs = append(o2.Glyphs, outlines.Glyphs[gid])
			oldPIdx := outlines.FdSelect(gid)
			_, ok := pIdxMap[oldPIdx]
			if !ok {
				newPIdx := len(o2.Private)
				pIdxMap[oldPIdx] = newPIdx
				o2.Private = append(o2.Private, outlines.Private[oldPIdx])
			}
		}
		o2.FdSelect = func(gid glyph.ID) int {
			return pIdxMap[outlines.FdSelect(gid)]
		}
		if len(o2.Private) > 1 || o2.Glyphs[0].Name == "" {
			// Embed as a CID-keyed CFF font.
			o2.ROS = &type1.ROS{
				Registry:   "Adobe",
				Ordering:   "Identity",
				Supplement: 0,
			}
			o2.Gid2cid = make([]int32, len(includeGlyphs))
			for i, gid := range includeGlyphs {
				o2.Gid2cid[i] = int32(s.enc[gid])
			}
		} else {
			// Embed as a simple CFF font.
			o2.Encoding = make([]glyph.ID, 256)
			for i, gid := range includeGlyphs {
				o2.Encoding[s.enc[gid]] = glyph.ID(i)
			}
		}
		subsetInfo.Outlines = o2

	case *glyf.Outlines:
		newGid := make(map[glyph.ID]glyph.ID)
		todo := make(map[glyph.ID]bool)
		nextGid := glyph.ID(0)
		for _, gid := range includeGlyphs {
			newGid[gid] = nextGid
			nextGid++

			for _, gid2 := range outlines.Glyphs[gid].Components() {
				if _, ok := newGid[gid2]; !ok {
					todo[gid2] = true
				}
			}
		}
		for len(todo) > 0 {
			gid := pop(todo)
			includeGlyphs = append(includeGlyphs, gid)
			newGid[gid] = nextGid
			nextGid++

			for _, gid2 := range outlines.Glyphs[gid].Components() {
				if _, ok := newGid[gid2]; !ok {
					todo[gid2] = true
				}
			}
		}

		o2 := &glyf.Outlines{
			Tables: outlines.Tables,
			Maxp:   outlines.Maxp,
		}
		for _, gid := range includeGlyphs {
			g := outlines.Glyphs[gid]
			o2.Glyphs = append(o2.Glyphs, g.FixComponents(newGid))
			o2.Widths = append(o2.Widths, outlines.Widths[gid])
			// o2.Names = append(o2.Names, outlines.Names[gid])
		}
		subsetInfo.Outlines = o2

		encoding := sfntcmap.Format4{}
		for gid, c := range s.enc {
			encoding[uint16(c)] = newGid[gid]
		}
		subsetInfo.CMap = encoding
	}

	fontName := pdf.Name(subsetTag + "+" + subsetInfo.PostscriptName())

	q := 1000 / float64(subsetInfo.UnitsPerEm)

	var Widths pdf.Array
	pos := 0
	for i := firstCharCode; i <= lastCharCode; i++ {
		var width pdf.Integer
		if i == mapping[pos].CharCode {
			gid := mapping[pos].GID
			width = pdf.Integer(math.Round(s.widths[gid].AsFloat(q)))
			pos++
		}

		Widths = append(Widths, width)
	}

	FontDescriptorRef := w.Alloc()
	WidthsRef := w.Alloc()
	FontFileRef := w.Alloc()
	ToUnicodeRef := w.Alloc()

	Font := pdf.Dict{ // See section 9.6.2.1 of PDF 32000-1:2008.
		"Type":           pdf.Name("Font"),
		"BaseFont":       fontName,
		"FirstChar":      pdf.Integer(firstCharCode),
		"LastChar":       pdf.Integer(lastCharCode),
		"FontDescriptor": FontDescriptorRef,
		"Widths":         WidthsRef,
		"ToUnicode":      ToUnicodeRef,
	}

	rect := subsetInfo.BBox()
	fontBBox := &pdf.Rectangle{
		LLx: rect.LLx.AsFloat(q),
		LLy: rect.LLy.AsFloat(q),
		URx: rect.URx.AsFloat(q),
		URy: rect.URy.AsFloat(q),
	}

	FontDescriptor := pdf.Dict{ // See section 9.8.1 of PDF 32000-1:2008.
		"Type":        pdf.Name("FontDescriptor"),
		"FontName":    fontName,
		"Flags":       pdf.Integer(font.MakeFlags(subsetInfo, true)), // TODO(voss)
		"FontBBox":    fontBBox,
		"ItalicAngle": pdf.Number(subsetInfo.ItalicAngle),
		"Ascent":      pdf.Integer(math.Round(subsetInfo.Ascent.AsFloat(q))),
		"Descent":     pdf.Integer(math.Round(subsetInfo.Descent.AsFloat(q))),
		"CapHeight":   pdf.Integer(math.Round(subsetInfo.CapHeight.AsFloat(q))),
		"StemV":       pdf.Integer(70), // information not available in sfnt files
	}

	switch outlines := subsetInfo.Outlines.(type) {
	case *cff.Outlines:
		Font["Subtype"] = pdf.Name("Type1")
		FontDescriptor["FontFile3"] = FontFileRef

		_, err := w.WriteCompressed(
			[]*pdf.Reference{s.FontRef, FontDescriptorRef, WidthsRef},
			Font, FontDescriptor, Widths)
		if err != nil {
			return err
		}

		// Write the font file itself.
		// See section 9.9 of PDF 32000-1:2008 for details.
		fontFileDict := pdf.Dict{
			"Subtype": pdf.Name("Type1C"),
		}
		fontFileStream, _, err := w.OpenStream(fontFileDict, FontFileRef,
			&pdf.FilterInfo{Name: "FlateDecode"})
		if err != nil {
			return err
		}
		fontFile := cff.Font{
			FontInfo: subsetInfo.GetFontInfo(),
			Outlines: outlines,
		}
		err = fontFile.Encode(fontFileStream)
		if err != nil {
			return err
		}
		err = fontFileStream.Close()
		if err != nil {
			return err
		}

	case *glyf.Outlines:
		Font["Subtype"] = pdf.Name("TrueType")
		FontDescriptor["FontFile2"] = FontFileRef

		_, err := w.WriteCompressed(
			[]*pdf.Reference{s.FontRef, FontDescriptorRef, WidthsRef},
			Font, FontDescriptor, Widths)
		if err != nil {
			return err
		}

		// Write the font file itself.
		// See section 9.9 of PDF 32000-1:2008 for details.
		size := w.NewPlaceholder(10)
		fontFileDict := pdf.Dict{
			"Length1": size,
		}
		compress := &pdf.FilterInfo{Name: pdf.Name("LZWDecode")}
		if w.Version >= pdf.V1_2 {
			compress = &pdf.FilterInfo{Name: pdf.Name("FlateDecode")}
		}
		fontFileStream, _, err := w.OpenStream(fontFileDict, FontFileRef, compress)
		if err != nil {
			return err
		}
		n, err := subsetInfo.PDFEmbedTrueType(fontFileStream)
		if err != nil {
			return err
		}
		err = size.Set(pdf.Integer(n))
		if err != nil {
			return err
		}
		err = fontFileStream.Close()
		if err != nil {
			return err
		}
	}

	var cc2text []font.SimpleMapping
	for gid, text := range s.text {
		charCode := s.enc[gid]
		cc2text = append(cc2text, font.SimpleMapping{CharCode: charCode, Text: text})
	}
	err := font.WriteToUnicodeSimple(w, subsetTag, cc2text, ToUnicodeRef)
	if err != nil {
		return err
	}

	return nil
}
