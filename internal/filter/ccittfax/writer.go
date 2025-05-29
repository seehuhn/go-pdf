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
	"fmt"
	"io"
)

// Writer encodes data using CCITT Fax compression.
type Writer struct {
	p      Params
	w      *bufio.Writer
	closed bool

	lineBytes int
	line      []byte
	refLine   []byte
	numRows   int

	byteVal   byte
	validBits int

	kCounter int
}

// NewWriter creates a new CCITT Fax encoder with the given parameters.
func NewWriter(w io.Writer, p *Params) *Writer {
	pCopy := *p
	if pCopy.Columns == 0 {
		pCopy.Columns = 1728
	}

	lineBytes := (pCopy.Columns + 7) / 8
	currentLine := make([]byte, 0, lineBytes)

	var referenceLine []byte
	if pCopy.K != 0 {
		// Initialize reference line to all white
		referenceLine := make([]byte, lineBytes)
		if pCopy.BlackIs1 {
			// When BlackIs1=true, white pixels are 0 (already initialized)
		} else {
			// When BlackIs1=false, white pixels are 1
			for i := range referenceLine {
				referenceLine[i] = 0xFF
			}
		}
	}

	out := &Writer{
		w:         bufio.NewWriter(w),
		p:         pCopy,
		lineBytes: lineBytes,
		line:      currentLine,
		refLine:   referenceLine,
		kCounter:  0, // start with 1D line
	}
	return out
}

// Close finalizes the CCITT Fax stream.
func (w *Writer) Close() error {
	if w.closed {
		return nil
	}

	if w.p.K < 0 {
		// Group 4 EOFB: 000000000001000000000001
		if err := w.writeBits(0b000000000001_000000000001, 24); err != nil {
			return err
		}
	} else if w.p.K == 0 && w.p.EndOfLine {
		for range 6 {
			if err := w.writeBits(0b000000000001, 12); err != nil {
				return err
			}
		}
	}

	if err := w.flushBits(); err != nil {
		return err
	}

	w.closed = true
	return nil
}

func (w *Writer) Write(p []byte) (n int, err error) {
	for len(p) > 0 {
		k := min(w.lineBytes-len(w.line), len(p))
		w.line = append(w.line, p[:k]...)
		p = p[k:]
		n += k

		if len(w.line) > 0 && w.p.MaxRows > 0 && w.numRows >= w.p.MaxRows {
			return n, errTooManyRows
		}

		if len(w.line) == w.lineBytes {
			err = w.writeRow()
			if err != nil {
				return n, err
			}
			w.numRows++
		}
	}

	return n, nil
}

func (w *Writer) writeBits(code uint32, length uint8) error {
	if length > 32 {
		return fmt.Errorf("writeBits: invalid length %d", length)
	}

	for bit := uint32(1) << (length - 1); bit > 0; bit >>= 1 {
		if code&bit != 0 {
			w.byteVal |= 1 << (7 - w.validBits)
		}
		w.validBits++

		if w.validBits >= 8 {
			if err := w.w.WriteByte(w.byteVal); err != nil {
				return err
			}
			w.byteVal = 0
			w.validBits = 0
		}
	}
	return nil
}

func (w *Writer) flushBits() error {
	if w.validBits > 0 {
		if err := w.w.WriteByte(w.byteVal); err != nil {
			return err
		}
		w.byteVal = 0
		w.validBits = 0
	}
	return w.w.Flush()
}

func (w *Writer) writeRow() error {
	// check that bits beyond w.p.Columns are zero
	if w.p.Columns%8 != 0 {
		lastByteIndex := (w.p.Columns - 1) / 8
		if lastByteIndex < len(w.line) {
			unusedBits := 8 - (w.p.Columns % 8)
			mask := byte((1 << unusedBits) - 1)
			if w.line[lastByteIndex]&mask != 0 {
				return fmt.Errorf("bits beyond column %d are not zero", w.p.Columns)
			}
		}
	}

	// Write EOL for Group 3 modes
	if w.p.EndOfLine && w.p.K >= 0 {
		if err := w.writeBits(0b000000000001, 12); err != nil {
			return err
		}

		// Tag bit for G3 2D: 0 = next line is 2D, 1 = next line is 1D
		if w.p.K > 0 {
			var tagBit uint32
			if w.kCounter == 0 {
				tagBit = 1
			}
			if err := w.writeBits(tagBit, 1); err != nil {
				return err
			}
		}
	}

	var err error
	if w.p.K < 0 {
		err = w.encodeG4Line()
	} else if w.p.K == 0 {
		err = w.encode1DLine()
	} else {
		if w.kCounter > 0 {
			err = w.encode2DLine()
			w.kCounter--
		} else {
			err = w.encode1DLine()
			w.kCounter = w.p.K - 1
		}
	}
	if err != nil {
		return err
	}

	if w.p.EncodedByteAlign {
		if err := w.flushBits(); err != nil {
			return err
		}
	}

	w.line, w.refLine = w.refLine[:0], w.line

	return nil
}

