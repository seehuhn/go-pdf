// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2021  Jochen Voss <voss@seehuhn.de>
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

// Some code here is taken from "image/png" (and then modified).  Use of this
// source code is governed by a BSD-style license, which is reproduced here:
//
//     Copyright (c) 2009 The Go Authors. All rights reserved.
//
//     Redistribution and use in source and binary forms, with or without
//     modification, are permitted provided that the following conditions are
//     met:
//
//        * Redistributions of source code must retain the above copyright
//     notice, this list of conditions and the following disclaimer.
//        * Redistributions in binary form must reproduce the above
//     copyright notice, this list of conditions and the following disclaimer
//     in the documentation and/or other materials provided with the
//     distribution.
//        * Neither the name of Google Inc. nor the names of its
//     contributors may be used to endorse or promote products derived from
//     this software without specific prior written permission.
//
//     THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
//     "AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
//     LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
//     A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT
//     OWNER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
//     SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT
//     LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
//     DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
//     THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
//     (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
//     OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.

// Frequencies of filter types used in the PDF files on my system:
//     165622 FlateDecode
//      11334 CCITTFaxDecode
//       7595 DCTDecode
//       3440 LZWDecode
//       3431 ASCII85Decode
//        455 JBIG2Decode
//        166 ASCIIHexDecode
//         78 JPXDecode
//          5 RunLengthDecode

package pdf

import (
	"compress/zlib"
	"errors"
	"io"
	"strconv"
)

// FilterInfo describes one PDF stream filter.
type FilterInfo struct {
	Name  Name
	Parms Dict
}

func (fi *FilterInfo) getFilter() (filter, error) {
	switch fi.Name {
	case "FlateDecode":
		return ffFromDict(fi.Parms), nil
	default:
		return nil, errors.New("unsupported filter type " + string(fi.Name))
	}
}

type flateFilter struct {
	Predictor        int
	Colors           int
	BitsPerComponent int
	Columns          int
	EarlyChange      bool
}

func ffFromDict(parms Dict) *flateFilter {
	res := &flateFilter{ // set defaults
		Predictor:        1,
		Colors:           1,
		BitsPerComponent: 8,
		Columns:          1,
		EarlyChange:      true,
	}
	if parms == nil {
		return res
	}

	if val, ok := parms["Predictor"].(Integer); ok {
		res.Predictor = int(val)
	}
	if val, ok := parms["Colors"].(Integer); ok {
		res.Colors = int(val)
	}
	if val, ok := parms["BitsPerComponent"].(Integer); ok {
		res.BitsPerComponent = int(val)
	}
	if val, ok := parms["Columns"].(Integer); ok {
		res.Columns = int(val)
	}
	if val, ok := parms["EarlyChange"].(Integer); ok {
		res.EarlyChange = (val != 0)
	}
	return res
}

func (ff *flateFilter) ToDict() Dict {
	res := Dict{}
	needed := false
	if ff.Predictor != 1 {
		res["Predictor"] = Integer(ff.Predictor)
		needed = true
	}
	if ff.Colors != 1 {
		res["Colors"] = Integer(ff.Colors)
		needed = true
	}
	if ff.BitsPerComponent != 8 {
		res["BitsPerComponent"] = Integer(ff.BitsPerComponent)
		needed = true
	}
	if ff.Columns != 1 {
		res["Columns"] = Integer(ff.Columns)
		needed = true
	}
	if !ff.EarlyChange {
		res["EarlyChange"] = Integer(0)
		needed = true
	}
	if !needed {
		return nil
	}
	return res
}

func (ff *flateFilter) Decode(r io.Reader) (io.Reader, error) {
	if !ff.EarlyChange {
		return nil, errors.New("unsupported /EarlyChange setting")
	}

	var res io.Reader
	var err error
	res, err = zlib.NewReader(r)
	if err != nil {
		return nil, err
	}
	switch {
	case ff.Predictor == 1:
		// pass
	case ff.Predictor >= 10 && ff.Predictor <= 15:
		if ff.Colors < 1 || ff.Colors > 64 {
			return nil, errors.New("invalid number of colour channels")
		}
		if ff.BitsPerComponent < 1 || ff.BitsPerComponent > 32 {
			return nil, errors.New("invalid number of bits per component")
		}
		if ff.Columns < 1 || ff.Columns > 1<<20 {
			return nil, errors.New("invalid number of columns")
		}
		res = ff.newPngReader(res)
	default:
		return nil, errors.New("unsupported predictor " + strconv.Itoa(ff.Predictor))
	}
	return res, nil
}

type pngReader struct {
	r io.Reader

	bytesPerPixel int

	cr   []byte // current row
	pr   []byte // previous row
	pend []byte // data already converted, but not yet read by client
}

func (ff *flateFilter) newPngReader(r io.Reader) *pngReader {
	res := &pngReader{
		r: r,
	}
	bitsPerPixel := ff.BitsPerComponent * ff.Colors
	res.bytesPerPixel = (bitsPerPixel + 7) / 8

	// The +1 is for the per-row filter type, which is at cr[0].
	rowSize := 1 + (bitsPerPixel*ff.Columns+7)/8
	res.cr = make([]uint8, rowSize)
	res.pr = make([]uint8, rowSize)

	return res
}

