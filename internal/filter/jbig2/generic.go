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

package jbig2

import (
	"seehuhn.de/go/pdf/graphics/bitmap"
)

// genericRegionParams holds the parameters for generic region decoding/encoding.
type genericRegionParams struct {
	MMR         bool
	Width       int
	Height      int
	Template    int // 0-3
	TPGDON      bool
	ExtTemplate bool
	UseSkip     bool
	Skip        *bitmap.Bitmap
	ATX         [12]int8
	ATY         [12]int8
}

// genericContextSize returns the number of context bytes needed for a
// generic region template (16, 13, 10, or 10 bits for templates 0-3).
func genericContextSize(template int) int {
	switch template {
	case 0:
		return 1 << 16
	case 1:
		return 1 << 13
	default:
		return 1 << 10
	}
}

// TPGD context values for SLTP decoding (Figures 8-11 of spec).
// Computed using the reading-order context bit layout below,
// with AT pixels at their nominal positions.
var tpgdContexts = [4]uint16{
	0: 0x9B25, // template 0
	1: 0x0795, // template 1
	2: 0x00E5, // template 2
	3: 0x0195, // template 3
}

func getPixel(bm *bitmap.Bitmap, x, y int) int {
	if bm.GetPixel(x, y) {
		return 1
	}
	return 0
}

func decodeGenericRegion(pool *bitmapPool, dec *mqDecoder, p *genericRegionParams, cx []byte) (*bitmap.Bitmap, error) {
	bm, err := pool.allocBitmap(p.Width, p.Height)
	if err != nil {
		return nil, err
	}
	if p.Width == 0 || p.Height == 0 {
		return bm, nil
	}

	if cx == nil {
		cx = make([]byte, genericContextSize(p.Template))
	}

	ltp := 0
	for y := 0; y < p.Height; y++ {
		if p.TPGDON {
			sltp := dec.decode(&cx[tpgdContexts[p.Template]])
			ltp ^= sltp
		}

		if ltp != 0 {
			if y > 0 {
				for x := 0; x < p.Width; x++ {
					bm.SetPixel(x, y, bm.GetPixel(x, y-1))
				}
			}
			continue
		}

		for x := 0; x < p.Width; x++ {
			if p.UseSkip && p.Skip != nil && p.Skip.GetPixel(x, y) {
				continue
			}

			context := buildContext(bm, x, y, p)
			d := dec.decode(&cx[context])
			if d != 0 {
				bm.SetPixel(x, y, true)
			}
		}
	}

	return bm, nil
}

