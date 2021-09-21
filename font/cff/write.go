// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2021  Jochen Voss <voss@seehuhn.de>
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

package cff

import (
	"io"

	"seehuhn.de/go/pdf/font"
)

func (font *cffFont) writeSubset(w io.Writer, subset []font.GlyphID) (int, error) {
	total := 0

	// Header
	header := []byte{
		1, // major
		0, // minor
		4, // hdrSize
		3, // offSize
	}
	n, err := w.Write(header)
	total += n
	if err != nil {
		return n, err
	}

	// Name INDEX
	n, err = writeIndex(w, [][]byte{[]byte(font.Name)})
	total += n
	if err != nil {
		return n, err
	}

	return total, nil
}
