// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2023  Jochen Voss <voss@seehuhn.de>
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

package type1

import (
	"fmt"
	"math"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/postscript/funit"
	"seehuhn.de/go/postscript/type1"
	"seehuhn.de/go/postscript/type1/names"

	"seehuhn.de/go/sfnt/glyph"

	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/subset"
	"seehuhn.de/go/pdf/font/tounicode"
)

type Font struct {
	names    []string
	outlines *type1.Font
	*font.Geometry

	CMap map[rune]glyph.ID
	lig  map[glyph.Pair]glyph.ID
	kern map[glyph.Pair]funit.Int16
}

func New(psFont *type1.Font) (*Font, error) {
	glyphNames := psFont.GlyphList()
	nameGid := make(map[string]glyph.ID, len(glyphNames))
	for i, name := range glyphNames {
		nameGid[name] = glyph.ID(i)
	}

	widths := make([]funit.Int16, len(glyphNames))
	extents := make([]funit.Rect16, len(glyphNames))
	for i, name := range glyphNames {
		gi := psFont.GlyphInfo[name]
		widths[i] = gi.WidthX
		extents[i] = gi.BBox
	}

	geometry := &font.Geometry{
		UnitsPerEm:   psFont.UnitsPerEm,
		Widths:       widths,
		GlyphExtents: extents,

		Ascent:             psFont.Ascent,
		Descent:            psFont.Descent,
		BaseLineSkip:       (psFont.Ascent - psFont.Descent) * 6 / 5, // TODO(voss)
		UnderlinePosition:  psFont.FontInfo.UnderlinePosition,
		UnderlineThickness: psFont.FontInfo.UnderlineThickness,
	}

	cMap := make(map[rune]glyph.ID)
	isDingbats := psFont.FontInfo.FontName == "ZapfDingbats"
	for gid, name := range glyphNames {
		rr := names.ToUnicode(name, isDingbats)
		if len(rr) != 1 {
			continue
		}
		r := rr[0]

		if _, exists := cMap[r]; exists {
			continue
		}
		cMap[r] = glyph.ID(gid)
	}

	lig := make(map[glyph.Pair]glyph.ID)
	for left, name := range glyphNames {
		gi := psFont.GlyphInfo[name]
		for right, repl := range gi.Ligatures {
			lig[glyph.Pair{Left: glyph.ID(left), Right: nameGid[right]}] = nameGid[repl]
		}
	}

	kern := make(map[glyph.Pair]funit.Int16)
	for _, k := range psFont.Kern {
		left, right := nameGid[k.Left], nameGid[k.Right]
		kern[glyph.Pair{Left: left, Right: right}] = k.Adjust
	}

	res := &Font{
		names:    glyphNames,
		outlines: psFont,
		Geometry: geometry,
		CMap:     cMap,
		lig:      lig,
		kern:     kern,
	}
	return res, nil
}

func (f *Font) Embed(w pdf.Putter, resName pdf.Name) (font.Embedded, error) {
	e := &embedded{
		Font: f,
		w:    w,
		Resource: pdf.Resource{
			Ref:  w.Alloc(),
			Name: resName,
		},
		SimpleEncoder: cmap.NewSimpleEncoder(),
	}
	w.AutoClose(e)
	return e, nil
}

func (f *Font) Layout(s string, ptSize float64) glyph.Seq {
	rr := []rune(s)

	gg := make(glyph.Seq, 0, len(rr))
	var prev glyph.ID
	for i, r := range rr {
		gid := f.CMap[r]
		if i > 0 {
			if repl, ok := f.lig[glyph.Pair{Left: prev, Right: gid}]; ok {
				gg[len(gg)-1].Gid = repl
				gg[len(gg)-1].Text = append(gg[len(gg)-1].Text, r)
				prev = repl
				continue
			}
		}
		gg = append(gg, glyph.Info{
			Gid:  gid,
			Text: []rune{r},
		})
		prev = gid
	}

	for i, g := range gg {
		if i > 0 {
			if adj, ok := f.kern[glyph.Pair{Left: prev, Right: g.Gid}]; ok {
				gg[i-1].Advance += adj
			}
		}
		gg[i].Advance = f.Widths[g.Gid]
		prev = g.Gid
	}

	return gg
}

type embedded struct {
	*Font

	w pdf.Putter
	pdf.Resource

	cmap.SimpleEncoder
	closed bool
}

