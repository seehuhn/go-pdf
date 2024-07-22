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
	"os"
	"sort"

	"golang.org/x/exp/maps"

	"seehuhn.de/go/postscript/type1/names"

	sfntcff "seehuhn.de/go/sfnt/cff"
	"seehuhn.de/go/sfnt/glyph"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cff"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/loader"
	"seehuhn.de/go/pdf/font/opentype"
	"seehuhn.de/go/pdf/font/truetype"
	"seehuhn.de/go/pdf/font/type1"
	"seehuhn.de/go/pdf/font/type3"
)

// FontFromFile represents a font which has been extracted from a PDF file.
type FontFromFile interface {
	font.Font

	ForeachGlyph(s pdf.String, yield func(gid glyph.ID, text []rune, width float64, isSpace bool))

	FontData() interface{}

	Key() pdf.Reference
}

// ReadFont extracts a font from a PDF file.
func (r *Reader) ReadFont(ref pdf.Object, name pdf.Name) (F FontFromFile, err error) {
	if ref, ok := ref.(pdf.Reference); ok {
		if res, ok := r.fontCache[ref]; ok {
			return res, nil
		}
		defer func() {
			if err == nil {
				r.fontCache[ref] = F
			}
		}()
	}

	fontDicts, err := font.ExtractDicts(r.R, ref)
	if err != nil {
		return nil, err
	}

	if L := r.loader; L != nil && fontDicts.FontProgram == nil {
		tp := loader.FontTypeType1 // TODO(voss): try all supported types
		if stm, err := L.Open(string(fontDicts.PostScriptName), tp); err == nil {
			fontDicts.Type = font.Type1
			fontDicts.FontProgram = &pdf.Stream{R: stm}
			fontDicts.FontProgramRef = pdf.NewInternalReference(r.nextInternalRef)
			r.nextInternalRef++
		} else if !os.IsNotExist(err) {
			return nil, err
		}
	}

	res := pdf.Res{
		Data: ref,
	}

	// TODO(voss): make this less repetitive
	switch fontDicts.Type {
	case font.Type1: // Type 1 fonts (including the standard 14 fonts)
		info, err := type1.Extract(r.R, fontDicts)
		if err != nil {
			return nil, err
		}
		widths := info.GetWidths()
		glyphNames := info.GlyphList()
		rev := make(map[string]glyph.ID, len(glyphNames))
		for i, name := range glyphNames {
			rev[name] = glyph.ID(i)
		}
		encoding := make([]glyph.ID, 256)
		for c, name := range info.Encoding {
			encoding[c] = rev[name]
		}
		text := make([][]rune, 256)
		if info.ToUnicode != nil {
			text = info.ToUnicode.GetSimpleMapping()
		} else {
			for c, name := range info.Encoding {
				text[c] = names.ToUnicode(name, fontDicts.PostScriptName == "ZapfDingbats")
			}
		}
		res := &fromFileSimple{
			Res:      res,
			widths:   widths,
			encoding: encoding,
			text:     text,
			fontData: info.Font,
			key:      fontDicts.FontProgramRef,
		}
		return res, nil

	case font.CFFComposite: // CFF font data without wrapper (composite font)
		info, err := cff.ExtractComposite(r.R, fontDicts)
		if err != nil {
			return nil, err
		}
		_ = info
		panic("not implemented") // TODO(voss): implement

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
		} else {
			for c, gid := range info.Encoding {
				name := info.Font.Glyphs[gid].Name
				text[c] = names.ToUnicode(name, fontDicts.PostScriptName == "ZapfDingbats")
			}
		}
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
		panic("not implemented") // TODO(voss): implement

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
		} else {
			outlines := info.Font.Outlines.(*sfntcff.Outlines)
			for c, gid := range info.Encoding {
				name := outlines.Glyphs[gid].Name
				text[c] = names.ToUnicode(name, fontDicts.PostScriptName == "ZapfDingbats")
			}
		}
		// TODO(voss): other methods for extracting the text mapping
		res := &fromFileSimple{
			Res:      res,
			widths:   widths,
			encoding: info.Encoding,
			text:     text,
			fontData: info.Font,
			key:      fontDicts.FontProgramRef,
		}
		return res, nil

	case font.OpenTypeGlyfComposite: // OpenType fonts with glyf outline (composite font)
		info, err := opentype.ExtractGlyfComposite(r.R, fontDicts)
		if err != nil {
			return nil, err
		}
		_ = info
		panic("not implemented") // TODO(voss): implement

	case font.OpenTypeGlyfSimple: // OpenType fonts with glyf outline (simple font)
		info, err := opentype.ExtractGlyfSimple(r.R, fontDicts)
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
		// TODO: other methods for extracting the text mapping???
		res := &fromFileSimple{
			Res:      res,
			widths:   widths,
			encoding: info.Encoding,
			text:     text,
			fontData: info.Font,
			key:      fontDicts.FontProgramRef,
		}
		return res, nil

	case font.TrueTypeComposite: // TrueType fonts (composite font)
		info, err := truetype.ExtractComposite(r.R, fontDicts)
		if err != nil {
			return nil, err
		}

		glyph := make(map[string]glyphData)
		m := info.CMap.GetMapping()
		if info.ToUnicode == nil {
			// TODO(voss): do something clever here
		}
		tuMap := info.ToUnicode.GetMapping()
		var s pdf.String
		for code, cid := range m {
			s = info.CMap.Append(s[:0], code)
			if int(cid) < len(info.CID2GID) {
				gid := info.CID2GID[cid]
				glyph[string(s)] = glyphData{
					gid:   gid,
					text:  tuMap[code],
					width: info.Font.GlyphWidthPDF(gid),
				}
			}
		}

		res := &fromFileComposite{
			Res:         res,
			cs:          info.CMap.CodeSpaceRange,
			writingMode: info.CMap.WMode,
			glyph:       glyph,
			fontData:    F,
			key:         fontDicts.FontProgramRef,
		}
		return res, nil

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
		// TODO: other methods for extracting the text mapping???
		res := &fromFileSimple{
			Res:      res,
			widths:   widths,
			encoding: info.Encoding,
			text:     text,
			fontData: info.Font,
			key:      fontDicts.FontProgramRef,
		}
		return res, nil

	case font.Type3: // Type 3 fonts
		info, err := type3.Extract(r.R, fontDicts)
		if err != nil {
			return nil, err
		}

		glyphNames := maps.Keys(info.Glyphs)
		glyphNames = append(glyphNames, "")
		sort.Strings(glyphNames)
		rev := make(map[string]glyph.ID, len(glyphNames))
		for i, name := range glyphNames {
			rev[name] = glyph.ID(i)
		}
		encoding := make([]glyph.ID, 256)
		text := make([][]rune, 256)
		for c, name := range info.Encoding {
			encoding[c] = rev[name]
			text[c] = names.ToUnicode(name, false)
		}

		res := &fromFileSimple{
			Res:      res,
			widths:   info.Widths,
			encoding: encoding,
			text:     text,
			fontData: nil,
			key:      fontDicts.FontProgramRef,
		}
		return res, nil

	default:
		// TODO(voss): implement proper error handling
		panic("unknown font type")
	}
}

