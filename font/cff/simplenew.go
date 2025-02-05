// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2025  Jochen Voss <voss@seehuhn.de>
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

package cff

import (
	"fmt"
	"math/bits"

	"golang.org/x/exp/maps"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/dict"
	"seehuhn.de/go/pdf/font/glyphdata"
	"seehuhn.de/go/pdf/font/glyphdata/cffglyphs"
	"seehuhn.de/go/pdf/font/pdfenc"
	"seehuhn.de/go/postscript/type1/names"
	"seehuhn.de/go/sfnt/cff"
	"seehuhn.de/go/sfnt/glyph"
)

var _ interface {
	font.Layouter
} = (*instanceNew)(nil)

type instanceNew struct {
	Font *cff.Font
}

func (f *instanceNew) PostScriptName() string {
	return f.Font.FontName
}

func (f *instanceNew) GetGeometry() *font.Geometry {
	panic("not implemented") // TODO: Implement
}

func (f *instanceNew) Layout(seq *font.GlyphSeq, ptSize float64, s string) *font.GlyphSeq {
	panic("not implemented") // TODO: Implement
}

func (f *instanceNew) Embed(rm *pdf.ResourceManager) (pdf.Native, font.Embedded, error) {
	e := newEmbeddedSimpleNew(rm.Out.Alloc(), f.Font)
	return e.Ref, e, nil
}

var _ interface {
	font.EmbeddedLayouter
	font.Scanner
	pdf.Finisher
} = (*embeddedSimpleNew)(nil)

type key struct {
	Gid  glyph.ID
	Text string
}

type embeddedSimpleNew struct {
	*dict.Type1
	Font *cff.Font
	Code map[key]byte
	Enc  map[byte]string
}

func newEmbeddedSimpleNew(ref pdf.Reference, font *cff.Font) *embeddedSimpleNew {
	enc := make(map[byte]string)
	dict := &dict.Type1{
		Ref:            ref,
		PostScriptName: font.FontName,
		Encoding: func(code byte) string {
			return enc[code]
		},
	}
	e := &embeddedSimpleNew{
		Type1: dict,
		Font:  font,
		Code:  make(map[key]byte),
		Enc:   enc,
	}
	return e
}

func (e *embeddedSimpleNew) allocateCode(glyphName string, dingbats bool, target *pdfenc.Encoding) byte {
	var r rune
	rr := names.ToUnicode(glyphName, dingbats)
	if len(rr) > 0 {
		r = rr[0]
	}

	bestScore := -1
	bestCode := byte(0)
	for codeInt := 0; codeInt < 256; codeInt++ {
		code := byte(codeInt)
		if _, alreadyUsed := e.Enc[code]; alreadyUsed {
			continue
		}
		var score int
		stdName := target.Encoding[code]
		if stdName == glyphName {
			// If the glyph is in the target encoding, and the corresponding
			// code is still free, then use it.
			bestCode = code
			break
		} else if stdName == ".notdef" || stdName == "" {
			// fill up the unused slots first
			score += 100
		} else if !(code == 32 && glyphName != "space") {
			// Try to keep code 32 for the space character,
			// in order to not break the PDF word spacing parameter.
			score += 10
		}
		score += bits.TrailingZeros16(uint16(r) ^ uint16(code))

		if score > bestScore {
			bestScore = score
			bestCode = code
		}
	}

	return bestCode
}

func (e *embeddedSimpleNew) AppendEncoded(s pdf.String, gid glyph.ID, text string) (pdf.String, float64) {
	key := key{Gid: gid, Text: text}
	if code, ok := e.Code[key]; !ok {
		return append(s, code), e.Width[gid]
	}

	// TODO(voss): invent glyph names, if needed.
	glyphName := e.Font.Outlines.Glyphs[gid].Name

	var code byte
	if len(e.Code) < 256 {
		code = e.allocateCode(glyphName, e.Font.FontName == "ZapfDingbats", &pdfenc.Standard)
	}

	e.Code[key] = code
	e.Enc[code] = glyphName
	e.Text[code] = text
	e.Width[gid] = e.Font.GlyphWidthPDF(gid)
	return append(s, code), e.Width[gid]
}

func (e *embeddedSimpleNew) Finish(rm *pdf.ResourceManager) error {
	if len(e.Code) > 256 {
		return fmt.Errorf("too many distinct glyphs used in font %q", e.Font.FontName)
	}

	// subset the font
	gidIsUsed := make(map[glyph.ID]struct{})
	gidIsUsed[0] = struct{}{} // always include .notdef
	for key := range e.Code {
		gidIsUsed[key.Gid] = struct{}{}
	}
	glyphs := maps.Keys(gidIsUsed)

	subset := &cff.Font{
		FontInfo: e.Font.FontInfo,
		Outlines: e.Font.Outlines.Subset(glyphs),
	}

	// TODO(voss): convert to simple font, if needed

	// TODO(voss): finish this

	e.FontType = glyphdata.CFFSimple
	e.FontRef = rm.Out.Alloc()

	err := e.Type1.WriteToPDF(rm)
	if err != nil {
		return err
	}

	err = cffglyphs.Embed(rm.Out, e.FontType, e.FontRef, subset)
	if err != nil {
		return err
	}

	return nil
}
