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

	"seehuhn.de/go/pdf/graphics/bitmap"
)

// processPatternDict decodes a pattern dictionary segment (§6.7, §7.4.4).
func (d *decoder) processPatternDict(hdr *segmentHeader, data []byte) error {
	if len(data) < 7 {
		return fmt.Errorf("pattern dictionary data too short: %d bytes", len(data))
	}

	// segment header (§7.4.4.1)
	flags := data[0]
	hdmmr := flags&1 != 0
	hdTemplate := int((flags >> 1) & 3)

	hdpw := int(data[1])
	hdph := int(data[2])
	if hdpw == 0 || hdph == 0 {
		return fmt.Errorf("invalid pattern dimensions: %dx%d", hdpw, hdph)
	}
	grayMax := int(binary.BigEndian.Uint32(data[3:7]))

	offset := 7

	// AT positions (§7.4.4.2: present only for HDTEMPLATE=0)
	var hdATX [4]int8
	var hdATY [4]int8
	if !hdmmr && hdTemplate == 0 {
		if offset+8 > len(data) {
			return fmt.Errorf("pattern dictionary AT flags truncated")
		}
		for i := range 4 {
			hdATX[i] = int8(data[offset+i*2])
			hdATY[i] = int8(data[offset+i*2+1])
		}
		offset += 8
	}

	// cap number of patterns relative to available data
	if grayMax+1 > len(data)*maxExpansion {
		return fmt.Errorf("pattern count %d too large for %d bytes of data", grayMax+1, len(data))
	}

	// collective bitmap: all patterns side by side
	collectiveWidth := int(int64(grayMax+1) * int64(hdpw))
	if err := checkBitmapSize(collectiveWidth, hdph); err != nil {
		return err
	}

	var collectiveBM *bitmap.Bitmap
	if hdmmr {
		var err error
		collectiveBM, _, err = decodeMMR(data[offset:], collectiveWidth, hdph)
		if err != nil {
			return err
		}
	} else {
		p := &genericRegionParams{
			Width:    collectiveWidth,
			Height:   hdph,
			Template: hdTemplate,
		}
		switch hdTemplate {
		case 0:
			copy(p.ATX[:], hdATX[:])
			copy(p.ATY[:], hdATY[:])
		case 1:
			p.ATX[0] = 3
			p.ATY[0] = -1
		case 2, 3:
			p.ATX[0] = 2
			p.ATY[0] = -1
		}

		dec := newMQDecoder(data[offset:])
		var err error
		collectiveBM, err = decodeGenericRegion(dec, p, nil)
		if err != nil {
			return err
		}
	}

	// split collective bitmap into individual patterns (§6.7.5 step 4)
	patterns := make([]*bitmap.Bitmap, grayMax+1)
	for i := range patterns {
		xOff := i * hdpw
		pat := bitmap.New(hdpw, hdph)
		for y := range hdph {
			for x := range hdpw {
				pat.SetPixel(x, y, collectiveBM.GetPixel(xOff+x, y))
			}
		}
		patterns[i] = pat
	}

	d.segments[hdr.Number] = segmentResult{header: hdr, patterns: patterns}
	return nil
}
