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

package cidenc

import (
	"iter"

	"golang.org/x/text/unicode/norm"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/postscript/cid"
)

var _ Encoding = (*CompositeUTF8)(nil)

type CompositeUTF8 struct {
	wMode cmap.WritingMode

	codec     *charcode.Codec
	info      map[charcode.Code]*codeInfo
	cid0Width float64

	code map[cidText]charcode.Code

	nextPrivate rune
}

func NewCompositeUtf8(cid0Width float64, wMode cmap.WritingMode) *CompositeUTF8 {
	codec, err := charcode.NewCodec(utf8cs)
	if err != nil {
		panic(err)
	}
	e := &CompositeUTF8{
		wMode:       wMode,
		codec:       codec,
		info:        make(map[charcode.Code]*codeInfo),
		cid0Width:   cid0Width,
		code:        make(map[cidText]charcode.Code),
		nextPrivate: 0x00_E000,
	}
	return e
}

// WritingMode indicates whether the font is for horizontal or vertical
// writing.
func (e *CompositeUTF8) WritingMode() cmap.WritingMode {
	return e.wMode
}

func (e *CompositeUTF8) DecodeWidth(s pdf.String) (float64, int) {
	for code := range e.Codes(s) {
		return code.Width / 1000, len(s)
	}
	return 0, 0
}

// Codes iterates over the character codes in a PDF string.
func (e *CompositeUTF8) Codes(s pdf.String) iter.Seq[*font.Code] {
	return func(yield func(yield *font.Code) bool) {
		var code font.Code
		for len(s) > 0 {
			c, k, valid := e.codec.Decode(s)

			if valid {
				info := e.info[c]
				if info != nil { // code is mapped to a CID
					code.CID = info.CID
					code.Width = info.Width
					code.Text = info.Text
				} else { // unmapped code
					code.CID = 0
					code.Width = e.cid0Width
					code.Text = ""
				}
			} else { // invalid code
				code.CID = 0
				code.Width = e.cid0Width
				code.Text = ""
			}

			if !yield(&code) {
				break
			}

			s = s[k:]
		}
	}
}

func (e *CompositeUTF8) GetCode(cid cid.CID, text string) (charcode.Code, bool) {
	key := cidText{cid, text}
	code, ok := e.code[key]
	return code, ok
}

// AllocateCode allocates a new code for the given character ID and text.
//
// The new code is chosen using based on the text, and equals the utf-8
// encoding where possible.
//
// The last argument is the width of the glyph in PDF glyph space units.
func (e *CompositeUTF8) AllocateCode(cidVal cid.CID, text string, width float64) (charcode.Code, error) {
	key := cidText{cidVal, text}
	if _, ok := e.code[key]; ok {
		return 0, ErrDuplicateCode
	}

	code, err := e.makeCode(text)
	if err != nil {
		return 0, err
	}

	e.info[code] = &codeInfo{CID: cidVal, Width: width, Text: text}
	e.code[key] = code

	return code, nil
}

func (e *CompositeUTF8) makeCode(text string) (charcode.Code, error) {
	if rr := []rune(norm.NFC.String(text)); len(rr) == 1 {
		code := runeToCode(rr[0])
		if _, alreadUsed := e.info[code]; !alreadUsed {
			return code, nil
		}
	}

	for {
		r := e.nextPrivate
		e.nextPrivate++
		if e.nextPrivate == 0x00_F900 {
			e.nextPrivate = 0x0F_0000
		} else if e.nextPrivate == 0x0F_FFFE {
			e.nextPrivate = 0x10_0000
		} else if e.nextPrivate == 0x10_FFFE {
			return 0, ErrOverflow
		}
		code := runeToCode(r)
		if _, alreadUsed := e.info[code]; !alreadUsed {
			return code, nil
		}
	}
}

func runeToCode(r rune) charcode.Code {
	var code charcode.Code
	for i, b := range []byte(string(r)) {
		code |= charcode.Code(b) << (8 * i)
	}
	return code
}

// utf8cs represents UTF-8-encoded character codes.
var utf8cs = charcode.CodeSpaceRange{
	{Low: []byte{0x00}, High: []byte{0x7F}},
	{Low: []byte{0xC2, 0x80}, High: []byte{0xDF, 0xBF}},
	{Low: []byte{0xE0, 0x80, 0x80}, High: []byte{0xEF, 0xBF, 0xBF}},
	{Low: []byte{0xF0, 0x80, 0x80, 0x80}, High: []byte{0xF4, 0xBF, 0xBF, 0xBF}},
}
