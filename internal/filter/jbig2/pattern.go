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

	// collective bitmap: all patterns side by side
	collectiveWidth, err := checkedMul(grayMax+1, hdpw)
	if err != nil {
		return fmt.Errorf("pattern dictionary: %w", err)
	}

	var collectiveBM *bitmap.Bitmap
	if hdmmr {
		var err error
		collectiveBM, _, err = decodeMMR(&d.pool, data[offset:], collectiveWidth, hdph)
		if err != nil {
			return err
		}
	} else {
		// AT positions are implied by Table 27 — not present in the
		// segment data (unlike generic region segments).
		p := &genericRegionParams{
			Width:    collectiveWidth,
			Height:   hdph,
			Template: hdTemplate,
		}
		switch hdTemplate {
		case 0:
			p.ATX[0] = int8(-hdpw)
			p.ATY[0] = 0
			p.ATX[1] = -3
			p.ATY[1] = -1
			p.ATX[2] = 2
			p.ATY[2] = -2
			p.ATX[3] = -2
			p.ATY[3] = -2
		case 1:
			p.ATX[0] = 3
			p.ATY[0] = -1
		case 2, 3:
			p.ATX[0] = 2
			p.ATY[0] = -1
		}

		dec := newMQDecoder(data[offset:])
		var err error
		collectiveBM, err = decodeGenericRegion(&d.pool, dec, p, nil)
		if err != nil {
			return err
		}
	}

	// split collective bitmap into individual patterns (§6.7.5 step 4)
	widths, err := d.pool.allocInts(grayMax + 1)
	if err != nil {
		return err
	}
	for i := range widths {
		widths[i] = hdpw
	}
	patterns, err := splitBitmapH(&d.pool, collectiveBM, widths)
	d.pool.freeBitmap(collectiveBM)
	d.pool.freeInts(widths)
	if err != nil {
		return err
	}

	d.segments[hdr.Number] = segmentResult{header: hdr, patterns: patterns}
	return nil
}
