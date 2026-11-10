// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package dct

import (
	"io"

	"seehuhn.de/go/pdf/internal/filter/dct/jpeg"
)

// EncodeParams describes the sample stream given to [Encode].
type EncodeParams = jpeg.EncodeParams

// Encode reads Width·Height·Components bytes from r and writes a
// baseline JPEG image to w.  The input holds 8-bit samples, interleaved
// channel-by-channel and row-by-row with no padding: 1 byte per pixel
// for grayscale, 3 for RGB and 4 for CMYK.  This is the layout
// [Decode] produces.
//
// If p.Transform is set, RGB data is stored as YCbCr and CMYK data as
// YCbCrK.  For 3 and 4 components an Adobe APP14 marker records the
// choice, so that [Decode] restores the original samples regardless of
// the ColorTransform value.
func Encode(w io.Writer, r io.Reader, p EncodeParams) error {
	return jpeg.EncodeStream(w, r, p)
}
