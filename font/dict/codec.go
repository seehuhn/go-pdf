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

package dict

import (
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/cmap"
)

func makeCodec(cmap *cmap.File, toUnicode *cmap.ToUnicodeFile) *charcode.Codec {
	// First try to use the the union of the code space ranges from the CMap
	// and the ToUnicode cmap.  If this fails, remove code space ranges from
	// the end one by one until we find a working codec.
	var csr charcode.CodeSpaceRange
	for _, r := range cmap.CodeSpaceRange {
		if r.IsValid() {
			csr = append(csr, r)
		}
	}
	if toUnicode != nil {
		for _, r := range toUnicode.CodeSpaceRange {
			if r.IsValid() {
				csr = append(csr, r)
			}
		}
	}
	for len(csr) > 0 {
		codec, err := charcode.NewCodec(csr)
		if err == nil {
			return codec
		}
		csr = csr[:len(csr)-1]
	}

	// As a fallback, use one-byte codes.
	codec, _ := charcode.NewCodec(charcode.Simple)
	return codec
}
