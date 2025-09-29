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
	"errors"
	"iter"

	"seehuhn.de/go/postscript/cid"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/mapping"
)

// fixed represents a CIDEncoder that uses a fixed CMap.
// No new codes can be allocated.
// Text and width information can be added.
type fixed struct {
	cmap  *cmap.File
	codec *charcode.Codec
	all   map[cid.CID]charcode.Code
	rev   map[charcode.Code]cid.CID
	text  map[charcode.Code]string
	width map[cid.CID]float64
}

var _ CIDEncoder = (*fixed)(nil)

// NewFromCMap creates a CIDEncoder from an existing CMap.
// This returns an error, if the CMap has an invalid code space range.
func NewFromCMap(cmap *cmap.File, cid0Width float64) (CIDEncoder, error) {
	codec, err := cmap.Codec()
	if err != nil {
		return nil, err
	}

	all := make(map[cid.CID]charcode.Code)
	rev := make(map[charcode.Code]cid.CID)
	for code, cid := range cmap.All(codec) {
		all[cid] = code
		rev[code] = cid
	}

	width := make(map[cid.CID]float64)
	width[0] = cid0Width

	return &fixed{
		cmap:  cmap,
		codec: codec,
		all:   all,
		rev:   rev,
		text:  make(map[charcode.Code]string),
		width: width,
	}, nil
}

// WritingMode indicates whether the font is for horizontal or vertical
// writing.
func (f *fixed) WritingMode() font.WritingMode {
	return f.cmap.WMode
}

// Codes iterates over the character codes in a PDF string.
// The iterator returns the information stored for each code.
func (f *fixed) Codes(s pdf.String) iter.Seq[*font.Code] {
	return func(yield func(*font.Code) bool) {
		var code font.Code
		for len(s) > 0 {
			c, k, valid := f.codec.Decode(s)
			if !valid {
				k = 1
				c = 0
			}

			if valid {
				cid := f.rev[c]
				code = font.Code{
					CID: cid,
					// Notdef:         ...
					Width:          f.width[cid] / 1000,
					Text:           f.text[c],
					UseWordSpacing: k == 1 && c == 0x20,
				}
			} else {
				code = font.Code{
					CID: 0,
					// Notdef:         ...,
					Width:          f.width[0] / 1000,
					Text:           f.text[c],
					UseWordSpacing: k == 1 && c == 0x20,
				}
			}

			if !yield(&code) {
				return
			}
			s = s[k:]
		}
	}
}

// MappedCodes iterates over all codes known to the encoder.
func (f *fixed) MappedCodes() iter.Seq2[charcode.Code, *Info] {
	return func(yield func(charcode.Code, *Info) bool) {
		var info Info
		for code, text := range f.text {
			cid := f.rev[code]
			info = Info{
				CID:   cid,
				Width: f.width[cid],
				Text:  text,
			}

			if !yield(code, &info) {
				break
			}
		}
	}
}

// Encode assigns a new code to a CID and stores the text and width.
func (f *fixed) Encode(cidVal cid.CID, text string, width float64) (charcode.Code, error) {
	code, ok := f.all[cidVal]
	if !ok {
		return 0, errors.New("CID not found in CMap")
	}

	if existingWidth, hasWidth := f.width[cidVal]; hasWidth {
		if existingWidth != width {
			return 0, errors.New("width already set to different value")
		}
	} else {
		f.width[cidVal] = width
	}

	if existingText, hasText := f.text[code]; hasText {
		if existingText != text {
			return 0, errors.New("text already set to different value")
		}
	} else {
		f.text[code] = text
	}

	return code, nil
}

// CMap returns the underlying CMap.
func (f *fixed) CMap(ros *cid.SystemInfo) *cmap.File {
	return f.cmap
}

// Codec returns the character code codec.
func (f *fixed) Codec() *charcode.Codec {
	return f.codec
}

// GetCode returns the character code for the given CID.
func (f *fixed) GetCode(cidVal cid.CID, text string) (charcode.Code, bool) {
	if _, ok := f.width[cidVal]; !ok {
		return 0, false
	}
	return f.all[cidVal], true
}

// Width returns the width of the given character code.
func (f *fixed) Width(code charcode.Code) float64 {
	return f.width[f.rev[code]]
}

// ToUnicode returns a ToUnicode CMap representing the text content
// of the mapped codes.
func (f *fixed) ToUnicode() *cmap.ToUnicodeFile {
	// TODO(voss): for the /Identity-H and /Identity-V CMaps,
	// we need to use the CIDSystemInfo of the font, rather than
	// the one from the CMap here.

	m := make(map[charcode.Code]string)

	implied, _ := mapping.GetCIDTextMapping(f.cmap.ROS.Registry, f.cmap.ROS.Ordering)

	var buf []byte
	for code, text := range f.text {
		if text == "" {
			continue
		}

		buf = f.codec.AppendCode(buf[:0], code)
		cidVal := f.cmap.LookupCID(buf)
		if text == implied[cidVal] {
			continue
		}

		m[code] = text
	}

	if len(m) == 0 {
		return nil
	}

	// We already checked that f.cmap.CodeSpaceRange is valid,
	// in NewFromCMap, so we will never get an error here.
	toUnicode, _ := cmap.NewToUnicodeFile(f.cmap.CodeSpaceRange, m)
	return toUnicode
}

func (f *fixed) CodesRemaining() int {
	return 0
}