func (e *embedded) Close() error {
	if e.closed {
		return nil
	}
	e.closed = true

	if e.SimpleEncoder.Overflow() {
		return fmt.Errorf("too many distinct glyphs used in font %q (%s)",
			e.Name, e.outlines.FontInfo.FontName)
	}
	e.SimpleEncoder = cmap.NewFrozenSimpleEncoder(e.SimpleEncoder)

	encodingGid := e.SimpleEncoder.Encoding()
	encoding := make([]string, 256)
	for i, gid := range encodingGid {
		encoding[i] = e.names[gid]
	}

	psFont := e.outlines
	var psSubset *type1.Font
	var subsetTag string
	if psFont.Outlines != nil {
		psSubset = &type1.Font{}
		*psSubset = *psFont
		psSubset.Outlines = make(map[string]*type1.Glyph)
		psSubset.GlyphInfo = make(map[string]*type1.GlyphInfo)

		if _, ok := psFont.Outlines[".notdef"]; ok {
			psSubset.Outlines[".notdef"] = psFont.Outlines[".notdef"]
			psSubset.GlyphInfo[".notdef"] = psFont.GlyphInfo[".notdef"]
		}
		for _, name := range encoding {
			if _, ok := psFont.Outlines[name]; ok {
				psSubset.Outlines[name] = psFont.Outlines[name]
				psSubset.GlyphInfo[name] = psFont.GlyphInfo[name]
			}
		}
		psSubset.Encoding = encoding

		var ss []subset.Glyph
		for origGid, name := range e.names {
			if _, ok := psSubset.Outlines[name]; ok {
				ss = append(ss, subset.Glyph{
					OrigGID: glyph.ID(origGid),
					CID:     type1.CID(len(ss)),
				})
			}
		}
		subsetTag = subset.Tag(ss, psFont.NumGlyphs())
	} else {
		psSubset = psFont
	}

	// TODO(voss): generated a ToUnicode map, if needed.

	t1 := &EmbedInfo{
		Font:      psSubset,
		SubsetTag: subsetTag,
		Encoding:  encoding,
		ResName:   e.Name,
	}
	return t1.Embed(e.w, e.Ref)
}

// EmbedInfo holds all the information needed to embed a Type 1 font
// into a PDF file.
type EmbedInfo struct {
	// Font is the (subsetted as needed) font to embed.
	Font *type1.Font

	// SubsetTag should be a unique tag for the font subset,
	// or the empty string if this is the full font.
	SubsetTag string

	// Encoding (a slice of length 256) is the encoding vector used by the client.
	// This is used to determine the `Encoding` entry of the PDF font dictionary.
	Encoding []string

	// ResName is the resource name for the font.
	// This is only used for PDF version 1.0.
	ResName pdf.Name

	IsSerif    bool
	IsScript   bool
	IsAllCap   bool
	IsSmallCap bool

	// ToUnicode (optional) is a map from character codes to unicode strings.
	ToUnicode map[charcode.CharCode][]rune
}

func (info *EmbedInfo) Embed(w pdf.Putter, fontDictRef pdf.Reference) error {
	useBuiltin := w.GetMeta().Version < pdf.V2_0 && IsBuiltin(info.Font)

	fontName := info.Font.FontInfo.FontName
	if info.SubsetTag != "" {
		fontName = info.SubsetTag + "+" + fontName
	}

	var toUnicodeRef pdf.Reference
	var fontFileRef pdf.Reference

	// See section 9.6.2.1 of PDF 32000-1:2008.
	fontDict := pdf.Dict{
		"Type":     pdf.Name("Font"),
		"Subtype":  pdf.Name("Type1"),
		"BaseFont": pdf.Name(fontName),
	}
	if w.GetMeta().Version == pdf.V1_0 {
		fontDict["Name"] = info.ResName
	}
	if enc := font.DescribeEncodingType1(info.Encoding, info.Font.Encoding); enc != nil {
		fontDict["Encoding"] = enc
	}
	if info.ToUnicode != nil {
		toUnicodeRef = w.Alloc()
		fontDict["ToUnicode"] = toUnicodeRef
	}
	compressedRefs := []pdf.Reference{fontDictRef}
	compressedObjects := []pdf.Object{fontDict}

	if !useBuiltin {
		psFont := info.Font

		widthsRef := w.Alloc()
		ww := make([]funit.Int16, 256)
		for i := range ww {
			ww[i] = psFont.GlyphInfo[info.Encoding[i]].WidthX
		}
		widthsInfo := font.CompressWidths(ww, psFont.UnitsPerEm)
		fontDict["FirstChar"] = widthsInfo.FirstChar
		fontDict["LastChar"] = widthsInfo.LastChar
		fontDict["Widths"] = widthsRef
		compressedRefs = append(compressedRefs, widthsRef)
		compressedObjects = append(compressedObjects, widthsInfo.Widths)

		FontDescriptorRef := w.Alloc()
		fontDict["FontDescriptor"] = FontDescriptorRef

		q := 1000 / float64(psFont.UnitsPerEm)
		bbox := psFont.BBox()
		fontBBox := &pdf.Rectangle{
			LLx: bbox.LLx.AsFloat(q),
			LLy: bbox.LLy.AsFloat(q),
			URx: bbox.URx.AsFloat(q),
			URy: bbox.URy.AsFloat(q),
		}

		// TODO(voss): correctly set the isSymbolic flag?
		isSymbolic := true

		fd := &font.Descriptor{
			FontName:     fontName,
			IsFixedPitch: psFont.FontInfo.IsFixedPitch,
			IsSerif:      info.IsSerif,
			IsSymbolic:   isSymbolic,
			IsScript:     info.IsScript,
			IsItalic:     psFont.FontInfo.ItalicAngle != 0,
			IsAllCap:     info.IsAllCap,
			IsSmallCap:   info.IsSmallCap,
			ForceBold:    psFont.Private.ForceBold,
			FontBBox:     fontBBox,
			ItalicAngle:  psFont.FontInfo.ItalicAngle,
			Ascent:       psFont.Ascent.AsFloat(q),
			Descent:      psFont.Descent.AsFloat(q),
			CapHeight:    psFont.CapHeight.AsFloat(q),
			// XHeight:      psFont.XHeight.AsFloat(q),
			StemV:        psFont.Private.StdVW * q,
			MissingWidth: widthsInfo.MissingWidth,
		}
		fontDescriptor := fd.AsDict()
		if psFont.Outlines != nil {
			fontFileRef = w.Alloc()
			fontDescriptor["FontFile"] = fontFileRef
		}
		compressedRefs = append(compressedRefs, FontDescriptorRef)
		compressedObjects = append(compressedObjects, fontDescriptor)
	}

	err := w.WriteCompressed(compressedRefs, compressedObjects...)
	if err != nil {
		return pdf.Wrap(err, "Type 1 font dicts")
	}

	if fontFileRef != 0 {
		// See section 9.9 of PDF 32000-1:2008.
		length1 := pdf.NewPlaceholder(w, 10)
		length2 := pdf.NewPlaceholder(w, 10)
		fontFileDict := pdf.Dict{
			"Length1": length1,
			"Length2": length2,
			"Length3": pdf.Integer(0),
		}
		fontFileStream, err := w.OpenStream(fontFileRef, fontFileDict, pdf.FilterCompress{})
		if err != nil {
			return err
		}
		l1, l2, err := info.Font.WritePDF(fontFileStream)
		if err != nil {
			return err
		}
		length1.Set(pdf.Integer(l1))
		length2.Set(pdf.Integer(l2))
		err = fontFileStream.Close()
		if err != nil {
			return err
		}
	}

	if toUnicodeRef != 0 {
		err = tounicode.Embed(w, toUnicodeRef, charcode.Simple, info.ToUnicode)
		if err != nil {
			return err
		}
	}

	return nil
}