func (r *pngReader) Read(b []byte) (int, error) {
	n := 0
	for len(b) > 0 {
		if len(r.pend) > 0 {
			m := copy(b, r.pend)
			n += m
			b = b[m:]
			r.pend = r.pend[m:]
			continue
		}
		_, err := io.ReadFull(r.r, r.cr)
		if err != nil {
			return n, err
		}

		// Apply the filter.
		ft := r.cr[0]
		if ft >= nFilter {
			return 0, errors.New("bad PNG filter type")
		}
		pngDec[ft](r.cr[1:], r.pr[1:], r.bytesPerPixel)
		r.pend = r.cr[1:]

		// The current row for y is the previous row for y+1.
		r.pr, r.cr = r.cr, r.pr
	}

	return n, nil
}

func (ff *flateFilter) Encode(w io.WriteCloser) (io.WriteCloser, error) {
	if !ff.EarlyChange {
		return nil, errors.New("unsupported /EarlyChange setting")
	}

	zw := zlib.NewWriter(w)

	close := func() error {
		err := zw.Close()
		if err != nil {
			return err
		}
		return w.Close()
	}

	switch {
	case ff.Predictor == 1:
		return &withClose{zw, close}, nil
	case ff.Predictor >= 10 && ff.Predictor <= 15:
		if ff.Colors < 1 || ff.Colors > 64 {
			return nil, errors.New("invalid number of colour channels")
		}
		if ff.BitsPerComponent < 1 || ff.BitsPerComponent > 32 {
			return nil, errors.New("invalid number of bits per component")
		}
		if ff.Columns < 1 || ff.Columns > 1<<20 {
			return nil, errors.New("invalid number of columns")
		}
		return ff.newPngWriter(zw, close), nil
	default:
		return nil, errors.New("unsupported predictor " + strconv.Itoa(ff.Predictor))
	}
}

type pngWriter struct {
	w     io.Writer
	close func() error

	predictor    int
	bitsPerPixel int

	cr  [nFilter][]uint8
	pr  []uint8
	pos int
}

func (ff *flateFilter) newPngWriter(w io.Writer, close func() error) *pngWriter {
	res := &pngWriter{
		w:     w,
		close: close,
	}
	if ff.Predictor > 10 || ff.Predictor < 15 {
		res.predictor = ff.Predictor - 10
	}
	res.bitsPerPixel = ff.BitsPerComponent * ff.Colors

	// cr[*] and pr are the bytes for the current and previous row. cr[0] is
	// unfiltered (or equivalently, filtered with the ftNone filter). cr[ft],
	// for non-zero filter types ft, are buffers for transforming cr[0] under
	// the other PNG filter types. These buffers are allocated once and re-used
	// for each row. The +1 is for the per-row filter type, which is at
	// cr[*][0].
	sz := 1 + (res.bitsPerPixel*ff.Columns+7)/8
	for i := range res.cr {
		res.cr[i] = make([]uint8, sz)
		res.cr[i][0] = uint8(i)
	}
	res.pr = make([]uint8, sz)

	return res
}

// Chooses the filter to use for encoding the current row, and applies it. The
// return value is the index of the filter and also of the row in cr that has
// had it applied.
//
// We try all five filter types, and pick the one that minimizes the sum of
// absolute differences. This is the same heuristic that libpng uses,
// although the filters are attempted in order of estimated most likely to
// be minimal, rather than in their enumeration order.
func (w *pngWriter) choosePredictor() int {
	cdat := w.cr[0][1:]
	pdat := w.pr[1:]
	bpp := (w.bitsPerPixel + 7) / 8

	best := maxInt
	filter := -1
	for _, ft := range []int{ftUp, ftNone, ftPaeth, ftSub, ftAverage} {
		out := w.cr[ft][1:]
		pngEnc[ft](out, cdat, pdat, bpp)

		sum := 0
		for _, c := range out {
			sum += abs8(c)
			if sum >= best {
				break
			}
		}
		if sum < best {
			best = sum
			filter = ft
		}
	}

	return filter
}

func (w *pngWriter) Write(p []byte) (int, error) {
	n := 0
	for len(p) > 0 {
		tmp := w.cr[0][1:]
		l := copy(tmp[w.pos:], p)
		p = p[l:]
		w.pos += l
		n += l
		if w.pos >= len(tmp) {
			var ft int
			if w.predictor > 0 {
				ft = w.predictor
				out := w.cr[ft][1:]
				cdat := w.cr[0][1:]
				pdat := w.pr[1:]
				bpp := (w.bitsPerPixel + 7) / 8
				pngEnc[ft](out, cdat, pdat, bpp)
			} else {
				ft = w.choosePredictor()
			}
			_, err := w.w.Write(w.cr[ft])
			if err != nil {
				return n, err
			}

			// The current row for y is the previous row for y+1.
			w.cr[0], w.pr = w.pr, w.cr[0]
			w.pos = 0
		}
	}
	return n, nil
}

