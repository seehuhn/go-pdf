// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

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
