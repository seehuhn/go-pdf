// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

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
