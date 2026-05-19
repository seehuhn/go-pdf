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
	"errors"
	"fmt"

	"seehuhn.de/go/pdf/internal/streamlimits"
)

// maxBytesPerRow caps a predictor row at the worst-case legitimate
// image row: an image at [streamlimits.MaxImageWidth] pixels with
// [streamlimits.MaxImageChannels] components at 16 bpc.  Any image
// XObject the library accepts thus has a row that fits, and the
// per-stream working memory the predict reader allocates is bounded
// at ~4×maxBytesPerRow per layer in the filter chain.
const maxBytesPerRow = streamlimits.MaxImageWidth * streamlimits.MaxImageChannels * 16 / 8

type Params struct {
	// Colors is the number of color components per pixel.
	// Valid range: at least 1, with predictor-specific limits:
	// - TIFF predictor (2): maximum 60 components
	// - PNG predictors (10-15): maximum 256 components
	// Common values: 1 (grayscale), 3 (RGB), 4 (CMYK/RGBA).
	// Only used if Predictor > 1.
	Colors int

	// BitsPerComponent is the number of bits used to represent each color component.
	// Valid values: 1, 2, 4, 8, or 16.
	// Most common value is 8 for typical images.
	BitsPerComponent int

	// Columns is the width of the image in pixels.
	// Valid range: at least 1.
	Columns int

	// Predictor is the prediction algorithm to use.
	// Valid values:
	//   1: No prediction - pass through the data unchanged
	//   2: TIFF horizontal differencing
	//  10: PNG None filter (no prediction) - use predictor type 1 instead
	//  11: PNG Sub filter (horizontal differencing)
	//  12: PNG Up filter (vertical differencing)
	//  13: PNG Average filter (average of left/up)
	//  14: PNG Paeth filter (complex prediction)
	//  15: PNG Optimum (auto-selects best per row) - better compressibility, slow to encode
	Predictor int
}

func (p *Params) Validate() error {
	// reject unsupported predictors before touching other fields
	switch p.Predictor {
	case 1:
		return nil
	case 2:
		if p.Colors < 1 || p.Colors > 60 {
			return fmt.Errorf("invalid Colors %d for TIFF predictor", p.Colors)
		}
	case 10, 11, 12, 13, 14, 15:
		if p.Colors < 1 || p.Colors > 256 {
			return fmt.Errorf("invalid Colors %d for PNG predictor", p.Colors)
		}
	default:
		return fmt.Errorf("invalid Predictor %d", p.Predictor)
	}

	switch p.BitsPerComponent {
	case 1, 2, 4, 8, 16:
		// valid
	default:
		return fmt.Errorf("invalid BitsPerComponent %d", p.BitsPerComponent)
	}

	if p.Columns < 1 || p.Columns > streamlimits.MaxImageWidth {
		return fmt.Errorf("invalid Columns %d", p.Columns)
	}

	// cap the row size to prevent a malicious /DecodeParms from forcing
	// a multi-gigabyte buffer allocation through individually-legal but
	// jointly-enormous parameter values
	totalBits := int64(p.Colors) * int64(p.BitsPerComponent) * int64(p.Columns)
	if (totalBits+7)/8 > maxBytesPerRow {
		return errors.New("predictor row too large")
	}

	return nil
}

// Derived values used throughout the implementation
func (p *Params) bitsPerPixel() int {
	return p.Colors * p.BitsPerComponent
}

func (p *Params) bitsPerRow() int {
	return p.bitsPerPixel() * p.Columns
}

func (p *Params) bytesPerRow() int {
	return (p.bitsPerRow() + 7) / 8 // Round up to bytes
}

func (p *Params) bytesPerPixel() int {
	return (p.bitsPerPixel() + 7) / 8 // For PNG predictors
}
