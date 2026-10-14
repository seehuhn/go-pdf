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

// This file is based on code from Go's "image/jpeg" package (and then
// modified).  Use of the original source code is governed by a BSD-style
// license, which is reproduced here:
//
//     Copyright 2011 The Go Authors.
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
//        * Neither the name of Google LLC nor the names of its
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

package jpeg

import (
	"bufio"
	"errors"
	"image/color"
	"io"
)

// div returns a/b rounded to the nearest integer, instead of rounded to zero.
func div(a, b int32) int32 {
	if a >= 0 {
		return (a + (b >> 1)) / b
	}
	return -((-a + (b >> 1)) / b)
}

// bitCount counts the number of bits needed to hold an integer.
var bitCount = [256]byte{
	0, 1, 2, 2, 3, 3, 3, 3, 4, 4, 4, 4, 4, 4, 4, 4,
	5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5,
	6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6,
	6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6,
	7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
	7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
	7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
	7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
	8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8,
	8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8,
	8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8,
	8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8,
	8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8,
	8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8,
	8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8,
	8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8,
}

type quantIndex int

const (
	quantIndexLuminance quantIndex = iota
	quantIndexChrominance
	nQuantIndex
)

// unscaledQuant are the unscaled quantization tables in zig-zag order. Each
// encoder copies and scales the tables according to its quality parameter.
// The values are derived from section K.1 of the spec, after converting from
// natural to zig-zag order.
var unscaledQuant = [nQuantIndex][blockSize]byte{
	// Luminance.
	{
		16, 11, 12, 14, 12, 10, 16, 14,
		13, 14, 18, 17, 16, 19, 24, 40,
		26, 24, 22, 22, 24, 49, 35, 37,
		29, 40, 58, 51, 61, 60, 57, 51,
		56, 55, 64, 72, 92, 78, 64, 68,
		87, 69, 55, 56, 80, 109, 81, 87,
		95, 98, 103, 104, 103, 62, 77, 113,
		121, 112, 100, 120, 92, 101, 103, 99,
	},
	// Chrominance.
	{
		17, 18, 18, 24, 21, 24, 47, 26,
		26, 47, 99, 66, 56, 66, 99, 99,
		99, 99, 99, 99, 99, 99, 99, 99,
		99, 99, 99, 99, 99, 99, 99, 99,
		99, 99, 99, 99, 99, 99, 99, 99,
		99, 99, 99, 99, 99, 99, 99, 99,
		99, 99, 99, 99, 99, 99, 99, 99,
		99, 99, 99, 99, 99, 99, 99, 99,
	},
}

type huffIndex int

const (
	huffIndexLuminanceDC huffIndex = iota
	huffIndexLuminanceAC
	huffIndexChrominanceDC
	huffIndexChrominanceAC
	nHuffIndex
)

// huffmanSpec specifies a Huffman encoding.
type huffmanSpec struct {
	// count[i] is the number of codes of length i+1 bits.
	count [16]byte
	// value[i] is the decoded value of the i'th codeword.
	value []byte
}

