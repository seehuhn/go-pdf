// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2025  Jochen Voss <voss@seehuhn.de>
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

package ccittfax

import (
	"bufio"
	"errors"
	"io"
)

// Reader decodes CCITT Fax compressed data.
type Reader struct {
	Params

	r         io.ByteReader
	err       error  // Last read error, if any
	current   uint32 // up to 4 bytes of input, valid bits MSB-aligned
	validBits int    // number of valid bits in current

	line    []byte // Current line being decoded
	refLine []byte // Reference line (previous line) for 2D decoding

	// numRows is the number of complete lines delivered to the caller.
	numRows int
}

// BufferBytes returns the working-memory size [NewReader] will allocate
// for these parameters: the line buffer plus, for 2D modes (K != 0),
// the reference line buffer.  Callers should charge their memory
// budget for this amount before calling [NewReader] or [NewReaderRaw].
func BufferBytes(p *Params) int {
	columns := p.Columns
	if columns == 0 {
		columns = 1728
	}
	if columns < 0 {
		return 0
	}
	lineBufSize := (columns + 7) / 8
	if p.K != 0 {
		return 2 * lineBufSize
	}
	return lineBufSize
}

// NewReader creates a new CCITT Fax decoder.
//
// The reader is buffered internally; callers that need to continue reading
// from r after decoding should use [NewReaderRaw] with a [bytes.Reader]
// or similar [io.ByteReader] instead.
func NewReader(r io.Reader, p *Params) (*Reader, error) {
	return NewReaderRaw(bufio.NewReader(r), p)
}

// NewReaderRaw creates a CCITT Fax decoder reading from an existing
// [io.ByteReader]. Unlike [NewReader], no additional buffering is added,
// so the caller can determine the number of bytes consumed after decoding.
func NewReaderRaw(r io.ByteReader, p *Params) (*Reader, error) {
	pCopy := *p
	if pCopy.Columns == 0 {
		pCopy.Columns = 1728 // Default as per PDF spec / common fax width
	}

	if pCopy.Columns < 0 || pCopy.Columns > maxColumns {
		return nil, errors.New("invalid Columns value")
	}

	lineBufSize := (pCopy.Columns + 7) / 8

	var refLine []byte
	if pCopy.K != 0 {
		refLine = make([]byte, lineBufSize) // all white
		if pCopy.BlackIs1 {
			// When BlackIs1=true, white pixels are 0 (already initialized)
		} else {
			// When BlackIs1=false, white pixels are 1
			for i := range refLine {
				refLine[i] = 0xFF
			}
		}
	}

	return &Reader{
		Params:  pCopy,
		r:       r,
		line:    make([]byte, 0, lineBufSize),
		refLine: refLine,
		numRows: 0,
	}, nil
}

// Read decodes the next scan line into the provided buffer.
// Returns the number of bytes written and any error.
func (r *Reader) Read(buf []byte) (n int, err error) {
	if len(buf) == 0 {
		return 0, nil
	}

	if len(r.line) == 0 && r.err == nil && (r.MaxRows == 0 || r.numRows < r.MaxRows) {
		r.decodeScanLine()
	}

	if len(r.line) > 0 {
		n = copy(buf, r.line)
		copy(r.line, r.line[n:])
		r.line = r.line[:len(r.line)-n]
		if len(r.line) == 0 {
			r.numRows++
		}
		return n, nil
	}

	if r.err != nil {
		return 0, r.err
	}
	return 0, io.EOF
}

func (r *Reader) decodeScanLine() {
	if r.K < 0 {
		r.decodeG4ScanLine()
	} else if r.K == 0 {
		r.decodeG3ScanLine1D()
	} else {
		r.decodeG3ScanLine2D()
	}

	copy(r.refLine, r.line)
}

// decodeG4ScanLine decodes a single Group 4 (T.6) scanline.
func (r *Reader) decodeG4ScanLine() {
	// Group 4 fax uses pure 2D encoding for all lines
	// with no EOL codes or line mode switching
	r.decode2D()

	// Check for EOFB (End of Facsimile Block)
	// EOFB in Group 4 is 24 bits: 000000000001000000000001
	if !r.IgnoreEndOfBlock && r.peekBits(24) == 0x001001 {
		r.consumeBits(24)
		r.err = io.EOF
	}
}

// decodeG3ScanLine1D decodes a single Group 3 1D scanline
// and stores the result in r.line.
func (r *Reader) decodeG3ScanLine1D() {
	r.line = r.line[:0]

	xpos := 0
	isWhite := true

	numEOL := 0

	for xpos < r.Columns && r.err == nil {
		runLength, state := r.decodeRun(isWhite)

		runLength = min(runLength, r.Columns-xpos)
		r.fillRowBits(xpos, xpos+runLength, isWhite != r.BlackIs1)
		xpos += runLength

		switch state {
		case S_EOL:
			r.waitForOne()
			if xpos == 0 {
				numEOL++
				if !r.IgnoreEndOfBlock && numEOL >= 6 {
					r.err = io.EOF
					return
				}
			} else {
				return
			}
		case S_TermW: // end of white run
			isWhite = false
		case S_TermB: // end of black run
			isWhite = true
		}
	}
}

// decodeG3ScanLine2D decodes a Group 3 2D scanline (K > 0).
func (r *Reader) decodeG3ScanLine2D() {
	for r.err == nil && r.peekBits(11) == 0 {
		r.consumeBits(11)
		r.waitForOne() // allow for fill bits
	}

	tp := r.readBits(1)
	if tp == 1 { // 1D mode
		r.decodeG3ScanLine1D()
	} else { // 2D mode
		r.decode2D()
	}
}

