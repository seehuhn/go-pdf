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

// Package dct implements the DCTDecode filter (PDF spec §7.4.8).
//
// Decoding is streaming: for single-scan baseline JPEGs peak memory
// is bounded by image width (one MCU stripe is held in memory at a
// time), so the filter is safe to use on attacker-supplied input
// regardless of the surrounding image dictionary.  For progressive
// JPEGs the per-component coefficient buffer is capped at
// [streamlimits.MaxImageBytes] to bound amplification beyond the
// existing pixel/byte caps; large progressive images are rejected at
// the cap rather than causing memory exhaustion.
//
// The filter treats DCTDecode output as a generic byte stream: a
// consumer that has reserved a buffer of W·H·nComp·bpc/8 bytes from
// the surrounding image dict's declared dimensions reads positionally
// from the decoded stream.  When the JPEG's intrinsic dimensions
// differ from the image dict's /Width and /Height (a PDF that the
// spec does not explicitly forbid), the consumer receives whatever
// bytes the decoder emits, truncated to the dict-declared budget.
// See viewer-tests/image/jpeg-mismatch for the observed behaviour of
// other PDF viewers and the rationale.
package dct

import (
	"bufio"
	"io"

	"seehuhn.de/go/membudget"
	"seehuhn.de/go/pdf/internal/filter/dct/jpeg"
)

// Decode decodes JPEG data from r and returns the raw pixel bytes,
// interleaved channel-by-channel and row-by-row with no padding.  For
// color images the output is RGB (3 bytes/pixel); for grayscale, 1
// byte/pixel; for CMYK, 4 bytes/pixel in PDF convention.
//
// If colorTransform is non-nil it overrides the APP14-based color
// transform decision per PDF spec table 13: 0 selects no transform
// (raw YCbCr or CMYK pass-through), 1 selects YCbCr→RGB (or YCCK
// transform for CMYK).
//
// Decoding happens lazily as the returned reader is drained, so
// malformed JPEGs surface their error on a Read call rather than from
// Decode itself.  Callers should Close the reader when they stop
// consuming; otherwise the producer goroutine remains blocked on the
// pipe.
func Decode(r io.Reader, colorTransform *int, budget *membudget.Budget) (io.ReadCloser, error) {
	pr, pw := io.Pipe()
	go func() {
		bw := bufio.NewWriter(pw)
		if err := jpeg.DecodeStream(r, colorTransform, bw, budget); err != nil {
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
