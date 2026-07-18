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

// Package vfinstance pins variable fonts to a single instance before embedding.
package vfinstance

import (
	"fmt"

	"seehuhn.de/go/sfnt"
	"seehuhn.de/go/sfnt/cff"
)

// Apply returns a static instance of info suitable for embedding.
//
// Every key in variations must match a variation axis tag of info; an unknown
// tag (which includes any non-empty variations on a static font) causes an
// error.  The values map axis tags to user-scale coordinates; axes omitted
// from variations keep their default value.
//
// A variable font is always instanced: with variations when non-nil, or at its
// default coordinates otherwise.  A subsettable font cannot carry variation
// tables, so a variable font must be pinned even when the caller requests no
// variations.  A static font with nil or empty variations is returned unchanged.
//
// Apply is idempotent: the returned font is static, so applying it again with
// nil variations yields the same pointer.
func Apply(info *sfnt.Font, variations map[string]float64) (*sfnt.Font, error) {
	if len(variations) > 0 {
		known := make(map[string]bool)
		for _, axis := range info.VariationAxes() {
			known[axis.Tag] = true
		}
		for tag := range variations {
			if !known[tag] {
				return nil, fmt.Errorf("unknown variation axis %q", tag)
			}
		}
	}

	// CFF2 outlines are inherently variable and must be pinned even without axes
	_, isCFF2 := info.Outlines.(*cff.OutlinesCFF2)

	if info.IsVariable() || isCFF2 {
		return info.Instantiate(variations)
	}

	// static font with no requested variations
	return info, nil
}
