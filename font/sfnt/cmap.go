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

package sfnt

import (
	"bytes"

	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/sfnt/cmap"
)

// makeCMap writes a cmap containing a 1,0,4 subtable to map character indices
// to glyph indices in a subsetted font.
func makeCMap(mapping []font.CMapEntry) ([]byte, error) {
	subtable := make(cmap.Format4)
	for _, entry := range mapping {
		if entry.GID == 0 {
			continue
		}
		subtable[entry.CharCode] = entry.GID
	}

	cmap := cmap.Table{
		cmap.Key{PlatformID: 1, EncodingID: 0}: subtable.Encode(0),
	}

	buf := &bytes.Buffer{}
	if err := cmap.Write(buf); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
