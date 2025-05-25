// Package ccittfax implements CCITT Group 3 and Group 4 fax compression
// as specified in ITU-T T.4 and T.6, compatible with PDF CCITTFaxDecode filter.
package ccittfax

import (
	"bufio"
	"io"
)

// Reader decodes CCITT Fax compressed data.
type Reader struct {
	p Params

	// Bit reading state
	r         *bufio.Reader
	err       error  // Last read error, if any
	current   uint32 // up to 4 bytes of input, valid bits MSB-aligned
	validBits int    // number of valid bits in current

	// Line buffers (pixel data, 1 bit per pixel, 0=white/1=black before BlackIs1 inversion)
	line    []byte // Current line being decoded
	refLine []byte // Reference line (previous line) for 2D decoding

	// Decoding state
	kCounter int

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
	}

	return &Reader{
		p:        pCopy,
		r:        bufio.NewReader(r),
		line:     make([]byte, 0, lineBufSize),
		refLine:  refLine,
		kCounter: pCopy.K,
		numRows:  0,
	}
}

// Read decodes the next scan line into the provided buffer.
// Returns the number of bytes written and any error.
func (r *Reader) Read(buf []byte) (n int, err error) {
	if len(r.line) == 0 && r.err == nil && (r.p.MaxRows == 0 || r.numRows < r.p.MaxRows) {
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
	if r.p.K < 0 {
		r.decodeG4ScanLine()
	} else if r.p.K == 0 {
		r.decodeG3ScanLine1D()
	} else {
		r.decodeG3ScanLine2D()
	}
}

// decodeG4ScanLine decodes a single Group 4 (T.6) scanline.
func (r *Reader) decodeG4ScanLine() {
	// Group 4 fax uses pure 2D encoding for all lines
	// with no EOL codes or line mode switching
	r.decode2D(true)

	// Check for EOFB (End of Facsimile Block)
	// EOFB in Group 4 is 24 bits: 000000000001000000000001
	if !r.p.IgnoreEndOfBlock && r.peekBits(12) == 0 {
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

	for xpos < r.p.Columns && r.err == nil {
		runLength, state := r.decodeRun(isWhite)

		runLength = min(runLength, r.p.Columns-xpos)
		r.fillRowBits(xpos, xpos+runLength, isWhite != r.p.BlackIs1)
		xpos += runLength

		switch state {
		case S_EOL:
			r.waitForOne()
			if xpos == 0 {
				numEOL++
				if !r.p.IgnoreEndOfBlock && numEOL >= 6 {
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
// It follows the PDF specification: the first line is 1D, followed by K-1 2D lines, repeating.
func (r *Reader) decodeG3ScanLine2D() {
	// Handle K-counter cycling:
	// - First line in group uses 1D encoding
	// - K-1 lines use 2D encoding
	if r.kCounter <= 0 {
		r.decodeG3ScanLine1D()
		r.kCounter = r.p.K - 1
		if r.kCounter <= 0 {
			r.kCounter = r.p.K
		}
		return
	}

	// Use 2D encoding for this line
	use2D := r.decode2D(false)

	// If the tag bit after EOL indicates 1D mode for the next line,
	// reset the K counter
	if !use2D {
		r.kCounter = r.p.K
	} else {
		r.kCounter--
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
// Returns true if this line uses 2D encoding, false if next line should use 1D encoding.
func (r *Reader) decode2D(isG4 bool) bool {
	r.line = r.line[:0]

	// For Group 3 only, handle EOL and tag bit at the beginning
	if !isG4 && r.p.EndOfLine {
		// Look for EOL (12 zeros followed by a 1)
		for r.peekBits(12) == 0 {
			r.consumeBits(12)
			if r.readBits(1) == 1 {
				// Found an EOL

				// For G3 2D, check tag bit to determine mode for next line
				if r.readBits(1) == 1 {
					// Tag bit = 1 means next line uses 1D encoding
					r.kCounter = r.p.K // Reset K counter
					return true        // This line still uses 2D encoding
				}
				// Tag bit = 0 means continue with 2D
				break
			}
		}
	}

	// EOFB/RTC detection for Group 3 - check for consecutive EOLs
	if !isG4 && r.p.EndOfLine {
		eolCount := 0
		for r.peekBits(12) == 0 && r.err == nil {
			r.consumeBits(12)
			if r.readBits(1) == 1 {
				eolCount++
				// 6 consecutive EOLs = RTC (Return to Control)
				if !r.p.IgnoreEndOfBlock && eolCount >= 6 {
					r.err = io.EOF
					return false
				}
			} else {
				break
			}
		}
	}

	// Initialize positions
	a0 := 0         // Current position on the line being decoded
	isWhite := true // Color at a0 (assumed white at start of line)

	// Main decoding loop - continue until we reach the end of the line
	for a0 < r.p.Columns && r.err == nil {
		// Find positions of changing elements
		b1 := r.findNextChangingElement(r.refLine, a0)
		b2 := r.findNextChangingElement(r.refLine, b1)

		// Read code to determine mode
		code := r.peekBits(4)

		if code == 0b0001 {
			// Pass mode: a0 jumps to position below b2
			r.consumeBits(4)
			a0 = b2
		} else if code>>1 == 0b001 {
			// Horizontal mode
			r.consumeBits(3)

			// First run (of current color)
			runLength, state := r.decodeRun(isWhite)
			runLength = min(runLength, r.p.Columns-a0)
			r.fillRowBits(a0, a0+runLength, isWhite != r.p.BlackIs1)
			a0 += runLength

			// If we got an EOL during run decoding, exit early
			if state == S_EOL {
				break
			}

			// Second run (of opposite color)
			runLength, state = r.decodeRun(!isWhite)
			runLength = min(runLength, r.p.Columns-a0)
			r.fillRowBits(a0, a0+runLength, isWhite == r.p.BlackIs1)
			a0 += runLength

			// If we got an EOL during run decoding, exit early
			if state == S_EOL {
				break
			}
		} else {
			// Vertical mode
			var offset int
			if (code & 0b1000) != 0 {
				// V(0)
				offset = 0
				r.consumeBits(1)
			} else if (code & 0b0100) != 0 {
				// V(R) or V(L)
				offset = 1
				if (code & 0b0010) != 0 {
					offset = -1 // V(L)
				}
				r.consumeBits(3)
			} else if (code & 0b0010) != 0 {
				// V(R2) or V(L2)
				offset = 2
				if (code & 0b0001) != 0 {
					offset = -2 // V(L2)
				}
				r.consumeBits(4)
			} else {
				// V(R3) or V(L3)
				offset = 3
				if (code & 0b0001) != 0 {
					offset = -3 // V(L3)
				}
				r.consumeBits(6)
			}

			// Place a1 at offset from b1
			a1 := b1 + offset

			// Ensure a1 is within bounds
			if a1 < a0 {
				a1 = a0
			}
			if a1 > r.p.Columns {
				a1 = r.p.Columns
			}

			// Fill from a0 to a1 with current color
			r.fillRowBits(a0, a1, isWhite != r.p.BlackIs1)

			// Move a0 to a1 and flip color
			a0 = a1
			isWhite = !isWhite
		}
	}

	// After decoding, copy current line to reference line for next scanline
	lineBufSize := (r.p.Columns + 7) / 8
	if len(r.refLine) < lineBufSize {
		r.refLine = make([]byte, lineBufSize)
	}
	copy(r.refLine, r.line)

	return true
}

// findNextChangingElement finds the next position where the bit value changes
// from the color at the starting position.
// It starts at bit position `start` and returns the position of the next color change,
// or r.p.Columns if no change is found before the end of the line.
func (r *Reader) findNextChangingElement(line []byte, start int) int {
	if start < 0 {
		panic("invalid start position")
	}

	startByteIndex := start / 8
	startBitIndex := 7 - (start % 8)
	var startBit byte
	if startByteIndex >= len(line) {
		// all bits beyond the end of the line are assummed to be 0
		return r.p.Columns
	}
	startBit = (line[startByteIndex] >> startBitIndex) & 1

	for pos := start + 1; pos < r.p.Columns; pos++ {
		byteIndex := pos / 8
		bitIndex := 7 - (pos % 8)

		if byteIndex >= len(line) {
			if startBit == 1 {
				return pos
			}
			return r.p.Columns
		}

		bit := (line[byteIndex] >> bitIndex) & 1
		if bit != startBit {
			return pos
		}
	}

	return r.p.Columns
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
