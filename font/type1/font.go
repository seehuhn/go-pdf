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

	"seehuhn.de/go/postscript/afm"
	"seehuhn.de/go/postscript/funit"
	"seehuhn.de/go/postscript/type1"
	"seehuhn.de/go/postscript/type1/names"

	"seehuhn.de/go/sfnt/glyph"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/encoding"
	"seehuhn.de/go/pdf/font/pdfenc"
	"seehuhn.de/go/pdf/font/subset"
	"seehuhn.de/go/pdf/graphics"
)

// Font is a Type 1 font.
type Font struct {
	ps         *type1.Font
	metrics    *afm.Info
	glyphNames []string

	CMap map[rune]glyph.ID
	lig  map[glyph.Pair]glyph.ID
	kern map[glyph.Pair]funit.Int16

	*font.Geometry
}

// New creates a new Type 1 PDF font from a Type 1 PostScript font.
func New(psFont *type1.Font, metrics *afm.Info) (*Font, error) {
	if psFont == nil && metrics == nil {
		return nil, fmt.Errorf("no font data given")
	}

	var glyphNames []string
	if psFont != nil {
		glyphNames = psFont.GlyphList()
	} else {
		glyphNames = metrics.GlyphList()
	}
	nameGid := make(map[string]glyph.ID, len(glyphNames))
	for i, name := range glyphNames {
		nameGid[name] = glyph.ID(i)
	}

	widths := make([]funit.Int16, len(glyphNames))
	extents := make([]funit.Rect16, len(glyphNames))
	geometry := &font.Geometry{
		Widths:       widths,
		GlyphExtents: extents,
	}

	var fontName string
	if psFont != nil {
		fontName = psFont.FontInfo.FontName
		for i, name := range glyphNames {
			g := psFont.Glyphs[name]
			widths[i] = g.WidthX
			extents[i] = g.BBox()
		}
		geometry.UnitsPerEm = uint16(math.Round(1 / psFont.FontMatrix[0]))
		geometry.UnderlinePosition = psFont.FontInfo.UnderlinePosition
		geometry.UnderlineThickness = psFont.FontInfo.UnderlineThickness
	} else {
		fontName = metrics.FontName
		for i, name := range glyphNames {
			gi := metrics.Glyphs[name]
			widths[i] = gi.WidthX
			extents[i] = gi.BBox
		}
		geometry.UnitsPerEm = 1000
		geometry.UnderlinePosition = metrics.UnderlinePosition
		geometry.UnderlineThickness = metrics.UnderlineThickness
	}
	if metrics != nil {
		geometry.Ascent = metrics.Ascent
		geometry.Descent = metrics.Descent
		geometry.BaseLineDistance = (metrics.Ascent - metrics.Descent) * 6 / 5 // TODO(voss)
	}

	cMap := make(map[rune]glyph.ID)
	isDingbats := fontName == "ZapfDingbats"
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
	kern := make(map[glyph.Pair]funit.Int16)
	if metrics != nil {
		for left, name := range glyphNames {
			gi := metrics.Glyphs[name]
			for right, repl := range gi.Ligatures {
				lig[glyph.Pair{Left: glyph.ID(left), Right: nameGid[right]}] = nameGid[repl]
			}
		}
		for _, k := range metrics.Kern {
			left, right := nameGid[k.Left], nameGid[k.Right]
			kern[glyph.Pair{Left: left, Right: right}] = k.Adjust
		}
	}

	res := &Font{
		ps:         psFont,
		metrics:    metrics,
		glyphNames: glyphNames,
		Geometry:   geometry,
		CMap:       cMap,
		lig:        lig,
		kern:       kern,
	}
	return res, nil
}

// Embed implements the [font.Font] interface.
func (f *Font) Embed(w pdf.Putter, resName pdf.Name) (font.Layouter, error) {
	res := &embedded{
		Font: f,
		w:    w,
		Res: graphics.Res{
			Ref:     w.Alloc(),
			DefName: resName,
		},
		SimpleEncoder: encoding.NewSimpleEncoder(),
	}
	w.AutoClose(res)
	return res, nil
}

