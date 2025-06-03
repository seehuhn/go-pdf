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
	Params
	w      *bufio.Writer
	closed bool

	lineBytes int
	line      []byte
	refLine   []byte
	numRows   int

	byteVal   byte
	validBits int

	count2D int
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
		refLine := make([]byte, lineBytes)
		if pCopy.BlackIs1 {
			// When BlackIs1=true, white pixels are 0 (already initialized)
		} else {
			// When BlackIs1=false, white pixels are 1
			for i := range refLine {
				refLine[i] = 0xFF
			}
		}
	}

	out := &Writer{
		Params:    pCopy,
		w:         bufio.NewWriter(w),
		lineBytes: lineBytes,
		line:      currentLine,
		refLine:   referenceLine,
		count2D:   0, // start with 1D line
	}
	return out
}

// Close finalizes the CCITT Fax stream.
func (w *Writer) Close() error {
	if w.closed {
		return nil
	}

	if w.K < 0 {
		// Group 4 EOFB: 000000000001000000000001
		if err := w.writeBits(0b000000000001_000000000001, 24); err != nil {
			return err
		}
	} else if w.K == 0 && w.EndOfLine {
		for range 6 {
			if err := w.writeBits(0b000000000001, 12); err != nil {
				return err
			}
		}
	} else if w.K > 0 && w.EndOfLine {
		for range 6 {
			if err := w.writeBits(0b0000000000011, 13); err != nil {
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

		if len(w.line) > 0 && w.MaxRows > 0 && w.numRows >= w.MaxRows {
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
	// check that bits beyond w.Columns are zero
	if w.Columns%8 != 0 {
		lastByteIndex := (w.Columns - 1) / 8
		if lastByteIndex < len(w.line) {
			unusedBits := 8 - (w.Columns % 8)
			mask := byte((1 << unusedBits) - 1)
			if w.line[lastByteIndex]&mask != 0 {
				return fmt.Errorf("bits beyond column %d are not zero", w.Columns)
			}
		}
	}

	switch {
	case w.K > 0:
		if w.EndOfLine {
			err := w.writeBits(0b000000000001, 12)
			if err != nil {
				return err
			}
		}
		if w.count2D >= w.K-1 {
			err := w.writeBits(0b1, 1) // 1 indicates a 1D-coded row
			if err != nil {
				return err
			}
			err = w.encode1DLine()
			if err != nil {
				return err
			}
			w.count2D = 0
		} else {
			err := w.writeBits(0b0, 1) // 0 indicates a 2D-coded row
			if err != nil {
				return err
			}
			err = w.encode2DLineG3()
			if err != nil {
				return err
			}
			w.count2D++
		}
	case w.K == 0:
		if w.EndOfLine {
			err := w.writeBits(0b000000000001, 12)
			if err != nil {
				return err
			}
		}
		err := w.encode1DLine()
		if err != nil {
			return err
		}
	case w.K < 0:
		err := w.encode2DLineG4()
		if err != nil {
			return err
		}
	}

	if w.EncodedByteAlign {
		if err := w.flushBits(); err != nil {
			return err
		}
	}

	w.line, w.refLine = w.refLine[:0], w.line

	return nil
}

func (w *Writer) encode1DLine() error {
	xpos := 0

	// The first run is always white.
	runBit := w.whiteBit()

	for xpos < w.Columns {
		runStart := xpos
		xpos = w.endOfRun(w.line, xpos, runBit)
		if err := w.encode1DRun(xpos-runStart, runBit); err != nil {
			return err
		}
		runBit = 1 - runBit // Toggle between white and black
	}
	return nil
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

func (w *Writer) encode2DLineG3() error {
	a0 := -1
	currentBit := w.whiteBit()
	for a0 < w.Columns {
		a1 := w.endOfRun(w.line, a0+1, currentBit)
		a2 := w.endOfRun(w.line, a1+1, 1-currentBit)

		b1, b2 := w.findB1B2(w.refLine, a0, currentBit)

		delta := a1 - b1
		switch {
		case b2 < a1: // pass mode
			if err := w.writeBits(0b0001, 4); err != nil {
				return err
			}
			a0 = b2

		case delta >= -3 && delta <= 3: // vertical mode
			// TODO(voss): maybe only use this if b1 < w.Columns?
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
			a0 = a1
			currentBit = 1 - currentBit

		default: // horizontal mode
			if err := w.writeBits(0b001, 3); err != nil {
				return err
			}
			if err := w.encode1DRun(a1-max(a0, 0), currentBit); err != nil {
				return err
			}
			if err := w.encode1DRun(a2-a1, 1-currentBit); err != nil {
				return err
			}

			a0 = a2
		}
	}
	return nil
}

func (w *Writer) encode2DLineG4() error {
	xpos := 0

	for xpos < w.Columns {
		a0 := xpos
		a0Val := w.getPixel(w.line, a0)

		a1 := w.endOfRun(w.line, a0, a0Val)

		// G4: b1 is first changing element on ref line to the right of a0
		b1 := w.endOfRun(w.refLine, a0, w.getPixel(w.refLine, a0))

		b2 := w.endOfRun(w.refLine, b1, w.getPixel(w.refLine, b1))

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

		a2 := w.endOfRun(w.line, a1, w.getPixel(w.line, a1))
		run2Length := a2 - a1
		if err := w.encode1DRun(run2Length, 1-a0Val); err != nil {
			return err
		}

		xpos = a2
	}
	return nil
}
