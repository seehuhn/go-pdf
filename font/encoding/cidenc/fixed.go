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
	text  map[charcode.Code]string
	width map[cid.CID]float64
}

var _ CIDEncoder = (*fixed)(nil)

// NewFromCMap creates a CIDEncoder from an existing CMap.
func NewFromCMap(cmap *cmap.File, cid0Width float64) (CIDEncoder, error) {
	codec, err := charcode.NewCodec(cmap.CodeSpaceRange)
	if err != nil {
		return nil, err
	}
	width := make(map[cid.CID]float64)
	width[0] = cid0Width
	return &fixed{
		cmap:  cmap,
		codec: codec,
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
		var buf []byte
		for len(s) > 0 {
			c, k, valid := f.codec.Decode(s)
			if !valid {
				k = 1
				c = 0
			}

			if valid {
				buf = f.codec.AppendCode(buf[:0], c)
				codeCID := f.cmap.LookupCID(buf)
				code.CID = codeCID
				code.Width = f.width[codeCID]
				code.Text = f.text[c]
			} else {
				code.CID = 0
				code.Width = f.width[0]
				code.Text = ""
			}
			code.Notdef = 0
			code.UseWordSpacing = k == 1 && c == 0x20

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
		for _, single := range f.cmap.CIDSingles {
			var code charcode.Code
			for i, b := range single.Code {
				code |= charcode.Code(b) << (8 * i)
			}
			info.CID = single.Value
			info.Width = f.width[single.Value]
			info.Text = f.text[code]
			if !yield(code, &info) {
				return
			}
		}
		for _, cidRange := range f.cmap.CIDRanges {
			var firstCode charcode.Code
			for i, b := range cidRange.First {
				firstCode |= charcode.Code(b) << (8 * i)
			}
			var lastCode charcode.Code
			for i, b := range cidRange.Last {
				lastCode |= charcode.Code(b) << (8 * i)
			}
			for code := firstCode; code <= lastCode; code++ {
				offset := code - firstCode
				cidVal := cidRange.Value + cid.CID(offset)
				info.CID = cidVal
				info.Width = f.width[cidVal]
				info.Text = f.text[code]
				if !yield(code, &info) {
					return
				}
			}
		}
	}
}

// AllocateCode assigns a new code to a CID and stores the text and width.
func (f *fixed) AllocateCode(cidVal cid.CID, text string, width float64) (charcode.Code, error) {
	code, found := f.GetCode(cidVal, text)
	if !found {
		return 0, errors.New("CID not found in fixed encoder")
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
	for _, single := range f.cmap.CIDSingles {
		if single.Value == cidVal {
			var code charcode.Code
			for i, b := range single.Code {
				code |= charcode.Code(b) << (8 * i)
			}
			return code, true
		}
	}
	for _, cidRange := range f.cmap.CIDRanges {
		var firstCode charcode.Code
		for i, b := range cidRange.First {
			firstCode |= charcode.Code(b) << (8 * i)
		}
		var lastCode charcode.Code
		for i, b := range cidRange.Last {
			lastCode |= charcode.Code(b) << (8 * i)
		}
		rangeSize := lastCode - firstCode + 1
		if cidVal >= cidRange.Value && cidVal < cidRange.Value+cid.CID(rangeSize) {
			offset := cidVal - cidRange.Value
			return firstCode + charcode.Code(offset), true
		}
	}
	return 0, false
}

// Width returns the width of the given character code.
func (f *fixed) Width(code charcode.Code) float64 {
	var buf []byte
	buf = f.codec.AppendCode(buf[:0], code)
	cidVal := f.cmap.LookupCID(buf)
	return f.width[cidVal]
}

// ToUnicode returns a ToUnicode CMap representing the text content
// of the mapped codes.
func (f *fixed) ToUnicode() *cmap.ToUnicodeFile {
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