// buildContext gathers template pixels in reading order (top→bottom, L→R).
// MSB = first pixel in reading order, LSB = last.
// AT pixels are placed at their geometric position in the reading order,
// independent of the configured AT pixel location (per spec Section 6.2.5.3).
func buildContext(bm *bitmap.Bitmap, x, y int, p *genericRegionParams) uint16 {
	px := func(dx, dy int) uint16 {
		return uint16(getPixel(bm, x+dx, y+dy))
	}
	at := func(i int) uint16 {
		return uint16(getPixel(bm, x+int(p.ATX[i]), y+int(p.ATY[i])))
	}

	switch p.Template {
	case 0:
		if p.ExtTemplate {
			// 16-bit context (12 AT + 4 fixed)
			// Same bit positions as non-extended, but most fixed pixels
			// become AT pixels. Only (-1,0), (1,-1), (0,-1), (-1,-1) remain fixed.
			return px(-1, 0) | // bit 0: fixed
				at(0)<<1 | // A₁ (nominal: -2, 0)
				at(6)<<2 | // A₇ (nominal: -3, 0)
				at(7)<<3 | // A₈ (nominal: -4, 0)
				at(9)<<4 | // A₁₀ (nominal: 3, -1)
				at(5)<<5 | // A₆ (nominal: 2, -1)
				px(1, -1)<<6 | // fixed
				px(0, -1)<<7 | // fixed
				px(-1, -1)<<8 | // fixed
				at(2)<<9 | // A₃ (nominal: -2, -1)
				at(11)<<10 | // A₁₂ (nominal: -3, -1)
				at(8)<<11 | // A₉ (nominal: 2, -2)
				at(4)<<12 | // A₅ (nominal: 1, -2)
				at(1)<<13 | // A₂ (nominal: 0, -2)
				at(3)<<14 | // A₄ (nominal: -1, -2)
				at(10)<<15 // A₁₁ (nominal: -2, -2)
		}
		// 16-bit context (4 AT + 12 fixed)
		// row -2: A₄(-2,-2), (-1,-2), (0,-2), (1,-2), A₃(2,-2)
		// row -1: A₂(-3,-1), (-2,-1), (-1,-1), (0,-1), (1,-1), (2,-1), A₁(3,-1)
		// row  0: (-4,0), (-3,0), (-2,0), (-1,0)
		return px(-1, 0) | // bit 0
			px(-2, 0)<<1 |
			px(-3, 0)<<2 |
			px(-4, 0)<<3 |
			at(0)<<4 | // A₁
			px(2, -1)<<5 |
			px(1, -1)<<6 |
			px(0, -1)<<7 |
			px(-1, -1)<<8 |
			px(-2, -1)<<9 |
			at(1)<<10 | // A₂
			at(2)<<11 | // A₃
			px(1, -2)<<12 |
			px(0, -2)<<13 |
			px(-1, -2)<<14 |
			at(3)<<15 // A₄

	case 1:
		// 13-bit context (1 AT + 12 fixed)
		// row -2: (-1,-2), (0,-2), (1,-2), (2,-2)
		// row -1: (-2,-1), (-1,-1), (0,-1), (1,-1), (2,-1), A₁(3,-1)
		// row  0: (-3,0), (-2,0), (-1,0)
		return px(-1, 0) | // bit 0
			px(-2, 0)<<1 |
			px(-3, 0)<<2 |
			at(0)<<3 | // A₁
			px(2, -1)<<4 |
			px(1, -1)<<5 |
			px(0, -1)<<6 |
			px(-1, -1)<<7 |
			px(-2, -1)<<8 |
			px(2, -2)<<9 |
			px(1, -2)<<10 |
			px(0, -2)<<11 |
			px(-1, -2)<<12

	case 2:
		// 10-bit context (1 AT + 9 fixed)
		// row -2: (-1,-2), (0,-2), (1,-2)
		// row -1: (-2,-1), (-1,-1), (0,-1), (1,-1), A₁(2,-1)
		// row  0: (-2,0), (-1,0)
		return px(-1, 0) | // bit 0
			px(-2, 0)<<1 |
			at(0)<<2 | // A₁
			px(1, -1)<<3 |
			px(0, -1)<<4 |
			px(-1, -1)<<5 |
			px(-2, -1)<<6 |
			px(1, -2)<<7 |
			px(0, -2)<<8 |
			px(-1, -2)<<9

	case 3:
		// 10-bit context (1 AT + 9 fixed)
		// row -1: (-3,-1), (-2,-1), (-1,-1), (0,-1), (1,-1), A₁(2,-1)
		// row  0: (-4,0), (-3,0), (-2,0), (-1,0)
		return px(-1, 0) | // bit 0
			px(-2, 0)<<1 |
			px(-3, 0)<<2 |
			px(-4, 0)<<3 |
			at(0)<<4 | // A₁
			px(1, -1)<<5 |
			px(0, -1)<<6 |
			px(-1, -1)<<7 |
			px(-2, -1)<<8 |
			px(-3, -1)<<9
	}
	return 0
}

func encodeGenericRegion(enc *mqEncoder, bm *bitmap.Bitmap, p *genericRegionParams, cx []byte) {
	if bm.Width() == 0 || bm.Height() == 0 {
		return
	}

	if cx == nil {
		cx = make([]byte, genericContextSize(p.Template))
	}

	ltp := 0
	for y := 0; y < bm.Height(); y++ {
		if p.TPGDON {
			typical := 0
			if y > 0 {
				typical = 1
				for x := 0; x < bm.Width(); x++ {
					if bm.GetPixel(x, y) != bm.GetPixel(x, y-1) {
						typical = 0
						break
					}
				}
			}
			sltp := ltp ^ typical
			enc.encode(&cx[tpgdContexts[p.Template]], sltp)
			ltp = typical
		}

		if ltp != 0 {
			continue
		}

		for x := 0; x < bm.Width(); x++ {
			if p.UseSkip && p.Skip != nil && p.Skip.GetPixel(x, y) {
				continue
			}

			context := buildContext(bm, x, y, p)
			d := getPixel(bm, x, y)
			enc.encode(&cx[context], d)
		}
	}
}
