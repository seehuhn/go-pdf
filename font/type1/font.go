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
	"slices"

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
	"seehuhn.de/go/pdf/font/widths"
)

// New creates a new Type 1 PDF font from a Type 1 PostScript font.
//
// At least one of `psFont` and `metrics` must be non-nil.
// The font program is embedded, if and only if `psFont` is non-nil.
// If metrics is non-nil, information about kerning and ligatures is extracted,
// and the corresponding fields in the PDF font descriptor are filled.
func New(psFont *type1.Font, metrics *afm.Metrics) (font.Embedder, error) {
	if psFont == nil && metrics == nil {
		return nil, fmt.Errorf("no font data given")
	}

	var glyphNames []string
	if psFont != nil {
		glyphNames = psFont.GlyphList()
	} else {
		glyphNames = metrics.GlyphList()
	}

	res := &embedder{
		psFont:     psFont,
		metrics:    metrics,
		glyphNames: glyphNames,
	}
	return res, nil
}

type embedder struct {
	psFont     *type1.Font
	metrics    *afm.Metrics
	glyphNames []string
}

func (f *embedder) Embed(w pdf.Putter, opt *font.Options) (font.Layouter, error) {
	if opt == nil {
		opt = &font.Options{}
	}

	if opt.Composite {
		return nil, fmt.Errorf("Type 1 fonts do not support composite embedding")
	}

	psFont := f.psFont
	metrics := f.metrics
	glyphNames := f.glyphNames

	geometry := &font.Geometry{}
	widths := make([]float64, len(glyphNames))
	extents := make([]pdf.Rectangle, len(glyphNames))
	var fontName string
	if psFont != nil {
		fontName = psFont.FontInfo.FontName
		for i, name := range glyphNames {
			g := psFont.Glyphs[name]
			widths[i] = float64(g.WidthX) * psFont.FontMatrix[0]
			extents[i] = glyphBoxtoPDF(g.BBox(), psFont.FontMatrix[:])
		}
		geometry.UnderlinePosition = float64(psFont.FontInfo.UnderlinePosition) * psFont.FontMatrix[3]
		geometry.UnderlineThickness = float64(psFont.FontInfo.UnderlineThickness) * psFont.FontMatrix[3]
	} else {
		fontName = metrics.FontName
		for i, name := range glyphNames {
			gi := metrics.Glyphs[name]
			widths[i] = gi.WidthX / 1000
			extents[i] = pdf.Rectangle{
				LLx: float64(gi.BBox.LLx) / 1000,
				LLy: float64(gi.BBox.LLy) / 1000,
				URx: float64(gi.BBox.URx) / 1000,
				URy: float64(gi.BBox.URy) / 1000,
			}
		}
		geometry.UnderlinePosition = metrics.UnderlinePosition / 1000
		geometry.UnderlineThickness = metrics.UnderlineThickness / 1000
	}
	if metrics != nil {
		geometry.Ascent = metrics.Ascent / 1000
		geometry.Descent = metrics.Descent / 1000
	}
	geometry.Widths = widths
	geometry.GlyphExtents = extents

	cmap := make(map[rune]glyph.ID)
	isDingbats := fontName == "ZapfDingbats"
	for gid, name := range glyphNames {
		rr := names.ToUnicode(name, isDingbats)
		if len(rr) != 1 {
			continue
		}
		r := rr[0]

		if _, exists := cmap[r]; exists {
			continue
		}
		cmap[r] = glyph.ID(gid)
	}

	nameGid := make(map[string]glyph.ID, len(glyphNames))
	for i, name := range glyphNames {
		nameGid[name] = glyph.ID(i)
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

	res := &embeddedSimple{
		w: w,
		Res: pdf.Res{
			Ref:     w.Alloc(),
			DefName: opt.ResName,
		},
		Geometry: geometry,

		psFont:     psFont,
		metrics:    metrics,
		glyphNames: glyphNames,

		cmap: cmap,
		lig:  lig,
		kern: kern,

		SimpleEncoder: encoding.NewSimpleEncoder(),
	}
	w.AutoClose(res)
	return res, nil
}

func glyphBoxtoPDF(b funit.Rect16, fMat []float64) pdf.Rectangle {
	bPDF := pdf.Rectangle{
		LLx: math.Inf(+1),
		LLy: math.Inf(+1),
		URx: math.Inf(-1),
		URy: math.Inf(-1),
	}
	corners := []struct{ x, y funit.Int16 }{
		{b.LLx, b.LLy},
		{b.LLx, b.URy},
		{b.URx, b.LLy},
		{b.URx, b.URy},
	}
	for _, c := range corners {
		xf := float64(c.x)
		yf := float64(c.y)
		x, y := fMat[0]*xf+fMat[2]*yf+fMat[4], fMat[1]*xf+fMat[3]*yf+fMat[5]
		bPDF.LLx = min(bPDF.LLx, x)
		bPDF.LLy = min(bPDF.LLy, y)
		bPDF.URx = max(bPDF.URx, x)
		bPDF.URy = max(bPDF.URy, y)
	}
	return bPDF
}

type embeddedSimple struct {
	w pdf.Putter
	pdf.Res
	*font.Geometry

	psFont     *type1.Font
	metrics    *afm.Metrics
	glyphNames []string
	cmap       map[rune]glyph.ID
	lig        map[glyph.Pair]glyph.ID
	kern       map[glyph.Pair]funit.Int16

	*encoding.SimpleEncoder

	closed bool
}

// Layout implements the [font.Layouter] interface.
func (f *embeddedSimple) Layout(ptSize float64, s string) *font.GlyphSeq {
	rr := []rune(s)

	gg := make([]font.Glyph, 0, len(rr))
	var prev glyph.ID
	for i, r := range rr {
		gid := f.cmap[r]
		if i > 0 {
			if repl, ok := f.lig[glyph.Pair{Left: prev, Right: gid}]; ok {
				gg[len(gg)-1].GID = repl
				gg[len(gg)-1].Text = append(gg[len(gg)-1].Text, r)
				prev = repl
				continue
			}
		}
		gg = append(gg, font.Glyph{
			GID:     gid,
			Text:    []rune{r},
			Advance: f.Widths[gid] * ptSize,
		})
		prev = gid
	}

	for i, g := range gg {
		if i > 0 {
			if adj, ok := f.kern[glyph.Pair{Left: prev, Right: g.GID}]; ok {
				gg[i-1].Advance += float64(adj) * ptSize / 1000
			}
		}
		prev = g.GID
	}

	res := &font.GlyphSeq{
		Seq: gg,
	}
	return res
}

func (f *embeddedSimple) GlyphWidth(gid glyph.ID) float64 {
	return f.Geometry.Widths[gid]
}

func (f *embeddedSimple) ForeachWidth(s pdf.String, yield func(width float64, is_space bool)) {
	for _, c := range s {
		gid := f.Encoding[c]
		yield(f.GlyphWidth(gid), c == ' ')
	}
}

func (f *embeddedSimple) CodeAndWidth(s pdf.String, gid glyph.ID, rr []rune) (pdf.String, float64, bool) {
	c := f.GIDToCode(gid, rr)
	return append(s, c), f.GlyphWidth(gid), c == ' '
}

func (f *embeddedSimple) Close() error {
	if f.closed {
		return nil
	}
	f.closed = true

	if f.SimpleEncoder.Overflow() {
		var fontName string
		if f.psFont != nil {
			fontName = f.psFont.FontInfo.FontName
		} else {
			fontName = f.metrics.FontName
		}
		return fmt.Errorf("too many distinct glyphs used in font %q (%s)",
			f.DefName, fontName)
	}

	encoding := make([]string, 256)
	for c, gid := range f.Encoding {
		name := f.glyphNames[gid]
		if name == ".notdef" && f.CodeIsUsed(byte(c)) {
			name = notdefForce
		}
		encoding[c] = name
	}

	var subsetTag string
	psSubset := f.psFont
	metricsSubset := f.metrics
	if psFont := f.psFont; psFont != nil {
		// only subset the font, if the font is embedded

		subsetEncoding := encoding
		needFixup := false
		for _, name := range subsetEncoding {
			if name == notdefForce {
				needFixup = true
				break
			}
		}
		if needFixup {
			subsetEncoding = slices.Clone(subsetEncoding)
			for c, name := range subsetEncoding {
				if name == notdefForce {
					subsetEncoding[c] = ".notdef"
				}
			}
		}

		psSubset = clone(psFont)
		psSubset.Glyphs = make(map[string]*type1.Glyph)

		if _, ok := psFont.Glyphs[".notdef"]; ok {
			psSubset.Glyphs[".notdef"] = psFont.Glyphs[".notdef"]
		}
		for _, name := range subsetEncoding {
			if _, ok := psFont.Glyphs[name]; ok {
				psSubset.Glyphs[name] = psFont.Glyphs[name]
			}
		}
		psSubset.Encoding = subsetEncoding

		if metrics := f.metrics; metrics != nil {
			metricsSubset = clone(metrics)
			metricsSubset.Glyphs = make(map[string]*afm.GlyphInfo)

			if _, ok := metrics.Glyphs[".notdef"]; ok {
				metricsSubset.Glyphs[".notdef"] = metrics.Glyphs[".notdef"]
			}
			for _, name := range subsetEncoding {
				if _, ok := metrics.Glyphs[name]; ok {
					metricsSubset.Glyphs[name] = metrics.Glyphs[name]
				}
			}
			metricsSubset.Encoding = subsetEncoding
		}

		var ss []glyph.ID
		for origGid, name := range f.glyphNames {
			if _, ok := psSubset.Glyphs[name]; ok {
				ss = append(ss, glyph.ID(origGid))
			}
		}
		subsetTag = subset.Tag(ss, psFont.NumGlyphs())
	}

	var fontName string
	if psSubset != nil {
		fontName = psSubset.FontInfo.FontName
	} else {
		fontName = metricsSubset.FontName
	}

	var toUnicode *cmap.ToUnicode
	toUniMap := f.ToUnicodeNew()
	for c, name := range encoding {
		got := names.ToUnicode(name, fontName == "ZapfDingbats")
		want := toUniMap[string(rune(c))]
		if !slices.Equal(got, want) {
			toUnicode = cmap.NewToUnicodeNew(charcode.Simple, toUniMap)
			break
		}
	}

	info := &EmbedInfo{
		Font:      psSubset,
		Metrics:   metricsSubset,
		SubsetTag: subsetTag,
		Encoding:  encoding,
		ResName:   f.DefName,
		ToUnicode: toUnicode,
	}
	return info.Embed(f.w, f.Ref.(pdf.Reference))
}

// EmbedInfo is the information needed to embed a Type 1 font.
type EmbedInfo struct {
	// Font (optional) is the (subsetted as needed) font to embed.
	// This is non-nil, if and only if the font program is embedded.
	// At least one of `Font` and `Metrics` must be non-nil.
	Font *type1.Font

	// Metrics (optional) are the font metrics for the font.
	// At least one of `Font` and `Metrics` must be non-nil.
	Metrics *afm.Metrics

	// SubsetTag should be a unique tag for the font subset,
	// or the empty string if this is the full font.
	SubsetTag string

	// Encoding (a slice of length 256) is the encoding vector used by the client.
	// When embedding a font, this is used to determine the `Encoding` entry in
	// the PDF font dictionary.
	Encoding []string

	// ResName is the resource name for the font (only used for PDF-1.0).
	ResName pdf.Name

	IsSerif  bool
	IsScript bool

	IsAllCap   bool
	IsSmallCap bool

	// ToUnicode (optional) is a map from character codes to unicode strings.
	ToUnicode *cmap.ToUnicode
}

// Extract extracts information about a Type 1 font from a PDF file.
//
// The `Font` field in the result is only filled if the font program
// is included in the file.  `Metrics` is always present, and contains
// all information available in the PDF file.
func Extract(r pdf.Getter, dicts *font.Dicts) (*EmbedInfo, error) {
	if err := dicts.Type.MustBe(font.Type1); err != nil {
		return nil, err
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

	var metrics *afm.Metrics
	if psFont == nil && isBuiltinName[string(fontName)] {
		afm, err := Builtin(fontName).AFM()
		if err != nil {
			panic(err) // should never happen
		}
		afm.FontName = string(fontName)
		metrics = afm
	} else {
		metrics = &afm.Metrics{
			Glyphs: map[string]*afm.GlyphInfo{
				".notdef": {},
			},
			FontName: string(fontName),
		}
	}
	if dicts.FontDescriptor != nil {
		metrics.Ascent = dicts.FontDescriptor.Ascent
		metrics.Descent = dicts.FontDescriptor.Descent
		metrics.CapHeight = dicts.FontDescriptor.CapHeight
		metrics.XHeight = dicts.FontDescriptor.XHeight
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

	res.ResName, _ = pdf.GetName(r, dicts.FontDict["Name"])

	if dicts.FontDescriptor != nil {
		res.IsSerif = dicts.FontDescriptor.IsSerif
		res.IsScript = dicts.FontDescriptor.IsScript
		res.IsAllCap = dicts.FontDescriptor.IsAllCap
		res.IsSmallCap = dicts.FontDescriptor.IsSmallCap
	}

	if info, _ := cmap.ExtractToUnicode(r, dicts.FontDict["ToUnicode"], charcode.Simple); info != nil {
		res.ToUnicode = info
	}

	return res, nil
}

// Embed implements the [font.Font] interface.
func (info *EmbedInfo) Embed(w pdf.Putter, fontDictRef pdf.Reference) error {
	postScriptName := info.PostScriptName()
	fontName := postScriptName
	if info.SubsetTag != "" {
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

	canOmit := pdf.GetVersion(w) < pdf.V2_0 && info.IsStandard()

	ww := info.GetWidths()
	for i := range ww {
		ww[i] *= 1000
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
		widthsInfo := widths.EncodeSimple(ww)
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

// PostScriptName returns the PostScript name of the font.
func (info *EmbedInfo) PostScriptName() string {
	if info.Font != nil {
		return info.Font.FontInfo.FontName
	}
	return info.Metrics.FontName
}

// IsStandard returns true if the font is one of the 14 standard PDF fonts.
func (info *EmbedInfo) IsStandard() bool {
	return isBuiltinName[info.PostScriptName()] && info.Font == nil
}

// BuiltinEncoding returns the builtin encoding vector for this font.
func (info *EmbedInfo) BuiltinEncoding() []string {
	if info.Font != nil {
		return info.Font.Encoding
	}
	return info.Metrics.Encoding
}

// GetWidths returns the widths of the 256 encoded characters.
// The returned widths are given in PDF text space units.
func (info *EmbedInfo) GetWidths() []float64 {
	ww := make([]float64, 256)
	if psFont := info.Font; psFont != nil {
		q := psFont.FontInfo.FontMatrix[0]
		notdefWidth := float64(psFont.Glyphs[".notdef"].WidthX) * q
		for i, name := range info.Encoding {
			if g, ok := psFont.Glyphs[name]; ok {
				ww[i] = float64(g.WidthX) * q
			} else {
				ww[i] = notdefWidth
			}
		}
	} else {
		notdefWidth := float64(info.Metrics.Glyphs[".notdef"].WidthX) / 1000
		for i, name := range info.Encoding {
			if g, ok := info.Metrics.Glyphs[name]; ok {
				ww[i] = float64(g.WidthX) / 1000
			} else {
				ww[i] = notdefWidth
			}
		}
	}
	return ww
}

// GlyphList returns the list of glyph names, in a standardised order.
// Glyph IDs, where used, are indices into this list.
func (info *EmbedInfo) GlyphList() []string {
	if info.Font != nil {
		return info.Font.GlyphList()
	}
	return info.Metrics.GlyphList()
}

func clone[T any](x *T) *T {
	y := *x
	return &y
}

// NotdefForce is a glyph name which is unlikely to be used by any real-world
// font. We map code points to this glyph name, when the user requests to
// typeset the .notdef glyph.
const notdefForce = ".notdef.force"
