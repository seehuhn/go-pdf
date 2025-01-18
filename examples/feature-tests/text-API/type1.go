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
	"math"
	"math/bits"
	"sort"
	"unicode"

	"seehuhn.de/go/postscript/afm"
	"seehuhn.de/go/postscript/psenc"
	"seehuhn.de/go/postscript/type1"
	"seehuhn.de/go/postscript/type1/names"

	"seehuhn.de/go/sfnt/glyph"
	"seehuhn.de/go/sfnt/os2"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/pdfenc"
	"seehuhn.de/go/pdf/font/simple"
	"seehuhn.de/go/pdf/font/subset"
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
	if fontData := t.Font; fontData != nil {
		fd.FontName = fontData.FontName
		fd.FontFamily = fontData.FamilyName
		fd.FontWeight = os2.WeightFromString(fontData.Weight)
		fd.FontBBox = fontData.FontBBoxPDF()
		fd.IsItalic = fontData.ItalicAngle != 0
		fd.ItalicAngle = fontData.ItalicAngle
		fd.IsFixedPitch = fontData.IsFixedPitch
		fd.ForceBold = fontData.Private.ForceBold
		fd.StemV = fontData.Private.StdVW
		fd.StemH = fontData.Private.StdHW
	}
	if metricsData := t.Metrics; metricsData != nil {
		fd.FontName = metricsData.FontName
		fd.FontBBox = metricsData.FontBBoxPDF()
		fd.CapHeight = metricsData.CapHeight
		fd.XHeight = metricsData.XHeight
		fd.Ascent = metricsData.Ascent
		fd.Descent = metricsData.Descent
		fd.IsItalic = metricsData.ItalicAngle != 0
		fd.ItalicAngle = metricsData.ItalicAngle
		fd.IsFixedPitch = metricsData.IsFixedPitch
	}

	enc := make(map[byte]string)

	dict := &simple.Type1Dict{
		Ref:            t.ref,
		PostScriptName: t.PostScriptName(),
		SubsetTag:      subsetTag,
		Descriptor:     fd,
		Encoding:       func(code byte) string { return enc[code] },
	}
	if psFont != nil {
		dict.GetFont = func() (simple.Type1FontData, error) {
			return psFont, nil
		}
	}

	notdefWidth := t.GlyphWidthPDF(".notdef")
	for code := range 256 {
		info, ok := t.codeToInfo[byte(code)]
		if ok {
			dict.Width[code] = info.W
			dict.Text[code] = info.Text
			enc[byte(code)] = t.getGlyphName(info.CID)
		} else {
			dict.Width[code] = notdefWidth
		}
	}
	if dict.Width[0] == dict.Width[255] {
		fd.MissingWidth = dict.Width[0]
	} else {
		left := 1
		for dict.Width[left] == dict.Width[0] {
			left++
		}
		right := 1
		for dict.Width[255-right] == dict.Width[255] {
			right++
		}
		if left >= right && left > 1 {
			fd.MissingWidth = dict.Width[0]
		} else if right > 1 {
			fd.MissingWidth = dict.Width[255]
		}
	}

	return dict.Finish(rm)
}
