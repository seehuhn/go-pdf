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

package image

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	"seehuhn.de/go/pdf"
)

// FlateSource writes raw pixel data to a PDF stream, compressed with
// FlateDecode and an optional PNG predictor.  It implements
// [graphics.ImageData].
type FlateSource struct {
	// WriteData writes the raw, uncompressed pixel data row by row,
	// with each row starting at a new byte boundary.
	WriteData func(io.Writer) error

	// Predictor selects the PNG predictor applied before Flate
	// compression.  Common values are 15 (PNG optimum, used for colour
	// images) and 12 (PNG up, used for grayscale soft-mask data).
	// A zero value disables the predictor.
	Predictor int

	// Width is the number of pixels per row, used as the Columns
	// parameter for PNG prediction.
	Width int

	// Colors is the number of colour channels, used as the Colors
	// parameter for PNG prediction.
	Colors int

	// BitsPerComponent is the number of bits per sample, used as the
	// BitsPerComponent parameter for PNG prediction.
	BitsPerComponent int
}

// Pixels returns the raw, uncompressed pixel data.
func (s *FlateSource) Pixels() ([]byte, error) {
	if s.WriteData == nil {
		return nil, errors.New("FlateSource.WriteData is nil")
	}
	var buf bytes.Buffer
	if err := s.WriteData(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// WriteStream implements [graphics.ImageData].
func (s *FlateSource) WriteStream(rm *pdf.EmbedHelper, ref pdf.Reference, dict pdf.Dict) error {
	if s.WriteData == nil {
		return errors.New("FlateSource.WriteData is nil")
	}

	parms := pdf.FilterCompress{}
	if s.Predictor != 0 {
		parms["Predictor"] = pdf.Integer(s.Predictor)
		if s.Colors > 0 {
			parms["Colors"] = pdf.Integer(s.Colors)
		}
		if s.BitsPerComponent > 0 {
			parms["BitsPerComponent"] = pdf.Integer(s.BitsPerComponent)
		}
		if s.Width > 0 {
			parms["Columns"] = pdf.Integer(s.Width)
		}
	}

	w, err := rm.Out().OpenStream(ref, dict, parms)
	if err != nil {
		return fmt.Errorf("cannot open image stream: %w", err)
	}
	if err := s.WriteData(w); err != nil {
		w.Close()
		return err
	}
	return w.Close()
}

// CCITTFaxSource writes raw 1-bit image data to a PDF stream, compressed
// with CCITTFaxDecode.  This is the default encoding for 1-bit image
// masks.  It implements [graphics.ImageData].
type CCITTFaxSource struct {
	// WriteData writes raw 1-bit image data as a continuous bit stream,
	// with each row starting at a new byte boundary.
	WriteData func(io.Writer) error

	// Width is the number of pixels per row, used as the Columns
	// parameter for the CCITTFax filter.
	Width int

	// K controls the encoding algorithm: negative values select
	// CCITT Group 4 (two-dimensional), zero selects Group 3
	// one-dimensional, and positive K selects Group 3 mixed
	// encoding.  Group 4 (K = -1) is recommended for most uses as
	// it compresses significantly better than Group 3.
	K int

	// BlackIs1, if true, sets BlackIs1=true in the filter parameters.
	BlackIs1 bool
}

// Pixels returns the raw, uncompressed pixel data.
func (s *CCITTFaxSource) Pixels() ([]byte, error) {
	if s.WriteData == nil {
		return nil, errors.New("CCITTFaxSource.WriteData is nil")
	}
	var buf bytes.Buffer
	if err := s.WriteData(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// WriteStream implements [graphics.ImageData].
func (s *CCITTFaxSource) WriteStream(rm *pdf.EmbedHelper, ref pdf.Reference, dict pdf.Dict) error {
	if s.WriteData == nil {
		return errors.New("CCITTFaxSource.WriteData is nil")
	}

	parms := pdf.FilterCCITTFax{
		"Columns": pdf.Integer(s.Width),
		"K":       pdf.Integer(s.K),
	}
	if s.BlackIs1 {
		parms["BlackIs1"] = pdf.Boolean(true)
	}

	w, err := rm.Out().OpenStream(ref, dict, parms)
	if err != nil {
		return fmt.Errorf("cannot open image mask stream: %w", err)
	}
	if err := s.WriteData(w); err != nil {
		w.Close()
		return err
	}
	return w.Close()
}
