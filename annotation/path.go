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

	"seehuhn.de/go/geom/vec"
	"seehuhn.de/go/pdf"
)

// encodePath validates and encodes a Path array for Ink, Polygon, and
// PolyLine annotations.  The first entry must have length 1 (moveto);
// subsequent entries must have length 1 (lineto) or length 3 (curveto,
// two control points followed by the endpoint).
func encodePath(path [][]vec.Vec2) (pdf.Array, error) {
	if len(path) == 0 {
		return nil, nil
	}
	if len(path[0]) != 1 {
		return nil, errors.New("first Path entry must have length 1 (moveto)")
	}
	pathArray := make(pdf.Array, len(path))
	for i, entry := range path {
		if i > 0 && len(entry) != 1 && len(entry) != 3 {
			return nil, fmt.Errorf("Path entry %d has %d points, expected 1 or 3", i, len(entry))
		}
		a := make(pdf.Array, 2*len(entry))
		for j, p := range entry {
			a[2*j] = pdf.Number(p.X)
			a[2*j+1] = pdf.Number(p.Y)
		}
		pathArray[i] = a
	}
	return pathArray, nil
}

// decodePath reads a Path array from a PDF dictionary and returns its
// contents as a slice of per-operator point lists: a single moveto point
// followed by lineto (1 point) or curveto (3 points) entries.
//
// Per the library's permissive-reader policy, the whole Path is dropped
// (nil returned with no error) if any entry has a non-conforming shape,
// so that the encoder's invariants hold for every value returned here.
// Non-malformed errors (e.g. IO failures) are propagated to the caller.
func decodePath(x *pdf.Extractor, path *pdf.CycleCheck, obj pdf.Object) ([][]vec.Vec2, error) {
	pathArray, err := pdf.Optional(x.GetArray(path, obj))
	if err != nil || len(pathArray) == 0 {
		return nil, err
	}
	out := make([][]vec.Vec2, len(pathArray))
	for i, pathEntry := range pathArray {
		coords, err := pdf.Optional(pdf.GetFloatArray(x.R, pathEntry))
		if err != nil {
			return nil, err
		}
		var n int
		switch {
		case len(coords) == 2:
			n = 1
		case i > 0 && len(coords) == 6:
			n = 3
		default:
			return nil, nil
		}
		pts := make([]vec.Vec2, n)
		for j := range n {
			pts[j] = vec.Vec2{X: coords[2*j], Y: coords[2*j+1]}
		}
		out[i] = pts
	}
	return out, nil
}
