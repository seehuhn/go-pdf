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
	"io"
	"math"
	"math/bits"
	"sort"
	"unicode"

	"seehuhn.de/go/postscript/afm"
	"seehuhn.de/go/postscript/psenc"
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
	"seehuhn.de/go/pdf/internal/stdmtx"
)

// TODO(voss): try to make an interface type which can hold either of
//     *type1.Font
//     *type1.Font with additional kerning information
//     *afm.Metrics
// And then get rid of the duplication caused by the two fields Font and Metrics.

// TODO(voss): gracefully deal with fonts where the .notdef glyph is missing?

var (
	_ font.Font = (*Type1Font)(nil)
)

type Type1Font struct {
	Font       *type1.Font
	Metrics    *afm.Metrics
	psFontName string

	// glyphNames is the list of all glyph names available in the font
	// (excluding .notdef), in alphabetical order.
	glyphNames []string

	// runeToName maps runes to their corresponding glyph names.
	// The unicode Private Use Areas are used to represent glyphs
	// which don't have a natural unicode mapping, or alternative
	// glyphs which map to an already used unicode code point.
	runeToName map[rune]string
}

// NewType1 creates a new Type1Font from a Type 1 font and its AFM metrics.
// Both font and metrics are optional, but at least one must be provided.
func NewType1(font *type1.Font, metrics *afm.Metrics) (*Type1Font, error) {
	if font == nil && metrics == nil {
		return nil, errors.New("both font and metrics are nil")
	}

	var psName string
	if font != nil {
		psName = font.FontName
	} else {
		psName = metrics.FontName
	}

	isDingbats := psName == "ZapfDingbats"

	var glyphNames []string
	if font != nil {
		for glyphName := range font.Glyphs {
			if glyphName != ".notdef" {
				glyphNames = append(glyphNames, glyphName)
			}
		}
	} else {
		for glyphName := range metrics.Glyphs {
			if glyphName != ".notdef" {
				glyphNames = append(glyphNames, glyphName)
			}
		}
	}
	sort.Strings(glyphNames)

	runeToName := make(map[rune]string)
	customCode := rune(0xE000) // start of the first PUA
	for _, glyphName := range glyphNames {
		rr := names.ToUnicode(glyphName, isDingbats)

		r := unicode.ReplacementChar
		if len(rr) == 1 && !isPrivateUse(rr[0]) {
			r = rr[0]
		}

		if _, exists := runeToName[r]; exists || r == unicode.ReplacementChar {
			r = customCode

			customCode++
			if customCode == 0x00_F900 {
				// we overflowed the first PUA, jump to the next
				customCode = 0x0F_0000
			}
		}

		runeToName[r] = glyphName
	}

	f := &Type1Font{
		Font:       font,
		Metrics:    metrics,
		psFontName: psName,

		glyphNames: glyphNames,
		runeToName: runeToName,
	}
	return f, nil
}

func isPrivateUse(r rune) bool {
	return (r >= 0xE000 && r <= 0xF8FF) || // BMP PUA
		(r >= 0xF0000 && r <= 0xFFFFD) || // Supplementary PUA-A
		(r >= 0x100000 && r <= 0x10FFFD) // Supplementary PUA-B
}

func (f *Type1Font) PostScriptName() string {
	return f.psFontName
}

// GlyphWidthPDF computes the width of a glyph in PDF glyph space units.
// If the glyph does not exist, the width of the .notdef glyph is returned.
func (f *Type1Font) GlyphWidthPDF(glyphName string) float64 {
	var w float64
	if f.Font != nil {
		w = f.Font.GlyphWidthPDF(glyphName)
	} else {
		w = f.Metrics.GlyphWidthPDF(glyphName)
	}
	w = math.Round(w*10) / 10
	return w
}

// Embed returns a reference to the font dictionary, and a Go object
// representing the font data.
//
// This implements the [font.Embedder] interface.
func (f *Type1Font) Embed(rm *pdf.ResourceManager) (pdf.Native, font.Embedded, error) {
	t := f.newTypesetter(rm)
	return t.ref, t, nil
}

var (
	_ font.Embedded = (*Typesetter)(nil)
	_ pdf.Finisher  = (*Typesetter)(nil)
)

type Typesetter struct {
	rm  *pdf.ResourceManager
	ref pdf.Reference

	*Type1Font

	runeToCode map[rune]byte
	codeToInfo map[byte]*font.CodeInfo
}

