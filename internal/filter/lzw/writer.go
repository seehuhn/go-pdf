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

package lzw

// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

import (
	"bufio"
	"errors"
	"io"
)

// A writer is a buffered, flushable writer.
type writer interface {
	io.ByteWriter
	Flush() error
}

const (
	// A code is a 12 bit value, stored as a uint32 when encoding to avoid
	// type conversions when shifting bits.
	maxCode     = 1<<12 - 1
	invalidCode = 1<<32 - 1
	// There are 1<<12 possible codes, which is an upper bound on the number of
	// valid hash table entries at any given point in time. tableSize is 4x that.
	tableSize = 4 * 1 << 12
	tableMask = tableSize - 1
	// A hash table entry is a uint32. Zero is an invalid entry since the
	// lower 12 bits of a valid entry must be a non-literal code.
	invalidEntry = 0
)

// Writer is an LZW compressor. It writes the compressed form of the data
// to an underlying writer (see [NewWriter]).
type Writer struct {
	// dst is the writer that compressed bytes are written to.
	dst writer
	// bits, nBits and currentWidth are the state for
	// converting a code stream into a byte stream.
	bits         uint32
	nBits        uint
	currentWidth uint
	// err is the first error encountered during writing. Closing the writer
	// will make any future Write calls return errClosed
	err error

	// hi is the code implied by the next code emission.
	// overflow is the code at which hi overflows the code width.
	hi, overflow uint32
	// savedCode is the accumulated code at the end of the most recent Write
	// call. It is equal to invalidCode if there was no such call.
	savedCode uint32
	// table is the hash table from 20-bit keys to 12-bit values. Each table
	// entry contains key<<12|val and collisions resolve by linear probing.
	// The keys consist of a 12-bit code prefix and an 8-bit byte suffix.
	// The values are a 12-bit code.
	table [tableSize]uint32

	earlyChange uint32 // the off-by-one error allowed by the PDF spec
}

// NewWriter creates a new io.WriteCloser.
// Writes to the returned io.WriteCloser are compressed and written to dst.
// It is the caller's responsibility to call Close on the WriteCloser when
// finished writing.
func NewWriter(dst io.Writer, earlyChange bool) (*Writer, error) {
	bw, ok := dst.(writer)
	if !ok && dst != nil {
		bw = bufio.NewWriter(dst)
	}

	w := &Writer{}
	w.dst = bw
	w.currentWidth = 1 + litWidth
	w.hi = eof
	w.overflow = 1 << (litWidth + 1)
	w.savedCode = invalidCode

	if earlyChange {
		w.earlyChange = 1
	}

	err := w.write(clear) // PDF expects a leading clear code.
	if err != nil {
		return nil, err
	}

	return w, nil
}

// Close closes the Writer, flushing any pending output.  It does not close
// w's underlying writer.
func (w *Writer) Close() error {
	if w.err == errClosed {
		return nil
	} else if w.err != nil {
		return w.err
	}

	// Make any future calls to Write return errClosed.
	w.err = errClosed

	// Write the savedCode if valid.
	if w.savedCode != invalidCode {
		if err := w.write(w.savedCode); err != nil {
			w.err = err
			return err
		}
		if err := w.incHi(); err != nil && err != errOutOfCodes {
			w.err = err
			return err
		}
	}
	// Write the eof code.
	if err := w.write(eof); err != nil {
		return err
	}

	// Write the final bits.
	if w.nBits > 0 {
		w.bits >>= 24
		if err := w.dst.WriteByte(uint8(w.bits)); err != nil {
			return err
		}
	}
	return w.dst.Flush()
}

// Write writes a compressed representation of p to w's underlying writer.
func (w *Writer) Write(p []byte) (n int, err error) {
	if w.err != nil {
		return 0, w.err
	}

	n = len(p)
	if n == 0 {
		return 0, nil
	}

	code := w.savedCode
	if code == invalidCode {
		// The first code sent is always a literal code.
		code, p = uint32(p[0]), p[1:]
	}
loop:
	for _, x := range p {
		literal := uint32(x)
		key := code<<8 | literal
		// If there is a hash table hit for this key then we continue the loop
		// and do not emit a code yet.
		hash := (key>>12 ^ key) & tableMask
		for h, t := hash, w.table[hash]; t != invalidEntry; {
			if key == t>>12 {
				code = t & maxCode
				continue loop
			}
			h = (h + 1) & tableMask
			t = w.table[h]
		}
		// Otherwise, write the current code, and literal becomes the start of
		// the next emitted code.
		if w.err = w.write(code); w.err != nil {
			return 0, w.err
		}
		code = literal
		// Increment e.hi, the next implied code. If we run out of codes, reset
		// the writer state (including clearing the hash table) and continue.
		if err1 := w.incHi(); err1 != nil {
			if err1 == errOutOfCodes {
				continue
			}
			w.err = err1
			return 0, w.err
		}
		// Otherwise, insert key -> e.hi into the map that e.table represents.
		for {
			if w.table[hash] == invalidEntry {
				w.table[hash] = (key << 12) | w.hi
				break
			}
			hash = (hash + 1) & tableMask
		}
	}
	w.savedCode = code
	return n, nil
}

// incHi increments e.hi and checks for both overflow and running out of
// unused codes. In the latter case, incHi sends a clear code, resets the
// writer state and returns errOutOfCodes.
func (w *Writer) incHi() error {
	w.hi++
	if w.hi+w.earlyChange == w.overflow {
		w.currentWidth++
		w.overflow <<= 1
	}
	if w.hi+w.earlyChange == maxCode {
		if err := w.write(clear); err != nil {
			return err
		}
		w.currentWidth = litWidth + 1
		w.hi = eof
		w.overflow = clear << 1
		for i := range w.table {
			w.table[i] = invalidEntry
		}
		return errOutOfCodes
	}
	return nil
}

// writeMSB writes the code c for "Most Significant Bits first" data.
func (w *Writer) write(c uint32) error {
	w.bits |= c << (32 - w.currentWidth - w.nBits)
	w.nBits += w.currentWidth
	for w.nBits >= 8 {
		if err := w.dst.WriteByte(byte(w.bits >> 24)); err != nil {
			return err
		}
		w.bits <<= 8
		w.nBits -= 8
	}
	return nil
}

// errOutOfCodes is an internal error that means that the writer has run out
// of unused codes and a clear code needs to be sent next.
var errOutOfCodes = errors.New("lzw: out of codes")