func (w *pngWriter) Close() error {
	if w.close != nil {
		return w.close()
	}
	return nil
}

// Filter type, as per the PNG spec.
const (
	ftNone    = 0
	ftSub     = 1
	ftUp      = 2
	ftAverage = 3
	ftPaeth   = 4
	nFilter   = 5
)

var pngDec = [nFilter]func([]byte, []byte, int){
	pngNoneDec,
	pngSubDec,
	pngUpDec,
	pngAverageDec,
	pngPaethDec,
}

var pngEnc = [nFilter]func([]byte, []byte, []byte, int){
	pngNoneEnc,
	pngSubEnc,
	pngUpEnc,
	pngAverageEnc,
	pngPaethEnc,
}

func pngNoneDec(cdat, pdat []byte, bpp int) {
	// No-op.
}

func pngNoneEnc(out, cdat, pdat []byte, bpp int) {
	copy(out, cdat)
}

func pngSubDec(cdat, pdat []byte, bpp int) {
	for i := bpp; i < len(cdat); i++ {
		cdat[i] += cdat[i-bpp]
	}
}

func pngSubEnc(out, cdat, pdat []byte, bpp int) {
	for i := 0; i < bpp; i++ {
		out[i] = cdat[i]
	}
	for i := bpp; i < len(out); i++ {
		out[i] = cdat[i] - cdat[i-bpp]
	}
}

func pngUpDec(cdat, pdat []byte, bpp int) {
	for i, p := range pdat {
		cdat[i] += p
	}
}

func pngUpEnc(out, cdat, pdat []byte, bpp int) {
	for i := 0; i < len(out); i++ {
		out[i] = cdat[i] - pdat[i]
	}
}

func pngAverageDec(cdat, pdat []byte, bpp int) {
	// The first column has no column to the left of it, so it is a special
	// case.  We know that the first column exists because we verify Columns>0
	// in flateFilter.Decode().
	for i := 0; i < bpp; i++ {
		cdat[i] += pdat[i] / 2
	}
	for i := bpp; i < len(cdat); i++ {
		cdat[i] += byte((int(cdat[i-bpp]) + int(pdat[i])) / 2)
	}
}

func pngAverageEnc(out, cdat, pdat []byte, bpp int) {
	for i := 0; i < bpp; i++ {
		out[i] = cdat[i] - pdat[i]/2
	}
	for i := bpp; i < len(out); i++ {
		out[i] = cdat[i] - uint8((int(cdat[i-bpp])+int(pdat[i]))/2)
	}
}

// pngPaethDec implements the Paeth filter function, as per the PNG
// specification.
func pngPaethDec(cdat, pdat []byte, bpp int) {
	var a, b, c, pa, pb, pc int
	for i := 0; i < bpp; i++ {
		a, c = 0, 0
		for j := i; j < len(cdat); j += bpp {
			b = int(pdat[j])
			pa = b - c
			pb = a - c
			pc = abs(pa + pb)
			pa = abs(pa)
			pb = abs(pb)
			if pa <= pb && pa <= pc {
				// No-op.
			} else if pb <= pc {
				a = b
			} else {
				a = c
			}
			a += int(cdat[j])
			a &= 0xff
			cdat[j] = uint8(a)
			c = b
		}
	}
}

// pngPaethEnc implements the Paeth filter function, as per the PNG
// specification.
func pngPaethEnc(out, cdat, pdat []byte, bpp int) {
	for i := 0; i < bpp; i++ {
		out[i] = cdat[i] - pdat[i]
	}
	for i := bpp; i < len(out); i++ {
		a := cdat[i-bpp]
		b := pdat[i]
		c := pdat[i-bpp]

		// This is an optimized version of the sample code in the PNG spec.
		pc := int(c)
		pa := int(b) - pc
		pb := int(a) - pc
		pc = abs(pa + pb)
		pa = abs(pa)
		pb = abs(pb)
		var x byte
		if pa <= pb && pa <= pc {
			x = a
		} else if pb <= pc {
			x = b
		} else {
			x = c
		}

		out[i] = cdat[i] - x
	}
}

// intSize is either 32 or 64.
const intSize = 32 << (^uint(0) >> 63)

const maxInt = int(^uint(0) >> 1)

func abs(x int) int {
	// m := -1 if x < 0. m := 0 otherwise.
	m := x >> (intSize - 1)

	// In two's complement representation, the negative number of any number
	// (except the smallest one) can be computed by flipping all the bits and
	// add 1. This is faster than code with a branch.
	// See Hacker's Delight, section 2-4.
	return (x ^ m) - m
}

// The absolute value of a byte interpreted as a signed int8.
func abs8(d uint8) int {
	if d < 128 {
		return int(d)
	}
	return 256 - int(d)
}

type withoutClose struct {
	io.Writer
}

func (w withoutClose) Close() error {
	return nil
}

type withClose struct {
	io.Writer
	close func() error
}

func (w *withClose) Close() error {
	return w.close()
}

type filter interface {
	ToDict() Dict
	Encode(w io.WriteCloser) (io.WriteCloser, error)
	Decode(r io.Reader) (io.Reader, error)
}
