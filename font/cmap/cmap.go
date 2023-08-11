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

package cmap

import (
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/postscript/type1"
)

// https://adobe-type-tools.github.io/font-tech-notes/pdfs/5014.CIDFont_Spec.pdf
// https://adobe-type-tools.github.io/font-tech-notes/pdfs/5099.CMapResources.pdf

type Info struct {
	Name    string
	Version float64
	ROS     *type1.CIDSystemInfo
	CS      charcode.CodeSpaceRange
	CSFile  charcode.CodeSpaceRange
	WMode   int
	UseCMap string
	Singles []Single
	Ranges  []Range
}

// Single specifies that character code Code represents the given CID.
type Single struct {
	Code  charcode.CharCode
	Value type1.CID
}

// Range describes a range of character codes with consecutive CIDs.
// First and Last are the first and last code points in the range.
// Value is the CID of the first code point in the range.
type Range struct {
	First charcode.CharCode
	Last  charcode.CharCode
	Value type1.CID
}
