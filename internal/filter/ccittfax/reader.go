// Package ccittfax implements CCITT Group 3 and Group 4 fax compression
// as specified in ITU-T T.4 and T.6, compatible with PDF CCITTFaxDecode filter.
package ccittfax

import (
	"bufio"
	"io"
)

// Reader decodes CCITT Fax compressed data.
type Reader struct {
	Params

	r         *bufio.Reader
	err       error  // Last read error, if any
	current   uint32 // up to 4 bytes of input, valid bits MSB-aligned
	validBits int    // number of valid bits in current

	line    []byte // Current line being decoded
	refLine []byte // Reference line (previous line) for 2D decoding

	// numRows is the number of complete lines delivered to the caller.
	numRows int
}

// NewReader creates a new CCITT Fax decoder.
func NewReader(r io.Reader, p *Params) *Reader {
	pCopy := *p
	if pCopy.Columns == 0 {
		pCopy.Columns = 1728 // Default as per PDF spec / common fax width
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
		r:       bufio.NewReader(r),
		line:    make([]byte, 0, lineBufSize),
		refLine: refLine,
		numRows: 0,
	}
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
	if !r.IgnoreEndOfBlock && r.peekBits(12) == 1 {
		r.consumeBits(12)
		if r.peekBits(12) == 1 {
			r.consumeBits(12)
			r.err = io.EOF
		}
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
	for r.peekBits(11) == 0 {
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
	r.consumeBits(int(entry.Width))

	return int(entry.Param), entry.State
}

// decode2D is a helper function that handles the common 2D decoding process
// used by both Group 3 2D and Group 4 encodings.
func (r *Reader) decode2D() {
	r.line = r.line[:0]

	white := r.whiteBit()

	a0 := -1
	currentCol := white
	for a0 < r.Columns && r.err == nil {
		value := r.peekBits(7)
		entry := mainTable[value]

		if entry.State == S_EOL {
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
			// First run (of current color)
			runLength, _ := r.decodeRun(currentCol == white)
			runLength = min(runLength, r.Columns-a0)
			a0 = max(a0, 0)
			r.fillRowBits(a0, a0+runLength, currentCol == 1)
			a0 += runLength

			// Second run (of opposite color)
			runLength, _ = r.decodeRun(currentCol != white)
			runLength = min(runLength, r.Columns-a0)
			r.fillRowBits(a0, a0+runLength, currentCol == 0)
			a0 += runLength

		case S_Vert:
			a1 := b1 + int(int16(entry.Param))
			r.fillRowBits(a0, a1, currentCol == 1)
			currentCol = 1 - currentCol
			a0 = a1

		case S_Ext:
			panic("not implemented: S_Ext")
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

func (b *Reader) peekBits(n int) uint32 {
	if n > 24 {
		panic("invalid n")
	}

	for b.validBits < n {
		var x byte
		if b.err == nil { // after the first error, use an inifinite stream of zeros
			x, b.err = b.r.ReadByte()
		}
		b.current |= uint32(x) << (24 - b.validBits)
		b.validBits += 8
	}
	return b.current >> (32 - n)
}

func (b *Reader) consumeBits(n int) {
	if b.validBits < n {
		b.peekBits(n)
	}
	b.current <<= n
	b.validBits -= n
}

func (b *Reader) readBits(n int) uint32 {
	res := b.peekBits(n)
	b.consumeBits(n)
	return res
}

// waitForOne consumes bits, one by one, until a 1 has been consumed.
func (b *Reader) waitForOne() {
	for b.err == nil && b.readBits(1) == 0 {
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
