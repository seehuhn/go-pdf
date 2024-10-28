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
	"errors"
	"fmt"
	"math"
	"math/bits"
	"slices"
	"sort"

	"golang.org/x/exp/maps"

	"seehuhn.de/go/postscript/afm"
	"seehuhn.de/go/postscript/type1"
	"seehuhn.de/go/postscript/type1/names"

	"seehuhn.de/go/sfnt"
	"seehuhn.de/go/sfnt/cff"
	"seehuhn.de/go/sfnt/glyph"
	"seehuhn.de/go/sfnt/os2"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/encoding"
	"seehuhn.de/go/pdf/font/pdfenc"
	"seehuhn.de/go/pdf/font/subset"
	"seehuhn.de/go/pdf/font/widths"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/internal/stdmtx"
)

var _ font.Font = (*Type1Font)(nil)

type Type1Font struct {
	Font    *type1.Font
	Metrics *afm.Metrics
}

func (f *Type1Font) PostScriptName() string {
	if f.Font != nil {
		return f.Font.FontName
	}
	if f.Metrics != nil {
		return f.Metrics.FontName
	}
	return ""
}

// GlyphWidthPDF computes the width of a glyph in PDF glyph space units.
// If the glyph does not exist, the width of the .notdef glyph is returned.
func (f *Type1Font) GlyphWidthPDF(glyphName string) float64 {
	if f.Font != nil {
		return f.Font.GlyphWidthPDF(glyphName)
	}
	if f.Metrics != nil {
		return f.Metrics.GlyphWidthPDF(glyphName)
	}
	return 0
}

// Embed returns a reference to the font dictionary, and a Go object
// representing the font data.
//
// This implements the [font.Embedder] interface.
func (f *Type1Font) Embed(rm *pdf.ResourceManager) (pdf.Native, font.Embedded, error) {
	ref := rm.Out.Alloc()
	dicts := f.makeDicts(ref)
	return ref, dicts, nil
}

func (f *Type1Font) makeDicts(ref pdf.Reference) *Type1Dicts {
	fd := &font.Descriptor{}
	if ps := f.Font; ps != nil {
		fd.FontName = ps.FontName
		fd.FontFamily = ps.FamilyName
		fd.FontWeight = os2.WeightFromString(ps.Weight)
		fd.FontBBox = ps.FontBBoxPDF()
		fd.IsItalic = ps.ItalicAngle != 0
		fd.ItalicAngle = ps.ItalicAngle
		fd.IsFixedPitch = ps.IsFixedPitch
		fd.ForceBold = ps.Private.ForceBold
		fd.StemV = ps.Private.StdVW
		fd.StemH = ps.Private.StdHW
	}
	if m := f.Metrics; m != nil {
		fd.FontName = m.FontName
		fd.FontBBox = m.FontBBoxPDF()
		fd.CapHeight = m.CapHeight
		fd.XHeight = m.XHeight
		fd.Ascent = m.Ascent
		fd.Descent = m.Descent
		fd.IsItalic = m.ItalicAngle != 0
		fd.ItalicAngle = m.ItalicAngle
		fd.IsFixedPitch = m.IsFixedPitch
	}
	dicts := &Type1Dicts{
		ref:            ref,
		PostScriptName: f.Font.FontName,
		Descriptor:     fd,
		Encoding:       encoding.New(),
		Font:           f.Font,
	}
	return dicts
}

func (f *Type1Font) newTypesetter() *Typesetter {
	runeToName := make(map[rune]string)

	var glyphNames []string
	if f.Font != nil {
		glyphNames = maps.Keys(f.Font.Glyphs)
	} else {
		glyphNames = maps.Keys(f.Metrics.Glyphs)
	}

	// Sort the names so that ".notdef" comes first, and the rest is sorted
	// alphabetically.
	sort.Slice(glyphNames, func(i, j int) bool {
		iIsNotdef := glyphNames[i] == ".notdef"
		jIsNotdef := glyphNames[j] == ".notdef"
		if iIsNotdef && !jIsNotdef {
			return true
		}
		if jIsNotdef && !iIsNotdef {
			return false
		}
		return glyphNames[i] < glyphNames[j]
	})

	isZapf := f.PostScriptName() == "ZapfDingbats"

	replacementRune := rune(0xE000) // start of the first PUA
	for _, glyphName := range glyphNames {
		rr := names.ToUnicode(glyphName, isZapf)
		if len(rr) != 1 {
			continue
		}
		r := rr[0]

		for {
			_, exists := runeToName[r]
			if !exists {
				break
			}
			r = replacementRune
			replacementRune++
			if replacementRune == 0xF900 {
				// we overflowed the first PUA, jump to the next
				replacementRune = 0x0F_0000
			}
		}

		runeToName[r] = glyphName
	}

	return &Typesetter{
		runeToCode: make(map[rune]byte),
		codeToInfo: make(map[byte]*font.CodeInfo),

		glyphNames: glyphNames,
		runeToName: runeToName,
		isZapf:     isZapf,
	}
}

