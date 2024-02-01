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

package type3

import (
	"errors"
	"fmt"
	"io"
	"math"
	"sort"

	"golang.org/x/exp/maps"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/encoding"
	"seehuhn.de/go/pdf/font/pdfenc"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/postscript/funit"
	"seehuhn.de/go/postscript/type1/names"
	"seehuhn.de/go/sfnt/glyph"
)

// Font is a PDF Type 3 font.
type Font struct {
	Glyphs     map[string]*Glyph
	FontMatrix [6]float64

	Ascent       funit.Int16
	Descent      funit.Int16
	BaseLineSkip funit.Int16

	ItalicAngle  float64
	IsFixedPitch bool
	IsSerif      bool
	IsScript     bool
	IsAllCap     bool
	IsSmallCap   bool
	ForceBold    bool

	Resources *pdf.Resources

	NumOpen int // -1 if the font is embedded, otherwise the number of glyphs not yet closed
}

// Glyph is a glyph in a type 3 font.
type Glyph struct {
	WidthX funit.Int16
	BBox   funit.Rect16
	Data   []byte
}

// New creates a new type 3 font.
// Initally the font does not contain any glyphs.
// Use [Font.AddGlyph] to add glyphs to the font.
// Once the font is embedded the first time, no more glyphs can be added.
func New(unitsPerEm uint16) *Font {
	m := [6]float64{
		1 / float64(unitsPerEm), 0,
		0, 1 / float64(unitsPerEm),
		0, 0,
	}
	res := &Font{
		FontMatrix: m,
		Glyphs:     map[string]*Glyph{},
		Resources:  &pdf.Resources{},
	}

	return res
}

// glyphList returns a list of all glyph names in the font. The list starts
// with the empty string (to avoid allocating GID 0), followed by the glyph
// names in alphabetical order.
func (f *Font) glyphList() []string {
	glyphNames := maps.Keys(f.Glyphs)
	glyphNames = append(glyphNames, "")
	sort.Strings(glyphNames)
	return glyphNames
}

// Embed implements the [font.Font] interface.
func (f *Font) Embed(w pdf.Putter, resName pdf.Name) (font.Layouter, error) {
	if f.NumOpen > 0 {
		return nil, fmt.Errorf("font: %d glyphs not closed", f.NumOpen)
	}
	f.NumOpen = -1

	glyphNames := f.glyphList()
	cmap := map[rune]glyph.ID{}
	for gid, name := range glyphNames {
		if rr := names.ToUnicode(string(name), false); len(rr) == 1 {
			cmap[rr[0]] = glyph.ID(gid)
		}
	}

	res := &embedded{
		Font:       f,
		GlyphNames: glyphNames,
		w:          w,
		Res: graphics.Res{
			Ref:     w.Alloc(),
			DefName: resName,
		},
		CMap:          cmap,
		SimpleEncoder: encoding.NewSimpleEncoder(),
	}
	w.AutoClose(res)
	return res, nil
}

type embedded struct {
	*Font
	GlyphNames []string

	w pdf.Putter
	graphics.Res

	CMap map[rune]glyph.ID
	*encoding.SimpleEncoder
	closed bool
}

// GetGeometry implements the [font.Layouter] interface.
func (f *embedded) GetGeometry() *font.Geometry {
	glyphExtents := make([]pdf.Rectangle, len(f.GlyphNames))
	widths := make([]float64, len(f.GlyphNames))
	for i, name := range f.GlyphNames {
		if i == 0 {
			continue
		}
		glyphExtents[i] = glyphBoxtoPDF(f.Glyphs[name].BBox, f.Font.FontMatrix[:])
		widths[i] = float64(f.Glyphs[name].WidthX) * f.Font.FontMatrix[0]
	}

	res := &font.Geometry{
		Ascent:           float64(f.Ascent) * f.Font.FontMatrix[3],
		Descent:          float64(f.Descent) * f.Font.FontMatrix[3],
		BaseLineDistance: float64(f.BaseLineSkip) * f.Font.FontMatrix[3],
		GlyphExtents:     glyphExtents,
		Widths:           widths,
	}
	return res
}

func glyphBoxtoPDF(b funit.Rect16, M []float64) pdf.Rectangle {
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
		x, y := M[0]*xf+M[2]*yf+M[4], M[1]*xf+M[3]*yf+M[5]
		bPDF.LLx = min(bPDF.LLx, x)
		bPDF.LLy = min(bPDF.LLy, y)
		bPDF.URx = max(bPDF.URx, x)
		bPDF.URy = max(bPDF.URy, y)
	}
	return bPDF
}

