// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package image

import (
	"image"
)

// colorKey evaluates a colour key on len(pix) pixels with n components: it
// sets pix[i] to 0 where every component of pixel i lies in its key range,
// and to 255 elsewhere.  sample returns component c of pixel i.
func colorKey(pix []uint8, n int, key []uint16, sample func(i, c int) int32) {
	for i := range pix {
		pix[i] = 0
		for c := range n {
			v := sample(i, c)
			if v < int32(key[2*c]) || v > int32(key[2*c+1]) {
				pix[i] = 255
				break
			}
		}
	}
}

// rawColorKey evaluates the colour key on one row of the packed samples
// of an image other than JPXDecode, and stores the result in dst, which
// has one entry per pixel.  The key must have two entries per colour
// component, and bpc must be 1, 2, 4, 8 or 16.
func rawColorKey(dst []uint8, row []byte, n, bpc int, key []uint16) {
	sample := func(i, c int) int32 {
		k := i*n + c
		switch bpc {
		case 8:
			return int32(row[k])
		case 16:
			return int32(row[2*k])<<8 | int32(row[2*k+1])
		default:
			perByte := 8 / bpc
			shift := (perByte - 1 - k%perByte) * bpc
			return int32(row[k/perByte]>>shift) & (1<<bpc - 1)
		}
	}
	colorKey(dst, n, key, sample)
}

// jpxColorKey evaluates the colour key on the samples of a JPXDecode
// image, chosen as in [Dict.loadJPX].  The key must have two entries per
// colour component.
func jpxColorKey(s *jpxSamples, w, h int, key []uint16) *image.Alpha {
	sample := func(i, c int) int32 {
		if s.planes != nil {
			return s.planes[c][i]
		}
		return s.entries[s.index[i]][c]
	}
	alpha := image.NewAlpha(image.Rect(0, 0, w, h))
	colorKey(alpha.Pix, s.cs.Channels(), key, sample)
	return alpha
}