type Typesetter struct {
	Type1Font

	runeToCode map[rune]byte
	codeToInfo map[byte]*font.CodeInfo

	glyphNames []string
	runeToName map[rune]string
	isZapf     bool
}

func (t *Typesetter) AppendEncoded(codes pdf.String, gid glyph.ID, s string) pdf.String {
	for _, r := range s {
		code, seen := t.runeToCode[r]
		if seen {
			codes = append(codes, code)
			continue
		}

		glyphName := t.getName(r)
		code = t.findCode(glyphName, t.isZapf)
		cid := t.nameToCID(glyphName)

		t.runeToCode[r] = code
		t.codeToInfo[code] = &font.CodeInfo{
			CID:    cid,
			Notdef: 0,
			Text:   string([]rune{r}),
			W:      t.GlyphWidthPDF(glyphName),
		}
		codes = append(codes, code)
	}
	return codes
}

func (t *Typesetter) nameToCID(name string) cmap.CID {
	i := sort.SearchStrings(t.glyphNames, name)
	if i < len(t.glyphNames) && t.glyphNames[i] == name {
		return cmap.CID(i)
	}
	return 0
}

func (t *Typesetter) getName(r rune) string {
	if name, ok := t.runeToName[r]; ok {
		return name
	}
	return ".notdef"
}

func (t *Typesetter) findCode(glyphName string, isZapf bool) byte {
	bestScore := -1
	bestCode := byte(0)
	for codeInt := 0; codeInt < 256; codeInt++ {
		code := byte(codeInt)
		if _, alreadyUsed := t.codeToInfo[code]; alreadyUsed {
			continue
		}

		stdName := pdfenc.Standard.Encoding[code]
		if stdName == glyphName {
			// If r is in the standard encoding, and the corresponding
			// code is still free, then use it.
			return code
		}

		var score int
		switch {
		case code == 0:
			// try to reserve the first code for the .notdef glyph
			score = 10
		case code == 32:
			// try to keep code 32 for the space character
			score = 20
		case stdName == ".notdef":
			// try to use gaps in the standard encoding first
			score = 40
		default:
			score = 30
		}

		if rr := names.ToUnicode(glyphName, isZapf); len(rr) == 1 {
			score += bits.TrailingZeros16(uint16(rr[0]) ^ uint16(code))
		}

		if score > bestScore {
			bestScore = score
			bestCode = code
		}
	}
	return bestCode
}

var _ font.Embedded = (*Type1Dicts)(nil)

type Type1Dicts struct {
	ref            pdf.Reference
	PostScriptName string
	SubsetTag      string
	Name           pdf.Name

	// Descriptor is the font descriptor dictionary.
	// To following fields are ignored: FontName, MissingWidth.
	Descriptor *font.Descriptor

	Encoding *encoding.Encoding
	Widths   [256]float64
	Text     [256]string

	// Font (optional) is the font data to embed.
	// This must be one of *type1.Font, *cff.Font, or *sfnt.Font.
	Font Type1FontData
}

type Type1FontData interface {
	PostScriptName() string
	GetEncoding() []string
}

var (
	_ Type1FontData = (*type1.Font)(nil)
	_ Type1FontData = (*cff.Font)(nil)
	_ Type1FontData = (*sfnt.Font)(nil)
)

func (d *Type1Dicts) WritingMode() cmap.WritingMode {
	return cmap.Horizontal
}

func (d *Type1Dicts) DecodeWidth(s pdf.String) (float64, int) {
	if len(s) == 0 {
		return 0, 0
	}
	return d.Widths[s[0]], 1
}