// Layout implements the [font.Layouter] interface.
func (f *embedded) Layout(ptSize float64, s string) *font.GlyphSeq {
	rr := []rune(s)

	q := f.Font.FontMatrix[0] * ptSize

	gg := make([]font.Glyph, 0, len(rr))
	for _, r := range rr {
		gid, ok := f.CMap[r]
		if !ok {
			continue
		}
		gg = append(gg, font.Glyph{
			GID:     gid,
			Text:    []rune{r},
			Advance: float64(f.Glyphs[f.GlyphNames[gid]].WidthX) * q,
		})
	}
	res := &font.GlyphSeq{
		Seq: gg,
	}
	return res
}

func (f *embedded) ForeachWidth(s pdf.String, yield func(width float64, is_space bool)) {
	for _, c := range s {
		gid := f.Encoding[c]
		name := f.GlyphNames[gid]
		width := float64(f.Glyphs[name].WidthX) * f.Font.FontMatrix[0]
		yield(width, c == ' ')
	}
}

func (f *embedded) CodeAndWidth(s pdf.String, gid glyph.ID, rr []rune) (pdf.String, float64, bool) {
	name := f.GlyphNames[gid]
	width := float64(f.Glyphs[name].WidthX) * f.Font.FontMatrix[0]
	c := f.GIDToCode(gid, rr)
	return append(s, c), width, c == ' '
}

func (f *embedded) Close() error {
	if f.closed {
		return nil
	}
	f.closed = true

	if f.SimpleEncoder.Overflow() {
		return fmt.Errorf("too many distinct glyphs used in Type 3 font %q",
			f.DefName)
	}

	encodingGid := f.Encoding
	encoding := make([]string, 256)

	subset := make(map[string]*Glyph)
	for i, gid := range encodingGid {
		// Gid 0 maps to the empty glyph name, which is not in the charProcs map.
		if glyph := f.Glyphs[f.GlyphNames[gid]]; glyph != nil {
			name := f.GlyphNames[gid]
			encoding[i] = name
			subset[name] = glyph
		}
	}

	var descriptor *font.Descriptor
	if pdf.IsTagged(f.w) {
		descriptor = &font.Descriptor{
			IsFixedPitch: f.IsFixedPitch,
			IsSerif:      f.IsSerif,
			IsScript:     f.IsScript,
			IsItalic:     f.ItalicAngle != 0,
			IsAllCap:     f.IsAllCap,
			IsSmallCap:   f.IsSmallCap,
			ForceBold:    f.ForceBold,
			ItalicAngle:  f.ItalicAngle,
			StemV:        -1,
		}
		if pdf.GetVersion(f.w) == pdf.V1_0 {
			// required by PDF 2.0 specification errata, if the font dictionary has a Name entry
			// https://pdf-issues.pdfa.org/32000-2-2020/clause09.html#H9.8.1
			descriptor.FontName = string(f.DefName)
		}
	}

	// var toUnicode *tounicode.Info
	// TODO(voss): construct a toUnicode map, when needed

	info := &EmbedInfo{
		FontMatrix: f.Font.FontMatrix,
		Glyphs:     subset,
		Resources:  f.Resources,
		Encoding:   encoding,
		// ToUnicode:  toUnicode,
		ResName: f.DefName,

		ItalicAngle: f.ItalicAngle,

		IsFixedPitch: f.IsFixedPitch,
		IsSerif:      f.IsSerif,
		IsScript:     f.IsScript,
		ForceBold:    f.ForceBold,

		IsAllCap:   f.IsAllCap,
		IsSmallCap: f.IsSmallCap,
	}
	return info.Embed(f.w, f.Ref)
}

// EmbedInfo contains the information needed to embed a type 3 font into a PDF document.
type EmbedInfo struct {
	Glyphs map[string]*Glyph

	FontMatrix [6]float64

	// Encoding (a slice of length 256) is the encoding vector used by the client.
	// This is used to construct the `Encoding` entry of the PDF font dictionary.
	Encoding []string

	// ResName is the resource name for the font.
	// This is only used for PDF version 1.0.
	ResName pdf.Name

	// Resources is the resource dictionary for the font.
	Resources *pdf.Resources

	ItalicAngle float64

	IsFixedPitch bool
	IsSerif      bool
	IsScript     bool
	ForceBold    bool

	IsAllCap   bool
	IsSmallCap bool

	// ToUnicode (optional) is a map from character codes to unicode strings.
	ToUnicode *cmap.ToUnicode
}