func (f *Type1Font) newTypesetter(rm *pdf.ResourceManager) *Typesetter {
	return &Typesetter{
		rm:  rm,
		ref: rm.Out.Alloc(),

		Type1Font: f,

		runeToCode: make(map[rune]byte),
		codeToInfo: make(map[byte]*font.CodeInfo),
	}
}

// WritingMode returns 0 to indicate horizontal writing.
//
// This implements the [font.Embedded] interface.
func (t *Typesetter) WritingMode() cmap.WritingMode {
	return cmap.Horizontal
}

// DecodeWidth reads one character code from the given string and returns
// the width of the corresponding glyph in PDF text space units (still to
// be multiplied by the font size) as well as the number of bytes consumed.
//
// This implements the [font.Embedded] interface.
func (t *Typesetter) DecodeWidth(s pdf.String) (float64, int) {
	if len(s) == 0 {
		return 0, 0
	}

	c := s[0]
	info, ok := t.codeToInfo[c]
	var w float64 // PDF glyph space units
	if ok {
		w = info.W
	} else {
		w = t.GlyphWidthPDF(".notdef")
	}
	return w / 1000, 1
}

func (t *Typesetter) AppendEncoded(codes pdf.String, s string) pdf.String {
	for _, r := range s {
		text := string([]rune{r})

		code, seen := t.runeToCode[r]
		if seen {
			codes = append(codes, code)
			continue
		}

		glyphName := t.getName(r)
		code = t.registerGlyph(glyphName, text, t.GlyphWidthPDF(glyphName))
		t.runeToCode[r] = code

		codes = append(codes, code)
	}
	return codes
}

func (t *Typesetter) registerGlyph(glyphName, text string, w float64) byte {
	code := t.newCode(glyphName, text, t.psFontName == "ZapfDingbats")
	t.codeToInfo[code] = &font.CodeInfo{
		CID:    t.getCID(glyphName),
		Notdef: 0,
		Text:   text,
		W:      w,
	}
	return code
}

func (t *Typesetter) getCID(name string) cmap.CID {
	i := sort.SearchStrings(t.glyphNames, name)
	if i < len(t.glyphNames) && t.glyphNames[i] == name {
		return cmap.CID(i) + 1
	}
	return 0
}

func (t *Typesetter) getGlyphName(cid cmap.CID) string {
	if int(cid) == 0 {
		return ".notdef"
	}
	return t.glyphNames[cid-1]
}

func (t *Typesetter) getName(r rune) string {
	if name, ok := t.runeToName[r]; ok {
		return name
	}
	return ".notdef"
}

