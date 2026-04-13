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
	"encoding/binary"
	"fmt"
	"math/bits"

	"seehuhn.de/go/pdf/graphics/bitmap"
)

// processHalftoneRegion decodes a halftone region segment (§6.6, §7.4.5).
func (d *decoder) processHalftoneRegion(hdr *segmentHeader, data []byte) error {
	if len(data) < 17+21 {
		return fmt.Errorf("halftone region data too short")
	}

	rsi := parseRegionSegmentInfo(data[:17])
	hdr2 := data[17:]

	// halftone region flags (1 byte, §7.4.5.1)
	flags := hdr2[0]
	hmmr := flags&1 != 0
	htemplate := int((flags >> 1) & 3)
	henableSkip := flags&8 != 0
	hcombOp := bitmap.CombOp((flags >> 4) & 7)
	hdefPixel := flags&0x80 != 0

	// grid parameters (§7.4.5.2)
	hgw := int(binary.BigEndian.Uint32(hdr2[1:5]))
	hgh := int(binary.BigEndian.Uint32(hdr2[5:9]))
	hgx := int(int32(binary.BigEndian.Uint32(hdr2[9:13])))
	hgy := int(int32(binary.BigEndian.Uint32(hdr2[13:17])))
	hrx := int(binary.BigEndian.Uint16(hdr2[17:19]))
	hry := int(binary.BigEndian.Uint16(hdr2[19:21]))

	offset := 17 + 21

	// collect patterns from referred pattern dictionary segment
	var patterns []*bitmap.Bitmap
	var hpw, hph int
	for _, refNum := range hdr.RefSegments {
		if ref, ok := d.segments[refNum]; ok && ref.patterns != nil {
			patterns = ref.patterns
			if len(patterns) > 0 {
				hpw = patterns[0].Width()
				hph = patterns[0].Height()
			}
		}
	}
	if len(patterns) == 0 {
		return fmt.Errorf("halftone region: no pattern dictionary found")
	}
	hnumpats := len(patterns)

	// bits per gray-scale value
	hbpp := bits.Len(uint(hnumpats - 1))
	if hbpp == 0 {
		hbpp = 1
	}

	// validate grid dimensions to prevent overflow in loops and allocations
	if _, err := checkedMul(hgw, hgh); err != nil {
		return fmt.Errorf("halftone grid: %w", err)
	}

	// compute skip bitmap if enabled (§6.6.5.1)
	var hskip *bitmap.Bitmap
	if henableSkip {
		var err error
		hskip, err = allocBitmap(&d.memBudget, hgw, hgh)
		if err != nil {
			return err
		}
		for mg := range hgh {
			for ng := range hgw {
				x := (hgx + mg*hry + ng*hrx) >> 8
				y := (hgy + mg*hrx - ng*hry) >> 8
				if x+hpw <= 0 || x >= int(rsi.Width) || y+hph <= 0 || y >= int(rsi.Height) {
					hskip.SetPixel(ng, mg, true)
				}
			}
		}
	}

	// decode gray-scale image (Annex C)
	grayImage, err := decodeGrayScaleImage(&d.memBudget, data[offset:], hmmr, htemplate, hbpp, hgw, hgh, henableSkip, hskip)
	if err != nil {
		return err
	}

	// fill region with default pixel (§6.6.5.2 step 1)
	bm, err := allocBitmap(&d.memBudget, int(rsi.Width), int(rsi.Height))
	if err != nil {
		return err
	}
	if hdefPixel {
		for i := range bm.Pix {
			bm.Pix[i] = 0xFF
		}
	}

	// place patterns (§6.6.5.3)
	for mg := range hgh {
		for ng := range hgw {
			if henableSkip && hskip.GetPixel(ng, mg) {
				continue
			}

			gi := grayImage[mg*hgw+ng]
			if gi >= hnumpats {
				gi = 0
			}
			pat := patterns[gi]

			x := (hgx + mg*hry + ng*hrx) >> 8
			y := (hgy + mg*hrx - ng*hry) >> 8

			bm.Combine(pat, x, y, hcombOp)
		}
	}

	freeBitmap(&d.memBudget, hskip)
	freeInts(&d.memBudget, grayImage)

	// composite onto page (skip for intermediate segments)
	if hdr.Type != segIntermediateHalftone && d.pageBitmap != nil {
		op := bitmap.CombOp(rsi.CombOp)
		d.pageBitmap.Combine(bm, int(rsi.X), int(rsi.Y), op)
		freeBitmap(&d.memBudget, bm)
		bm = nil
	}

	d.segments[hdr.Number] = segmentResult{header: hdr, bm: bm}
	return nil
}

// decodeGrayScaleImage decodes a gray-scale image from bitplanes (Annex C).
// Returns a flat array of gray-scale values, row-major [hgh][hgw].
func decodeGrayScaleImage(
	budget *int64,
	data []byte,
	gsmmr bool, gstemplate int,
	gsbpp, gsw, gsh int,
	useSkip bool, skip *bitmap.Bitmap,
) ([]int, error) {
	if gsbpp > maxBitplanes {
		return nil, fmt.Errorf("jbig2: %d bitplanes exceeds maximum %d", gsbpp, maxBitplanes)
	}

	n, err := checkedMul(gsw, gsh)
	if err != nil {
		return nil, err
	}
	result, err := allocInts(budget, n)
	if err != nil {
		return nil, err
	}

	// decode bitplanes incrementally (§C.5.1)
	// Instead of allocating all gsbpp bitmaps simultaneously, we use two:
	// running (cumulative XOR) and current (just decoded).
	// After XOR, running holds the final Gray-decoded value for bit j.
	var running *bitmap.Bitmap
	dataOffset := 0

	// for arithmetic coding, all bitplanes share a single MQ state
	// and a single context array
	var arithDec *mqDecoder
	var arithCx []byte
	if !gsmmr {
		arithDec = newMQDecoder(data)
		arithCx = make([]byte, genericContextSize(gstemplate))
	}

	for j := gsbpp - 1; j >= 0; j-- {
		var current *bitmap.Bitmap
		if gsmmr {
			var n int
			current, n, err = decodeMMR(budget, data[dataOffset:], gsw, gsh)
			if err != nil {
				return nil, err
			}
			dataOffset += n
		} else {
			p := &genericRegionParams{
				Width:    gsw,
				Height:   gsh,
				Template: gstemplate,
			}
			if useSkip && skip != nil {
				p.UseSkip = true
				p.Skip = skip
			}
			atx, aty := halftoneATPositions(gstemplate)
			copy(p.ATX[:], atx[:])
			copy(p.ATY[:], aty[:])

			current, err = decodeGenericRegion(budget, arithDec, p, arithCx)
			if err != nil {
				return nil, err
			}
		}

		// Gray-code XOR (§C.5.1 step 3b)
		if running == nil {
			running = current
		} else {
			for i := range running.Pix {
				running.Pix[i] ^= current.Pix[i]
			}
			freeBitmap(budget, current)
		}

		// accumulate bit j into gray-scale values (§C.5.1 step 4)
		for y := range gsh {
			for x := range gsw {
				if running.GetPixel(x, y) {
					result[y*gsw+x] |= 1 << j
				}
			}
		}
	}
	freeBitmap(budget, running)

	return result, nil
}
