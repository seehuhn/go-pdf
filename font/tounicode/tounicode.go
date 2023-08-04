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
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/postscript/type1"
)

type Info struct {
	Name      pdf.Name
	ROS       *type1.CIDSystemInfo
	CodeSpace charcode.CodeSpaceRange
	Singles   []Single
	Ranges    []Range
}

// Single specifies that character code Code represents the given unicode string.
type Single struct {
	Code  charcode.CharCode
	Value []rune
}

// Range describes a range of character codes.
// First and Last are the first and last code points in the range.
// Values is a list of unicode strings.  If the list has length one, then the
// replacement character is incremented by one for each code point in the
// range.  Otherwise, the list must have the length Last-First+1, and specify
// the value for each code point in the range.
type Range struct {
	First  charcode.CharCode
	Last   charcode.CharCode
	Values [][]rune
}

func Embed(w pdf.Putter, ref pdf.Reference, cs charcode.CodeSpaceRange, m map[charcode.CharCode][]rune) error {
	// TODO(voss): is the CIDSystemInfo correct?
	touni := &Info{
		Name: makeName(m),
		ROS: &type1.CIDSystemInfo{
			Registry:   "Adobe",
			Ordering:   "UCS",
			Supplement: 0,
		},
		CodeSpace: cs,
	}
	touni.FromMapping(m)
	touniStream, err := w.OpenStream(ref, nil, pdf.FilterCompress{})
	if err != nil {
		return err
	}
	err = touni.Write(touniStream)
	if err != nil {
		return fmt.Errorf("embedding ToUnicode cmap: %w", err)
	}
	return touniStream.Close()
}