// decodeFullRun decodes a complete run length, consuming any makeup codes
// followed by the terminating code.
func (r *Reader) decodeFullRun(isWhite bool) int {
	total := 0
	// A well-formed run has at most a few makeup codes followed by a
	// terminating code. Limit iterations to catch malformed data that
	// produces endless makeup codes from buffered bits.
	for range 64 {
		runLength, st := r.decodeRun(isWhite)
		total += runLength
		if st == S_TermW || st == S_TermB || st == S_EOL || r.err != nil {
			break
		}
		if total > r.Columns {
			break
		}
	}
	return total
}

// decodeRun decodes a run of pixels of the specified color.
// Returns the run length and a state indication (like S_EOL, S_TermW, S_TermB).
func (r *Reader) decodeRun(isWhite bool) (int, state) {
	var entry decodeNode
	if isWhite {
		value := r.peekBits(12)
		entry = whiteTable[value]
	} else {
		value := r.peekBits(13)
		entry = blackTable[value]
	}
	if entry.Width == 0 {
		// invalid bit pattern: no table entry matches
		r.err = errors.New("ccittfax: invalid run-length code")
		return 0, entry.State
	}
	r.consumeBits(int(entry.Width))

	return int(entry.Param), entry.State
}

// decode2D is a helper function that handles the common 2D decoding process
// used by both Group 3 2D and Group 4 encodings.
func (r *Reader) decode2D() {
	r.line = r.line[:0]

	white := r.whiteBit()

	a0 := -1
	prevA0 := -2         // track forward progress
	prevCol := white ^ 1 // impossible initial value (opposite of starting color)
	currentCol := white
	for a0 < r.Columns && r.err == nil {
		// guard against malformed data that doesn't advance the cursor
		if a0 == prevA0 && currentCol == prevCol {
			r.err = errors.New("ccittfax: no forward progress in 2D decode")
			return
		}
		prevA0 = a0
		prevCol = currentCol
		value := r.peekBits(7)
		entry := mainTable[value]

		if entry.State == S_EOL {
			// TODO(voss): add error handling
			if r.peekBits(11) == 0 {
				if a0 >= 0 {
					// error ...
				}
			} else {
				// error ...
			}
			break // End of line reached
		} else {
			r.consumeBits(int(entry.Width))
		}

		b1, b2 := r.findB1B2(r.refLine, a0, currentCol)

		switch entry.State {
		case S_Pass:
			r.fillRowBits(a0, b2, currentCol == 1)
			a0 = b2 // a0 jumps to position below b2

		case S_Horiz:
			// first run (of current color)
			runLength := r.decodeFullRun(currentCol == white)
			runLength = min(runLength, r.Columns-a0)
			a0 = max(a0, 0)
			r.fillRowBits(a0, a0+runLength, currentCol == 1)
			a0 += runLength

			// second run (of opposite color)
			runLength = r.decodeFullRun(currentCol != white)
			runLength = min(runLength, r.Columns-a0)
			r.fillRowBits(a0, a0+runLength, currentCol == 0)
			a0 += runLength

		case S_Vert:
			a1 := b1 + int(int16(entry.Param))
			r.fillRowBits(a0, a1, currentCol == 1)
			currentCol = 1 - currentCol
			a0 = a1

		case S_Ext:
			r.err = errors.New("ccittfax: unsupported extension code")
			return
		}
	}
}

// fillRowBits fills a range of bits in r.line. The line is taken to be a
// stream of bits, MSB first.  The line buffer is first extended using 0 bytes
// to contain at least end bits.  Then, if fill is true, bit positions from
// start (included) to end (excluded) are set to 1.  If fill is false,
// existing bits are left unchanged.
func (r *Reader) fillRowBits(start, end int, fill bool) {
	if start >= end {
		return
	}

	// Calculate how many bytes we need to hold 'end' bits
	requiredBytes := (end + 7) / 8

	// Extend line buffer if needed
	for len(r.line) < requiredBytes {
		r.line = append(r.line, 0)
	}

	// If not filling (white run), we're done - the buffer is already filled with zeros
	if !fill {
		return
	}

	// Fill with 1s (black) from start to end
	for pos := start; pos < end; pos++ {
		if pos < 0 {
			continue
		}
		bytePos := pos / 8
		bitPos := 7 - (pos % 8) // MSB first: bit 7 is leftmost
		r.line[bytePos] |= 1 << bitPos
	}
}

func (r *Reader) peekBits(n int) uint32 {
	if n > 24 {
		panic("invalid n")
	}

	for r.validBits < n {
		var x byte
		if r.err == nil { // after the first error, use an inifinite stream of zeros
			x, r.err = r.r.ReadByte()
		}
		r.current |= uint32(x) << (24 - r.validBits)
		r.validBits += 8
	}
	return r.current >> (32 - n)
}

func (r *Reader) consumeBits(n int) {
	if r.validBits < n {
		r.peekBits(n)
	}
	r.current <<= n
	r.validBits -= n
}

func (r *Reader) readBits(n int) uint32 {
	res := r.peekBits(n)
	r.consumeBits(n)
	return res
}

// waitForOne consumes bits, one by one, until a 1 has been consumed.
func (r *Reader) waitForOne() {
	for r.err == nil && r.readBits(1) == 0 {
		// pass
	}
}

// decodeNode defines the structure of entries in the Huffman decoding tables.
type decodeNode struct {
	State state
	Width uint8
	Param uint16
}

// encodeNode defines a table entry for encoding run lengths to code bits
type encodeNode struct {
	Code  uint32 // The bit pattern to output
	Width uint8  // Number of bits in the code
}

type state uint8
