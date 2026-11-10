// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package fontgeom

import (
	"testing"

	"seehuhn.de/go/pdf/font"
)

func TestFillHeights(t *testing.T) {
	cases := []struct {
		name                  string
		in                    font.Geometry
		leading, cap, xHeight float64
	}{
		{
			name:    "nothing reported",
			leading: defaultLeading,
			cap:     defaultCapHeight,
			xHeight: defaultXHeight,
		},
		{
			name:    "all reported",
			in:      font.Geometry{Leading: 1.4, CapHeight: 0.7, XHeight: 0.52},
			leading: 1.4, cap: 0.7, xHeight: 0.52,
		},
		{
			// a reported value is kept even where it breaks the ordering
			name:    "cap height above leading",
			in:      font.Geometry{Leading: 0.5, CapHeight: 0.7},
			leading: 0.5, cap: 0.7, xHeight: defaultXHeight,
		},
		{
			// the estimates are clamped to the values already in place
			name:    "short leading",
			in:      font.Geometry{Leading: 0.3},
			leading: 0.3, cap: 0.3, xHeight: 0.3,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			g := c.in
			FillHeights(&g)
			if g.Leading != c.leading {
				t.Errorf("Leading = %g, want %g", g.Leading, c.leading)
			}
			if g.CapHeight != c.cap {
				t.Errorf("CapHeight = %g, want %g", g.CapHeight, c.cap)
			}
			if g.XHeight != c.xHeight {
				t.Errorf("XHeight = %g, want %g", g.XHeight, c.xHeight)
			}
		})
	}
}