// theHuffmanSpec is the Huffman encoding specifications.
//
// This encoder uses the same Huffman encoding for all images. It is also the
// same Huffman encoding used by section K.3 of the spec.
//
// The DC tables have 12 decoded values, called categories.
//
// The AC tables have 162 decoded values: bytes that pack a 4-bit Run and a
// 4-bit Size. There are 16 valid Runs and 10 valid Sizes, plus two special R|S
// cases: 0|0 (meaning EOB) and F|0 (meaning ZRL).
var theHuffmanSpec = [nHuffIndex]huffmanSpec{
	// Luminance DC.
	{
		[16]byte{0, 1, 5, 1, 1, 1, 1, 1, 1, 0, 0, 0, 0, 0, 0, 0},
		[]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11},
	},
	// Luminance AC.
	{
		[16]byte{0, 2, 1, 3, 3, 2, 4, 3, 5, 5, 4, 4, 0, 0, 1, 125},
		[]byte{
			0x01, 0x02, 0x03, 0x00, 0x04, 0x11, 0x05, 0x12,
			0x21, 0x31, 0x41, 0x06, 0x13, 0x51, 0x61, 0x07,
			0x22, 0x71, 0x14, 0x32, 0x81, 0x91, 0xa1, 0x08,
			0x23, 0x42, 0xb1, 0xc1, 0x15, 0x52, 0xd1, 0xf0,
			0x24, 0x33, 0x62, 0x72, 0x82, 0x09, 0x0a, 0x16,
			0x17, 0x18, 0x19, 0x1a, 0x25, 0x26, 0x27, 0x28,
			0x29, 0x2a, 0x34, 0x35, 0x36, 0x37, 0x38, 0x39,
			0x3a, 0x43, 0x44, 0x45, 0x46, 0x47, 0x48, 0x49,
			0x4a, 0x53, 0x54, 0x55, 0x56, 0x57, 0x58, 0x59,
			0x5a, 0x63, 0x64, 0x65, 0x66, 0x67, 0x68, 0x69,
			0x6a, 0x73, 0x74, 0x75, 0x76, 0x77, 0x78, 0x79,
			0x7a, 0x83, 0x84, 0x85, 0x86, 0x87, 0x88, 0x89,
			0x8a, 0x92, 0x93, 0x94, 0x95, 0x96, 0x97, 0x98,
			0x99, 0x9a, 0xa2, 0xa3, 0xa4, 0xa5, 0xa6, 0xa7,
			0xa8, 0xa9, 0xaa, 0xb2, 0xb3, 0xb4, 0xb5, 0xb6,
			0xb7, 0xb8, 0xb9, 0xba, 0xc2, 0xc3, 0xc4, 0xc5,
			0xc6, 0xc7, 0xc8, 0xc9, 0xca, 0xd2, 0xd3, 0xd4,
			0xd5, 0xd6, 0xd7, 0xd8, 0xd9, 0xda, 0xe1, 0xe2,
			0xe3, 0xe4, 0xe5, 0xe6, 0xe7, 0xe8, 0xe9, 0xea,
			0xf1, 0xf2, 0xf3, 0xf4, 0xf5, 0xf6, 0xf7, 0xf8,
			0xf9, 0xfa,
		},
	},
	// Chrominance DC.
	{
		[16]byte{0, 3, 1, 1, 1, 1, 1, 1, 1, 1, 1, 0, 0, 0, 0, 0},
		[]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11},
	},
	// Chrominance AC.
	{
		[16]byte{0, 2, 1, 2, 4, 4, 3, 4, 7, 5, 4, 4, 0, 1, 2, 119},
		[]byte{
			0x00, 0x01, 0x02, 0x03, 0x11, 0x04, 0x05, 0x21,
			0x31, 0x06, 0x12, 0x41, 0x51, 0x07, 0x61, 0x71,
			0x13, 0x22, 0x32, 0x81, 0x08, 0x14, 0x42, 0x91,
			0xa1, 0xb1, 0xc1, 0x09, 0x23, 0x33, 0x52, 0xf0,
			0x15, 0x62, 0x72, 0xd1, 0x0a, 0x16, 0x24, 0x34,
			0xe1, 0x25, 0xf1, 0x17, 0x18, 0x19, 0x1a, 0x26,
			0x27, 0x28, 0x29, 0x2a, 0x35, 0x36, 0x37, 0x38,
			0x39, 0x3a, 0x43, 0x44, 0x45, 0x46, 0x47, 0x48,
			0x49, 0x4a, 0x53, 0x54, 0x55, 0x56, 0x57, 0x58,
			0x59, 0x5a, 0x63, 0x64, 0x65, 0x66, 0x67, 0x68,
			0x69, 0x6a, 0x73, 0x74, 0x75, 0x76, 0x77, 0x78,
			0x79, 0x7a, 0x82, 0x83, 0x84, 0x85, 0x86, 0x87,
			0x88, 0x89, 0x8a, 0x92, 0x93, 0x94, 0x95, 0x96,
			0x97, 0x98, 0x99, 0x9a, 0xa2, 0xa3, 0xa4, 0xa5,
			0xa6, 0xa7, 0xa8, 0xa9, 0xaa, 0xb2, 0xb3, 0xb4,
			0xb5, 0xb6, 0xb7, 0xb8, 0xb9, 0xba, 0xc2, 0xc3,
			0xc4, 0xc5, 0xc6, 0xc7, 0xc8, 0xc9, 0xca, 0xd2,
			0xd3, 0xd4, 0xd5, 0xd6, 0xd7, 0xd8, 0xd9, 0xda,
			0xe2, 0xe3, 0xe4, 0xe5, 0xe6, 0xe7, 0xe8, 0xe9,
			0xea, 0xf2, 0xf3, 0xf4, 0xf5, 0xf6, 0xf7, 0xf8,
			0xf9, 0xfa,
		},
	},
}

