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

	// compute skip bitmap if enabled (§6.6.5.1)
	var hskip *bitmap.Bitmap
	if henableSkip {
		if err := checkBitmapSize(hgw, hgh); err != nil {
			return err
		}
		hskip = bitmap.New(hgw, hgh)
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
	grayImage, err := decodeGrayScaleImage(data[offset:], hmmr, htemplate, hbpp, hgw, hgh, henableSkip, hskip)
	if err != nil {
		return err
	}

	// fill region with default pixel (§6.6.5.2 step 1)
	if err := checkBitmapSize(int(rsi.Width), int(rsi.Height)); err != nil {
		return err
	}
	bm := bitmap.New(int(rsi.Width), int(rsi.Height))
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

	// composite onto page
	if d.pageBitmap != nil {
		op := bitmap.CombOp(rsi.CombOp)
		d.pageBitmap.Combine(bm, int(rsi.X), int(rsi.Y), op)
	}

	d.segments[hdr.Number] = segmentResult{header: hdr, bm: bm}
	return nil
}

// decodeGrayScaleImage decodes a gray-scale image from bitplanes (Annex C).
// Returns a flat array of gray-scale values, row-major [hgh][hgw].
func decodeGrayScaleImage(
	data []byte,
	gsmmr bool, gstemplate int,
	gsbpp, gsw, gsh int,
	useSkip bool, skip *bitmap.Bitmap,
) ([]int, error) {
	if int64(gsw)*int64(gsh) > maxPixels {
		return nil, fmt.Errorf("gray-scale image too large: %d x %d", gsw, gsh)
	}
	if int64(gsw)*int64(gsh) > int64(len(data))*maxExpansion {
		return nil, fmt.Errorf("gray-scale image %dx%d too large for %d bytes of data", gsw, gsh, len(data))
	}
	result := make([]int, gsw*gsh)

	// decode GSBPP bitplanes (§C.5.1)
	planes := make([]*bitmap.Bitmap, gsbpp)
	dataOffset := 0

	for j := gsbpp - 1; j >= 0; j-- {
		if gsmmr {
			// MMR-coded bitplane
			var n int
			var err error
			planes[j], n, err = decodeMMR(data[dataOffset:], gsw, gsh)
			if err != nil {
				return nil, err
			}
			dataOffset += n
		} else {
			// arithmetic-coded bitplane
			p := &genericRegionParams{
				Width:    gsw,
				Height:   gsh,
				Template: gstemplate,
			}
			if useSkip && skip != nil {
				p.UseSkip = true
				p.Skip = skip
			}
			// AT pixel positions for gray-scale (Table C.4)
			atx, aty := halftoneATPositions(gstemplate)
			copy(p.ATX[:], atx[:])
			copy(p.ATY[:], aty[:])

			dec := newMQDecoder(data[dataOffset:])
			var err error
			planes[j], err = decodeGenericRegion(dec, p, nil)
			if err != nil {
				return nil, err
			}
			// advance past the MQ data consumed
			dataOffset += dec.bp + 1
		}

		// Gray-code XOR (§C.5.1 step 3b)
		if j < gsbpp-1 {
			for y := range gsh {
				for x := range gsw {
					above := planes[j+1].GetPixel(x, y)
					cur := planes[j].GetPixel(x, y)
					planes[j].SetPixel(x, y, above != cur)
				}
			}
		}
	}

	// assemble bitplanes into gray-scale values (§C.5.1 step 4)
	for y := range gsh {
		for x := range gsw {
			v := 0
			for j := range gsbpp {
				if planes[j].GetPixel(x, y) {
					v |= 1 << j
				}
			}
			result[y*gsw+x] = v
		}
	}

	return result, nil
}
