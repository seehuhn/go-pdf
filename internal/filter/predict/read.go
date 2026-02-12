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

// reader applies undoes the effects of a prediction filter on the data read from it.
// This is used on decompressed data when reading LZW/Flate compressed streams in PDF.
type reader struct {
	r      io.ReadCloser
	params *Params

	// State for processing
	prevRow      []byte   // Previous row buffer (PNG predictors)
	prevValues   []uint32 // Previous component values (TIFF predictor)
	inputBuffer  []byte   // Buffer for reading encoded data
	outputBuffer []byte   // Buffer for decoded data
	outputPos    int      // Position in output buffer
	outputLen    int      // Length of valid data in output buffer
	needRowData  int      // Bytes needed to complete current row
	eof          bool     // End of file reached
}

// NewReader creates a new io.Reader that applies the prediction filter with the
// given parameters. The function returns an error if the parameters are
// invalid. For predictor 1 (no prediction), it returns the original reader.
func NewReader(r io.ReadCloser, p *Params) (io.ReadCloser, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}

	if p.Predictor == 1 {
		return r, nil // Identity case - no prediction needed
	}

	reader := &reader{
		r:            r,
		params:       p,
		outputBuffer: make([]byte, p.bytesPerRow()*2), // Extra space for safety
	}

	// Initialize state based on predictor type
	if p.Predictor >= 10 && p.Predictor <= 15 {
		// PNG predictor - need previous row buffer and input for tag bytes
		bufSize := p.bytesPerPixel() + p.bytesPerRow()
		reader.prevRow = make([]byte, bufSize)
		reader.inputBuffer = make([]byte, p.bytesPerRow()+1) // +1 for tag byte
		reader.needRowData = p.bytesPerRow() + 1
		// Initialize padding bytes to 0
		for i := 0; i < p.bytesPerPixel(); i++ {
			reader.prevRow[i] = 0
		}
	} else if p.Predictor == 2 {
		// TIFF predictor - need previous component values
		reader.prevValues = make([]uint32, p.Colors)
		reader.inputBuffer = make([]byte, p.bytesPerRow())
		reader.needRowData = p.bytesPerRow()
	}

	return reader, nil
}

func (r *reader) Close() error {
	return r.r.Close()
}

// Read implements the [io.Reader] interface.
func (r *reader) Read(p []byte) (n int, err error) {
	totalRead := 0

	for totalRead < len(p) {
		// Return any buffered output data first
		if r.outputPos < r.outputLen {
			available := r.outputLen - r.outputPos
			copyLen := min(len(p)-totalRead, available)
			copy(p[totalRead:], r.outputBuffer[r.outputPos:r.outputPos+copyLen])
			r.outputPos += copyLen
			totalRead += copyLen
			continue
		}

		// Check if we've reached EOF
		if r.eof {
			break
		}

		// Read and decode more data
		needed := r.needRowData
		bytesRead, readErr := io.ReadFull(r.r, r.inputBuffer[:needed])
		if readErr != nil {
			if readErr == io.EOF || readErr == io.ErrUnexpectedEOF {
				r.eof = true
				if bytesRead == 0 {
					break
				}
				// Process partial data if available
			} else {
				return totalRead, readErr
			}
		}

		// Decode the row
		var decodedRow []byte
		var decodeErr error

		switch r.params.Predictor {
		case 2:
			decodedRow, decodeErr = r.decodeTIFFRow(r.inputBuffer[:bytesRead])
		case 10, 11, 12, 13, 14, 15:
			decodedRow, decodeErr = r.decodePNGRow(r.inputBuffer[:bytesRead])
		}

		if decodeErr != nil {
			return totalRead, decodeErr
		}

		// Store decoded data in output buffer
		copy(r.outputBuffer, decodedRow)
		r.outputLen = len(decodedRow)
		r.outputPos = 0
	}

	if totalRead == 0 && r.eof {
		return 0, io.EOF
	}

	return totalRead, nil
}

// decodeTIFFRow decodes a TIFF predictor row
func (r *reader) decodeTIFFRow(encodedData []byte) ([]byte, error) {
	result := make([]byte, len(encodedData))
	copy(result, encodedData)

	switch r.params.BitsPerComponent {
	case 1:
		r.decodeTIFF1Bit(result)
	case 2:
		r.decodeTIFF2Bit(result)
	case 4:
		r.decodeTIFF4Bit(result)
	case 8:
		r.decodeTIFF8Bit(result)
	case 16:
		r.decodeTIFF16Bit(result)
	}

	// Reset previous values for next row
	for i := range r.prevValues {
		r.prevValues[i] = 0
	}

	return result, nil
}

func (r *reader) decodeTIFF1Bit(data []byte) {
	componentsPerRow := r.params.Colors * r.params.Columns

	for byteIdx := range data {
		original := data[byteIdx]
		var result byte

		for fragIdx := range 8 {
			componentIdx := byteIdx*8 + fragIdx
			if componentIdx >= componentsPerRow {
				break
			}

			shift := 7 - fragIdx
			encoded := (original >> shift) & 1

			colorIdx := componentIdx % r.params.Colors
			var current byte
			if componentIdx < r.params.Colors {
				// First pixel: no prediction was applied
				current = encoded
			} else {
				// Subsequent pixels: reverse XOR with previous
				current = encoded ^ byte(r.prevValues[colorIdx]&1)
			}

			result |= current << shift
			r.prevValues[colorIdx] = uint32(current)
		}

		data[byteIdx] = result
	}
}

