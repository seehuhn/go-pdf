// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package image

import (
	"bytes"
	"errors"
	"io"
	"iter"

	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
)

// RowReader decodes an image one row at a time, from top to bottom.
// Use [Dict.Rows] to create a RowReader.
type RowReader struct {
	// Width and Height give the size of the decoded image, in pixels.
	Width, Height int

	// CS is the colour space of the samples.
	CS color.Space

	d      *Dict
	decode []float64
	key    []uint16 // the colour key of a packed image, if any
	jpx    *Data    // the decoded image, for JPXDecode
	err    error
}

// Row is one row of a decoded image.
type Row struct {
	// Pix holds the colour values of the pixels, from left to right, in the
	// colour space of the image.  Each pixel occupies one value per colour
	// component.
	Pix []float32

	// Alpha (optional) holds the opacity of each pixel, as [Data.Alpha]
	// does.
	Alpha []float32

	// Key (optional) holds the colour key mask of the row, as
	// [Data.KeyMask] does: 0 at pixels the colour key masks out, and 255
	// elsewhere.
	Key []uint8
}

// Rows returns a reader for the rows of the image.  The rows hold the values
// which [Dict.Load] returns in a [Data], but are decoded one at a time, so
// that the samples of the whole image need not be held in memory.
//
// A JPXDecode image is decoded as a whole when Rows is called.
func (d *Dict) Rows() (*RowReader, error) {
	if d.Data != nil && d.Data.IsJPX() {
		data, err := d.loadJPX(0)
		if err != nil {
			return nil, err
		}
		r := &RowReader{
			Width:  data.Rect.Dx(),
			Height: data.Rect.Dy(),
			CS:     data.CS,
			jpx:    data,
		}
		return r, nil
	}

	cs := d.ColorSpace
	if cs == nil {
		return nil, errors.New("missing ColorSpace")
	}
	if d.BitsPerComponent == 0 {
		return nil, errors.New("missing BitsPerComponent")
	}
	ncomp := cs.Channels()
	if ncomp == 0 {
		return nil, errors.New("pattern color spaces are not valid for images")
	}

	// a Decode array shorter than 2*ncomp would make readSamples index out of
	// range; fall back to the default, as the read path in ExtractDict does
	decode := d.Decode
	if len(decode) < 2*ncomp {
		decode = DefaultDecode(cs, d.BitsPerComponent)
	}

	r := &RowReader{
		Width:  d.Width,
		Height: d.Height,
		CS:     cs,
		d:      d,
		decode: decode,
	}
	if len(d.MaskColors) == 2*ncomp {
		r.key = d.MaskColors
	}
	return r, nil
}

// All returns an iterator over the rows of the image, from top to bottom,
// together with their indices.  The slices of a [Row] are valid only until
// the next iteration.  All can be called more than once; each call starts
// again at the first row.
//
// If the image data cannot be decoded, the iteration stops early; use
// [RowReader.Err] after the loop to check for this.
func (r *RowReader) All() iter.Seq2[int, Row] {
	return func(yield func(int, Row) bool) {
		r.err = nil
		if r.jpx != nil {
			r.jpxRows(yield)
			return
		}
		row := Row{Pix: make([]float32, r.Width*r.CS.Channels())}
		if r.key != nil {
			row.Key = make([]uint8, r.Width)
		}
		r.err = r.decodeRows(func(int) Row { return row }, yield)
	}
}

// Err returns the error, if any, which ended the most recent iteration over
// [RowReader.All].
func (r *RowReader) Err() error {
	return r.err
}

// jpxRows yields the rows of a decoded JPXDecode image.
func (r *RowReader) jpxRows(yield func(int, Row) bool) {
	data := r.jpx
	w := r.Width
	for y := range r.Height {
		row := Row{Pix: data.Pix[y*data.Stride:][:w*data.NComp]}
		if data.Alpha != nil {
			row.Alpha = data.Alpha[y*w:][:w]
		}
		if data.KeyMask != nil {
			row.Key = data.KeyMask.Pix[y*data.KeyMask.Stride:][:w]
		}
		if !yield(y, row) {
			return
		}
	}
}

// decodeRows decodes the rows of a packed image in order.  Row y is decoded
// into the slices which dst(y) returns, and is then passed to yield.
// Samples missing from short image data count as 0.
func (r *RowReader) decodeRows(dst func(y int) Row, yield func(int, Row) bool) error {
	d := r.d
	width := r.Width
	ncomp := r.CS.Channels()
	bpc := d.BitsPerComponent
	rowSamples := width * ncomp
	rowBytes := (rowSamples*bpc + 7) / 8

	var lut [][256]float32
	var unpack *unpacker
	switch {
	case bpc == 8:
		lut = decodeLUT8(ncomp, r.decode)
	case bpc < 8 && rowSamples*r.Height >= unpackTableSize(ncomp, bpc):
		// for images smaller than the tables, building them would cost
		// more than it saves
		unpack = newUnpacker(ncomp, bpc, r.decode)
	}

	src, err := openPixels(d.Data)
	if err != nil {
		return err
	}
	defer src.Close()

	return eachPackedRow(src, rowBytes, r.Height, func(y int, buf []byte) bool {
		row := dst(y)
		switch {
		case lut != nil:
			in, out := buf[:rowSamples], row.Pix[:rowSamples]
			for i := 0; i < rowSamples; i += ncomp {
				for c := range ncomp {
					out[i+c] = lut[c][in[i+c]]
				}
			}
		case unpack != nil:
			unpack.row(row.Pix[:rowSamples], buf)
		default:
			for x := range width {
				readSamples(buf, width, ncomp, bpc, x, 0, r.decode, row.Pix[x*ncomp:][:ncomp])
			}
		}
		if r.key != nil {
			rawColorKey(row.Key[:width], buf, ncomp, bpc, r.key)
		}
		return yield(y, row)
	})
}

// eachPackedRow reads height rows of rowBytes bytes from src and passes them
// to fn in order, until fn returns false.  The buffer is reused between
// rows.  Samples missing from short data count as 0.
func eachPackedRow(src io.Reader, rowBytes, height int, fn func(y int, buf []byte) bool) error {
	buf := make([]byte, rowBytes)
	more := true   // whether src may hold more data
	stale := false // whether buf holds samples of an earlier row
	for y := range height {
		if more {
			n, err := readRow(src, buf)
			if err != nil {
				return err
			}
			if n < rowBytes {
				clear(buf[n:])
				more = false
				stale = n > 0
			}
		} else if stale {
			clear(buf)
			stale = false
		}
		if !fn(y, buf) {
			return nil
		}
	}
	return nil
}

// readRow fills buf from r.  It returns the number of bytes read, which is
// less than len(buf) only when the data ends.
func readRow(r io.Reader, buf []byte) (int, error) {
	n := 0
	for n < len(buf) {
		k, err := r.Read(buf[n:])
		n += k
		if err == io.EOF {
			break
		}
		if err != nil {
			return n, err
		}
	}
	return n, nil
}

// pixelStreamer is implemented by image data which can deliver the bytes
// [graphics.ImageData.Pixels] returns incrementally.
type pixelStreamer interface {
	pixelReader() (io.ReadCloser, error)
}

// openPixels returns a reader for the packed samples of an image other than
// JPXDecode.
func openPixels(data graphics.ImageData) (io.ReadCloser, error) {
	if s, ok := data.(pixelStreamer); ok {
		return s.pixelReader()
	}
	raw, err := data.Pixels()
	if err != nil {
		return nil, err
	}
	return io.NopCloser(bytes.NewReader(raw)), nil
}
