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

package image

import (
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/internal/limits"
)

// ImageDataLimit returns an upper bound on the decoded byte count for an image
// with the given parameters.  The bound is the minimum of a dictionary-derived
// size (Width × Height × Channels × BitsPerComponent, padded to byte rows,
// with small slack) and the absolute ceiling [limits.MaxImageBytes].
//
// The channel count comes from the resolved colour space, so DeviceN gets its
// actual component count and ICCBased gets the profile's N (validated
// elsewhere to be in {1, 3, 4}).  The absolute ceiling still applies, so a
// pathological DeviceN with hundreds of components cannot exceed it.
func ImageDataLimit(width, height, bpc int, cs color.Space) int64 {
	if width <= 0 || height <= 0 || bpc <= 0 || cs == nil {
		return limits.MaxImageBytes
	}
	n := cs.Channels()
	if n <= 0 {
		return limits.MaxImageBytes
	}
	return limits.ImageDataLimit(width, height, n, bpc)
}

// imageMaskDataLimit returns the decoded byte bound for a 1-bit-per-pixel image mask.
func imageMaskDataLimit(width, height int) int64 {
	return limits.ImageDataLimit(width, height, 1, 1)
}