func (t *Typesetter) newCode(glyphName, text string, isDingbats bool) byte {
	bestScore := -1
	bestCode := byte(0)
	for codeInt, stdName := range pdfenc.Standard.Encoding {
		code := byte(codeInt)

		if _, alreadyUsed := t.codeToInfo[code]; alreadyUsed {
			continue
		}

		if stdName == glyphName {
			// If r is in the standard encoding (and the corresponding
			// code is still available) then use it.
			return code
		}

		var score int
		switch {
		case code == 0:
			// try to reserve code 0 for the .notdef glyph
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

		// As a last resort, try to match the last bits of the unicode code point.
		rr := []rune(text)
		if len(rr) == 0 {
			rr = names.ToUnicode(glyphName, isDingbats)
		}
		if len(rr) > 0 {
			// Because combining characters come after the base character,
			// we use the first character here.
			score += bits.TrailingZeros16(uint16(rr[0]) ^ uint16(code))
		}

		if score > bestScore {
			bestScore = score
			bestCode = code
		}
	}
	return bestCode
}

func (t *Typesetter) glyphsUsed() map[string]struct{} {
	all := make(map[string]struct{})
	for _, info := range t.codeToInfo {
		glyphName := t.getGlyphName(info.CID)
		all[glyphName] = struct{}{}
	}
	return all
}

func clone[T any](x *T) *T {
	y := *x
	return &y
}

// Finish writes the font dictionary to the PDF file.
// After this has been called, no new codes can be allocated.
func (t *Typesetter) Finish(rm *pdf.ResourceManager) error {
	var subsetTag string
	psFont := t.Type1Font.Font
	if glyphsUsed := t.glyphsUsed(); len(glyphsUsed) < len(t.glyphNames) && psFont != nil {
		// subset the font

		psSubset := clone(psFont)
		psSubset.Glyphs = make(map[string]*type1.Glyph)
		if glyph, ok := psFont.Glyphs[".notdef"]; ok {
			psSubset.Glyphs[".notdef"] = glyph
		}
		for name := range glyphsUsed {
			if glyph, ok := psFont.Glyphs[name]; ok {
				psSubset.Glyphs[name] = glyph
			}
		}
		// We always use the standard encoding here to minimize the
		// size of the embedded font data.  The actual encoding is then
		// set in the PDF font dictionary.
		psSubset.Encoding = psenc.StandardEncoding[:]
		psFont = psSubset

		// TODO(voss): find a better way to generate a subset tag
		var gg []glyph.ID
		for gid, glyphName := range t.glyphNames {
			if _, used := glyphsUsed[glyphName]; used {
				gg = append(gg, glyph.ID(gid+1))
			}
		}
		subsetTag = subset.Tag(gg, len(t.glyphNames)+1)
	}

	fd := &font.Descriptor{}
	if ps := t.Font; ps != nil {
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
	if m := t.Metrics; m != nil {
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

	enc := make(map[byte]string)

	dicts := &TypeFontDict{
		Ref:            t.ref,
		PostScriptName: t.PostScriptName(),
		SubsetTag:      subsetTag,
		Descriptor:     fd,
		Encoding:       func(code byte) string { return enc[code] },
	}
	if psFont != nil {
		dicts.GetFont = func() (Type1FontData, error) {
			return psFont, nil
		}
	}

	notdefWidth := t.GlyphWidthPDF(".notdef")
	for code := range 256 {
		info, ok := t.codeToInfo[byte(code)]
		if ok {
			dicts.Width[code] = info.W
			dicts.Text[code] = info.Text
			enc[byte(code)] = t.getGlyphName(info.CID)
		} else {
			dicts.Width[code] = notdefWidth
		}
	}
	if dicts.Width[0] == dicts.Width[255] {
		fd.MissingWidth = dicts.Width[0]
	} else {
		left := 1
		for dicts.Width[left] == dicts.Width[0] {
			left++
		}
		right := 1
		for dicts.Width[255-right] == dicts.Width[255] {
			right++
		}
		if left >= right && left > 1 {
			fd.MissingWidth = dicts.Width[0]
		} else if right > 1 {
			fd.MissingWidth = dicts.Width[255]
		}
	}

	_, _, err := pdf.ResourceManagerEmbed(rm, dicts)
	if err != nil {
		return err
	}
	return nil
}

var (
	_ Type1FontData = (*type1.Font)(nil)
	_ Type1FontData = (*cff.Font)(nil)
	_ Type1FontData = (*sfnt.Font)(nil)
)

// Type1FontData is a font which can be used with a Type 1 font dictionary.
// This must be one of [*type1.Font], [*cff.Font] or [*sfnt.Font].
type Type1FontData interface {
	PostScriptName() string
	BuiltinEncoding() []string
}

var _ font.Embedded = (*TypeFontDict)(nil)

// TypeFontDict represents a Type 1 font dictionary.
type TypeFontDict struct {
	Ref            pdf.Reference
	PostScriptName string
	SubsetTag      string
	Name           pdf.Name

	// Descriptor is the font descriptor.
	// The FontName field is ignored.
	Descriptor *font.Descriptor

	// Encoding maps character codes to glyph names.
	Encoding encoding.Type1

	// Width contains the glyph widths for all character codes,
	// in PDF glyph space units.
	Width [256]float64

	// Text contains the text content for each character code.
	Text [256]string

	// GetFont (optional) returns the font data to embed.
	// If this is nil, the font data is not embedded in the PDF file.
	GetFont func() (Type1FontData, error)
}

func (d *TypeFontDict) WritingMode() cmap.WritingMode {
	return cmap.Horizontal
}

func (d *TypeFontDict) DecodeWidth(s pdf.String) (float64, int) {
	if len(s) == 0 {
		return 0, 0
	}
	return d.Width[s[0]], 1
}

// ExtractType1Dicts reads a Type 1 font dictionary from a PDF file.
func ExtractType1Dicts(r pdf.Getter, obj pdf.Object) (*TypeFontDict, error) {
	fontDict, err := pdf.GetDictTyped(r, obj, "Font")
	if err != nil {
		return nil, err
	}
	subtype, err := pdf.GetName(r, fontDict["Subtype"])
	if err != nil {
		return nil, err
	}
	if subtype != "" && subtype != "Type1" {
		return nil, &pdf.MalformedFileError{
			Err: fmt.Errorf("expected font subtype Type1, got %q", subtype),
		}
	}

	d := &TypeFontDict{}
	d.Ref, _ = obj.(pdf.Reference)

	baseFont, err := pdf.GetName(r, fontDict["BaseFont"])
	if err != nil {
		return nil, err
	}
	if m := subset.TagRegexp.FindStringSubmatch(string(baseFont)); m != nil {
		d.PostScriptName = m[2]
		d.SubsetTag = m[1]
	} else {
		d.PostScriptName = string(baseFont)
	}

	// StdInfo will be non-nil, if the PostScript name indicates one of the
	// standard 14 fonts. In this case, we use the corresponding metrics as
	// default values, in case they are missing from the font dictionary.
	stdInfo := stdmtx.Metrics[d.PostScriptName]

	d.Name, _ = pdf.GetName(r, fontDict["Name"])

	fdDict, err := pdf.GetDictTyped(r, fontDict["FontDescriptor"], "FontDescriptor")
	if err != nil && !pdf.IsMalformed(err) {
		return nil, err
	}
	fd, _ := font.ExtractDescriptor(r, fdDict)
	if fd == nil && stdInfo != nil {
		fd = &font.Descriptor{
			FontName:     d.PostScriptName,
			FontFamily:   stdInfo.FontFamily,
			FontStretch:  os2.WidthNormal,
			FontWeight:   stdInfo.FontWeight,
			IsFixedPitch: stdInfo.IsFixedPitch,
			IsSerif:      stdInfo.IsSerif,
			IsSymbolic:   stdInfo.IsSymbolic,
			IsItalic:     stdInfo.ItalicAngle != 0,
			FontBBox:     stdInfo.FontBBox,
			ItalicAngle:  stdInfo.ItalicAngle,
			Ascent:       stdInfo.Ascent,
			Descent:      stdInfo.Descent,
			CapHeight:    stdInfo.CapHeight,
			XHeight:      stdInfo.XHeight,
			StemV:        stdInfo.StemV,
			StemH:        stdInfo.StemH,
		}
	} else if fd == nil { // only possible for invalid PDF files
		fd = &font.Descriptor{
			FontName: d.PostScriptName,
		}
	}
	d.Descriptor = fd

	isNonSymbolic := !fd.IsSymbolic
	isExternal := fdDict["FontFile"] == nil && fdDict["FontFile3"] == nil
	nonSymbolicExt := isNonSymbolic && isExternal
	enc, err := encoding.ExtractType1New(r, fontDict["Encoding"], nonSymbolicExt)
	if err != nil {
		return nil, err
	}
	d.Encoding = enc

	firstChar, _ := pdf.GetInteger(r, fontDict["FirstChar"])
	widths, _ := pdf.GetArray(r, fontDict["Widths"])
	if widths != nil && len(widths) <= 256 && firstChar >= 0 && firstChar < 256 {
		for c := range widths {
			d.Width[c] = fd.MissingWidth
		}
		for i, w := range widths {
			w, err := pdf.GetNumber(r, w)
			if err != nil {
				continue
			}
			if code := firstChar + pdf.Integer(i); code < 256 {
				d.Width[byte(code)] = float64(w)
			}
		}
	} else if stdInfo != nil {
		for c := range 256 {
			w, ok := stdInfo.Width[enc(byte(c))]
			if !ok {
				w = stdInfo.Width[".notdef"]
			}
			d.Width[c] = w
		}
	}

	// First try to derive text content from the glyph names.
	// This can be overridden by the ToUnicode CMap, below.
	for code := range 256 {
		glyphName := enc(byte(code))
		if glyphName == "" || glyphName == encoding.UseBuiltin || glyphName == ".notdef" {
			continue
		}

		rr := names.ToUnicode(glyphName, d.PostScriptName == "ZapfDingbats")
		d.Text[code] = string(rr)
	}

	toUnicode, err := cmap.ExtractToUnicodeNew(r, fontDict["ToUnicode"])
	if err != nil && !pdf.IsMalformed(err) {
		return nil, err
	}
	if toUnicode != nil {
		// TODO(voss): implement an iterator on toUnicode to do this
		// more efficiently?
		for code := range 256 {
			rr := toUnicode.Lookup([]byte{byte(code)})
			d.Text[code] = string(rr)
		}
	}

	getFont, err := makeFontReader(r, fdDict)
	if err != nil && !pdf.IsMalformed(err) {
		return nil, err
	}
	d.GetFont = getFont

	return d, nil
}

func makeFontReader(r pdf.Getter, fd pdf.Dict) (func() (Type1FontData, error), error) {
	s, err := pdf.GetStream(r, fd["FontFile"])
	if err != nil && !pdf.IsMalformed(err) {
		return nil, err
	}
	if s != nil {
		getFont := func() (Type1FontData, error) {
			fontData, err := pdf.DecodeStream(r, s, 0)
			if err != nil {
				return nil, err
			}
			font, err := type1.Read(fontData)
			if err != nil {
				return nil, err
			}
			return font, nil
		}
		return getFont, nil
	}

	s, err = pdf.GetStream(r, fd["FontFile3"])
	if err != nil && !pdf.IsMalformed(err) {
		return nil, err
	}
	if s == nil {
		return nil, nil
	}

	subType, _ := pdf.GetName(r, s.Dict["Subtype"])
	switch subType {
	case "Type1C":
		getFont := func() (Type1FontData, error) {
			fontData, err := pdf.DecodeStream(r, s, 0)
			if err != nil {
				return nil, err
			}
			body, err := io.ReadAll(fontData)
			if err != nil {
				return nil, err
			}
			font, err := cff.Read(bytes.NewReader(body))
			if err != nil {
				return nil, err
			}
			return font, nil
		}
		return getFont, nil

	case "OpenType":
		getFont := func() (Type1FontData, error) {
			fontData, err := pdf.DecodeStream(r, s, 0)
			if err != nil {
				return nil, err
			}
			font, err := sfnt.Read(fontData)
			if err != nil {
				return nil, err
			}
			return font, nil
		}
		return getFont, nil

	default:
		return nil, nil
	}
}

// Embed adds the font dictionary to the PDF file.
func (d *TypeFontDict) Embed(rm *pdf.ResourceManager) (pdf.Native, pdf.Unused, error) {
	var zero pdf.Unused

	var psFont Type1FontData
	if d.GetFont != nil {
		font, err := d.GetFont()
		if err != nil {
			return nil, zero, err
		}
		psFont = font
	}

	// Check that all data is valid and consistent.
	if d.Ref == 0 {
		return nil, zero, errors.New("missing font dictionary reference")
	}
	if psFont != nil {
		switch f := psFont.(type) {
		case *type1.Font:
			// pass
		case *cff.Font:
			if f.IsCIDKeyed() {
				return nil, zero, errors.New("CID-keyed fonts not allowed")
			}
		case *sfnt.Font:
			o, _ := f.Outlines.(*cff.Outlines)
			if o == nil {
				return nil, zero, errors.New("missing CFF table")
			} else if o.IsCIDKeyed() {
				return nil, zero, errors.New("CID-keyed fonts not allowed")
			}
		default:
			return nil, zero, fmt.Errorf("unsupported font type %T", psFont)
		}
	}
	if d.SubsetTag != "" && !subset.IsValidTag(d.SubsetTag) {
		return nil, zero, fmt.Errorf("invalid subset tag: %s", d.SubsetTag)
	}

	w := rm.Out

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
	isExternal := psFont == nil
	encodingObj, err := d.Encoding.AsPDF(isNonSymbolic && isExternal, w.GetOptions())
	if err != nil {
		return nil, zero, err
	}
	if encodingObj != nil {
		fontDict["Encoding"] = encodingObj
	}

	compressedObjects := []pdf.Object{fontDict}
	compressedRefs := []pdf.Reference{d.Ref}

	stdInfo := stdmtx.Metrics[d.PostScriptName]

	var fontFileRef pdf.Reference
	trimFontDict := (psFont == nil &&
		stdInfo != nil &&
		w.GetOptions().HasAny(pdf.OptTrimStandardFonts) &&
		widthsAreCompatible(d.Width[:], stdInfo, d.Encoding) &&
		fontDescriptorIsCompatible(d.Descriptor, stdInfo))
	if !trimFontDict {
		fdRef := w.Alloc()
		fdDict := d.Descriptor.AsDict()
		fdDict["FontName"] = pdf.Name(baseFont)
		if psFont != nil {
			fontFileRef = w.Alloc()
			switch psFont.(type) {
			case *type1.Font:
				fdDict["FontFile"] = fontFileRef
			case *cff.Font, *sfnt.Font:
				fdDict["FontFile3"] = fontFileRef
			}
		}

		fontDict["FontDescriptor"] = fdRef

		// TODO(voss): Introduce a helper function for constructing the widths
		// array.
		lastChar := 255
		for lastChar > 0 && d.Width[lastChar] == d.Descriptor.MissingWidth {
			lastChar--
		}
		firstChar := 0
		for firstChar < lastChar && d.Width[firstChar] == d.Descriptor.MissingWidth {
			firstChar++
		}
		widths := make(pdf.Array, 0, lastChar-firstChar+1)
		for i := firstChar; i <= lastChar; i++ {
			widths = append(widths, pdf.Number(d.Width[i]))
		}

		fontDict["FirstChar"] = pdf.Integer(firstChar)
		fontDict["LastChar"] = pdf.Integer(lastChar)
		if len(widths) > 10 {
			widthRef := w.Alloc()
			fontDict["Widths"] = widthRef
			compressedObjects = append(compressedObjects, widths)
			compressedRefs = append(compressedRefs, widthRef)
		} else {
			fontDict["Widths"] = widths
		}

		compressedObjects = append(compressedObjects, fdDict)
		compressedRefs = append(compressedRefs, fdRef)
	}

	toUnicodeData := make(map[byte]string)
	for code := range 256 {
		glyphName := d.Encoding(byte(code))
		switch glyphName {
		case "":
			// unused code, nothing to do

		case encoding.UseBuiltin:
			if d.Text[code] != "" {
				toUnicodeData[byte(code)] = d.Text[code]
			}

		default:
			rr := names.ToUnicode(glyphName, d.PostScriptName == "ZapfDingbats")
			if text := d.Text[code]; text != string(rr) {
				toUnicodeData[byte(code)] = text
			}
		}
	}
	if len(toUnicodeData) > 0 {
		tuInfo := cmap.MakeSimpleToUnicode(toUnicodeData)
		ref, _, err := pdf.ResourceManagerEmbed(rm, tuInfo)
		if err != nil {
			return nil, zero, fmt.Errorf("ToUnicode cmap: %w", err)
		}
		fontDict["ToUnicode"] = ref
	}

	err = w.WriteCompressed(compressedRefs, compressedObjects...)
	if err != nil {
		return nil, zero, pdf.Wrap(err, "Type 1 font dicts")
	}

	switch f := psFont.(type) {
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
			return nil, zero, fmt.Errorf("open Type1 stream: %w", err)
		}
		l1, l2, err := f.WritePDF(fontStm)
		if err != nil {
			return nil, zero, fmt.Errorf("write Type1 stream: %w", err)
		}
		err = length1.Set(pdf.Integer(l1))
		if err != nil {
			return nil, zero, fmt.Errorf("Type1 stream: length1: %w", err)
		}
		err = length2.Set(pdf.Integer(l2))
		if err != nil {
			return nil, zero, fmt.Errorf("Type1 stream: length2: %w", err)
		}
		err = fontStm.Close()
		if err != nil {
			return nil, zero, fmt.Errorf("close Type1 stream: %w", err)
		}

	case *cff.Font:
		fontStmDict := pdf.Dict{
			"Subtype": pdf.Name("Type1C"),
		}
		fontStm, err := w.OpenStream(fontFileRef, fontStmDict, pdf.FilterCompress{})
		if err != nil {
			return nil, zero, fmt.Errorf("open CFF stream: %w", err)
		}
		err = f.Write(fontStm)
		if err != nil {
			return nil, zero, fmt.Errorf("write CFF stream: %w", err)
		}
		err = fontStm.Close()
		if err != nil {
			return nil, zero, fmt.Errorf("close CFF stream: %w", err)
		}

	case *sfnt.Font:
		fontStmDict := pdf.Dict{
			"Subtype": pdf.Name("OpenType"),
		}
		fontStm, err := w.OpenStream(fontFileRef, fontStmDict, pdf.FilterCompress{})
		if err != nil {
			return nil, zero, fmt.Errorf("open OpenType stream: %w", err)
		}
		err = f.WriteOpenTypeCFFPDF(fontStm)
		if err != nil {
			return nil, zero, fmt.Errorf("write OpenType stream: %w", err)
		}
		err = fontStm.Close()
		if err != nil {
			return nil, zero, fmt.Errorf("close OpenType stream: %w", err)
		}
	}

	return d.Ref, zero, nil
}

// widthsAreCompatible returns true, if the glyph widths ww are compatible with
// the standard font metrics.  The object encObj is the value of the font
// dictionary's Encoding entry.
//
// EncObj must be valid and must be a direct object.  Do not pass encObj values
// read from files without validation.
func widthsAreCompatible(ww []float64, info *stdmtx.FontData, enc encoding.Type1) bool {
	for code := range 256 {
		name := enc(byte(code))
		if name == "" {
			continue
		}
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
