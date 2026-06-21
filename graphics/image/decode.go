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
	"math"

	"seehuhn.de/go/pdf/graphics/color"
)

// DefaultDecode returns the default Decode array for an image
// with the given color space and bits per component.
// The returned array has 2*cs.Channels() entries, with pairs
// [Dmin, Dmax] for each channel.
//
// bpc must be a valid image bit depth (1, 2, 4, 8, or 16); callers that
// read bpc from a file must validate it first.
func DefaultDecode(cs color.Space, bpc int) []float64 {
	var n int
	if cs != nil {
		n = cs.Channels()
	}
	d := make([]float64, 2*n)
	switch cs := cs.(type) {
	case *color.SpaceLab:
		d[0], d[1] = 0, 100
		d[2], d[3] = cs.Ranges[0], cs.Ranges[1]
		d[4], d[5] = cs.Ranges[2], cs.Ranges[3]
	case *color.SpaceICCBased:
		copy(d, cs.Ranges[:2*n])
	case *color.SpaceIndexed:
		d[0], d[1] = 0, float64(int(1)<<bpc-1)
	default:
		for i := range n {
			d[2*i+1] = 1
		}
	}
	return d
}

// expectedDataSize computes the expected byte count for image data with the
// given dimensions. It returns -1 if the computation overflows.
func expectedDataSize(width, channels, bpc, height int) int {
	// all factors are positive (validated by callers)
	bitsPerRow := int64(width) * int64(channels) * int64(bpc)
	bytesPerRow := (bitsPerRow + 7) / 8

	size := bytesPerRow * int64(height)
	if size < 0 || size > math.MaxInt {
		return -1
	}
	return int(size)
}

// normalizeData pads or truncates data to match expectedSize.
// If expectedSize is negative (overflow) or unreasonably large,
// the data is returned as-is to avoid excessive allocation.
func normalizeData(data []byte, expectedSize int) []byte {
	if expectedSize < 0 || expectedSize > maxImageDataSize {
		return data
	}
	if len(data) >= expectedSize {
		return data[:expectedSize]
	}
	return append(data, make([]byte, expectedSize-len(data))...)
}

// maxImageDataSize is the largest image data size we'll allocate during
// normalization (1 GiB). This guards against malicious dimensions.
const maxImageDataSize = 1 << 30
