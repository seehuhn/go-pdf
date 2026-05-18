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

package jpeg

import (
	"image/color"
	"io"
)

// DecodeStream decodes a JPEG image from r and writes raw pixel bytes
// to w, row by row.  The output format depends on the JPEG's component
// count and color decisions:
//
//   - 1 component: 1 byte per pixel (grayscale).
//   - 3 components, YCbCr → RGB: 3 bytes per pixel (R, G, B).
//   - 3 components, raw RGB pass-through (no APP14 transform, or
//     ColorTransform=0 override): 3 bytes per pixel (component planes
//     interpreted positionally).
//   - 4 components, YCCK: 4 bytes per pixel (255-R, 255-G, 255-B, K).
//   - 4 components, raw CMYK: 4 bytes per pixel (the four component
//     planes in scan order, no inversion).
//
// If colorTransform is non-nil it overrides the JPEG's APP14-based
// color transform decision (0 = no transform, 1 = YCbCr/YCCK).
//
// For single-scan baseline JPEGs DecodeStream emits one MCU stripe at
// a time directly from a stripe-sized internal buffer, so peak memory
// is bounded by the image width.  For progressive JPEGs and the rare
// multi-scan baseline case it falls back to the full-buffer path and
// emits the result after the entire JPEG has been parsed.
func DecodeStream(r io.Reader, colorTransform *int, w io.Writer) error {
	var d decoder
	d.colorTransformOverride = colorTransform
	d.streamOut = w
	return d.decode(r)
}

// useYCCK reports whether 4-component output should go through the YCCK
// path (YCbCr → RGB inverted to CMY, with raw K) rather than being
// emitted as raw CMYK planes.  Encodes the APP14 + colorTransformOverride
// decision per PDF spec table 13.
func (d *decoder) useYCCK() bool {
	if d.colorTransformOverride != nil {
		return *d.colorTransformOverride != 0
	}
	if d.adobeTransformValid {
		return d.adobeTransform != adobeTransformUnknown
	}
	return false
}

// emitStripe converts and writes one MCU stripe (the contents of
// d.y/d.cb/d.cr/d.blackPix after the inner loops of
// [decoder.processSOS] finish MCU row `my`) to d.streamOut.
func (d *decoder) emitStripe(my int) error {
	v0 := d.comp[0].v
	yStart := 8 * v0 * my
	yEnd := min(yStart+8*v0, d.height)
	return d.emit(d.streamOut, yStart, yEnd)
}

// emitGray writes grayscale bytes for rows [yStart, yEnd) to w.
func (d *decoder) emitGray(w io.Writer, yStart, yEnd int) error {
	width := d.width
	for y := yStart; y < yEnd; y++ {
		off := (y - yStart) * d.yStride
		if _, err := w.Write(d.y[off : off+width]); err != nil {
			return err
		}
	}
	return nil
}

// emitYCbCr applies the YCbCr → RGB conversion per pixel and writes
// 3-bytes-per-pixel RGB rows.  Chroma plane offsets are computed from
// the stripe-local row (y - yStart) and the recorded chroma subsample.
func (d *decoder) emitYCbCr(w io.Writer, yStart, yEnd int) error {
	width := d.width
	row := make([]byte, width*3)
	for y := yStart; y < yEnd; y++ {
		ly := y - yStart
		yRow := ly * d.yStride
		cRow := (ly / d.vRatio) * d.cStride
		i := 0
		for x := range width {
			cOff := cRow + x/d.hRatio
			r, g, b := color.YCbCrToRGB(d.y[yRow+x], d.cb[cOff], d.cr[cOff])
			row[i] = r
			row[i+1] = g
			row[i+2] = b
			i += 3
		}
		if _, err := w.Write(row); err != nil {
			return err
		}
	}
	return nil
}

// emitYCbCrAsRGB emits Y, Cb, Cr as R, G, B respectively, with no
// colour conversion.  This is the ColorTransform=0 / RGB pass-through
// case described in [decoder.isRGB].
func (d *decoder) emitYCbCrAsRGB(w io.Writer, yStart, yEnd int) error {
	width := d.width
	row := make([]byte, width*3)
	for y := yStart; y < yEnd; y++ {
		ly := y - yStart
		yRow := ly * d.yStride
		cRow := (ly / d.vRatio) * d.cStride
		i := 0
		for x := range width {
			cOff := cRow + x/d.hRatio
			row[i] = d.y[yRow+x]
			row[i+1] = d.cb[cOff]
			row[i+2] = d.cr[cOff]
			i += 3
		}
		if _, err := w.Write(row); err != nil {
			return err
		}
	}
	return nil
}

// emitYCCK applies the YCCK → CMYK conversion (YCbCr → RGB, then
// invert RGB to obtain CMY) and writes 4-bytes-per-pixel CMYK rows in
// PDF convention (0 = no ink); K is taken from blackPix without
// inversion since the Adobe-stored K is already PDF-convention after
// the implicit double-inversion through the YCbCr/RGB matrix.
func (d *decoder) emitYCCK(w io.Writer, yStart, yEnd int) error {
	width := d.width
	row := make([]byte, width*4)
	for y := yStart; y < yEnd; y++ {
		ly := y - yStart
		yRow := ly * d.yStride
		cRow := (ly / d.vRatio) * d.cStride
		bRow := ly * d.blackStride
		i := 0
		for x := range width {
			cOff := cRow + x/d.hRatio
			r, g, b := color.YCbCrToRGB(d.y[yRow+x], d.cb[cOff], d.cr[cOff])
			row[i] = 255 - r
			row[i+1] = 255 - g
			row[i+2] = 255 - b
			row[i+3] = d.blackPix[bRow+x]
			i += 4
		}
		if _, err := w.Write(row); err != nil {
			return err
		}
	}
	return nil
}

// emitRawCMYK writes the four component planes (Y, Cb, Cr, blackPix)
// as raw CMYK bytes without any inversion: Adobe CMYK JPEGs store
// pixel values in PDF convention (0 = no ink) once the Adobe sign
// convention is reconciled with PDF, so a positional pass-through
// produces the correct output.
func (d *decoder) emitRawCMYK(w io.Writer, yStart, yEnd int) error {
	width := d.width
	row := make([]byte, width*4)
	for y := yStart; y < yEnd; y++ {
		ly := y - yStart
		yRow := ly * d.yStride
		cRow := (ly / d.vRatio) * d.cStride
		bRow := ly * d.blackStride
		i := 0
		for x := range width {
			cOff := cRow + x/d.hRatio
			row[i] = d.y[yRow+x]
			row[i+1] = d.cb[cOff]
			row[i+2] = d.cr[cOff]
			row[i+3] = d.blackPix[bRow+x]
			i += 4
		}
		if _, err := w.Write(row); err != nil {
			return err
		}
	}
	return nil
}
