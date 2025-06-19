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

	"seehuhn.de/go/postscript/cid"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/cmap"
)

var _ CIDEncoder = (*compositeIdentity)(nil)

type compositeIdentity struct {
	wMode font.WritingMode

	codec  *charcode.Codec
	info   map[charcode.Code]*codeInfo
	notdef []notdefRange
	cid0   *notdefInfo

	code map[key]charcode.Code
}

func NewCompositeIdentity(cid0Width float64, wMode font.WritingMode) CIDEncoder {
	codec, err := charcode.NewCodec(charcode.UCS2)
	if err != nil {
		panic(err)
	}
	e := &compositeIdentity{
		wMode:  wMode,
		codec:  codec,
		info:   make(map[charcode.Code]*codeInfo),
		notdef: nil, // TODO(voss)
		cid0: &notdefInfo{
			CID:   0,
			Width: cid0Width,
		},
		code: make(map[key]charcode.Code),
	}
	return e
}

func (e *compositeIdentity) WritingMode() font.WritingMode {
	return e.wMode
}

func (e *compositeIdentity) Codes(s pdf.String) iter.Seq[*font.Code] {
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
					code.Notdef = notdef.CID
					code.Text = info.Text
				} else { // unmapped code
					code.CID = notdef.CID
					code.Width = notdef.Width
					code.Notdef = 0
					code.Text = ""
				}
			} else { // invalid code
				code.CID = e.cid0.CID
				code.Width = e.cid0.Width
				code.Notdef = 0
				code.Text = ""
			}

			if !yield(&code) {
				break
			}

			s = s[k:]
		}
	}
}

func (e *compositeIdentity) Codec() *charcode.Codec {
	return e.codec
}

func (e *compositeIdentity) GetCode(cid cid.CID, text string) (charcode.Code, bool) {
	key := key{cid, text}
	code, ok := e.code[key]
	return code, ok
}

func (e *compositeIdentity) AllocateCode(cidVal cid.CID, text string, width float64) (charcode.Code, error) {
	key := key{cidVal, text}
	if _, ok := e.code[key]; ok {
		return 0, ErrDuplicateCode
	}

	code, err := e.makeCode(cidVal)
	if err != nil {
		return 0, err
	}

	e.info[code] = &codeInfo{CID: cidVal, Width: width, Text: text}
	e.code[key] = code

	return code, nil
}

func (e *compositeIdentity) makeCode(cidVal cid.CID) (charcode.Code, error) {
	if cidVal >= 0x01_0000 {
		return 0, ErrOverflow
	}

	return charcode.Code(cidVal&0xFF00)>>8 | charcode.Code(cidVal&0x00FF)<<8, nil
}

func (e *compositeIdentity) get(c charcode.Code) *codeInfo {
	if info, ok := e.info[c]; ok {
		return info
	}
	return &codeInfo{
		CID:   0,
		Width: e.cid0.Width,
		Text:  "",
	}
}

func (e *compositeIdentity) CMap(*cid.SystemInfo) *cmap.File {
	var name string
	if e.wMode == font.Vertical {
		name = "Identity-V"
	} else {
		name = "Identity-H"
	}
	return cmap.Predefined(name)
}

func (e *compositeIdentity) Width(c charcode.Code) float64 {
	return e.get(c).Width
}

func (e *compositeIdentity) MappedCodes() iter.Seq2[charcode.Code, *Info] {
	return func(yield func(charcode.Code, *Info) bool) {
		var code Info
		for c, info := range e.info {
			code.CID = info.CID
			code.Width = info.Width
			code.Text = info.Text
			if !yield(c, &code) {
				return
			}
		}
	}
}

func (e *compositeIdentity) ToUnicode() *cmap.ToUnicodeFile {
	m := make(map[charcode.Code]string, len(e.info))
	for c, info := range e.info {
		m[c] = info.Text
	}

	toUnicode, err := cmap.NewToUnicodeFile(charcode.UCS2, m)
	if err != nil {
		panic("unreachable")
	}

	return toUnicode
}
