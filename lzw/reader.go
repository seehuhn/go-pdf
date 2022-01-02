// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2022  Jochen Voss <voss@seehuhn.de>
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

// Some code here is taken from "compress/lzw" (and then modified).  Use of
// this source code is governed by a BSD-style license, which is reproduced
// here:
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

// Package lzw implements the Lempel-Ziv-Welch compressed data format,
// described in T. A. Welch, ``A Technique for High-Performance Data
// Compression'', Computer, 17(6) (June 1984), pp 8-19.
//
// In particular, it implements LZW as used by the PDF file
// format, which means variable-width codes up to 12 bits and the first
// two non-literal codes are a clear code and an EOF code.
// Both the correct and the "early change" variant are implemented.
package lzw

import (
	"bufio"
	"errors"
	"io"
)

const (
	litWidth = 8
	maxWidth = 12

	clear              = 1 << litWidth
	eof                = clear + 1
	flushBuffer        = 1 << maxWidth
	decoderInvalidCode = 0xffff
)

// Reader is an io.Reader which can be used to read compressed data in the
// LZW format.
type Reader struct {
	src          io.ByteReader
	bits         uint32
	nBits        uint
	currentWidth uint
	err          error

	// The first 1<<litWidth codes are literal codes.
	// The next two codes mean clear and EOF.
	// Other valid codes are in the range [lo, hi] where lo := clear + 2,
	// with the upper bound incrementing on each code seen.
	//
	// overflow is the code at which hi overflows the code width. It always
	// equals 1 << width.
	//
	// last is the most recently seen code, or decoderInvalidCode.
	//
	// An invariant is that hi < overflow.
	hi, overflow, last uint16

	// Each code c in [lo, hi] expands to two or more bytes. For c != hi:
	//   suffix[c] is the last of these bytes.
	//   prefix[c] is the code for all but the last byte.
	//   This code can either be a literal code or another code in [lo, c).
	// The c == hi case is a special case.
	suffix [1 << maxWidth]uint8
	prefix [1 << maxWidth]uint16

	// output is the temporary output buffer.
	// Literal codes are accumulated from the start of the buffer.
	// Non-literal codes decode to a sequence of suffixes that are first
	// written right-to-left from the end of the buffer before being copied
	// to the start of the buffer.
	// It is flushed when it contains >= 1<<maxWidth bytes,
	// so that there is always room to decode an entire code.
	output [2 * 1 << maxWidth]byte
	o      int    // write index into output
	toRead []byte // bytes to return from Read

	earlyChange uint16 // the off-by-one error allowed by the PDF spec
}

// readMSB returns the next code for "Most Significant Bits first" data.
func (r *Reader) read() (uint16, error) {
	for r.nBits < r.currentWidth {
		x, err := r.src.ReadByte()
		if err != nil {
			return 0, err
		}
		r.bits |= uint32(x) << (24 - r.nBits)
		r.nBits += 8
	}
	code := uint16(r.bits >> (32 - r.currentWidth))
	r.bits <<= r.currentWidth
	r.nBits -= r.currentWidth
	return code, nil
}

// Read implements io.Reader, reading uncompressed bytes from its underlying Reader.
func (r *Reader) Read(b []byte) (int, error) {
	for {
		if len(r.toRead) > 0 {
			n := copy(b, r.toRead)
			r.toRead = r.toRead[n:]
			return n, nil
		}
		if r.err != nil {
			return 0, r.err
		}
		r.decode()
	}
}

// decode decompresses bytes from src and leaves them in r.toRead.
func (r *Reader) decode() {
	// Loop over the code stream, converting codes into decompressed bytes.
loop:
	for {
		code, err := r.read()
		if err != nil {
			if err == io.EOF {
				err = io.ErrUnexpectedEOF
			}
			r.err = err
			break
		}
		switch {
		case code < clear:
			// We have a literal code.
			r.output[r.o] = uint8(code)
			r.o++
			if r.last != decoderInvalidCode {
				// Save what the hi code expands to.
				r.suffix[r.hi] = uint8(code)
				r.prefix[r.hi] = r.last
			}
		case code == clear:
			r.currentWidth = 1 + uint(litWidth)
			r.hi = eof
			r.overflow = 1 << r.currentWidth
			r.last = decoderInvalidCode
			continue
		case code == eof:
			r.err = io.EOF
			break loop
		case code <= r.hi:
			c, i := code, len(r.output)-1
			if code == r.hi && r.last != decoderInvalidCode {
				// code == hi is a special case which expands to the last expansion
				// followed by the head of the last expansion. To find the head, we walk
				// the prefix chain until we find a literal code.
				c = r.last
				for c >= clear {
					c = r.prefix[c]
				}
				r.output[i] = uint8(c)
				i--
				c = r.last
			}
			// Copy the suffix chain into output and then write that to w.
			for c >= clear {
				r.output[i] = r.suffix[c]
				i--
				c = r.prefix[c]
			}
			r.output[i] = uint8(c)
			r.o += copy(r.output[r.o:], r.output[i:])
			if r.last != decoderInvalidCode {
				// Save what the hi code expands to.
				r.suffix[r.hi] = uint8(c)
				r.prefix[r.hi] = r.last
			}
		default:
			r.err = errors.New("lzw: invalid code")
			break loop
		}
		r.last, r.hi = code, r.hi+1
		if r.hi+r.earlyChange >= r.overflow {
			if r.currentWidth >= maxWidth {
				r.last = decoderInvalidCode
				// Undo the r.hi++ a few lines above, so that (1) we maintain
				// the invariant that r.hi < r.overflow, and (2) r.hi does not
				// eventually overflow a uint16.
				r.hi--
			} else {
				r.currentWidth++
				r.overflow = 1 << r.currentWidth
			}
		}
		if r.o >= flushBuffer {
			break
		}
	}
	// Flush pending output.
	r.toRead = r.output[:r.o]
	r.o = 0
}

var errClosed = errors.New("lzw: reader/writer is closed")

// Close closes the Reader and returns an error for any future read operation.
// It does not close the underlying io.Reader.
func (r *Reader) Close() error {
	if r.err == errClosed {
		return nil
	} else if r.err != nil && r.err != io.EOF {
		return r.err
	}

	r.err = errClosed // in case any Reads come along
	return nil
}

// NewReader creates a new io.ReadCloser.
// Reads from the returned io.ReadCloser read and decompress data from src.
// If src does not also implement io.ByteReader,
// the decompressor may read more data than necessary from src.
// It is the caller's responsibility to call Close() on the ReadCloser when
// finished reading.
//
// It is guaranteed that the underlying type of the returned io.ReadCloser
// is a *Reader.
func NewReader(src io.Reader, earlyChange bool) io.ReadCloser {
	br, ok := src.(io.ByteReader)
	if !ok && src != nil {
		br = bufio.NewReader(src)
	}

	r := &Reader{}
	r.src = br
	r.currentWidth = 1 + uint(litWidth)
	r.hi = eof
	r.overflow = uint16(1) << r.currentWidth
	r.last = decoderInvalidCode

	if earlyChange {
		r.earlyChange = 1
	}

	return r
}
