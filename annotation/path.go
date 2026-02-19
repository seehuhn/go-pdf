// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2026  Jochen Voss <voss@seehuhn.de>
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

package annotation

import (
	"errors"
	"fmt"

	"seehuhn.de/go/pdf"
)

// encodePath validates and encodes a Path array for Ink, Polygon, and
// PolyLine annotations.  The first entry must have length 2 (moveto),
// subsequent entries must have length 2 (lineto) or 6 (curveto).
func encodePath(path [][]float64) (pdf.Array, error) {
	if len(path) == 0 {
		return nil, nil
	}
	if len(path[0]) != 2 {
		return nil, errors.New("first Path entry must have length 2 (moveto)")
	}
	pathArray := make(pdf.Array, len(path))
	for i, entry := range path {
		if i > 0 && len(entry) != 2 && len(entry) != 6 {
			return nil, fmt.Errorf("Path entry %d has length %d, expected 2 or 6", i, len(entry))
		}
		a := make(pdf.Array, len(entry))
		for j, coord := range entry {
			a[j] = pdf.Number(coord)
		}
		pathArray[i] = a
	}
	return pathArray, nil
}
