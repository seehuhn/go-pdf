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
	"fmt"

	"seehuhn.de/go/postscript/type1/names"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/encoding"
	"seehuhn.de/go/pdf/font/pdfenc"
	"seehuhn.de/go/pdf/internal/stdmtx"
)

// FontFromFile represents a font which has been extracted from a PDF file.
type FontFromFile interface {
	font.Embedded

	Decode(s pdf.String) (*font.Code, int)

	FontData() interface{}

	Key() pdf.Reference
}

// ReadFont extracts a font from a PDF file.
func (r *Reader) ReadFont(ref pdf.Object) (F FontFromFile, err error) {
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

	info, err := font.ExtractDicts(r.R, ref)
	if err != nil {
		return nil, err
	}

	toUni, err := cmap.ExtractToUnicode(r.R, info.FontDict["ToUnicode"])
	if err != nil {
		return nil, err
	}

	if info.IsSimple() {
		return r.readSimpleFont(info, toUni)
	} else {
		return r.readCompositeFont(info, toUni)
	}
}

func (r *Reader) readSimpleFont(info *font.Dicts, toUni *cmap.ToUnicodeFile) (F FontFromFile, err error) {
	var enc *encoding.EncodingOld
	switch info.DictType {
	case font.DictTypeSimpleType1:
		enc, err = encoding.ExtractType1Old(r.R, info)
		if err != nil {
			return nil, err
		}
	case font.DictTypeSimpleTrueType:
		enc, err = encoding.ExtractTrueType(r.R, info.FontDict["Encoding"])
		if err != nil {
			return nil, err
		}
	case font.DictTypeType3:
		enc, err = encoding.ExtractType3Old(r.R, info.FontDict["Encoding"])
		if err != nil {
			return nil, err
		}
	}

	var widths []float64
	if info.FontDict["Widths"] == nil && info.IsStandardFont() {
		widths = make([]float64, 256)
		for code := range 256 {
			cid := enc.Decode(byte(code))
			glyphName := enc.GlyphName(cid)
			if glyphName == "" {
				switch info.PostScriptName {
				case "Symbol":
					glyphName = pdfenc.Symbol.Encoding[code]
				case "ZapfDingbats":
					glyphName = pdfenc.ZapfDingbats.Encoding[code]
				default:
					glyphName = pdfenc.Standard.Encoding[code]
				}
			}
			w, err := getStandardWidth(info.PostScriptName, glyphName)
			if err != nil {
				w, _ = getStandardWidth(info.PostScriptName, ".notdef")
			}
			widths[code] = w / 1000
		}
	} else {
		widths, err = r.extractWidths(info)
		if err != nil {
			return nil, err
		}
	}

	res := &SimpleFont{
		enc:    enc,
		info:   make([]*font.Code, 256),
		widths: widths,
		toUni:  toUni,
	}
	return res, nil
}

// getStandardWidth returns the width of glyphs in the 14 standard PDF fonts.
// The width is given in PDF glyph space units (i.e. are multiplied by 1000).
//
// TODO(voss): remove
func getStandardWidth(fontName, glyphName string) (float64, error) {
	m, ok := stdmtx.Metrics[fontName]
	if !ok {
		return 0, fmt.Errorf("unknown standard font %q", fontName)
	}

	w, ok := m.Width[glyphName]
	if !ok {
		return 0, fmt.Errorf("unknown glyph %q in font %q", glyphName, fontName)
	}

	return w, nil
}

func (r *Reader) extractWidths(info *font.Dicts) ([]float64, error) {
	firstChar, err := pdf.GetInteger(r.R, info.FontDict["FirstChar"])
	if err != nil {
		return nil, err
	}
	lastChar, err := pdf.GetInteger(r.R, info.FontDict["LastChar"])
	if err != nil {
		return nil, err
	}

	widths := make([]float64, 256)
	for c := range widths {
		widths[c] = info.FontDescriptor.MissingWidth / 1000
	}
	w, err := pdf.GetArray(r.R, info.FontDict["Widths"])
	if err != nil {
		return nil, err
	}
	if 0 <= firstChar && firstChar <= lastChar && lastChar < 256 {
		for code := firstChar; code <= lastChar; code++ {
			if int(code-firstChar) >= len(w) {
				break
			}
			w, err := pdf.GetNumber(r.R, w[code-firstChar])
			if err != nil {
				return nil, err
			}
			widths[code] = float64(w) / 1000
		}
	}

	return widths, nil
}

type SimpleFont struct {
	enc    *encoding.EncodingOld
	info   []*font.Code
	widths []float64
	toUni  *cmap.ToUnicodeFile
}

func (f *SimpleFont) WritingMode() font.WritingMode {
	return font.Horizontal
}

// DecodeWidth reads one character code from the given string and returns
// the width of the corresponding glyph in PDF text space units (still to
// be multiplied by the font size) and the number of bytes read from the
// string.
func (f *SimpleFont) DecodeWidth(s pdf.String) (float64, int) {
	if len(s) == 0 {
		return 0, 0
	}
	code := s[0]
	return f.widths[code], 1
}

func (f *SimpleFont) Decode(s pdf.String) (*font.Code, int) {
	if len(s) == 0 {
		return nil, 0
	}
	code := s[0]
	if info := f.info[code]; info != nil {
		return info, 1
	}

	cid := f.enc.Decode(code)

	text, found := f.toUni.Lookup(s[:1])
	if !found {
		glyphName := f.enc.GlyphName(cid)
		if glyphName != "" {
			text = string(names.ToUnicode(glyphName, false))
		}
	}
	// TODO(voss): any other methods for extracting the text mapping???

	res := &font.Code{
		CID:    cid,
		Notdef: 0,
		Text:   text,
		Width:  f.widths[code],
	}

	f.info[code] = res
	return res, 1
}

func (f *SimpleFont) FontData() interface{} {
	panic("not implemented") // TODO: Implement
}

func (f *SimpleFont) Key() pdf.Reference {
	panic("not implemented") // TODO: Implement
}