func (w *Writer) whiteBit() byte {
	if w.p.BlackIs1 {
		return 0
	}
	return 1
}

func (w *Writer) encode1DLine() error {
	xpos := 0

	// The spec requires that the first run is white
	runBit := w.whiteBit()

	for xpos < w.p.Columns {
		runStart := xpos
		for xpos < w.p.Columns && w.getPixel(w.line, xpos) == runBit {
			xpos++
		}
		if err := w.encode1DRun(xpos-runStart, runBit); err != nil {
			return err
		}
		runBit = 1 - runBit // Toggle between white and black
	}
	return nil
}

// getPixel returns the bit value at column x.
func (w *Writer) getPixel(lineData []byte, x int) byte {
	if x < 0 || x >= w.p.Columns {
		// Pixels outside the image are white
		if w.p.BlackIs1 {
			return 0
		}
		return 1
	}

	byteIndex := x / 8
	bitIndex := 7 - (x % 8)

	if byteIndex >= len(lineData) {
		if w.p.BlackIs1 {
			return 0
		}
		return 1
	}

	return (lineData[byteIndex] >> uint(bitIndex)) & 1
}

func (w *Writer) encode1DRun(runLength int, runBit byte) error {
	for runLength >= 2560 {
		lastIndex := len(extMakeupEncodeTable) - 1
		entry := extMakeupEncodeTable[lastIndex]
		err := w.writeBits(entry.Code, entry.Width)
		if err != nil {
			return err
		}
		runLength -= 2560
	}

	if runLength >= 64 {
		makeupIndex := runLength/64 - 1
		var entry encodeNode
		if runBit == w.whiteBit() {
			entry = whiteMakeupEncodeTable[makeupIndex]
		} else {
			entry = blackMakeupEncodeTable[makeupIndex]
		}
		err := w.writeBits(entry.Code, entry.Width)
		if err != nil {
			return err
		}
		runLength %= 64
	}

	var entry encodeNode
	if runBit == w.whiteBit() {
		entry = whiteTermEncodeTable[runLength]
	} else {
		entry = blackTermEncodeTable[runLength]
	}
	return w.writeBits(entry.Code, entry.Width)
}

func (w *Writer) encodeG4Line() error {
	return w.encode2DLineInternal(true)
}

func (w *Writer) encode2DLine() error {
	return w.encode2DLineInternal(false)
}

func (w *Writer) encode2DLineInternal(isG4 bool) error {
	xpos := 0

	for xpos < w.p.Columns {
		a0 := xpos
		a0Val := w.getPixel(w.line, a0)

		var b1 int
		if isG4 {
			// G4: b1 is first changing element on ref line to the right of a0
			b1 = w.findChangingElement(w.refLine, a0, w.getPixel(w.refLine, a0))
		} else {
			// G3: b1 is first element on ref line to the right of a0 with different color from a0
			b1 = w.findChangingElement(w.refLine, a0, a0Val)
		}

		a1 := w.findChangingElement(w.line, a0, a0Val)
		b2 := w.findChangingElement(w.refLine, b1, w.getPixel(w.refLine, b1))

		if b2 < a1 {
			// Pass mode
			if err := w.writeBits(0b0001, 4); err != nil {
				return err
			}
			xpos = b2
			continue
		}

		delta := a1 - b1
		if delta >= -3 && delta <= 3 {
			// Vertical mode
			var code uint32
			var width uint8
			switch delta {
			case 0:
				code, width = 0b1, 1
			case 1:
				code, width = 0b011, 3
			case 2:
				code, width = 0b000011, 6
			case 3:
				code, width = 0b0000011, 7
			case -1:
				code, width = 0b010, 3
			case -2:
				code, width = 0b000010, 6
			case -3:
				code, width = 0b0000010, 7
			}
			if err := w.writeBits(code, width); err != nil {
				return err
			}
			xpos = a1
			continue
		}

		// Horizontal mode
		if err := w.writeBits(0b001, 3); err != nil {
			return err
		}

		run1Length := a1 - a0
		if err := w.encode1DRun(run1Length, a0Val); err != nil {
			return err
		}

		a2 := w.findChangingElement(w.line, a1, w.getPixel(w.line, a1))
		run2Length := a2 - a1
		if err := w.encode1DRun(run2Length, 1-a0Val); err != nil {
			return err
		}

		xpos = a2
	}
	return nil
}

// findChangingElement finds the x-coordinate of the first pixel in lineData
// at or after startX whose value is different from refValue.
func (w *Writer) findChangingElement(lineData []byte, startX int, refValue byte) int {
	for x := startX; x < w.p.Columns; x++ {
		if w.getPixel(lineData, x) != refValue {
			return x
		}
	}
	return w.p.Columns
}
