// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package image

import (
	"image"
)

// colorKeyMask returns an alpha plane of the given size, with 0 where
// every component of a pixel lies in its key range and 255 elsewhere.
// sample returns component c of pixel i.
func colorKeyMask(w, h, n int, key []uint16, sample func(i, c int) int32) *image.Alpha {
	alpha := image.NewAlpha(image.Rect(0, 0, w, h))
	for i := range alpha.Pix {
		for c := range n {
			v := sample(i, c)
			if v < int32(key[2*c]) || v > int32(key[2*c+1]) {
				alpha.Pix[i] = 255
				break
			}
		}
	}
	return alpha
}

// rawColorKey evaluates the colour key on the packed samples of an image
// other than JPXDecode, where data has been padded to the full image
// size.  The key must have two entries per colour component, and bpc must
// be 1, 2, 4, 8 or 16.
func rawColorKey(data []byte, w, h, n, bpc int, key []uint16) *image.Alpha {
	rowBytes := (w*n*bpc + 7) / 8
	sample := func(i, c int) int32 {
		row := data[(i/w)*rowBytes:]
		k := (i%w)*n + c
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
	return colorKeyMask(w, h, n, key, sample)
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
	return colorKeyMask(w, h, s.cs.Channels(), key, sample)
}
