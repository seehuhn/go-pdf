// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package image

import (
	"errors"
	"iter"

	"seehuhn.de/go/pdf/graphics"
)

// SampleReader reads the packed samples of an image one row at a time, from
// top to bottom.  Use [Dict.SampleRows], [Mask.SampleRows] or
// [SoftMask.SampleRows] to create a SampleReader.  The image data is opened
// by [SampleReader.All] and no cleanup is needed.
type SampleReader struct {
	// Width and Height give the size of the image, in pixels.
	Width, Height int

	// RowBytes is the length of each row in bytes.
	RowBytes int

	data graphics.ImageData
	err  error
}

func newSampleReader(data graphics.ImageData, width, height, channels, bpc int) (*SampleReader, error) {
	switch {
	case data == nil:
		return nil, errors.New("missing image data")
	case data.IsJPX():
		return nil, errors.New("JPXDecode image has no packed samples")
	case width <= 0 || height <= 0:
		return nil, errors.New("invalid image size")
	}
	return &SampleReader{
		Width:    width,
		Height:   height,
		RowBytes: (width*channels*bpc + 7) / 8,
		data:     data,
	}, nil
}

// SampleRows returns a reader for the samples of the image exactly as they
// are stored in the image stream, after all filters have been applied.  Each
// pixel occupies one sample per colour component, BitsPerComponent bits wide,
// and every row starts at a byte boundary.  The Decode array and the colour
// key are not applied.
//
// SampleRows fails for JPXDecode images, whose samples are not stored in this
// form; use [Dict.Load] instead.  Errors in the image data are reported by
// [SampleReader.Err] during iteration.
func (d *Dict) SampleRows() (*SampleReader, error) {
	if d.ColorSpace == nil {
		return nil, errors.New("missing ColorSpace")
	}
	if d.BitsPerComponent == 0 {
		return nil, errors.New("missing BitsPerComponent")
	}
	ncomp := d.ColorSpace.Channels()
	if ncomp == 0 {
		return nil, errors.New("pattern color spaces are not valid for images")
	}
	return newSampleReader(d.Data, d.Width, d.Height, ncomp, d.BitsPerComponent)
}

// SampleRows returns a reader for the samples of the image mask, as
// [Dict.SampleRows] does for an image.  The samples are one bit wide.
func (m *Mask) SampleRows() (*SampleReader, error) {
	return newSampleReader(m.Source, m.Width, m.Height, 1, 1)
}

// SampleRows returns a reader for the samples of the soft mask, as
// [Dict.SampleRows] does for an image.  The Decode array is not applied.
func (sm *SoftMask) SampleRows() (*SampleReader, error) {
	if sm.BitsPerComponent == 0 {
		return nil, errors.New("missing BitsPerComponent")
	}
	return newSampleReader(sm.Source, sm.Width, sm.Height, 1, sm.BitsPerComponent)
}

// All returns an iterator over the rows of the image, from top to bottom,
// together with their indices.  The slice is valid only until the next
// iteration.  Samples missing from short image data count as 0, and data
// beyond the last row is ignored.
//
// All can be called more than once; each call opens the image data afresh and
// starts at the first row.  The data is closed when the iteration ends,
// including when the loop is left early.
//
// If the image data cannot be opened or read, the iteration stops early; use
// [SampleReader.Err] after the loop to check for this.
func (r *SampleReader) All() iter.Seq2[int, []byte] {
	return func(yield func(int, []byte) bool) {
		r.err = nil
		src, err := openPixels(r.data)
		if err != nil {
			r.err = err
			return
		}
		defer src.Close()
		r.err = eachPackedRow(src, r.RowBytes, r.Height, yield)
	}
}

// Err returns the error, if any, which ended the most recent iteration over
// [SampleReader.All].  It is nil if no iteration has run.
func (r *SampleReader) Err() error {
	return r.err
}