func (r *reader) decodeTIFF2Bit(data []byte) {
	componentsPerRow := r.params.Colors * r.params.Columns

	for byteIdx := range data {
		original := data[byteIdx]
		var result byte

		for fragIdx := range 4 {
			componentIdx := byteIdx*4 + fragIdx
			if componentIdx >= componentsPerRow {
				break
			}

			shift := 6 - fragIdx*2
			encoded := (original >> shift) & 0x03

			colorIdx := componentIdx % r.params.Colors
			var current byte
			if componentIdx < r.params.Colors {
				// First pixel: no prediction was applied
				current = encoded
			} else {
				// Subsequent pixels: reverse subtraction with previous
				current = (encoded + byte(r.prevValues[colorIdx])) & 0x03
			}

			result |= current << shift
			r.prevValues[colorIdx] = uint32(current)
		}

		data[byteIdx] = result
	}
}

func (r *reader) decodeTIFF4Bit(data []byte) {
	componentsPerRow := r.params.Colors * r.params.Columns

	for byteIdx := range data {
		original := data[byteIdx]
		var result byte

		for fragIdx := range 2 {
			componentIdx := byteIdx*2 + fragIdx
			if componentIdx >= componentsPerRow {
				break
			}

			shift := 4 - fragIdx*4
			encoded := (original >> shift) & 0x0F

			colorIdx := componentIdx % r.params.Colors
			var current byte
			if componentIdx < r.params.Colors {
				// First pixel: no prediction was applied
				current = encoded
			} else {
				// Subsequent pixels: reverse subtraction with previous
				current = (encoded + byte(r.prevValues[colorIdx])) & 0x0F
			}

			result |= current << shift
			r.prevValues[colorIdx] = uint32(current)
		}

		data[byteIdx] = result
	}
}

func (r *reader) decodeTIFF8Bit(data []byte) {
	// Initialize first component values
	for i := 0; i < r.params.Colors && i < len(data); i++ {
		r.prevValues[i] = uint32(data[i])
	}

	// Apply reverse differencing to subsequent components
	for i := r.params.Colors; i < len(data); i++ {
		componentIdx := i % r.params.Colors
		original := byte(int(data[i]) + int(r.prevValues[componentIdx]))
		data[i] = original
		r.prevValues[componentIdx] = uint32(original)
	}
}

func (r *reader) decodeTIFF16Bit(data []byte) {
	// Initialize first component values
	for i := 0; i < r.params.Colors && i*2+1 < len(data); i++ {
		r.prevValues[i] = uint32(data[i*2])<<8 | uint32(data[i*2+1])
	}

	// Apply reverse differencing to subsequent components
	for i := r.params.Colors * 2; i < len(data); i += 2 {
		componentIdx := (i / 2) % r.params.Colors
		diff := uint32(data[i])<<8 | uint32(data[i+1])
		current := diff + r.prevValues[componentIdx]
		data[i] = byte(current >> 8)
		data[i+1] = byte(current & 0xFF)
		r.prevValues[componentIdx] = current
	}
}

// decodePNGRow decodes a PNG predictor row
func (r *reader) decodePNGRow(encodedData []byte) ([]byte, error) {
	if len(encodedData) == 0 {
		return nil, io.EOF
	}

	// First byte is the algorithm tag
	algorithm := encodedData[0]
	rowData := encodedData[1:]

	result := make([]byte, len(rowData))
	bytesPerPixel := r.params.bytesPerPixel()

	for i := range rowData {
		var predictor byte

		switch algorithm {
		case 0: // None
			predictor = 0
		case 1: // Sub
			if i >= bytesPerPixel {
				predictor = result[i-bytesPerPixel]
			}
		case 2: // Up
			if len(r.prevRow) > bytesPerPixel+i {
				predictor = r.prevRow[bytesPerPixel+i]
			}
		case 3: // Average
			var left, up byte
			if i >= bytesPerPixel {
				left = result[i-bytesPerPixel]
			}
			if len(r.prevRow) > bytesPerPixel+i {
				up = r.prevRow[bytesPerPixel+i]
			}
			predictor = byte((int(left) + int(up)) / 2)
		case 4: // Paeth
			var left, up, upperLeft byte
			if i >= bytesPerPixel {
				left = result[i-bytesPerPixel]
			}
			if len(r.prevRow) > bytesPerPixel+i {
				up = r.prevRow[bytesPerPixel+i]
			}
			if i >= bytesPerPixel && len(r.prevRow) > i {
				upperLeft = r.prevRow[i]
			}
			predictor = paethPredictor(left, up, upperLeft)
		}

		result[i] = byte(int(rowData[i]) + int(predictor))
	}

	// Update previous row buffer
	if len(r.prevRow) >= bytesPerPixel+len(result) {
		copy(r.prevRow[bytesPerPixel:], result)
	}

	return result, nil
}
