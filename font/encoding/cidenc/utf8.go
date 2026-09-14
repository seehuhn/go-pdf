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
	"sync"

	"golang.org/x/text/unicode/norm"

	"seehuhn.de/go/postscript/cid"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/cmap"
)

var _ CIDEncoder = (*compositeUTF8)(nil)

type compositeUTF8 struct {
	wMode font.WritingMode

	codec     *charcode.Codec
	cid0Width float64

	// mu guards the fields below.  Readers hold it for one lookup at a time,
	// never across a yield.
	mu          sync.RWMutex
	info        map[charcode.Code]*codeInfo
	code        map[key]charcode.Code
	nextPrivate rune
}

func NewCompositeUtf8(cid0Width float64, wMode font.WritingMode) CIDEncoder {
	codec, err := charcode.NewCodec(charcode.UTF8)
	if err != nil {
		panic(err)
	}
	e := &compositeUTF8{
		wMode:       wMode,
		codec:       codec,
		info:        make(map[charcode.Code]*codeInfo),
		cid0Width:   cid0Width,
		code:        make(map[key]charcode.Code),
		nextPrivate: 0x00_E000,
	}
	return e
}

func (e *compositeUTF8) WritingMode() font.WritingMode {
	return e.wMode
}

func (e *compositeUTF8) Codes(s pdf.String) iter.Seq[font.Code] {
	// TODO(voss): track per-glyph W2 metrics so that font.Code.VerticalAdvance
	// can be populated for vertical writing mode.  Currently the encoder only
	// stores the horizontal width, so vertical advance falls back to the spec
	// default DW2 in the reader/builder.
	return func(yield func(font.Code) bool) {
		var code font.Code
		for len(s) > 0 {
			c, k, valid := e.codec.Decode(s)

			var info *codeInfo
			if valid {
				info = e.lookup(c)
			}
			if info != nil { // code is mapped to a CID
				code.CID = info.CID
				code.Width = info.Width / 1000
				code.Text = info.Text
			} else { // unmapped or invalid code
				code.CID = 0
				code.Width = e.cid0Width / 1000
				code.Text = ""
			}

			code.UseWordSpacing = (k == 1 && c == 0x20)

			if !yield(code) {
				break
			}

			s = s[k:]
		}
	}
}

func (e *compositeUTF8) Codec() *charcode.Codec {
	return e.codec
}

func (e *compositeUTF8) GetCode(cid cid.CID, text string) (charcode.Code, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	key := key{cid, text}
	code, ok := e.code[key]
	return code, ok
}

// Encode returns the code for a CID and text, allocating a new one on first
// use.  The width is used only when a code is allocated.
func (e *compositeUTF8) Encode(cidVal cid.CID, text string, width float64) (charcode.Code, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	key := key{cidVal, text}
	if code, ok := e.code[key]; ok {
		return code, nil
	}

	code, err := e.makeCode(text)
	if err != nil {
		return 0, err
	}

	e.info[code] = &codeInfo{CID: cidVal, Width: width, Text: text}
	e.code[key] = code

	return code, nil
}

func (e *compositeUTF8) makeCode(text string) (charcode.Code, error) {
	if rr := []rune(norm.NFC.String(text)); len(rr) == 1 {
		code := runeToCode(rr[0])
		if _, alreadUsed := e.info[code]; !alreadUsed {
			return code, nil
		}
	}

	for {
		r := e.nextPrivate
		e.nextPrivate++
		switch e.nextPrivate {
		case 0x00_F900:
			e.nextPrivate = 0x0F_0000
		case 0x0F_FFFE:
			e.nextPrivate = 0x10_0000
		case 0x10_FFFE:
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

// lookup returns the entry for a code, or nil for a code not allocated.  The
// entry stays valid after the lock is released, because entries are never
// changed once stored.
func (e *compositeUTF8) lookup(c charcode.Code) *codeInfo {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.info[c]
}

func (e *compositeUTF8) CMap(ros *cid.SystemInfo) *cmap.File {
	e.mu.RLock()
	m := make(map[charcode.Code]cid.CID, len(e.info))
	for c, info := range e.info {
		m[c] = info.CID
	}
	e.mu.RUnlock()
	cmapInfo := &cmap.File{
		Name:  "",
		ROS:   ros,
		WMode: e.wMode,
	}
	cmapInfo.SetMapping(e.Codec(), m)
	cmapInfo.UpdateName()
	return cmapInfo
}

func (e *compositeUTF8) Width(c charcode.Code) float64 {
	if info := e.lookup(c); info != nil {
		return info.Width
	}
	return e.cid0Width
}

func (e *compositeUTF8) MappedCodes() iter.Seq2[charcode.Code, *Info] {
	return func(yield func(charcode.Code, *Info) bool) {
		// the map is walked under the lock and yielded from outside it
		e.mu.RLock()
		type entry struct {
			c    charcode.Code
			info *codeInfo
		}
		entries := make([]entry, 0, len(e.info))
		for c, info := range e.info {
			entries = append(entries, entry{c, info})
		}
		e.mu.RUnlock()

		var code Info
		for _, en := range entries {
			code.CID = en.info.CID
			code.Width = en.info.Width
			code.Text = en.info.Text
			if !yield(en.c, &code) {
				return
			}
		}
	}
}

func (e *compositeUTF8) ToUnicode() *cmap.ToUnicodeFile {
	e.mu.RLock()
	m := make(map[charcode.Code]string, len(e.info))
	for c, info := range e.info {
		m[c] = info.Text
	}
	e.mu.RUnlock()

	toUnicode, err := cmap.NewToUnicodeFile(charcode.UTF8, m)
	if err != nil {
		panic("unreachable")
	}

	return toUnicode
}

func (e *compositeUTF8) CodesRemaining() int {
	const num_surrogates = 0xDFFF - 0xD800 + 1
	const num_range = 0x10_FFFF - 0x00_0000 + 1
	e.mu.RLock()
	defer e.mu.RUnlock()
	return num_range - num_surrogates - len(e.info)
}
