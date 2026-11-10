// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package decode

import (
	"seehuhn.de/go/geom/vec"
	"seehuhn.de/go/pdf"
)

// decodePath reads a Path array from a PDF dictionary and returns its
// contents as a slice of per-operator point lists: a single moveto point
// followed by lineto (1 point) or curveto (3 points) entries.
//
// Per the library's permissive-reader policy, the whole Path is dropped
// (nil returned with no error) if any entry has a non-conforming shape,
// so that the encoder's invariants hold for every value returned here.
// Non-malformed errors (e.g. IO failures) are propagated to the caller.
func decodePath(c pdf.Cursor, obj pdf.Object) ([][]vec.Vec2, error) {
	pathArray, err := pdf.Optional(c.Array(obj))
	if err != nil || len(pathArray) == 0 {
		return nil, err
	}
	out := make([][]vec.Vec2, len(pathArray))
	for i, pathEntry := range pathArray {
		coords, err := pdf.Optional(c.FloatArray(pathEntry))
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