// Layout implements the [font.Font] interface.
func (f *Font) Layout(s string) glyph.Seq {
	rr := []rune(s)

	gg := make(glyph.Seq, 0, len(rr))
	var prev glyph.ID
	for i, r := range rr {
		gid := f.CMap[r]
		if i > 0 {
			if repl, ok := f.lig[glyph.Pair{Left: prev, Right: gid}]; ok {
				gg[len(gg)-1].GID = repl
				gg[len(gg)-1].Text = append(gg[len(gg)-1].Text, r)
				prev = repl
				continue
			}
		}
		gg = append(gg, glyph.Info{
			GID:  gid,
			Text: []rune{r},
		})
		prev = gid
	}

	for i, g := range gg {
		if i > 0 {
			if adj, ok := f.kern[glyph.Pair{Left: prev, Right: g.GID}]; ok {
				gg[i-1].Advance += adj
			}
		}
		gg[i].Advance = f.Widths[g.GID]
		prev = g.GID
	}

	return gg
}

type embedded struct {
	*Font

	w pdf.Putter
	graphics.Res

	*encoding.SimpleEncoder
	closed bool
}

func (f *embedded) GlyphWidth(gid glyph.ID) float64 {
	if f.ps != nil {
		return float64(f.Geometry.Widths[gid]) * f.ps.FontInfo.FontMatrix[0]
	}
	return float64(f.Geometry.Widths[gid]) / float64(f.Geometry.UnitsPerEm)
}

func (f *embedded) ForeachWidth(s pdf.String, yield func(width float64, is_space bool)) {
	for _, c := range s {
		gid := f.Encoding[c]
		yield(f.GlyphWidth(gid), c == ' ')
	}
}

func (f *embedded) CodeAndWidth(s pdf.String, gid glyph.ID, rr []rune) (pdf.String, float64, bool) {
	c := f.GIDToCode(gid, rr)
	return append(s, c), f.GlyphWidth(gid), c == ' '
}

func (f *embedded) Close() error {
	if f.closed {
		return nil
	}
	f.closed = true

	if f.SimpleEncoder.Overflow() {
		return fmt.Errorf("too many distinct glyphs used in font %q (%s)",
			f.DefName, f.ps.FontInfo.FontName)
	}

	encodingGid := f.Encoding
	encoding := make([]string, 256)
	for i, gid := range encodingGid {
		encoding[i] = f.glyphNames[gid]
	}

	var psSubset *type1.Font
	var metricsSubset *afm.Info
	if psFont := f.ps; psFont != nil {
		psSubset = clone(psFont)
		psSubset.Glyphs = make(map[string]*type1.Glyph)

		if _, ok := psFont.Glyphs[".notdef"]; ok {
			psSubset.Glyphs[".notdef"] = psFont.Glyphs[".notdef"]
		}
		for _, name := range encoding {
			if _, ok := psFont.Glyphs[name]; ok {
				psSubset.Glyphs[name] = psFont.Glyphs[name]
			}
		}
		psSubset.Encoding = encoding
	}
	if metrics := f.metrics; metrics != nil {
		metricsSubset = clone(metrics)
		metricsSubset.Glyphs = make(map[string]*afm.GlyphInfo)

		if _, ok := metrics.Glyphs[".notdef"]; ok {
			metricsSubset.Glyphs[".notdef"] = metrics.Glyphs[".notdef"]
		}
		for _, name := range encoding {
			if _, ok := metrics.Glyphs[name]; ok {
				metricsSubset.Glyphs[name] = metrics.Glyphs[name]
			}
		}
		metricsSubset.Encoding = encoding
	}

	var subsetTag string
	if psFont := f.ps; psFont != nil {
		var ss []glyph.ID
		for origGid, name := range f.glyphNames {
			if _, ok := psSubset.Glyphs[name]; ok {
				ss = append(ss, glyph.ID(origGid))
			}
		}
		subsetTag = subset.Tag(ss, psFont.NumGlyphs())
	} else {
		var ss []glyph.ID
		for origGid, name := range f.glyphNames {
			if _, ok := metricsSubset.Glyphs[name]; ok {
				ss = append(ss, glyph.ID(origGid))
			}
		}
		subsetTag = subset.Tag(ss, f.metrics.NumGlyphs())
	}

	// TODO(voss): generated a ToUnicode map, if needed.

	info := &EmbedInfo{
		Font:      psSubset,
		Metrics:   metricsSubset,
		SubsetTag: subsetTag,
		Encoding:  encoding,
		ResName:   f.DefName,
	}
	return info.Embed(f.w, f.Ref)
}