func Extract(r pdf.Getter, dicts *font.Dicts) (*EmbedInfo, error) {
	if dicts.Type != font.Type1 && dicts.Type != font.Builtin {
		return nil, fmt.Errorf("expected %q or %q, got %q",
			font.Type1, font.Builtin, dicts.Type)
	}

	res := &EmbedInfo{}

	if dicts.FontProgram != nil {
		stm, err := pdf.DecodeStream(r, dicts.FontProgram, 0)
		if err != nil {
			return nil, err
		}
		t1, err := type1.Read(stm)
		if err != nil {
			return nil, err
		}

		unitsPerEm := uint16(math.Round(1 / t1.FontInfo.FontMatrix[0]))
		t1.UnitsPerEm = unitsPerEm

		q := 1000 * t1.FontInfo.FontMatrix[0]

		ascent := dicts.FontDescriptor.Ascent
		t1.Ascent = funit.Int16(math.Round(float64(ascent) / q))
		descent := dicts.FontDescriptor.Descent
		t1.Descent = funit.Int16(math.Round(float64(descent) / q))
		capHeight := dicts.FontDescriptor.CapHeight
		t1.CapHeight = funit.Int16(math.Round(float64(capHeight) / q))
		xHeight := dicts.FontDescriptor.XHeight // optional
		t1.XHeight = funit.Int16(math.Round(float64(xHeight) / q))

		res.Font = t1
	}

	baseFont, _ := pdf.GetName(r, dicts.FontDict["BaseFont"])
	if m := subset.TagRegexp.FindStringSubmatch(string(baseFont)); m != nil {
		res.SubsetTag = m[1]
	}

	if res.Font != nil {
		encoding, err := font.UndescribeEncodingType1(
			r, dicts.FontDict["Encoding"], res.Font.Encoding)
		if err == nil {
			res.Encoding = encoding
		}
	} else if t1, err := Builtin(baseFont).PSFont(); err == nil {
		res.Font = t1
		encoding, err := font.UndescribeEncodingType1(
			r, dicts.FontDict["Encoding"], t1.Encoding)
		if err == nil {
			res.Encoding = encoding
		}
	}

	if info, _ := tounicode.Extract(r, dicts.FontDict["ToUnicode"]); info != nil {
		// TODO(voss): check that the codespace ranges are compatible with the cmap.
		res.ToUnicode = info.GetMapping()
	}

	res.ResName, _ = pdf.GetName(r, dicts.FontDict["Name"])

	if dicts.FontDescriptor != nil {
		res.IsSerif = dicts.FontDescriptor.IsSerif
		res.IsScript = dicts.FontDescriptor.IsScript
		res.IsAllCap = dicts.FontDescriptor.IsAllCap
		res.IsSmallCap = dicts.FontDescriptor.IsSmallCap
	}

	return res, nil
}