// huffmanLUT is a compiled look-up table representation of a huffmanSpec.
// Each value maps to a uint32 of which the 8 most significant bits hold the
// codeword size in bits and the 24 least significant bits hold the codeword.
// The maximum codeword size is 16 bits.
type huffmanLUT []uint32

func (h *huffmanLUT) init(s huffmanSpec) {
	maxValue := 0
	for _, v := range s.value {
		if int(v) > maxValue {
			maxValue = int(v)
		}
	}
	*h = make([]uint32, maxValue+1)
	code, k := uint32(0), 0
	for i := range len(s.count) {
		nBits := uint32(i+1) << 24
		for j := uint8(0); j < s.count[i]; j++ {
			(*h)[s.value[k]] = nBits | code
			code++
			k++
		}
		code <<= 1
	}
}

// theHuffmanLUT are compiled representations of theHuffmanSpec.
var theHuffmanLUT [4]huffmanLUT

func init() {
	for i, s := range theHuffmanSpec {
		theHuffmanLUT[i].init(s)
	}
}

// writer is a buffered writer.
type writer interface {
	Flush() error
	io.Writer
	io.ByteWriter
}

// encoder encodes an image to the JPEG format.
type encoder struct {
	// w is the writer to write to. err is the first error encountered during
	// writing. All attempted writes after the first error become no-ops.
	w   writer
	err error
	// buf is a scratch buffer, large enough for a 4-component SOF0 body.
	buf [6 + 3*maxComponents]byte
	// bits and nBits are accumulated bits to write to w.
	bits, nBits uint32
	// quant is the scaled quantization tables, in zig-zag order.
	quant [nQuantIndex][blockSize]byte

	// p holds the image geometry and component layout.
	p EncodeParams
	// chroma is set when components 2 and 3 hold Cb and Cr.  These
	// components then use the chrominance tables and are subsampled
	// 4:2:0, while components 1 and 4 have sampling factors 2x2.
	chroma bool
	// stripe holds the samples of one MCU row, after colour conversion.
	stripe []byte
}

func (e *encoder) flush() {
	if e.err != nil {
		return
	}
	e.err = e.w.Flush()
}

func (e *encoder) write(p []byte) {
	if e.err != nil {
		return
	}
	_, e.err = e.w.Write(p)
}

func (e *encoder) writeByte(b byte) {
	if e.err != nil {
		return
	}
	e.err = e.w.WriteByte(b)
}

// emit emits the least significant nBits bits of bits to the bit-stream.
// The precondition is bits < 1<<nBits && nBits <= 16.
func (e *encoder) emit(bits, nBits uint32) {
	nBits += e.nBits
	bits <<= 32 - nBits
	bits |= e.bits
	for nBits >= 8 {
		b := uint8(bits >> 24)
		e.writeByte(b)
		if b == 0xff {
			e.writeByte(0x00)
		}
		bits <<= 8
		nBits -= 8
	}
	e.bits, e.nBits = bits, nBits
}

// emitHuff emits the given value with the given Huffman encoder.
func (e *encoder) emitHuff(h huffIndex, value int32) {
	x := theHuffmanLUT[h][value]
	e.emit(x&(1<<24-1), x>>24)
}

