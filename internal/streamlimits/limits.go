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

// Package streamlimits collects size caps used by readers of decoded
// PDF streams.  These caps defend against decompression bombs:
// attacker-controlled streams whose decoded size is grossly
// disproportionate to the input file size.
//
// Each constant is the maximum number of bytes a particular kind of
// decoded stream may produce.
package streamlimits

// ImageDataLimit returns an upper bound on the decoded byte count for an
// image with the given parameters.  The bound is
// min(⌈W × channels × bpc / 8⌉ × H, MaxImageBytes).
// It returns MaxImageBytes if any argument is non-positive or if the
// computation overflows.
func ImageDataLimit(width, height, channels, bpc int) int64 {
	if width <= 0 || height <= 0 || channels <= 0 || bpc <= 0 {
		return MaxImageBytes
	}
	bitsPerRow := int64(width) * int64(channels) * int64(bpc)
	bytesPerRow := (bitsPerRow + 7) / 8
	size := bytesPerRow * int64(height)
	if size < 0 || size > MaxImageBytes {
		return MaxImageBytes
	}
	return size
}

const (
	// MaxImageWidth and MaxImageHeight are absolute sanity caps on the pixel
	// dimensions of a single image.  Downstream arithmetic uses int64, so
	// this is not an overflow defense; it just bounds resource use.
	MaxImageWidth  = 1 << 16
	MaxImageHeight = 1 << 16

	// MaxImageBytes caps the decoded byte count of a single image
	// XObject, inline image, or thumbnail.
	MaxImageBytes = 256 << 20

	// MaxSampleBytes caps the decoded byte count of a Type 0 sampled
	// function's sample table.
	MaxSampleBytes = 16 << 20

	// MaxShadingBytes caps the decoded byte count of a Type 4-7
	// shading stream.
	MaxShadingBytes = 16 << 20

	// MaxICCProfileBytes caps the decoded byte count of an ICC color
	// profile stream.
	MaxICCProfileBytes = 32 << 20

	// MaxJBIG2GlobalsBytes caps the decoded byte count of a JBIG2
	// globals stream.
	MaxJBIG2GlobalsBytes = 4 << 20

	// MaxJBIG2PageBytes caps the decoded byte count of a JBIG2
	// per-page stream.  The jbig2 decoder applies its own internal
	// budget on bitmap allocations; this cap bounds only the raw
	// input buffer.
	MaxJBIG2PageBytes = 64 << 20

	// MaxCIDToGIDMapBytes caps the decoded byte count of a font's
	// CIDToGIDMap stream (= 65536 CIDs * 2 bytes/entry).
	MaxCIDToGIDMapBytes = 128 << 10

	// MaxIndexedLookupBytes caps the decoded byte count of an Indexed
	// color space lookup table.  PDF 32000-2 §8.6.6.3 bounds the
	// table at (hival+1) * n bytes with hival <= 255 and n <= 32 in
	// any realistic base color space, so 64 KB leaves generous slack.
	MaxIndexedLookupBytes = 64 << 10
)
