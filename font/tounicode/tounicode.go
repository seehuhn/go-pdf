// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2023  Jochen Voss <voss@seehuhn.de>
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

package tounicode

import (
	"fmt"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/sfnt/type1"
)

// https://adobe-type-tools.github.io/font-tech-notes/pdfs/5014.CIDFont_Spec.pdf
// https://adobe-type-tools.github.io/font-tech-notes/pdfs/5099.CMapResources.pdf
// https://www.adobe.com/content/dam/acom/en/devnet/acrobat/pdfs/5411.ToUnicode.pdf

// Info describes a mapping from character indices to Unicode character
// sequences.
type Info struct {
	CodeSpace []CodeSpaceRange
	Singles   []Single
	Ranges    []Range

	Name pdf.Name
	ROS  *type1.CIDSystemInfo
}

func (info *Info) containsCode(code type1.CID) bool {
	for _, r := range info.CodeSpace {
		if r.First <= code && code <= r.Last {
			return true
		}
	}
	return false
}

func (info *Info) containsRange(first, last type1.CID) bool {
	for _, r := range info.CodeSpace {
		if r.First <= first && last <= r.Last {
			return true
		}
	}
	return false
}

// A CodeSpaceRange describes a range of character codes, like "<00> <FF>"
// for one-byte codes or "<0000> <FFFF>" for two-byte codes.
type CodeSpaceRange struct {
	First type1.CID
	Last  type1.CID
}

func (c CodeSpaceRange) String() string {
	var format string
	if c.Last >= 1<<24 {
		format = "%08X"
	} else if c.Last >= 1<<16 {
		format = "%06X"
	} else if c.Last >= 1<<8 {
		format = "%04X"
	} else {
		format = "%02X"
	}
	return fmt.Sprintf("<"+format+"> <"+format+">", c.First, c.Last)
}

// Single describes a single character code.
// It specifies that character code Code represents the given UTF16-encoded
// unicode string.
type Single struct {
	Code  type1.CID
	UTF16 []uint16
}

// Range describes a range of character codes.
// First and Last are the first and last code points in the range. UTF16 is a
// list of UTF16-encoded unicode strings.  If the list has length one, then the
// replacement character is incremented by one for each code point in the
// range.  Otherwise, the list must have the length Last-First+1, and specify
// the UTF16 encoding for each code point in the range.
type Range struct {
	First type1.CID
	Last  type1.CID
	UTF16 [][]uint16
}
