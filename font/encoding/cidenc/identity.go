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

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/postscript/cid"
)

var _ Encoding = (*CompositeIdentity)(nil)

type CompositeIdentity struct {
	wMode cmap.WritingMode

	codec  *charcode.Codec
	info   map[charcode.Code]*codeInfo
	notdef []notdefRange
	cid0   *notdefInfo

	code map[cidText]charcode.Code
}

func NewCompositeIdentity(cid0Width float64, wMode cmap.WritingMode) *CompositeIdentity {
	codec, err := charcode.NewCodec(charcode.UCS2)
	if err != nil {
		panic(err)
	}
	e := &CompositeIdentity{
		wMode:  wMode,
		codec:  codec,
		info:   make(map[charcode.Code]*codeInfo),
		notdef: nil, // TODO(voss)
		cid0: &notdefInfo{
			CID:   0,
			Width: cid0Width,
		},
		code: make(map[cidText]charcode.Code),
	}
	return e
}

// WritingMode indicates whether the font is for horizontal or vertical
// writing.
func (e *CompositeIdentity) WritingMode() cmap.WritingMode {
	return e.wMode
}

func (e *CompositeIdentity) DecodeWidth(s pdf.String) (float64, int) {
	for code := range e.Codes(s) {
		return code.Width / 1000, len(s)
	}
	return 0, 0
}

// Codes iterates over the character codes in a PDF string.
func (e *CompositeIdentity) Codes(s pdf.String) iter.Seq[*font.Code] {
	return func(yield func(yield *font.Code) bool) {
		var code font.Code
		for len(s) > 0 {
			c, k, valid := e.codec.Decode(s)

			if valid {
				info := e.info[c]
				notdef := lookup(e.notdef, c)
				if notdef == nil {
					notdef = e.cid0
				}
				if info != nil { // code is mapped to a CID
					code.CID = info.CID
					code.Width = info.Width
					code.Text = info.Text
					code.Notdef = notdef.CID
				} else { // unmapped code
					code.CID = notdef.CID
					code.Width = notdef.Width
					code.Text = ""
					code.Notdef = 0
				}
			} else { // invalid code
				code.CID = e.cid0.CID
				code.Width = e.cid0.Width
				code.Text = ""
				code.Notdef = 0
			}

			if !yield(&code) {
				break
			}

			s = s[k:]
		}
	}
}

func (e *CompositeIdentity) GetCode(cid cid.CID, text string) (charcode.Code, bool) {
	key := cidText{cid, text}
	code, ok := e.code[key]
	return code, ok
}

// AllocateCode allocates a new code for the given character ID and text.
//
// The last argument is the width of the glyph in PDF glyph space units.
func (e *CompositeIdentity) AllocateCode(cidVal cid.CID, text string, width float64) (charcode.Code, error) {
	key := cidText{cidVal, text}
	if _, ok := e.code[key]; ok {
		return 0, ErrDuplicateCode
	}

	if cidVal >= 0x01_0000 {
		return 0, ErrOverflow
	}

	code := charcode.Code(cidVal&0xFF00)>>8 | charcode.Code(cidVal&0x00FF)<<8

	e.info[code] = &codeInfo{CID: cidVal, Width: width, Text: text}
	e.code[key] = code

	return code, nil
}