type fromFileSimple struct {
	pdf.Res
	widths   []float64
	encoding []glyph.ID
	text     [][]rune
	fontData interface{}
	key      pdf.Reference
}

func (f *fromFileSimple) WritingMode() int {
	return 0
}

func (f *fromFileSimple) ForeachWidth(s pdf.String, yield func(width float64, isSpace bool)) {
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

func (f *fromFileSimple) Embed(rm *pdf.ResourceManager) (font.Embedded, error) {
	panic("unreachable")
}

type fromFileComposite struct {
	pdf.Res
	cs          charcode.CodeSpaceRange
	writingMode int
	glyph       map[string]glyphData
	fontData    interface{}
	key         pdf.Reference
}

type glyphData struct {
	gid   glyph.ID
	text  []rune
	width float64
}

func (f *fromFileComposite) WritingMode() int {
	return f.writingMode
}

func (f *fromFileComposite) ForeachWidth(s pdf.String, yield func(width float64, isSpace bool)) {
	f.cs.AllCodes(s)(func(code pdf.String, valid bool) bool {
		// TODO(voss): notdef glyph(s)???
		if g, ok := f.glyph[string(code)]; ok {
			yield(g.width, len(code) == 1 && code[0] == ' ')
		}
		return true
	})
}

func (f *fromFileComposite) ForeachGlyph(s pdf.String, yield func(gid glyph.ID, text []rune, width float64, is_space bool)) {
	f.cs.AllCodes(s)(func(code pdf.String, valid bool) bool {
		// TODO(voss): notdef glyph(s)???
		if g, ok := f.glyph[string(code)]; ok {
			yield(g.gid, g.text, g.width, len(code) == 1 && code[0] == ' ')
		}
		return true
	})
}

func (f *fromFileComposite) FontData() interface{} {
	return f.fontData
}

func (f *fromFileComposite) Key() pdf.Reference {
	return f.key
}

func (f *fromFileComposite) Embed(rm *pdf.ResourceManager) (font.Embedded, error) {
	panic("unreachable")
}
