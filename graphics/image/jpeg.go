// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2023  Jochen Voss <voss@seehuhn.de>
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
	"maps"

	"seehuhn.de/go/membudget"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/internal/filter/dct"
)

// DCTSource writes raw 8-bit pixel data to a PDF stream, compressed as a JPEG
// image (DCTDecode).  It implements [graphics.ImageData].
//
// The samples are in the colour space of the enclosing [Dict]; the encoder
// does not convert between colour spaces.  ColorTransform only selects how the
// samples are represented inside the JPEG data, as luma/chroma components or
// as the original components.  It is a compression choice, and the decoded
// samples are in the same colour space either way.
type DCTSource struct {
	// WriteData writes the pixel data, Width*Height*Colors bytes, as
	// interleaved 8-bit samples, row by row from the top.
	WriteData func(io.Writer) error

	// Width and Height are the image dimensions in pixels.  JPEG limits
	// each to at most 65535.
	Width, Height int

	// Colors is the number of components per pixel: 1, 3 or 4.
	Colors int

	// ColorTransform selects whether three-component data is stored as
	// YCbCr and four-component data as YCbCrK.  It has no effect for
	// one-component data.  [NewDCTSource] enables it for RGB and CMYK data.
	ColorTransform bool

	// Quality is the JPEG quality, from 1 to 100.  The zero value selects
	// the encoder's default.
	Quality int
}

// NewDCTSource returns a source for JPEG-compressed image data.
//
// The image has the given size and is in the given colour space.  The writeData
// function must write width*height*cs.Channels() interleaved 8-bit samples.
func NewDCTSource(width, height int, cs color.Space, writeData func(io.Writer) error) *DCTSource {
	return &DCTSource{
		WriteData:      writeData,
		Width:          width,
		Height:         height,
		Colors:         cs.Channels(),
		ColorTransform: dctTransformFor(cs),
	}
}

// dctTransformFor reports whether a colour transform helps for image data in
// the given colour space.  The transform decorrelates the components of
// RGB-like and CMYK data; for other colour spaces the components do not have
// that structure.
func dctTransformFor(cs color.Space) bool {
	switch cs.Family() {
	case color.FamilyDeviceRGB, color.FamilyCalRGB, color.FamilyDeviceCMYK:
		return true
	case color.FamilyICCBased:
		n := cs.Channels()
		return n == 3 || n == 4
	}
	return false
}

func (s *DCTSource) check() error {
	if s.WriteData == nil {
		return errors.New("DCTSource.WriteData is nil")
	}
	if s.Width <= 0 || s.Height <= 0 {
		return fmt.Errorf("invalid DCT image size %dx%d", s.Width, s.Height)
	}
	if s.Colors != 1 && s.Colors != 3 && s.Colors != 4 {
		return fmt.Errorf("invalid DCT component count %d", s.Colors)
	}
	return nil
}

// CheckGeometry reports an error if the source does not match an image with
// the given size, number of colour channels and bits per component.
func (s *DCTSource) CheckGeometry(width, height, channels, bitsPerComponent int) error {
	if bitsPerComponent != 8 {
		return fmt.Errorf("DCTSource requires 8 bits per component, got %d", bitsPerComponent)
	}
	if s.Colors != channels {
		return fmt.Errorf("DCTSource has %d components, color space has %d", s.Colors, channels)
	}
	if s.Width != width || s.Height != height {
		return fmt.Errorf("DCTSource size %dx%d differs from image size %dx%d",
			s.Width, s.Height, width, height)
	}
	return nil
}

// encode compresses the pixel data and writes the JPEG data to w.  The pixel
// data is streamed from WriteData into the encoder.
func (s *DCTSource) encode(w io.Writer) error {
	pr, pw := io.Pipe()
	done := make(chan error, 1)
	go func() {
		err := s.WriteData(pw)
		pw.CloseWithError(err) // a nil error signals the end of the data
		done <- err
	}()

	params := dct.EncodeParams{
		Width:      s.Width,
		Height:     s.Height,
		Components: s.Colors,
		Transform:  s.ColorTransform,
		Quality:    s.Quality,
	}
	encErr := dct.Encode(w, pr, params)
	closeErr := encErr
	if closeErr == nil {
		closeErr = errTooMuchDCTData
	}
	// unblock WriteData if it is still writing
	pr.CloseWithError(closeErr)
	writeErr := <-done

	if encErr != nil {
		return encErr
	}
	return writeErr
}

var errTooMuchDCTData = errors.New("too much DCT image data")

// Pixels returns the raw, uncompressed pixel data.
//
// The data is passed through the JPEG encoder and decoder, so it is the data
// a reader of the PDF file obtains, which differs slightly from the data
// written by WriteData.
func (s *DCTSource) Pixels() ([]byte, error) {
	if err := s.check(); err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	if err := s.encode(&buf); err != nil {
		return nil, err
	}

	// the decoder charges its buffers to the budget; the margin covers
	// the fixed-size tables
	n := int64(s.Width) * int64(s.Height) * int64(s.Colors)
	rc, err := dct.Decode(&buf, nil, membudget.New(4*n+(1<<20)))
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(rc)
}

// IsJPX implements [graphics.ImageData].
func (s *DCTSource) IsJPX() bool { return false }

// WriteStream implements [graphics.ImageData].
func (s *DCTSource) WriteStream(rm *pdf.EmbedHelper, ref pdf.Reference, dict pdf.Dict) error {
	if err := s.check(); err != nil {
		return err
	}

	// The encoder writes a marker which selects the transform; the
	// parameter dictionary states the same choice.  The filter is
	// added to the dictionary since FilterDCT cannot encode.
	var parms pdf.FilterDCT
	if s.Colors != 1 {
		if s.ColorTransform {
			parms.ColorTransform = pdf.DCTColorTransformYCbCr
		} else {
			parms.ColorTransform = pdf.DCTColorTransformNone
		}
	}
	name, parmDict, err := parms.Info(rm.Out().GetMeta().Version)
	if err != nil {
		return err
	}
	dict = maps.Clone(dict)
	dict["Filter"] = name
	if parmDict != nil {
		dict["DecodeParms"] = parmDict
	}

	w, err := rm.Out().OpenStream(ref, dict)
	if err != nil {
		return err
	}
	if err := s.encode(w); err != nil {
		w.Close()
		return err
	}
	return w.Close()
}
