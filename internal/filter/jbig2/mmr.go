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
	"bytes"
	"fmt"
	"io"

	"seehuhn.de/go/pdf/graphics/bitmap"
	"seehuhn.de/go/pdf/internal/filter/ccittfax"
)

// decodeMMR decodes an MMR-coded (CCITT Group 4) generic region.
// It returns the decoded bitmap and the number of bytes consumed from data.
func decodeMMR(pool *bitmapPool, data []byte, width, height int) (*bitmap.Bitmap, int, error) {
	if width <= 0 || height <= 0 {
		return bitmap.New(0, 0), 0, nil
	}

	params := &ccittfax.Params{
		Columns:  width,
		K:        -1, // Group 4
		BlackIs1: true,
	}

	// route CCITT's per-line buffers through the pool so they share the
	// peak-only accounting with the JBIG2 bitmap allocations
	ccittBuf := ccittfax.BufferBytes(params)
	if err := pool.charge(ccittBuf); err != nil {
		return nil, 0, err
	}
	defer pool.release(ccittBuf)

	br := bytes.NewReader(data)
	reader, err := ccittfax.NewReaderRaw(br, params)
	if err != nil {
		return nil, 0, fmt.Errorf("MMR decode: %w", err)
	}

	bm, err := pool.allocBitmap(width, height)
	if err != nil {
		return nil, 0, err
	}

	if err := pool.chargeWork(int64(width) * int64(height)); err != nil {
		return nil, 0, err
	}

	buf := make([]byte, (width+7)/8)
	for y := range height {
		_, err := io.ReadFull(reader, buf)
		copy(bm.Pix[y*bm.Stride:], buf[:bm.Stride])
		if err != nil {
			break
		}
	}

	consumed := len(data) - br.Len()
	return bm, consumed, nil
}

// encodeMMR encodes a bitmap as MMR (CCITT Group 4) data.
func encodeMMR(bm *bitmap.Bitmap) ([]byte, error) {
	var buf bytes.Buffer
	params := &ccittfax.Params{
		Columns:  bm.Width(),
		K:        -1, // Group 4
		BlackIs1: true,
	}

	w, err := ccittfax.NewWriter(&buf, params)
	if err != nil {
		return nil, fmt.Errorf("MMR encode: %w", err)
	}

	stride := (bm.Width() + 7) / 8
	for y := range bm.Height() {
		row := bm.Pix[y*bm.Stride : y*bm.Stride+stride]
		if _, err := w.Write(row); err != nil {
			return nil, fmt.Errorf("MMR encode row %d: %w", y, err)
		}
	}

	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("MMR encode close: %w", err)
	}

	return buf.Bytes(), nil
}
