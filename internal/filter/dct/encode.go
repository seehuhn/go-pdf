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
