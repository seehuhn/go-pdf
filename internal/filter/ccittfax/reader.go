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
	err       error  // sticky error, returned once the pending line is drained
	readErr   error  // error from the underlying reader, other than io.EOF
	current   uint32 // up to 4 bytes of input, valid bits MSB-aligned
	validBits int    // number of valid bits in current
	padBits   int    // zero bits supplied past the end of the input

	line    []byte // Current line being decoded
	refLine []byte // Reference line (previous line) for 2D decoding and row substitution

	// refChanges holds the changing elements of refLine, rebuilt once per
	// 2D row so findB1B2 is O(log n) rather than a pixel scan (§ params.go).
	refChanges []int

	// numRows is the number of complete lines delivered to the caller.
	numRows int

	numDamaged  int  // damaged rows substituted so far
	prevDamaged bool // whether the previous row was substituted
}

// needsRefLine reports whether decoding keeps a copy of the previous row:
// for 2D modes as the reference line, and where damaged rows are tolerated
// as the substitute row.
func (p Params) needsRefLine() bool {
	return p.K != 0 || (p.EndOfLine && p.DamagedRowsBeforeError > 0)
}

// BufferBytes returns the working-memory size [NewReader] will allocate
// for these parameters: the line buffer plus, for 2D modes (K != 0) and
// where damaged rows are tolerated, the reference line buffer and, for
// 2D modes, the reference-line changing-elements index (up to one int per
// column).  Callers should charge their memory budget for this amount
// before calling [NewReader] or [NewReaderRaw].
func BufferBytes(p *Params) int {
	columns := p.Columns
	if columns == 0 {
		columns = 1728
	}
	if columns < 0 {
		return 0
	}
	lineBufSize := (columns + 7) / 8
	n := lineBufSize
	if p.needsRefLine() {
		n += lineBufSize
	}
	if p.K != 0 {
		const intBytes = 8 // 64-bit int width of the changing-elements index
		n += columns * intBytes
	}
	return n
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

	res := &Reader{
		Params: pCopy,
		r:      r,
		line:   make([]byte, 0, lineBufSize),
	}
	if pCopy.needsRefLine() {
		// all white, with the bits beyond Columns zero as in decoded rows
		res.fillRowBits(0, pCopy.Columns, !pCopy.BlackIs1)
		res.refLine = append([]byte(nil), res.line...)
		res.line = res.line[:0]
	}
	return res, nil
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

// decodeScanLine decodes the next row into r.line.  On end of data,
// end of block, or an unrecoverable decoding error, r.line is left empty
// and r.err is set.
func (r *Reader) decodeScanLine() {
	r.line = r.line[:0]

	if r.EncodedByteAlign && !r.EndOfLine {
		// Without EOL codes the fill bits precede the row.  With EOL
		// codes the fill precedes the EOL instead and skipEOL consumes it;
		// aligning here would eat the leading zeros of the EOL.
		r.consumeBits(r.validBits % 8)
	}

	numEOL, tag, ended := r.skipEOL()
	if numEOL >= 2 && !r.IgnoreEndOfBlock {
		// EOFB (K < 0) or RTC (K >= 0); no valid row is empty, so two
		// consecutive EOL codes can only mean end of block
		r.err = io.EOF
		return
	}
	if ended {
		r.err = io.EOF
		if r.readErr != nil {
			r.err = r.readErr
		}
		return
	}

	var pos int
	switch {
	case r.K < 0:
		pos = r.decode2D()
	case r.K == 0:
		pos = r.decode1D()
	default:
		if numEOL == 0 {
			tag = r.readBits(1)
		}
		if tag == 1 {
			pos = r.decode1D()
		} else {
			pos = r.decode2D()
		}
	}

	if r.err != nil {
		r.repairRow(pos)
		if r.err != nil {
			r.line = r.line[:0]
			return
		}
	} else {
		r.prevDamaged = false
	}

	if r.K < 0 && !r.IgnoreEndOfBlock && r.peekBits(24) == 0x001001 {
		// consume the EOFB right away, so that a caller which reads a
		// known number of rows sees the full extent of the encoded data
		r.consumeBits(24)
		r.err = io.EOF
	}

	copy(r.refLine, r.line)
}

// skipEOL consumes fill bits and end-of-line codes preceding a row.
// It returns the number of EOL codes consumed, for K > 0 the tag bit
// following the last one, and whether the input is exhausted.
func (r *Reader) skipEOL() (numEOL int, tag uint32, ended bool) {
	for {
		if r.skipFill() {
			return numEOL, tag, true
		}
		if r.peekBits(12) != 0b000000000001 {
			return numEOL, tag, false
		}
		r.consumeBits(12)
		numEOL++
		if r.K > 0 {
			tag = r.readBits(1)
		}
	}
}

// skipFill consumes zero bits as long as at least twelve are ahead, and
// reports whether only padding past the end of the input remains.
// No run-length or mode code has more than seven leading zeros, so
// twelve zero bits can only be fill before an EOL, or the end of the data.
func (r *Reader) skipFill() bool {
	for r.peekBits(12) == 0 {
		if r.validBits <= r.padBits {
			return true
		}
		r.consumeBits(1)
	}
	return false
}

// repairRow handles a row whose decoding failed after pos pixels.
// A row cut short by the end of the input is completed with white
// pixels.  Otherwise, where the parameters allow it, the row is replaced
// by the previous row (or a white row) and decoding resumes at the next
// EOL code; r.err is cleared in both cases.
func (r *Reader) repairRow(pos int) {
	white := !r.BlackIs1
	if r.skipFill() {
		r.err = nil
		r.fillRowBits(max(pos, 0), r.Columns, white)
		return
	}

	if r.K < 0 || !r.EndOfLine || r.numDamaged >= r.DamagedRowsBeforeError {
		return
	}
	r.numDamaged++
	r.err = nil

	r.line = r.line[:0]
	if r.prevDamaged {
		r.fillRowBits(0, r.Columns, white)
	} else {
		r.line = append(r.line, r.refLine...)
	}
	r.prevDamaged = true

	// resynchronise at the next EOL code, which skipEOL will consume
	for r.peekBits(12) != 0b000000000001 && r.validBits > r.padBits {
		r.consumeBits(1)
	}
}

// decode1D decodes a one-dimensionally coded row into r.line and returns
// the number of pixels decoded.
func (r *Reader) decode1D() int {
	xpos := 0
	isWhite := true

	for xpos < r.Columns && r.err == nil {
		runLength, state := r.decodeRun(isWhite)
		if state == S_EOL {
			r.err = errPrematureEOL
			break
		}

		runLength = min(runLength, r.Columns-xpos)
		r.fillRowBits(xpos, xpos+runLength, isWhite != r.BlackIs1)
		xpos += runLength

		switch state {
		case S_TermW: // end of white run
			isWhite = false
		case S_TermB: // end of black run
			isWhite = true
		}
	}
	return xpos
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
// An EOL code is reported but left in the input.
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
		r.err = errInvalidRunCode
		return 0, entry.State
	}
	if entry.State == S_EOL {
		return 0, S_EOL
	}
	r.consumeBits(int(entry.Width))

	return int(entry.Param), entry.State
}

