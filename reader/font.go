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

package reader

import (
	"errors"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cff"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/opentype"
	"seehuhn.de/go/pdf/font/truetype"
	"seehuhn.de/go/pdf/font/type1"
	"seehuhn.de/go/pdf/font/type3"
	"seehuhn.de/go/sfnt/glyph"
)

// FontFromFile represents a font which has been extracted from a PDF file.
type FontFromFile interface {
	font.Embedded

	// TODO(voss): how to handle glyph selection in Type 1 font?
	ForeachGlyph(s pdf.String, yield func(gid glyph.ID, text []rune, width float64, is_space bool))

	FontData() interface{}

	Key() pdf.Reference
}

// ReadFont extracts a font from a PDF file.
func (r *Reader) ReadFont(ref pdf.Object, name pdf.Name) (FontFromFile, error) {
	fontDicts, err := font.ExtractDicts(r.R, ref)
	if err != nil {
		return nil, err
	}

	res := font.Res{
		DefName: name,
		Ref:     ref,
	}

	// TODO(voss): make this less repetitive
	switch fontDicts.Type {
	case font.Type1: // Type 1 fonts (including the standard 14 fonts)
		info, err := type1.Extract(r.R, fontDicts)
		if err != nil {
			return nil, err
		}
		_ = info
		panic("not implemented")
	case font.CFFComposite: // CFF font data without wrapper (composite font)
		info, err := cff.ExtractComposite(r.R, fontDicts)
		if err != nil {
			return nil, err
		}
		_ = info
		panic("not implemented")
	case font.CFFSimple: // CFF font data without wrapper (simple font)
		info, err := cff.ExtractSimple(r.R, fontDicts)
		if err != nil {
			return nil, err
		}
		widths := make([]float64, 256)
		for c := range widths {
			widths[c] = info.Font.GlyphWidthPDF(info.Encoding[c])
		}
		text := make([][]rune, 256)
		if info.ToUnicode != nil {
			text = info.ToUnicode.GetSimpleMapping()
		}
		// TODO: other methods for extracting the text mapping
		res := &fromFileSimple{
			Res:      res,
			widths:   widths,
			encoding: info.Encoding,
			text:     text,
			fontData: info.Font,
			key:      fontDicts.FontProgramRef,
		}
		return res, nil
	case font.MMType1: // Multiple Master type 1 fonts
		return nil, errors.New("Multiple Master type 1 fonts not supported")
	case font.OpenTypeCFFComposite: // CFF fonts in an OpenType wrapper (composite font)
		info, err := opentype.ExtractCFFComposite(r.R, fontDicts)
		if err != nil {
			return nil, err
		}
		_ = info
		panic("not implemented")
	case font.OpenTypeCFFSimple: // CFF font data in an OpenType wrapper (simple font)
		info, err := opentype.ExtractCFFSimple(r.R, fontDicts)
		if err != nil {
			return nil, err
		}
		widths := make([]float64, 256)
		for c := range widths {
			widths[c] = info.Font.GlyphWidthPDF(info.Encoding[c])
		}
		text := make([][]rune, 256)
		if info.ToUnicode != nil {
			text = info.ToUnicode.GetSimpleMapping()
		}
		// TODO(voss): other methods for extracting the text mapping
		res := &fromFileSimple{
			Res:      res,
			widths:   widths,
			encoding: info.Encoding,
			text:     text,
			fontData: info.Font,
		}
		return res, nil
	case font.OpenTypeGlyfComposite: // OpenType fonts with glyf outline (composite font)
		info, err := opentype.ExtractGlyfComposite(r.R, fontDicts)
		if err != nil {
			return nil, err
		}
		_ = info
		panic("not implemented")
	case font.OpenTypeGlyfSimple: // OpenType fonts with glyf outline (simple font)
		info, err := opentype.ExtractGlyfSimple(r.R, fontDicts)
		if err != nil {
			return nil, err
		}
		_ = info
		panic("not implemented")
	case font.TrueTypeComposite: // TrueType fonts (composite font)
		info, err := truetype.ExtractComposite(r.R, fontDicts)
		if err != nil {
			return nil, err
		}
		_ = info
		panic("not implemented")
	case font.TrueTypeSimple: // TrueType fonts (simple font)
		info, err := truetype.ExtractSimple(r.R, fontDicts)
		if err != nil {
			return nil, err
		}
		widths := make([]float64, 256)
		for c := range widths {
			widths[c] = info.Font.GlyphWidthPDF(info.Encoding[c])
		}
		text := make([][]rune, 256)
		if info.ToUnicode != nil {
			text = info.ToUnicode.GetSimpleMapping()
		}
		// TODO: other methods for extracting the text mapping
		res := &fromFileSimple{
			Res:      res,
			widths:   widths,
			encoding: info.Encoding,
			text:     text,
			fontData: info.Font,
		}
		return res, nil
	case font.Type3: // Type 3 fonts
		info, err := type3.Extract(r.R, fontDicts)
		if err != nil {
			return nil, err
		}
		_ = info
		panic("not implemented")
	default:
		panic("unknown font type")
	}
}

type fromFileSimple struct {
	font.Res
	widths   []float64
	encoding []glyph.ID
	text     [][]rune
	fontData interface{}
	key      pdf.Reference
}

func (f *fromFileSimple) WritingMode() int {
	return 0
}

func (f *fromFileSimple) ForeachWidth(s pdf.String, yield func(width float64, is_space bool)) {
	for _, c := range s {
		yield(f.widths[c], c == ' ')
	}
}

func (f *fromFileSimple) ForeachGlyph(s pdf.String, yield func(gid glyph.ID, text []rune, width float64, is_space bool)) {
	for _, c := range s {
		yield(f.encoding[c], f.text[c], f.widths[c], c == ' ')
	}
}

func (f *fromFileSimple) FontData() interface{} {
	return f.fontData
}

func (f *fromFileSimple) Key() pdf.Reference {
	return f.key
}

type fromFileComposite struct {
	font.Res
	cs          charcode.CodeSpaceRange
	writingMode int
	width       map[string]float64
	glyph       map[string]glyphData
	dw          float64
	fontData    interface{}
}

type glyphData struct {
	gid  glyph.ID
	text []rune
}

func (f *fromFileComposite) WritingMode() int {
	return f.writingMode
}

func (f *fromFileComposite) ForeachWidth(s pdf.String, yield func(width float64, is_space bool)) {
	f.cs.AllCodes(s)(func(code pdf.String, valid bool) bool {
		w, ok := f.width[string(code)]
		if !ok {
			w = f.dw
		}
		yield(w, len(code) == 1 && code[0] == ' ')
		return true
	})
}

func (f *fromFileComposite) ForeachGlyph(s pdf.String, yield func(gid glyph.ID, text []rune, width float64, is_space bool)) {
	f.cs.AllCodes(s)(func(code pdf.String, valid bool) bool {
		w, ok := f.width[string(code)]
		if !ok {
			w = f.dw
		}
		g := f.glyph[string(code)]
		yield(g.gid, g.text, w, len(code) == 1 && code[0] == ' ')
		return true
	})
}

func (f *fromFileComposite) FontData() interface{} {
	return f.fontData
}