// EmbedInfo holds all the information needed to embed a Type 1 font
// into a PDF file.
type EmbedInfo struct {
	// Font (optional) is the (subsetted as needed) font to embed.
	// This is non-nil, if and only if the font program is embedded.
	// At least one of `Font` and `Metrics` must be non-nil.
	Font *type1.Font

	// Metrics (optional) are the font metrics for the font.
	// At least one of `Font` and `Metrics` must be non-nil.
	Metrics *afm.Info

	// SubsetTag should be a unique tag for the font subset,
	// or the empty string if this is the full font.
	SubsetTag string

	// Encoding (a slice of length 256) is the encoding vector used by the client.
	// When writing a font, this is used to determine the `Encoding` entry of
	// the PDF font dictionary.
	Encoding []string

	// ResName is the resource name for the font.
	// This is only used for PDF version 1.0.
	ResName pdf.Name

	IsSerif  bool
	IsScript bool

	IsAllCap   bool
	IsSmallCap bool

	// ToUnicode (optional) is a map from character codes to unicode strings.
	ToUnicode *cmap.ToUnicode
}

// PostScriptName returns the PostScript name of the font.
func (info *EmbedInfo) PostScriptName() string {
	if info.Font != nil {
		return info.Font.FontInfo.FontName
	}
	return info.Metrics.FontName
}

// IsBuiltin returns true if the font is one of the 14 standard PDF fonts.
func (info *EmbedInfo) IsBuiltin() bool {
	return isBuiltinName[info.PostScriptName()] && info.Font == nil
}

// BuiltinEncoding returns the builtin encoding vector for this font.
func (info *EmbedInfo) BuiltinEncoding() []string {
	if info.Font != nil {
		return info.Font.Encoding
	}
	return info.Metrics.Encoding
}

// Embed implements the [font.Font] interface.
func (info *EmbedInfo) Embed(w pdf.Putter, fontDictRef pdf.Reference) error {
	postScriptName := info.PostScriptName()
	fontName := postScriptName
	if info.SubsetTag != "" && info.Font != nil {
		fontName = info.SubsetTag + "+" + fontName
	}

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
	if enc := encoding.DescribeEncodingType1(info.Encoding, info.BuiltinEncoding()); enc != nil {
		fontDict["Encoding"] = enc
	}
	var toUnicodeRef pdf.Reference
	if info.ToUnicode != nil {
		toUnicodeRef = w.Alloc()
		fontDict["ToUnicode"] = toUnicodeRef
	}

	compressedRefs := []pdf.Reference{fontDictRef}
	compressedObjects := []pdf.Object{fontDict}

	canOmit := pdf.GetVersion(w) < pdf.V2_0 && info.IsBuiltin()

	ww := make([]float64, 256)
	if psFont := info.Font; psFont != nil {
		q := 1000 * psFont.FontInfo.FontMatrix[0]
		for i, name := range info.Encoding {
			ww[i] = float64(psFont.Glyphs[name].WidthX) * q
		}
	} else {
		for i, name := range info.Encoding {
			ww[i] = float64(info.Metrics.Glyphs[name].WidthX)
		}
	}
	if canOmit {
		wwStd := Builtin(postScriptName).StandardWidths(info.Encoding)
		for i, w := range ww {
			if math.Abs(w-wwStd[i]) >= 0.1 {
				canOmit = false
				break
			}
		}
	}
	if !canOmit {
		widthsRef := w.Alloc()
		widthsInfo := font.EncodeWidthsSimple(ww)
		fontDict["FirstChar"] = widthsInfo.FirstChar
		fontDict["LastChar"] = widthsInfo.LastChar
		fontDict["Widths"] = widthsRef
		compressedRefs = append(compressedRefs, widthsRef)
		compressedObjects = append(compressedObjects, widthsInfo.Widths)

		fdRef := w.Alloc()
		fontDict["FontDescriptor"] = fdRef

		fd := &font.Descriptor{
			FontName:     fontName,
			IsSerif:      info.IsSerif,
			IsScript:     info.IsScript,
			IsAllCap:     info.IsAllCap,
			IsSmallCap:   info.IsSmallCap,
			MissingWidth: widthsInfo.MissingWidth,
		}

		if metrics := info.Metrics; metrics != nil {
			fd.IsFixedPitch = metrics.IsFixedPitch
			fd.CapHeight = float64(metrics.CapHeight)
			fd.XHeight = float64(metrics.XHeight)
			fd.Ascent = float64(metrics.Ascent)
			fd.Descent = float64(metrics.Descent)
		}
		if psFont := info.Font; psFont != nil {
			fd.IsFixedPitch = psFont.FontInfo.IsFixedPitch
			fd.ForceBold = psFont.Private.ForceBold
			q := 1000 * psFont.FontInfo.FontMatrix[0]
			fd.StemV = psFont.Private.StdVW * q
			fontFileRef = w.Alloc()
		}

		isSymbolic := false
		var italicAngle float64
		var fontBBox *pdf.Rectangle
		if psFont := info.Font; psFont != nil {
			for name := range psFont.Glyphs {
				if name != ".notdef" && !pdfenc.IsStandardLatin[name] {
					isSymbolic = true
					break
				}
			}
			italicAngle = psFont.FontInfo.ItalicAngle
			bbox := psFont.BBox()
			q := 1000 * psFont.FontInfo.FontMatrix[0]
			fontBBox = &pdf.Rectangle{
				LLx: bbox.LLx.AsFloat(q),
				LLy: bbox.LLy.AsFloat(q),
				URx: bbox.URx.AsFloat(q),
				URy: bbox.URy.AsFloat(q),
			}
		} else {
			metrics := info.Metrics
			for name := range metrics.Glyphs {
				if name != ".notdef" && !pdfenc.IsStandardLatin[name] {
					isSymbolic = true
					break
				}
			}
			italicAngle = metrics.ItalicAngle
			// TODO(voss): fontBBox
		}
		fd.IsSymbolic = isSymbolic
		fd.IsItalic = italicAngle != 0
		fd.ItalicAngle = italicAngle
		fd.FontBBox = fontBBox

		fontDescriptor := fd.AsDict()
		if fontFileRef != 0 {
			fontDescriptor["FontFile"] = fontFileRef
		}
		compressedObjects = append(compressedObjects, fontDescriptor)
		compressedRefs = append(compressedRefs, fdRef)
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
		err = length1.Set(pdf.Integer(l1))
		if err != nil {
			return err
		}
		err = length2.Set(pdf.Integer(l2))
		if err != nil {
			return err
		}
		err = fontFileStream.Close()
		if err != nil {
			return err
		}
	}

	if toUnicodeRef != 0 {
		err = info.ToUnicode.Embed(w, toUnicodeRef)
		if err != nil {
			return err
		}
	}

	return nil
}

