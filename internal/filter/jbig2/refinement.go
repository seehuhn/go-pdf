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

// refinementParams holds parameters for generic refinement region decoding.
type refinementParams struct {
	Width     int
	Height    int
	Template  int // 0 or 1
	Reference *bitmap.Bitmap
	RefDX     int
	RefDY     int
	TPGRON    bool
	ATX       [2]int8
	ATY       [2]int8
}

// decodeRefinementRegion decodes a generic refinement region.
func decodeRefinementRegion(budget *int64, dec *mqDecoder, p *refinementParams, cx []byte) (*bitmap.Bitmap, error) {
	bm, err := allocBitmap(budget, p.Width, p.Height)
	if err != nil {
		return nil, err
	}
	if p.Width == 0 || p.Height == 0 {
		return bm, nil
	}

	if cx == nil {
		tmp := make([]byte, 1<<13)
		cx = tmp
	}

	ltp := 0
	for y := 0; y < p.Height; y++ {
		if p.TPGRON {
			sltp := dec.decode(&cx[refTPGRContexts[p.Template]])
			ltp ^= sltp
		}

		for x := 0; x < p.Width; x++ {
			if ltp != 0 {
				if refTypicalPrediction(p, x, y) {
					rx := x - p.RefDX
					ry := y - p.RefDY
					bm.SetPixel(x, y, p.Reference.GetPixel(rx, ry))
					continue
				}
			}

			context := buildRefContext(bm, p, x, y)
			d := dec.decode(&cx[context])
			if d != 0 {
				bm.SetPixel(x, y, true)
			}
		}
	}

	return bm, nil
}

// encodeRefinementRegion encodes a bitmap as a generic refinement region
// relative to p.Reference, returning MQ-coded bytes.
func encodeRefinementRegion(bm *bitmap.Bitmap, p *refinementParams) []byte {
	if bm.Width() == 0 || bm.Height() == 0 {
		return nil
	}

	enc := newMQEncoder()
	encodeRefinementRegionInline(enc, bm, p, nil)
	enc.flush()
	return enc.bytes()
}

// encodeRefinementRegionInline encodes a refinement region into an existing
// MQ encoder. This is used by text region encoding where the refinement
// data is interleaved with the instance data in the same MQ stream.
// If cx is nil, a fresh context array is allocated.
func encodeRefinementRegionInline(enc *mqEncoder, bm *bitmap.Bitmap, p *refinementParams, cx []byte) {
	if cx == nil {
		if p.Template == 0 {
			cx = make([]byte, 1<<13)
		} else {
			cx = make([]byte, 1<<10)
		}
	}

	ltp := 0
	for y := range bm.Height() {
		if p.TPGRON {
			// determine whether we can use ltp=1 for this row:
			// every typical pixel must match the reference
			canUseLTP := true
			for x := range bm.Width() {
				if refTypicalPrediction(p, x, y) {
					rx := x - p.RefDX
					ry := y - p.RefDY
					if bm.GetPixel(x, y) != p.Reference.GetPixel(rx, ry) {
						canUseLTP = false
						break
					}
				}
			}
			typical := 0
			if canUseLTP {
				typical = 1
			}
			sltp := ltp ^ typical
			enc.encode(&cx[refTPGRContexts[p.Template]], sltp)
			ltp = typical
		}

		for x := range bm.Width() {
			if ltp != 0 && refTypicalPrediction(p, x, y) {
				continue
			}
			context := buildRefContext(bm, p, x, y)
			d := getPixel(bm, x, y)
			enc.encode(&cx[context], d)
		}
	}
}

func refTypicalPrediction(p *refinementParams, x, y int) bool {
	rx := x - p.RefDX
	ry := y - p.RefDY
	ref := p.Reference

	v := ref.GetPixel(rx, ry)
	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			if ref.GetPixel(rx+dx, ry+dy) != v {
				return false
			}
		}
	}
	return true
}

// TPGR context values for SLTP decoding (Figures 14-15 of spec).
// All decoded pixels are 0 and all reference pixels are 0 except
// the center reference pixel rpx(0,0) which is 1.
var refTPGRContexts = [2]uint16{
	0: 0x0010, // template 0: bit 4 = rpx(0,0)
	1: 0x0008, // template 1: bit 3 = rpx(0,0)
}

// buildRefContext forms the arithmetic context for refinement coding.
// Reference pixel access: ref_x = x - GRREFERENCEDX + dx (§6.3.5).
func buildRefContext(bm *bitmap.Bitmap, p *refinementParams, x, y int) uint16 {
	px := func(dx, dy int) uint16 {
		if bm.GetPixel(x+dx, y+dy) {
			return 1
		}
		return 0
	}
	rpx := func(dx, dy int) uint16 {
		rx := x + dx - p.RefDX
		ry := y + dy - p.RefDY
		if p.Reference.GetPixel(rx, ry) {
			return 1
		}
		return 0
	}

	switch p.Template {
	case 0:
		// 13-bit context: 3 decoded + 2 AT + 8 reference (§6.3.5.3)
		decAt := px(int(p.ATX[0]), int(p.ATY[0]))
		refAt := rpx(int(p.ATX[1]), int(p.ATY[1]))

		return rpx(1, 1) | // bit 0
			rpx(0, 1)<<1 | // bit 1
			rpx(-1, 1)<<2 | // bit 2
			rpx(1, 0)<<3 | // bit 3
			rpx(0, 0)<<4 | // bit 4
			rpx(-1, 0)<<5 | // bit 5
			rpx(1, -1)<<6 | // bit 6
			rpx(0, -1)<<7 | // bit 7
			px(-1, 0)<<8 | // bit 8
			px(1, -1)<<9 | // bit 9
			px(0, -1)<<10 | // bit 10
			decAt<<11 | // bit 11: AT₁
			refAt<<12 // bit 12: AT₂

	case 1:
		// 10-bit context: 4 decoded + 6 reference (§6.3.5.3)
		return rpx(1, 1) | // bit 0
			rpx(0, 1)<<1 | // bit 1
			rpx(1, 0)<<2 | // bit 2
			rpx(0, 0)<<3 | // bit 3
			rpx(-1, 0)<<4 | // bit 4
			rpx(0, -1)<<5 | // bit 5
			px(-1, 0)<<6 | // bit 6
			px(1, -1)<<7 | // bit 7
			px(0, -1)<<8 | // bit 8
			px(-1, -1)<<9 // bit 9
	}
	return 0
}