// emitHuffRLE emits a run of runLength copies of value encoded with the given
// Huffman encoder.
func (e *encoder) emitHuffRLE(h huffIndex, runLength, value int32) {
	a, b := value, value
	if a < 0 {
		a, b = -value, value-1
	}
	var nBits uint32
	if a < 0x100 {
		nBits = uint32(bitCount[a])
	} else {
		nBits = 8 + uint32(bitCount[a>>8])
	}
	e.emitHuff(h, runLength<<4|int32(nBits))
	if nBits > 0 {
		e.emit(uint32(b)&(1<<nBits-1), nBits)
	}
}

// writeMarkerHeader writes the header for a marker with the given length.
func (e *encoder) writeMarkerHeader(marker uint8, markerlen int) {
	e.buf[0] = 0xff
	e.buf[1] = marker
	e.buf[2] = uint8(markerlen >> 8)
	e.buf[3] = uint8(markerlen & 0xff)
	e.write(e.buf[:4])
}

// writeDQT writes the Define Quantization Table marker.
func (e *encoder) writeDQT() {
	const markerlen = 2 + int(nQuantIndex)*(1+blockSize)
	e.writeMarkerHeader(dqtMarker, markerlen)
	for i := range e.quant {
		e.writeByte(uint8(i))
		e.write(e.quant[i][:])
	}
}

// mcuSize returns the width and height of an MCU in pixels.
func (e *encoder) mcuSize() int {
	if e.chroma {
		return 16
	}
	return 8
}

// samplingFactor returns the horizontal and vertical sampling factor of
// component i.
func (e *encoder) samplingFactor(i int) int {
	if e.chroma && (i == 0 || i == 3) {
		return 2
	}
	return 1
}

// quantIndexOf returns the quantization and Huffman table index used for
// component i.
func (e *encoder) quantIndexOf(i int) quantIndex {
	if e.chroma && (i == 1 || i == 2) {
		return quantIndexChrominance
	}
	return quantIndexLuminance
}

// writeAPP14 writes an Adobe APP14 marker with the given transform flag
// (0 = none, 1 = YCbCr, 2 = YCbCrK).
func (e *encoder) writeAPP14(transform byte) {
	e.writeMarkerHeader(app14Marker, 14)
	e.write([]byte{'A', 'd', 'o', 'b', 'e', 0x00, 0x64, 0x00, 0x00, 0x00, 0x00, transform})
}

// writeSOF0 writes the Start Of Frame (Baseline Sequential) marker.
func (e *encoder) writeSOF0() {
	nComponent := e.p.Components
	markerlen := 8 + 3*nComponent
	e.writeMarkerHeader(sof0Marker, markerlen)
	e.buf[0] = 8 // 8-bit color.
	e.buf[1] = uint8(e.p.Height >> 8)
	e.buf[2] = uint8(e.p.Height & 0xff)
	e.buf[3] = uint8(e.p.Width >> 8)
	e.buf[4] = uint8(e.p.Width & 0xff)
	e.buf[5] = uint8(nComponent)
	for i := range nComponent {
		f := uint8(e.samplingFactor(i))
		e.buf[3*i+6] = uint8(i + 1)
		e.buf[3*i+7] = f<<4 | f
		e.buf[3*i+8] = uint8(e.quantIndexOf(i))
	}
	e.write(e.buf[:6+3*nComponent])
}

// writeDHT writes the Define Huffman Table marker.
func (e *encoder) writeDHT() {
	markerlen := 2
	specs := theHuffmanSpec[:]
	if !e.chroma {
		// Drop the Chrominance tables.
		specs = specs[:2]
	}
	for _, s := range specs {
		markerlen += 1 + 16 + len(s.value)
	}
	e.writeMarkerHeader(dhtMarker, markerlen)
	for i, s := range specs {
		e.writeByte("\x00\x10\x01\x11"[i])
		e.write(s.count[:])
		e.write(s.value)
	}
}