// Extract extracts information about a Type 1 font from a PDF file.
func Extract(r pdf.Getter, dicts *font.Dicts) (*EmbedInfo, error) {
	if dicts.Type != font.Type1 {
		return nil, fmt.Errorf("expected %q, got %q", font.Type1, dicts.Type)
	}

	res := &EmbedInfo{}

	var psFont *type1.Font
	if dicts.FontProgram != nil {
		stm, err := pdf.DecodeStream(r, dicts.FontProgram, 0)
		if err != nil {
			return nil, err
		}
		psFont, err = type1.Read(stm)
		if err != nil {
			return nil, err
		}
	}
	res.Font = psFont

	fontName, _ := pdf.GetName(r, dicts.FontDict["BaseFont"])
	if m := subset.TagRegexp.FindStringSubmatch(string(fontName)); m != nil {
		res.SubsetTag = m[1]
		fontName = pdf.Name(m[2])
	}

	var metrics *afm.Info
	if isBuiltinName[string(fontName)] {
		afm, err := Builtin(fontName).AFM()
		if err != nil {
			panic(err) // should never happen
		}
		afm.FontName = string(fontName)
		metrics = afm
	} else {
		metrics = &afm.Info{}
	}
	if dicts.FontDescriptor != nil {
		metrics.Ascent = funit.Int16(math.Round(dicts.FontDescriptor.Ascent))
		metrics.Descent = funit.Int16(math.Round(dicts.FontDescriptor.Descent))
		metrics.CapHeight = funit.Int16(math.Round(dicts.FontDescriptor.CapHeight))
		metrics.XHeight = funit.Int16(math.Round(dicts.FontDescriptor.XHeight))
	}
	res.Metrics = metrics

	if psFont != nil {
		encoding, err := encoding.UndescribeEncodingType1(
			r, dicts.FontDict["Encoding"], psFont.Encoding)
		if err == nil {
			res.Encoding = encoding
		}
	} else {
		encoding, err := encoding.UndescribeEncodingType1(
			r, dicts.FontDict["Encoding"], metrics.Encoding)
		if err == nil {
			res.Encoding = encoding
		}
	}

	if info, _ := cmap.ExtractToUnicode(r, dicts.FontDict["ToUnicode"], charcode.Simple); info != nil {
		res.ToUnicode = info
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

func clone[T any](x *T) *T {
	y := *x
	return &y
}
