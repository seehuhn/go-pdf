// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2026  Jochen Voss <voss@seehuhn.de>
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

package dct

import (
	"bufio"
	"io"

	"seehuhn.de/go/pdf/internal/filter/dct/jpeg"
)

// Decode decodes JPEG data from r and returns the raw pixel bytes.
// If colorTransform is non-nil, it overrides the JPEG's APP14-based
// color transform decision per PDF spec table 13.
//
// The output contains interleaved channel bytes, row by row, with no
// padding.  For color images, the output is RGB (3 bytes per pixel).
// For grayscale images, the output is 1 byte per pixel.  For CMYK
// images, the output is 4 bytes per pixel.
//
// Decoding happens lazily as the returned reader is drained: malformed
// JPEGs surface their error on a Read call rather than from Decode
// itself.
func Decode(r io.Reader, colorTransform *int) (io.ReadCloser, error) {
	pr, pw := io.Pipe()
	go func() {
		bw := bufio.NewWriter(pw)
		if err := jpeg.DecodeStream(r, colorTransform, bw); err != nil {
			pw.CloseWithError(err)
			return
		}
		if err := bw.Flush(); err != nil {
			pw.CloseWithError(err)
			return
		}
		pw.Close()
	}()
	return pr, nil
}