// Finish writes the font dictionary to the PDF file.
// This implements [pdf.Finisher].
func (d *Type1Dicts) Finish(rm *pdf.ResourceManager) error {
	_, _, err := pdf.ResourceManagerEmbed(rm, d)
	return err
}

func (d *Type1Dicts) Embed(rm *pdf.ResourceManager) (pdf.Native, pdf.Unused, error) {
	var zero pdf.Unused

	w := rm.Out

	// Check that all data is valid and consistent.
	if d.Font != nil {
		switch f := d.Font.(type) {
		case *cff.Font:
			if f.IsCIDKeyed() {
				return d.ref, zero, errors.New("CID-keyed fonts not allowed")
			}
		case *sfnt.Font:
			o, _ := f.Outlines.(*cff.Outlines)
			if o == nil {
				return d.ref, zero, errors.New("missing CFF table")
			} else if o.IsCIDKeyed() {
				return d.ref, zero, errors.New("CID-keyed fonts not allowed")
			}
		default:
			return d.ref, zero, fmt.Errorf("unsupported font type %T", d.Font)
		}

		fontName := d.Font.PostScriptName()
		if fontName != d.PostScriptName {
			return d.ref, zero, fmt.Errorf("font name mismatch: %s != %s", fontName, d.PostScriptName)
		}
	}
	if d.SubsetTag != "" && !subset.IsValidTag(d.SubsetTag) {
		return d.ref, zero, fmt.Errorf("invalid subset tag: %s", d.SubsetTag)
	}

	var baseFont pdf.Name
	if d.SubsetTag != "" {
		baseFont = pdf.Name(d.SubsetTag + "+" + d.PostScriptName)
	} else {
		baseFont = pdf.Name(d.PostScriptName)
	}

	fontDict := pdf.Dict{
		"Type":     pdf.Name("Font"),
		"Subtype":  pdf.Name("Type1"),
		"BaseFont": baseFont,
	}
	if d.Name != "" {
		fontDict["Name"] = d.Name
	}

	isNonSymbolic := !d.Descriptor.IsSymbolic
	isExternal := d.Font == nil
	encodingObj, err := d.Encoding.AsPDFType1(isNonSymbolic && isExternal, w.GetOptions())
	if err != nil {
		return d.ref, zero, err
	}
	if encodingObj != nil {
		fontDict["Encoding"] = encodingObj
	}

	compressedObjects := []pdf.Object{fontDict}
	compressedRefs := []pdf.Reference{d.ref}

	stdInfo, isStdFont := stdmtx.Metrics[d.PostScriptName]

	// Since `encodingObject` may map more codes than `d.Encoding`,
	// we need to use `encodingObject` to check the glyph width.

	var fontFileRef pdf.Reference
	trimFontDict := (d.Font == nil &&
		isStdFont &&
		w.GetOptions().HasAny(pdf.OptTrimStandardFonts) &&
		widthsAreCompatible(d.Widths[:], stdInfo, encodingObj) &&
		fontDescriptorIsCompatible(d.Descriptor, stdInfo))
	if !trimFontDict {
		descRef := w.Alloc()
		desc := d.Descriptor.AsDict()
		desc["FontName"] = pdf.Name(baseFont)
		if d.Font != nil {
			fontFileRef = w.Alloc()
			switch d.Font.(type) {
			case *type1.Font:
				desc["FontFile"] = fontFileRef
			case *cff.Font, *sfnt.Font:
				desc["FontFile3"] = fontFileRef
			}
		}

		fontDict["FontDescriptor"] = descRef

		widthRef := w.Alloc()
		widthInfo := widths.EncodeSimple(d.Widths[:])
		fontDict["FirstChar"] = widthInfo.FirstChar
		fontDict["LastChar"] = widthInfo.LastChar
		fontDict["Widths"] = widthRef
		if widthInfo.MissingWidth != 0 {
			desc["MissingWidth"] = pdf.Number(widthInfo.MissingWidth)
		} else {
			delete(desc, "MissingWidth")
		}

		compressedObjects = append(compressedObjects, desc, widthInfo.Widths)
		compressedRefs = append(compressedRefs, descRef, widthRef)
	}

	var builtinEncoding []string
	if d.Font != nil {
		builtinEncoding = d.Font.GetEncoding()
	} else if isStdFont {
		builtinEncoding = stdInfo.Encoding
	}

	needsToUnicode := false
	for code := range 256 {
		cid := d.Encoding.Decode(byte(code))
		if cid == 0 { // unmapped code
			if d.Text[code] != "" {
				needsToUnicode = true
				break
			}
			continue
		}

		glyphName := d.Encoding.GlyphName(cid)
		if glyphName == "" && code < len(builtinEncoding) && builtinEncoding[code] != ".notdef" {
			glyphName = builtinEncoding[code]
		}

		if glyphName == "" {
			if d.Text[code] != "" {
				needsToUnicode = true
				break
			}
			continue
		}

		rr := names.ToUnicode(glyphName, d.PostScriptName == "ZapfDingbats")
		if d.Text[code] != string(rr) {
			needsToUnicode = true
			break
		}
	}
	if needsToUnicode {
		tuInfo := cmap.MakeSimpleToUnicode(d.Text[:])
		ref, _, err := pdf.ResourceManagerEmbed(rm, tuInfo)
		if err != nil {
			return d.ref, zero, fmt.Errorf("ToUnicode cmap: %w", err)
		}
		fontDict["ToUnicode"] = ref
	}

	err = w.WriteCompressed(compressedRefs, compressedObjects...)
	if err != nil {
		return d.ref, zero, pdf.Wrap(err, "Type 1 font dicts")
	}

	switch f := d.Font.(type) {
	case *type1.Font:
		length1 := pdf.NewPlaceholder(w, 10)
		length2 := pdf.NewPlaceholder(w, 10)
		fontStmDict := pdf.Dict{
			"Length1": length1,
			"Length2": length2,
			"Length3": pdf.Integer(0),
		}
		fontStm, err := w.OpenStream(fontFileRef, fontStmDict, pdf.FilterCompress{})
		if err != nil {
			return d.ref, zero, fmt.Errorf("open Type1 stream: %w", err)
		}
		l1, l2, err := f.WritePDF(fontStm)
		if err != nil {
			return d.ref, zero, fmt.Errorf("write Type1 stream: %w", err)
		}
		err = length1.Set(pdf.Integer(l1))
		if err != nil {
			return d.ref, zero, fmt.Errorf("Type1 stream: length1: %w", err)
		}
		err = length2.Set(pdf.Integer(l2))
		if err != nil {
			return d.ref, zero, fmt.Errorf("Type1 stream: length2: %w", err)
		}
		err = fontStm.Close()
		if err != nil {
			return d.ref, zero, fmt.Errorf("close Type1 stream: %w", err)
		}

	case *cff.Font:
		fontStmDict := pdf.Dict{
			"Subtype": pdf.Name("Type1C"),
		}
		fontStm, err := w.OpenStream(fontFileRef, fontStmDict, pdf.FilterCompress{})
		if err != nil {
			return d.ref, zero, fmt.Errorf("open CFF stream: %w", err)
		}
		err = f.Write(fontStm)
		if err != nil {
			return d.ref, zero, fmt.Errorf("write CFF stream: %w", err)
		}
		err = fontStm.Close()
		if err != nil {
			return d.ref, zero, fmt.Errorf("close CFF stream: %w", err)
		}

	case *sfnt.Font:
		fontStmDict := pdf.Dict{
			"Subtype": pdf.Name("OpenType"),
		}
		fontStm, err := w.OpenStream(fontFileRef, fontStmDict, pdf.FilterCompress{})
		if err != nil {
			return d.ref, zero, fmt.Errorf("open OpenType stream: %w", err)
		}
		err = f.WriteOpenTypeCFFPDF(fontStm)
		if err != nil {
			return d.ref, zero, fmt.Errorf("write OpenType stream: %w", err)
		}
		err = fontStm.Close()
		if err != nil {
			return d.ref, zero, fmt.Errorf("close OpenType stream: %w", err)
		}
	}

	return d.ref, zero, nil
}

