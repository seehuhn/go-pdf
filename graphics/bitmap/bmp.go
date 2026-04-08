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

package bitmap

import (
	"encoding/binary"
	"fmt"
	"io"
)

// ReadBMP reads a 1-bit BMP image from r.
// Only uncompressed 1-bit-per-pixel BMP files are supported.
func ReadBMP(r io.Reader) (*Bitmap, error) {
	// file header (14 bytes)
	var fh [14]byte
	if _, err := io.ReadFull(r, fh[:]); err != nil {
		return nil, fmt.Errorf("BMP file header: %w", err)
	}
	if fh[0] != 'B' || fh[1] != 'M' {
		return nil, fmt.Errorf("not a BMP file")
	}
	pixelOffset := binary.LittleEndian.Uint32(fh[10:14])

	// info header (at least 40 bytes for BITMAPINFOHEADER)
	var ih [40]byte
	if _, err := io.ReadFull(r, ih[:]); err != nil {
		return nil, fmt.Errorf("BMP info header: %w", err)
	}
	headerSize := binary.LittleEndian.Uint32(ih[0:4])
	if headerSize < 40 {
		return nil, fmt.Errorf("unsupported BMP header size %d", headerSize)
	}
	width := int(int32(binary.LittleEndian.Uint32(ih[4:8])))
	height := int(int32(binary.LittleEndian.Uint32(ih[8:12])))
	bpp := binary.LittleEndian.Uint16(ih[14:16])
	compression := binary.LittleEndian.Uint32(ih[16:20])

	if bpp != 1 {
		return nil, fmt.Errorf("unsupported BMP bit depth %d, want 1", bpp)
	}
	if compression != 0 {
		return nil, fmt.Errorf("unsupported BMP compression %d", compression)
	}
	if width <= 0 {
		return nil, fmt.Errorf("invalid BMP width %d", width)
	}

	// height can be negative (top-down)
	topDown := height < 0
	if topDown {
		height = -height
	}

	// read remaining header bytes
	if headerSize > 40 {
		extra := make([]byte, headerSize-40)
		if _, err := io.ReadFull(r, extra); err != nil {
			return nil, fmt.Errorf("BMP extended header: %w", err)
		}
	}

	// read the 2-entry color table to determine which index is black
	// each entry is 4 bytes: B, G, R, reserved
	invertPixels := false
	colorTableSize := int(pixelOffset) - 14 - int(headerSize)
	if colorTableSize < 0 {
		return nil, fmt.Errorf("invalid BMP pixel offset %d", pixelOffset)
	}
	if colorTableSize >= 8 {
		ct := make([]byte, colorTableSize)
		if _, err := io.ReadFull(r, ct); err != nil {
			return nil, fmt.Errorf("BMP color table: %w", err)
		}
		// check if palette entry 0 is darker than entry 1
		lum0 := int(ct[0]) + int(ct[1]) + int(ct[2])
		lum1 := int(ct[4]) + int(ct[5]) + int(ct[6])
		if lum0 < lum1 {
			// entry 0 is darker (black): need to invert because our
			// convention is bit 1 = black
			invertPixels = true
		}
	} else if colorTableSize > 0 {
		if _, err := io.ReadFull(r, make([]byte, colorTableSize)); err != nil {
			return nil, fmt.Errorf("BMP skip color table: %w", err)
		}
	}

	bm := New(width, height)
	if len(bm.Pix) == 0 {
		return nil, fmt.Errorf("BMP image too large: %d x %d", width, height)
	}

	// BMP rows are padded to 4-byte boundaries
	bmpStride := ((width + 31) / 32) * 4
	rowBuf := make([]byte, bmpStride)

	for i := 0; i < height; i++ {
		if _, err := io.ReadFull(r, rowBuf); err != nil {
			return nil, fmt.Errorf("BMP row %d: %w", i, err)
		}
		var y int
		if topDown {
			y = i
		} else {
			y = height - 1 - i
		}
		// copy row, optionally inverting bits based on palette
		dst := bm.Pix[y*bm.Stride : y*bm.Stride+bm.Stride]
		copy(dst, rowBuf[:bm.Stride])
		if invertPixels {
			for j := range dst {
				dst[j] ^= 0xFF
			}
		}
	}

	// clear any trailing bits in the last byte of each row
	tailBits := width % 8
	if tailBits > 0 {
		mask := byte(0xFF) << (8 - tailBits)
		for y := 0; y < height; y++ {
			bm.Pix[y*bm.Stride+bm.Stride-1] &= mask
		}
	}

	return bm, nil
}
