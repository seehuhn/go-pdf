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

package predict

import (
	"io"
)

// writer applies a prediction filter to the data written to it.
// This is used to prepare data for the LZW/Flate compression filters in PDF.
type writer struct {
	w      io.WriteCloser
	params *Params

	// State for processing
	rowLeft    int      // Bytes remaining in current row
	prevRow    []byte   // Previous row buffer (PNG predictors)
	prevValues []uint16 // Previous component values (TIFF predictor)
	tempBuffer []byte   // Temporary buffer for partial data
	tempLen    int      // Length of data in tempBuffer
}

// NewWriter creates a new io.WriteCloser that applies the prediction filter with the
// given parameters. The function returns an error if the parameters are
// invalid. For predictor 1 (no prediction), it returns the original writer.
func NewWriter(w io.WriteCloser, p *Params) (io.WriteCloser, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}

	if p.Predictor == 1 {
		return w, nil // Identity case - no prediction needed
	}

	writer := &writer{
		w:          w,
		params:     p,
		rowLeft:    p.bytesPerRow(),
		tempBuffer: make([]byte, p.bytesPerRow()*2), // Extra space for safety
	}

	// Initialize state based on predictor type
	if p.Predictor >= 10 && p.Predictor <= 15 {
		// PNG predictor - need previous row buffer
		bufSize := p.bytesPerPixel() + p.bytesPerRow()
		writer.prevRow = make([]byte, bufSize)
		// Initialize padding bytes to 0
		for i := 0; i < p.bytesPerPixel(); i++ {
			writer.prevRow[i] = 0
		}
	} else if p.Predictor == 2 {
		// TIFF predictor - need previous component values
		writer.prevValues = make([]uint16, p.Colors)
	}

	return writer, nil
}

// Write implements the [io.Writer] interface.
func (w *writer) Write(data []byte) (n int, err error) {
	if len(data) == 0 {
		return 0, nil
	}

	totalWritten := 0
	pos := 0

	for pos < len(data) {
		// Copy data to temp buffer if we have partial data or need to process row boundaries
		available := len(data) - pos
		needed := w.params.bytesPerRow() - w.tempLen

		copyLen := min(available, needed)

		copy(w.tempBuffer[w.tempLen:], data[pos:pos+copyLen])
		w.tempLen += copyLen
		pos += copyLen
		totalWritten += copyLen

		// Process complete rows
		if w.tempLen == w.params.bytesPerRow() {
			if err := w.processRow(w.tempBuffer[:w.tempLen]); err != nil {
				return totalWritten, err
			}
			w.tempLen = 0
		}
	}

	return totalWritten, nil
}

// processRow processes a complete row of data
func (w *writer) processRow(rowData []byte) error {
	var encodedRow []byte
	var err error

	switch w.params.Predictor {
	case 2:
		encodedRow, err = w.applyTIFFPredictor(rowData)
	case 10:
		encodedRow, err = w.applyPNGPredictor(rowData, 0) // None
	case 11:
		encodedRow, err = w.applyPNGPredictor(rowData, 1) // Sub
	case 12:
		encodedRow, err = w.applyPNGPredictor(rowData, 2) // Up
	case 13:
		encodedRow, err = w.applyPNGPredictor(rowData, 3) // Average
	case 14:
		encodedRow, err = w.applyPNGPredictor(rowData, 4) // Paeth
	case 15:
		encodedRow, err = w.applyPNGOptimumPredictor(rowData) // Optimum
	default:
		return nil // Should not happen due to validation
	}

	if err != nil {
		return err
	}

	_, err = w.w.Write(encodedRow)
	return err
}

// applyTIFFPredictor applies TIFF horizontal differencing
func (w *writer) applyTIFFPredictor(rowData []byte) ([]byte, error) {
	result := make([]byte, len(rowData))
	copy(result, rowData)

	switch w.params.BitsPerComponent {
	case 1:
		w.applyTIFF1Bit(result)
	case 2:
		w.applyTIFF2Bit(result)
	case 4:
		w.applyTIFF4Bit(result)
	case 8:
		w.applyTIFF8Bit(result)
	case 16:
		w.applyTIFF16Bit(result)
	}

	return result, nil
}

func (w *writer) applyTIFF1Bit(data []byte) {
	componentsPerRow := w.params.Colors * w.params.Columns

	for byteIdx, original := range data {
		var result byte

		for fragIdx := range 8 {
			componentIdx := byteIdx*8 + fragIdx
			if componentIdx >= componentsPerRow {
				break
			}

			shift := 7 - fragIdx
			current := (original >> shift) & 1

			colorIdx := componentIdx % w.params.Colors
			var predicted byte
			if componentIdx < w.params.Colors {
				// First pixel: no prediction
				predicted = current
			} else {
				// Subsequent pixels: XOR with previous
				predicted = current ^ byte(w.prevValues[colorIdx]&1)
			}

			result |= predicted << shift
			w.prevValues[colorIdx] = uint16(current)
		}

		data[byteIdx] = result
	}
}