// widthsAreCompatible returns true if the glyph widths ww are compatible with
// the standard font metrics.  The object encObj is the value of the font
// dictionary's Encoding entry.  It most be valid and must be a direct object.
func widthsAreCompatible(ww []float64, info *stdmtx.FontData, encObj pdf.Object) bool {
	// decode the enc object by hand
	// TODO(voss): extract this into a helper function
	var enc []string
	switch obj := encObj.(type) {
	case nil:
		enc = info.Encoding
	case pdf.Name:
		switch obj {
		case "WinAnsiEncoding":
			enc = pdfenc.WinAnsi.Encoding[:]
		case "MacRomanEncoding":
			enc = pdfenc.MacRoman.Encoding[:]
		case "MacExpertEncoding":
			enc = pdfenc.MacExpert.Encoding[:]
		default:
			panic("unreachable")
		}
	case pdf.Dict:
		enc := info.Encoding
		switch obj["BaseEncoding"] {
		case pdf.Name("WinAnsiEncoding"):
			enc = pdfenc.WinAnsi.Encoding[:]
		case pdf.Name("MacRomanEncoding"):
			enc = pdfenc.MacRoman.Encoding[:]
		case pdf.Name("MacExpertEncoding"):
			enc = pdfenc.MacExpert.Encoding[:]
		}
		if diff, _ := obj["Differences"].(pdf.Array); len(diff) > 0 {
			enc = slices.Clone(enc)
			idx := 0
			for _, obj := range diff {
				switch obj := obj.(type) {
				case pdf.Name:
					enc[idx] = string(obj)
					idx++
				case pdf.Integer:
					idx = int(obj)
				}
			}
		}
	}

	for code, name := range enc {
		if math.Abs(ww[code]-info.Width[name]) > 0.5 {
			return false
		}
	}
	return true
}