// writeBlock writes a block of pixel data using the given quantization table,
// returning the post-quantized DC value of the DCT-transformed block. b is in
// natural (not zig-zag) order.
func (e *encoder) writeBlock(b *block, q quantIndex, prevDC int32) int32 {
	fdct(b)
	// Emit the DC delta.
	dc := div(b[0], 8*int32(e.quant[q][0]))
	e.emitHuffRLE(huffIndex(2*q+0), 0, dc-prevDC)
	// Emit the AC components.
	h, runLength := huffIndex(2*q+1), int32(0)
	for zig := 1; zig < blockSize; zig++ {
		ac := div(b[unzig[zig]], 8*int32(e.quant[q][zig]))
		if ac == 0 {
			runLength++
		} else {
			for runLength > 15 {
				e.emitHuff(h, 0xf0)
				runLength -= 16
			}
			e.emitHuffRLE(h, runLength, ac)
			runLength = 0
		}
	}
	if runLength > 0 {
		e.emitHuff(h, 0x00)
	}
	return dc
}

// scale scales the 16x16 region represented by the 4 src blocks to the 8x8
// dst block.
func scale(dst *block, src *[4]block) {
	for i := range 4 {
		dstOff := (i&2)<<4 | (i&1)<<2
		for y := range 4 {
			for x := range 4 {
				j := 16*y + 2*x
				sum := src[i][j] + src[i][j+1] + src[i][j+8] + src[i][j+9]
				dst[8*y+x+dstOff] = (sum + 2) >> 2
			}
		}
	}
}

// writeSOS writes the Start Of Scan marker, followed by the entropy-coded
// image data read from r.
func (e *encoder) writeSOS(r io.Reader) error {
	nComponent := e.p.Components
	e.writeMarkerHeader(sosMarker, 6+2*nComponent)
	e.writeByte(uint8(nComponent))
	for i := range nComponent {
		q := uint8(e.quantIndexOf(i))
		e.writeByte(uint8(i + 1))
		e.writeByte(q<<4 | q)
	}
	// Section B.2.3 of the spec says that for sequential DCTs, the
	// bytes Ss, Se, Ah|Al should be 0x00, 0x3f, 0x00<<4 | 0x00.
	e.write([]byte{0x00, 0x3f, 0x00})

	mcu := e.mcuSize()
	e.stripe = make([]byte, mcu*e.p.Width*nComponent)
	var prevDC [maxComponents]int32
	for y := 0; y < e.p.Height; y += mcu {
		nRows := min(mcu, e.p.Height-y)
		if err := e.readStripe(r, nRows); err != nil {
			return err
		}
		e.convertStripe(nRows)
		e.writeStripe(nRows, &prevDC)
		if e.err != nil {
			return e.err
		}
	}
	// Pad the last byte with 1's.
	e.emit(0x7f, 7)
	return nil
}

// readStripe reads nRows rows of samples from r into e.stripe.
func (e *encoder) readStripe(r io.Reader, nRows int) error {
	_, err := io.ReadFull(r, e.stripe[:nRows*e.p.Width*e.p.Components])
	if err == io.EOF {
		err = io.ErrUnexpectedEOF
	}
	return err
}

// convertStripe applies the colour transform, if any, in place to the
// first nRows rows of e.stripe.  RGB becomes YCbCr; CMYK becomes YCbCrK,
// where the YCbCr part is computed from the inverted CMY values.
func (e *encoder) convertStripe(nRows int) {
	if !e.chroma {
		return
	}
	nc := e.p.Components
	pix := e.stripe[:nRows*e.p.Width*nc]
	for i := 0; i < len(pix); i += nc {
		r, g, b := pix[i], pix[i+1], pix[i+2]
		if nc == 4 {
			r, g, b = 255-r, 255-g, 255-b
		}
		pix[i], pix[i+1], pix[i+2] = color.RGBToYCbCr(r, g, b)
	}
}

// fillBlock copies the 8x8 region of component c whose top-left corner is
// at column x and stripe row y into b.  Samples beyond the right edge of
// the image or beyond the nRows valid stripe rows replicate the last
// column or row.
func (e *encoder) fillBlock(b *block, c, x, y, nRows int) {
	nc := e.p.Components
	rowLen := e.p.Width * nc
	xmax := e.p.Width - 1
	for j := range 8 {
		row := e.stripe[min(y+j, nRows-1)*rowLen:]
		for i := range 8 {
			b[8*j+i] = int32(row[min(x+i, xmax)*nc+c])
		}
	}
}

