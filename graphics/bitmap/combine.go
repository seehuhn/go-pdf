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

import "image"

// CombOp specifies how to combine pixels when compositing one bitmap onto another.
// These correspond to the JBIG2 combination operators (ITU-T T.88, Section 4.3).
type CombOp int

const (
	CombOpOR      CombOp = iota // union of pixels (default)
	CombOpAND                   // intersection of pixels
	CombOpXOR                   // exclusive or
	CombOpXNOR                  // exclusive nor
	CombOpReplace               // overwrite destination
)

// Combine composites src onto b at position (x, y) using the given operator.
// The position specifies where src.Rect.Min maps to in b's coordinate space.
// Pixels outside b's bounds are clipped.
func (b *Bitmap) Combine(src *Bitmap, x, y int, op CombOp) {
	if src == nil || src.Rect.Empty() || b.Rect.Empty() {
		return
	}

	// source region in b's coordinate space
	sr := src.Rect.Add(image.Pt(x-src.Rect.Min.X, y-src.Rect.Min.Y))
	// clip to destination bounds
	clip := sr.Intersect(b.Rect)
	if clip.Empty() {
		return
	}

	for cy := clip.Min.Y; cy < clip.Max.Y; cy++ {
		sx0 := clip.Min.X - sr.Min.X + src.Rect.Min.X
		sy := cy - sr.Min.Y + src.Rect.Min.Y

		ddy := cy - b.Rect.Min.Y
		sdy := sy - src.Rect.Min.Y

		dxStart := clip.Min.X - b.Rect.Min.X
		sxStart := sx0 - src.Rect.Min.X
		width := clip.Dx()

		// fast path: both source and destination are byte-aligned
		if dxStart%8 == 0 && sxStart%8 == 0 && width%8 == 0 {
			b.combineAligned(ddy, dxStart/8, src, sdy, sxStart/8, width/8, op)
			continue
		}

		// slow path: pixel by pixel
		for i := range width {
			dx := dxStart + i
			sx := sxStart + i
			sv := src.Pix[sdy*src.Stride+sx/8]>>(7-sx%8)&1 != 0
			dOff := ddy*b.Stride + dx/8
			dBit := byte(1) << (7 - dx%8)
			dv := b.Pix[dOff]&dBit != 0
			var rv bool
			switch op {
			case CombOpOR:
				rv = dv || sv
			case CombOpAND:
				rv = dv && sv
			case CombOpXOR:
				rv = dv != sv
			case CombOpXNOR:
				rv = dv == sv
			case CombOpReplace:
				rv = sv
			default:
				rv = dv || sv
			}
			if rv {
				b.Pix[dOff] |= dBit
			} else {
				b.Pix[dOff] &^= dBit
			}
		}
	}
}

func (b *Bitmap) combineAligned(dRow, dByteCol int, src *Bitmap, sRow, sByteCol, nBytes int, op CombOp) {
	dOff := dRow*b.Stride + dByteCol
	sOff := sRow*src.Stride + sByteCol
	switch op {
	case CombOpOR:
		for i := range nBytes {
			b.Pix[dOff+i] |= src.Pix[sOff+i]
		}
	case CombOpAND:
		for i := range nBytes {
			b.Pix[dOff+i] &= src.Pix[sOff+i]
		}
	case CombOpXOR:
		for i := range nBytes {
			b.Pix[dOff+i] ^= src.Pix[sOff+i]
		}
	case CombOpXNOR:
		for i := range nBytes {
			b.Pix[dOff+i] = ^(b.Pix[dOff+i] ^ src.Pix[sOff+i])
		}
	case CombOpReplace:
		copy(b.Pix[dOff:dOff+nBytes], src.Pix[sOff:sOff+nBytes])
	default:
		for i := range nBytes {
			b.Pix[dOff+i] |= src.Pix[sOff+i]
		}
	}
}