// Embed implements the [font.Font] interface.
func (info *EmbedInfo) Embed(w pdf.Putter, fontDictRef pdf.Reference) error {
	if len(info.FontMatrix) != 6 {
		return errors.New("invalid font matrix")
	}
	fontMatrix := make(pdf.Array, len(info.FontMatrix))
	for i, x := range info.FontMatrix {
		fontMatrix[i] = pdf.Number(x)
	}

	charProcs := make(pdf.Dict, len(info.Glyphs))
	for name := range info.Glyphs {
		charProcs[pdf.Name(name)] = w.Alloc()
	}

	differences := pdf.Array{}
	prev := 256
	for code, name := range info.Encoding {
		name := pdf.Name(name)
		if _, exists := charProcs[name]; !exists {
			continue
		}
		if code != prev+1 {
			differences = append(differences, pdf.Integer(code))
		}
		differences = append(differences, name)
		prev = code
	}
	encoding := pdf.Dict{
		"Differences": differences,
	}

	ww := make([]funit.Int16, 256)
	for i := range ww {
		name := info.Encoding[i]
		g := info.Glyphs[name]
		if g != nil {
			ww[i] = g.WidthX
		}
	}
	var firstChar pdf.Integer
	lastChar := pdf.Integer(255)
	for lastChar > 0 && ww[lastChar] == 0 {
		lastChar--
	}
	for firstChar < lastChar && ww[firstChar] == 0 {
		firstChar++
	}
	widths := make(pdf.Array, lastChar-firstChar+1)
	for i := range widths {
		// TODO(voss): These widths shall be interpreted in glyph space as
		// specified by FontMatrix (unlike the widths of a Type 1 font, which
		// are in thousandths of a unit of text space).  If FontMatrix
		// specifies a rotation, only the horizontal component of the
		// transformed width shall be used.
		widths[i] = pdf.Integer(ww[int(firstChar)+i])
	}

	// See section 9.6.4 of PDF 32000-1:2008.
	fontDict := pdf.Dict{
		"Type":       pdf.Name("Font"),
		"Subtype":    pdf.Name("Type3"),
		"FontBBox":   &pdf.Rectangle{}, // [0 0 0 0] is always valid for Type 3 fonts
		"FontMatrix": fontMatrix,
		"CharProcs":  charProcs,
		"Encoding":   encoding,
		"FirstChar":  firstChar,
		"LastChar":   lastChar,
		"Widths":     widths,
	}
	if w.GetMeta().Version == pdf.V1_0 {
		fontDict["Name"] = info.ResName
	}
	if !info.Resources.IsEmpty() {
		resources := pdf.AsDict(info.Resources)
		fontDict["Resources"] = resources
	}
	var toUnicodeRef pdf.Reference
	if info.ToUnicode != nil {
		toUnicodeRef = w.Alloc()
		fontDict["ToUnicode"] = toUnicodeRef
	}

	compressedRefs := []pdf.Reference{fontDictRef}
	compressedObjects := []pdf.Object{fontDict}

	if pdf.IsTagged(w) {
		isSymbolic := true
		for name := range charProcs {
			if !pdfenc.IsStandardLatin[string(name)] {
				isSymbolic = false
				break
			}
		}

		fd := &font.Descriptor{
			IsFixedPitch: info.IsFixedPitch,
			IsSerif:      info.IsSerif,
			IsSymbolic:   isSymbolic,
			IsScript:     info.IsScript,
			IsItalic:     info.ItalicAngle != 0,
			IsAllCap:     info.IsAllCap,
			IsSmallCap:   info.IsSmallCap,
			ForceBold:    info.ForceBold,
			ItalicAngle:  info.ItalicAngle,
			StemV:        -1,
		}
		if name, ok := fontDict["Name"].(pdf.Name); ok {
			fd.FontName = string(name)
		}

		fdRef := w.Alloc()
		fontDict["FontDescriptor"] = fdRef

		fontDescriptor := fd.AsDict()
		compressedObjects = append(compressedObjects, fontDescriptor)
		compressedRefs = append(compressedRefs, fdRef)
	}

	err := w.WriteCompressed(compressedRefs, compressedObjects...)
	if err != nil {
		return pdf.Wrap(err, "Type 3 font dicts")
	}

	for name, g := range info.Glyphs {
		gRef := charProcs[pdf.Name(name)].(pdf.Reference)
		stm, err := w.OpenStream(gRef, nil, pdf.FilterCompress{})
		if err != nil {
			return nil
		}
		_, err = stm.Write(g.Data)
		if err != nil {
			return nil
		}
		err = stm.Close()
		if err != nil {
			return nil
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

// Extract extracts information about a type 3 font from a PDF file.
func Extract(r pdf.Getter, dicts *font.Dicts) (*EmbedInfo, error) {
	if err := dicts.Type.MustBe(font.Type3); err != nil {
		return nil, err
	}

	res := &EmbedInfo{}

	charProcs, err := pdf.GetDict(r, dicts.FontDict["CharProcs"])
	if err != nil {
		return nil, pdf.Wrap(err, "CharProcs")
	}
	glyphs := make(map[string]*Glyph, len(charProcs))
	for name, ref := range charProcs {
		stm, err := pdf.GetStream(r, ref)
		if stm == nil {
			err = errors.New("stream not found")
		}
		if err != nil {
			return nil, pdf.Wrap(err, fmt.Sprintf("CharProcs[%s]", name))
		}
		decoded, err := pdf.DecodeStream(r, stm, 0)
		if err != nil {
			return nil, pdf.Wrap(err, fmt.Sprintf("decoding CharProcs[%s]", name))
		}
		data, err := io.ReadAll(decoded)
		if err != nil {
			return nil, pdf.Wrap(err, fmt.Sprintf("reading CharProcs[%s]", name))
		}
		g := &Glyph{Data: data}
		setGlyphGeometry(g, data)
		glyphs[string(name)] = g
	}
	res.Glyphs = glyphs

	fontMatrix, err := pdf.GetArray(r, dicts.FontDict["FontMatrix"])
	if err != nil {
		return nil, pdf.Wrap(err, "FontMatrix")
	}
	if len(fontMatrix) != 6 {
		return nil, errors.New("invalid font matrix")
	}
	for i, x := range fontMatrix {
		xi, err := pdf.GetNumber(r, x)
		if err != nil {
			return nil, pdf.Wrap(err, fmt.Sprintf("FontMatrix[%d]", i))
		}
		res.FontMatrix[i] = float64(xi)
	}

	encoding, err := pdf.GetDict(r, dicts.FontDict["Encoding"])
	if err != nil {
		return nil, pdf.Wrap(err, "Encoding")
	}
	differences, err := pdf.GetArray(r, encoding["Differences"])
	if err != nil {
		return nil, pdf.Wrap(err, "Encoding.Differences")
	}
	res.Encoding = make([]string, 256)
	code := 0
	for _, obj := range differences {
		obj, err = pdf.Resolve(r, obj)
		if err != nil {
			return nil, err
		}
		switch obj := obj.(type) {
		case pdf.Integer:
			code = int(obj)
		case pdf.Name:
			name := string(obj)
			if _, exists := glyphs[name]; exists && code < 256 {
				res.Encoding[code] = name
			}
			code++
		}
	}

	if info, _ := cmap.ExtractToUnicode(r, dicts.FontDict["ToUnicode"], charcode.Simple); info != nil {
		res.ToUnicode = info
	}

	res.ResName, _ = pdf.GetName(r, dicts.FontDict["Name"])

	resources, err := pdf.GetDict(r, dicts.FontDict["Resources"])
	if err != nil {
		return nil, pdf.Wrap(err, "Resources")
	}
	res.Resources = &pdf.Resources{}
	err = pdf.DecodeDict(r, res.Resources, resources)
	if err != nil {
		return nil, pdf.Wrap(err, "decoding Resources")
	}

	if dicts.FontDescriptor != nil {
		res.ItalicAngle = dicts.FontDescriptor.ItalicAngle
		res.IsFixedPitch = dicts.FontDescriptor.IsFixedPitch
		res.IsSerif = dicts.FontDescriptor.IsSerif
		res.IsScript = dicts.FontDescriptor.IsScript
		res.IsAllCap = dicts.FontDescriptor.IsAllCap
		res.IsSmallCap = dicts.FontDescriptor.IsSmallCap
		res.ForceBold = dicts.FontDescriptor.ForceBold
	}

	return res, nil
}