// decode2D decodes a two-dimensionally coded row (Group 3 2D or Group 4)
// into r.line and returns the number of pixels decoded.
func (r *Reader) decode2D() int {
	white := r.whiteBit()
	r.refChanges = r.changingElements(r.refLine, r.refChanges[:0])

	a0 := -1
	prevA0 := -2         // track forward progress
	prevCol := white ^ 1 // impossible initial value (opposite of starting color)
	currentCol := white
	for a0 < r.Columns && r.err == nil {
		// guard against malformed data that doesn't advance the cursor
		if a0 == prevA0 && currentCol == prevCol {
			r.err = errNoProgress
			break
		}
		prevA0 = a0
		prevCol = currentCol
		value := r.peekBits(7)
		entry := mainTable[value]

		if entry.State == S_EOL {
			// No 2D mode code begins with seven zero bits, so the lookup
			// table only reports S_EOL for an all-zero prefix; anything
			// that is not a full end-of-line marker is an invalid code.
			// The EOL itself is left in the input.
			if r.peekBits(11) != 0 {
				r.err = errInvalid2DCode
			} else {
				r.err = errPrematureEOL
			}
			break
		}
		r.consumeBits(int(entry.Width))

		b1, b2 := findB1B2FromChanges(r.refChanges, r.Columns, a0, currentCol, white)

		switch entry.State {
		case S_Pass:
			r.fillRowBits(a0, b2, currentCol == 1)
			a0 = b2 // a0 jumps to position below b2

		case S_Horiz:
			// first run (of current color)
			a0 = max(a0, 0)
			runLength := r.decodeFullRun(currentCol == white)
			runLength = min(runLength, r.Columns-a0)
			r.fillRowBits(a0, a0+runLength, currentCol == 1)
			a0 += runLength

			// second run (of opposite color)
			runLength = r.decodeFullRun(currentCol != white)
			runLength = min(runLength, r.Columns-a0)
			r.fillRowBits(a0, a0+runLength, currentCol == 0)
			a0 += runLength

		case S_Vert:
			a1 := min(b1+int(int16(entry.Param)), r.Columns)
			r.fillRowBits(a0, a1, currentCol == 1)
			currentCol = 1 - currentCol
			a0 = a1

		case S_Ext:
			r.err = errExtension
		}
	}
	return max(a0, 0)
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
		x, err := r.r.ReadByte()
		if err != nil {
			// past the end of the input, supply zero bits
			x = 0
			r.padBits += 8
			if err != io.EOF && r.readErr == nil {
				r.readErr = err
			}
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
