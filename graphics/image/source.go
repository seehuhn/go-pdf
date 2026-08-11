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
	"seehuhn.de/go/pdf/graphics/color"
)

// FlateSource writes raw pixel data to a PDF stream, compressed with
// FlateDecode and an optional PNG predictor.  It implements
// [graphics.ImageData].
type FlateSource struct {
	// WriteData writes the raw, uncompressed pixel data row by row,
	// with each row starting at a new byte boundary.
	WriteData func(io.Writer) error

	// Predictor selects the PNG predictor applied before Flate compression.
	// Both a zero value and [pdf.FlatePredictorNone] disable the predictor.
	// [NewFlateSource] fills this in from the colour space and bit depth;
	// callers who want a different predictor can overwrite it.
	Predictor pdf.FlatePredictor

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

// NewFlateSource returns a source for image data.
//
// The image has width pixels per row, is in the given colour space and at the
// given bit depth.
func NewFlateSource(width int, cs color.Space, bitsPerComponent int, writeData func(io.Writer) error) *FlateSource {
	return &FlateSource{
		WriteData:        writeData,
		Predictor:        flatePredictorFor(cs, bitsPerComponent),
		Width:            width,
		Colors:           cs.Channels(),
		BitsPerComponent: bitsPerComponent,
	}
}

// flatePredictorFor picks a PNG predictor for image data in the given colour
// space at the given bit depth.
func flatePredictorFor(cs color.Space, bitsPerComponent int) pdf.FlatePredictor {
	// a sample is a palette position rather than a brightness, so the
	// difference between neighbours carries no information
	if cs.Family() == color.FamilyIndexed {
		return pdf.FlatePredictorNone
	}
	// a byte holds several samples, and differencing whole bytes across
	// sample boundaries costs more than it saves
	if bitsPerComponent < 8 {
		return pdf.FlatePredictorNone
	}
	return pdf.FlatePredictorPNGOptimum
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

// IsJPX implements [graphics.ImageData].
func (s *FlateSource) IsJPX() bool { return false }

// WriteStream implements [graphics.ImageData].
func (s *FlateSource) WriteStream(rm *pdf.EmbedHelper, ref pdf.Reference, dict pdf.Dict) error {
	if s.WriteData == nil {
		return errors.New("FlateSource.WriteData is nil")
	}

	// Colors, BitsPerComponent and Columns are only allowed alongside a real
	// predictor; setting them otherwise is a write-time error.
	parms := pdf.FilterCompress{}
	if s.Predictor != 0 && s.Predictor != pdf.FlatePredictorNone {
		parms.Predictor = s.Predictor
		if s.Colors > 0 {
			parms.Colors = s.Colors
		}
		if s.BitsPerComponent > 0 {
			parms.BitsPerComponent = s.BitsPerComponent
		}
		if s.Width > 0 {
			parms.Columns = s.Width
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

	// K controls the encoding algorithm: negative values select CCITT
	// Group 4 (two-dimensional), zero selects Group 3 one-dimensional, and
	// positive K selects Group 3 mixed encoding.
	//
	// Group 4 is the right choice for data written to a file, and
	// [NewCCITTFaxSource] selects it.  It compresses better than either
	// Group 3 mode on any image with enough vertical structure to be worth
	// encoding this way; where it does not, the image is one this filter
	// expands rather than compresses.  The Group 3 modes restart the encoding
	// periodically so that a transmission error damages only part of the
	// image, which is of no use once the data is stored in a file.
	//
	// The zero value selects Group 3 one-dimensional.
	K int

	// BlackIs1, if true, sets BlackIs1=true in the filter parameters.
	BlackIs1 bool
}

// NewCCITTFaxSource returns a source for 1-bit image data.
//
// The image has width pixels per row.  The data is encoded with CCITT
// Group 4; see the K field for why the other modes are not offered here.
func NewCCITTFaxSource(width int, writeData func(io.Writer) error) *CCITTFaxSource {
	return &CCITTFaxSource{
		WriteData: writeData,
		Width:     width,
		K:         -1,
	}
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

// IsJPX implements [graphics.ImageData].
func (s *CCITTFaxSource) IsJPX() bool { return false }

// WriteStream implements [graphics.ImageData].
func (s *CCITTFaxSource) WriteStream(rm *pdf.EmbedHelper, ref pdf.Reference, dict pdf.Dict) error {
	if s.WriteData == nil {
		return errors.New("CCITTFaxSource.WriteData is nil")
	}

	parms := pdf.FilterCCITTFax{
		Columns: s.Width,
		K:       s.K,
	}
	if s.BlackIs1 {
		parms.BlackIs1 = true
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