func (w *writer) applyTIFF2Bit(data []byte) {
	componentsPerRow := w.params.Colors * w.params.Columns

	for byteIdx, original := range data {
		var result byte

		for fragIdx := range 4 {
			componentIdx := byteIdx*4 + fragIdx
			if componentIdx >= componentsPerRow {
				break
			}

			shift := 6 - fragIdx*2
			current := (original >> shift) & 0x03

			colorIdx := componentIdx % w.params.Colors
			var predicted byte
			if componentIdx < w.params.Colors {
				// First pixel: no prediction
				predicted = current
			} else {
				// Subsequent pixels: subtract previous
				predicted = (current - byte(w.prevValues[colorIdx])) & 0x03
			}

			result |= predicted << shift
			w.prevValues[colorIdx] = uint16(current)
		}

		data[byteIdx] = result
	}
}

func (w *writer) applyTIFF4Bit(data []byte) {
	componentsPerRow := w.params.Colors * w.params.Columns

	for byteIdx, original := range data {
		var result byte

		for fragIdx := range 2 {
			componentIdx := byteIdx*2 + fragIdx
			if componentIdx >= componentsPerRow {
				break
			}

			shift := 4 - fragIdx*4
			current := (original >> shift) & 0x0F

			colorIdx := componentIdx % w.params.Colors
			var predicted byte
			if componentIdx < w.params.Colors {
				// First pixel: no prediction
				predicted = current
			} else {
				// Subsequent pixels: subtract previous
				predicted = (current - byte(w.prevValues[colorIdx])) & 0x0F
			}

			result |= predicted << shift
			w.prevValues[colorIdx] = uint16(current)
		}

		data[byteIdx] = result
	}
}

func (w *writer) applyTIFF8Bit(data []byte) {
	for componentIdx, current := range data {
		colorIdx := componentIdx % w.params.Colors
		if componentIdx >= w.params.Colors {
			data[componentIdx] = (current - byte(w.prevValues[colorIdx]))
		}
		w.prevValues[colorIdx] = uint16(current)
	}
}

func (w *writer) applyTIFF16Bit(data []byte) {
	for byteIdx := 0; byteIdx+1 < len(data); byteIdx += 2 {
		current := uint16(data[byteIdx])<<8 | uint16(data[byteIdx+1])
		componentIdx := byteIdx / 2
		colorIdx := componentIdx % w.params.Colors
		if componentIdx >= w.params.Colors {
			predicted := current - w.prevValues[colorIdx]
			data[byteIdx] = byte(predicted >> 8)
			data[byteIdx+1] = byte(predicted)
		}
		w.prevValues[colorIdx] = current
	}
}

// applyPNGPredictor applies PNG prediction with specified algorithm
func (w *writer) applyPNGPredictor(rowData []byte, algorithm byte) ([]byte, error) {
	// PNG rows have tag byte + data
	result := make([]byte, 1+len(rowData))
	result[0] = algorithm

	bytesPerPixel := w.params.bytesPerPixel()

	for i := range rowData {
		var predictor byte

		switch algorithm {
		case 0: // None
			predictor = 0
		case 1: // Sub
			if i >= bytesPerPixel {
				predictor = rowData[i-bytesPerPixel]
			}
		case 2: // Up
			if len(w.prevRow) > bytesPerPixel+i {
				predictor = w.prevRow[bytesPerPixel+i]
			}
		case 3: // Average
			var left, up byte
			if i >= bytesPerPixel {
				left = rowData[i-bytesPerPixel]
			}
			if len(w.prevRow) > bytesPerPixel+i {
				up = w.prevRow[bytesPerPixel+i]
			}
			predictor = byte((int(left) + int(up)) / 2)
		case 4: // Paeth
			var left, up, upperLeft byte
			if i >= bytesPerPixel {
				left = rowData[i-bytesPerPixel]
			}
			if len(w.prevRow) > bytesPerPixel+i {
				up = w.prevRow[bytesPerPixel+i]
			}
			if i >= bytesPerPixel && len(w.prevRow) > i {
				upperLeft = w.prevRow[i]
			}
			predictor = paethPredictor(left, up, upperLeft)
		}

		result[1+i] = byte(int(rowData[i]) - int(predictor))
	}

	// Update previous row buffer
	if len(w.prevRow) >= bytesPerPixel+len(rowData) {
		copy(w.prevRow[bytesPerPixel:], rowData)
	}

	return result, nil
}

// applyPNGOptimumPredictor chooses the best PNG predictor for this row
func (w *writer) applyPNGOptimumPredictor(rowData []byte) ([]byte, error) {
	// Simple implementation: always use Sub (algorithm 1)
	// A more sophisticated implementation would analyze the data
	return w.applyPNGPredictor(rowData, 1)
}

// paethPredictor implements the Paeth prediction algorithm
func paethPredictor(a, b, c byte) byte {
	// a = left, b = above, c = upper left
	p := int(a) + int(b) - int(c)
	pa := abs(p - int(a))
	pb := abs(p - int(b))
	pc := abs(p - int(c))

	if pa <= pb && pa <= pc {
		return a
	}
	if pb <= pc {
		return b
	}
	return c
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// Close implements the [io.Closer] interface.
func (w *writer) Close() error {
	// Process any remaining partial data
	if w.tempLen > 0 {
		// This shouldn't happen with well-formed data, but handle gracefully
		padding := make([]byte, w.params.bytesPerRow()-w.tempLen)
		copy(w.tempBuffer[w.tempLen:], padding)
		if err := w.processRow(w.tempBuffer[:w.params.bytesPerRow()]); err != nil {
			return err
		}
	}

	return w.w.Close()
}
