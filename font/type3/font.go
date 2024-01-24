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

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/encoding"
	"seehuhn.de/go/pdf/font/pdfenc"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/postscript/funit"
	"seehuhn.de/go/postscript/type1"
	"seehuhn.de/go/sfnt/glyph"
)

// Font is a PDF type 3 font.
type Font struct {
	glyphNames []string
	Glyphs     map[string]*Glyph

	Ascent             funit.Int16
	Descent            funit.Int16
	BaseLineSkip       funit.Int16
	UnderlinePosition  funit.Float64
	UnderlineThickness funit.Float64

	ItalicAngle  float64
	IsFixedPitch bool
	IsSerif      bool
	IsScript     bool
	IsItalic     bool
	IsAllCap     bool
	IsSmallCap   bool
	ForceBold    bool

	FontMatrix [6]float64
	Resources  *pdf.Resources

	CMap map[rune]glyph.ID

	numOpen int
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
		CMap:       map[rune]glyph.ID{},
	}

	// Gid 0 maps to the empty glyph name, because code outside this package
	// uses gid 0 to indicate a missing glyph.  This is not a problem, because
	// we don't accept empty glyph names in [Font.AddGlyph].
	res.glyphNames = append(res.glyphNames, "")

	return res
}

// Embed implements the [font.Font] interface.
func (f *Font) Embed(w pdf.Putter, resName pdf.Name) (font.Layouter, error) {
	if f.numOpen != 0 {
		return nil, fmt.Errorf("font: %d glyphs not closed", f.numOpen)
	}
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

func (f *Font) UnitsPerEm() uint16 {
	return uint16(math.Round(1 / f.FontMatrix[0]))
}

// GetGeometry implements the [font.Font] interface.
func (f *Font) GetGeometry() *font.Geometry {
	glyphNames := f.glyphNames

	glyphExtents := make([]funit.Rect16, len(glyphNames))
	widths := make([]funit.Int16, len(glyphNames))
	for i, name := range glyphNames {
		if i == 0 {
			continue
		}
		glyphExtents[i] = f.Glyphs[name].BBox
		// TODO(voss): is `withds` ordered correctly?
		widths[i] = f.Glyphs[name].WidthX
	}

	res := &font.Geometry{
		UnitsPerEm:         f.UnitsPerEm(),
		Ascent:             f.Ascent,
		Descent:            f.Descent,
		BaseLineDistance:   f.BaseLineSkip,
		UnderlinePosition:  f.UnderlinePosition,
		UnderlineThickness: f.UnderlineThickness,
		GlyphExtents:       glyphExtents,
		Widths:             widths,
	}
	return res
}

// Layout implements the [font.Font] interface.
func (f *Font) Layout(s string) glyph.Seq {
	rr := []rune(s)

	gg := make(glyph.Seq, 0, len(rr))
	for _, r := range rr {
		gid, ok := f.CMap[r]
		if !ok {
			continue
		}
		gg = append(gg, glyph.Info{
			GID:     gid,
			Text:    []rune{r},
			Advance: f.Glyphs[f.glyphNames[gid]].WidthX,
		})
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

func (f *embedded) ForeachWidth(s pdf.String, yield func(width float64, is_space bool)) {
	for _, c := range s {
		gid := f.Encoding[c]
		name := f.glyphNames[gid]
		width := float64(f.Glyphs[name].WidthX) * f.Font.FontMatrix[0]
		yield(width, c == ' ')
	}
}

func (f *embedded) CodeAndWidth(s pdf.String, gid glyph.ID, rr []rune) (pdf.String, float64, bool) {
	name := f.glyphNames[gid]
	width := float64(f.Glyphs[name].WidthX) * f.Font.FontMatrix[0]
	c := f.GIDToCode(gid, rr)
	return append(s, c), width, c == ' '
}

func (f *embedded) CodeToWidth(c byte) float64 {
	gid := f.Encoding[c]
	name := f.glyphNames[gid]
	return float64(f.Glyphs[name].WidthX) * f.Font.FontMatrix[0]
}

func (f *embedded) FontMatrix() []float64 {
	return f.Font.FontMatrix[:]
}

func (e *embedded) Close() error {
	if e.closed {
		return nil
	}
	e.closed = true

	if e.SimpleEncoder.Overflow() {
		return fmt.Errorf("too many distinct glyphs used in Type 3 font %q",
			e.DefName)
	}

	encodingGid := e.Encoding
	encoding := make([]string, 256)

	subset := make(map[string]*Glyph)
	for i, gid := range encodingGid {
		// Gid 0 maps to the empty glyph name, which is not in the charProcs map.
		if glyph := e.Glyphs[e.glyphNames[gid]]; glyph != nil {
			name := e.glyphNames[gid]
			encoding[i] = name
			subset[name] = glyph
		}
	}

	var descriptor *font.Descriptor
	if pdf.IsTagged(e.w) {
		descriptor = &font.Descriptor{
			IsFixedPitch: e.IsFixedPitch,
			IsSerif:      e.IsSerif,
			IsScript:     e.IsScript,
			IsItalic:     e.IsItalic,
			IsAllCap:     e.IsAllCap,
			IsSmallCap:   e.IsSmallCap,
			ForceBold:    e.ForceBold,
			ItalicAngle:  e.ItalicAngle,
			StemV:        -1,
		}
		if pdf.GetVersion(e.w) == pdf.V1_0 {
			// required by PDF 2.0 specification errata, if the font dictionary has a Name entry
			// https://pdf-issues.pdfa.org/32000-2-2020/clause09.html#H9.8.1
			descriptor.FontName = string(e.DefName)
		}
	}

	// var toUnicode *tounicode.Info
	// TODO(voss): construct a toUnicode map, when needed

	info := &EmbedInfo{
		FontMatrix: e.Font.FontMatrix,
		Glyphs:     subset,
		Resources:  e.Resources,
		Encoding:   encoding,
		// ToUnicode:  toUnicode,
		ResName: e.DefName,

		ItalicAngle: e.ItalicAngle,

		IsFixedPitch: e.IsFixedPitch,
		IsSerif:      e.IsSerif,
		IsScript:     e.IsScript,
		ForceBold:    e.ForceBold,

		IsAllCap:   e.IsAllCap,
		IsSmallCap: e.IsSmallCap,
	}
	return info.Embed(e.w, e.Ref)
}

func (e *embedded) GlyphWidth(cid type1.CID) float64 {
	// TODO(voss): is this correct?
	return float64(e.Glyphs[e.glyphNames[cid]].WidthX)
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