func fontDescriptorIsCompatible(fd *font.Descriptor, stdInfo *stdmtx.FontData) bool {
	if fd.FontFamily != "" && fd.FontFamily != stdInfo.FontFamily {
		return false
	}
	if fd.FontWeight != 0 && fd.FontWeight != stdInfo.FontWeight {
		return false
	}
	if fd.IsFixedPitch != stdInfo.IsFixedPitch {
		return false
	}
	if fd.IsSerif != stdInfo.IsSerif {
		return false
	}
	if fd.IsSymbolic != stdInfo.IsSymbolic {
		return false
	}
	if math.Abs(fd.ItalicAngle-stdInfo.ItalicAngle) > 0.1 {
		return false
	}
	if fd.Ascent != 0 && math.Abs(fd.Ascent-stdInfo.Ascent) > 0.5 {
		return false
	}
	if fd.Descent != 0 && math.Abs(fd.Descent-stdInfo.Descent) > 0.5 {
		return false
	}
	if fd.CapHeight != 0 && math.Abs(fd.CapHeight-stdInfo.CapHeight) > 0.5 {
		return false
	}
	if fd.XHeight != 0 && math.Abs(fd.XHeight-stdInfo.XHeight) > 0.5 {
		return false
	}
	if fd.StemV != 0 && math.Abs(fd.StemV-stdInfo.StemV) > 0.5 {
		return false
	}
	if fd.StemH != 0 && math.Abs(fd.StemH-stdInfo.StemH) > 0.5 {
		return false
	}
	return true
}

func type1Rune(w *graphics.Writer, f *type1.Font, r rune) {
	cmap := make(map[rune]string)
	for glyphName := range f.Glyphs {
		rr := names.ToUnicode(glyphName, f.FontName == "ZapfDingbats")
		if len(rr) != 1 {
			panic("unexpected number of runes")
		}
		cmap[rr[0]] = glyphName
	}
	enc := encoding.New()

	// -----------------------------------------------------------------------

	glyphName, ok := cmap[r]
	if !ok {
		panic("missing rune")
	}
	gidInt := slices.Index(f.GlyphList(), glyphName)
	if gidInt < 0 {
		panic("missing")
	}
	gid := glyph.ID(gidInt)
	text := string([]rune{r})

	code, isNew := allocateCode(gid, text)
	if isNew {
		// builtinEncoding[code] = glyphName

		cid := enc.Allocate(glyphName)
		w := f.Glyphs[glyphName].WidthX

		info := &font.CodeInfo{
			CID:    cid,
			Notdef: 0,
			Text:   string([]rune{r}),
			W:      w,
		}
		setCodeInfo(code, info)
	}

	w.TextShowRaw(code)
}