// writeStripe encodes the MCUs of one MCU row, using the first nRows
// rows of e.stripe.  prevDC holds the DC predictors of all components.
func (e *encoder) writeStripe(nRows int, prevDC *[maxComponents]int32) {
	var (
		// The blocks are in natural (not zig-zag) order.
		b  block
		cb [4]block
	)
	mcu := e.mcuSize()
	for x := 0; x < e.p.Width; x += mcu {
		for c := range e.p.Components {
			q := e.quantIndexOf(c)
			if e.chroma && e.samplingFactor(c) == 1 {
				// subsampled chroma
				for i := range 4 {
					e.fillBlock(&cb[i], c, x+(i&1)*8, (i&2)*4, nRows)
				}
				scale(&b, &cb)
				prevDC[c] = e.writeBlock(&b, q, prevDC[c])
				continue
			}
			f := e.samplingFactor(c)
			for i := range f * f {
				e.fillBlock(&b, c, x+(i%f)*8, (i/f)*8, nRows)
				prevDC[c] = e.writeBlock(&b, q, prevDC[c])
			}
		}
	}
}

// DefaultQuality is the default quality encoding parameter.
const DefaultQuality = 75

// EncodeParams describes the sample stream given to [EncodeStream].
type EncodeParams struct {
	// Width and Height are the image dimensions in pixels, in the range
	// 1 to 65535.
	Width, Height int

	// Components is the number of colour components per pixel: 1, 3 or 4.
	Components int

	// Transform selects the colour transform.  For 3 components the
	// input RGB samples are stored as YCbCr, for 4 components the input
	// CMYK samples are stored as YCbCrK.  In both cases the chroma
	// components are subsampled 4:2:0.  If Transform is false, the
	// components are stored unchanged without subsampling.  Transform
	// is ignored for 1 component.
	Transform bool

	// Quality ranges from 1 to 100 inclusive, higher is better.
	// The value 0 selects [DefaultQuality].
	Quality int
}

// EncodeStream reads Width*Height*Components bytes of interleaved 8-bit
// samples from r, row by row without padding, and writes a baseline JPEG
// image to w.
//
// For 3 and 4 components an Adobe APP14 marker records whether the colour
// transform was applied.  Memory use is proportional to the image width.
func EncodeStream(w io.Writer, r io.Reader, p EncodeParams) error {
	if p.Width < 1 || p.Width >= 1<<16 || p.Height < 1 || p.Height >= 1<<16 {
		return errors.New("jpeg: invalid image size")
	}
	if p.Components != 1 && p.Components != 3 && p.Components != 4 {
		return errors.New("jpeg: unsupported number of components")
	}
	quality := p.Quality
	if quality == 0 {
		quality = DefaultQuality
	} else if quality < 1 || quality > 100 {
		return errors.New("jpeg: quality out of range")
	}

	var e encoder
	if ww, ok := w.(writer); ok {
		e.w = ww
	} else {
		e.w = bufio.NewWriter(w)
	}
	e.p = p
	e.chroma = p.Transform && p.Components >= 3
	e.initQuant(quality)

	// Write the Start Of Image marker.
	e.write([]byte{0xff, soiMarker})
	if p.Components >= 3 {
		var transform byte
		if e.chroma {
			transform = byte(p.Components - 2)
		}
		e.writeAPP14(transform)
	}
	e.writeDQT()
	e.writeSOF0()
	e.writeDHT()
	if err := e.writeSOS(r); err != nil {
		return err
	}
	// Write the End Of Image marker.
	e.write([]byte{0xff, eoiMarker})
	e.flush()
	return e.err
}

// initQuant scales the quantization tables for the given quality.
func (e *encoder) initQuant(quality int) {
	// Convert from a quality rating to a scaling factor.
	var scale int
	if quality < 50 {
		scale = 5000 / quality
	} else {
		scale = 200 - quality*2
	}
	for i := range e.quant {
		for j := range e.quant[i] {
			x := int(unscaledQuant[i][j])
			x = (x*scale + 50) / 100
			if x < 1 {
				x = 1
			} else if x > 255 {
				x = 255
			}
			e.quant[i][j] = uint8(x)
		}
	}
}
