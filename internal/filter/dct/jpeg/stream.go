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
	"image"
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
	if _, err := d.decode(r, false); err != nil {
		return err
	}
	if d.streaming {
		return nil
	}
	return d.emitFull(w)
}

// useYCCK reports whether 4-component output should go through the YCCK
// path (YCbCr → RGB inverted to CMY, with raw K) rather than being
// emitted as raw CMYK planes.  Mirrors the decision in [applyBlack].
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
// d.img1/d.img3/d.blackPix after the inner loops of [decoder.processSOS]
// finish MCU row `my`) to d.streamOut.
func (d *decoder) emitStripe(my, mxx, myy int) error {
	_ = mxx
	_ = myy
	v0 := 1
	if d.nComp >= 1 {
		v0 = d.comp[0].v
	}
	yStart := 8 * v0 * my
	yEnd := min(yStart+8*v0, d.height)
	return d.emitRows(d.streamOut, yStart, yEnd)
}

// emitFull walks the full-image buffer that the (non-streaming) decode
// path built and writes pixel bytes to w.
func (d *decoder) emitFull(w io.Writer) error {
	if d.img1 == nil && d.img3 == nil {
		return FormatError("missing SOS marker")
	}
	return d.emitRows(w, 0, d.height)
}

// emitRows writes image rows in the half-open range [yStart, yEnd) to w.
// The plane offsets are computed from y - yStart, which matches stripe-
// local coordinates in streaming mode and global coordinates in full
// mode (where yStart is 0).
func (d *decoder) emitRows(w io.Writer, yStart, yEnd int) error {
	width := d.width
	if d.nComp == 1 {
		return emitGrayRows(w, d.img1.Pix, d.img1.Stride, yStart, yEnd, width)
	}
	if d.nComp == 3 {
		if d.isRGB() {
			return emitYCbCrPlanesAsRGBRows(w, d.img3, yStart, yEnd, width)
		}
		return emitYCbCrAsRGBRows(w, d.img3, yStart, yEnd, width)
	}
	if d.useYCCK() {
		return emitYCCKRows(w, d.img3, d.blackPix, d.blackStride, yStart, yEnd, width)
	}
	return emitRawCMYKRows(w, d.img3, d.blackPix, d.blackStride, yStart, yEnd, width)
}

// emitGrayRows writes grayscale bytes for rows [yStart, yEnd) to w.
// pix is the underlying *image.Gray Pix slice; stride is the row stride
// of that buffer.  Each output row is `width` bytes.
func emitGrayRows(w io.Writer, pix []byte, stride, yStart, yEnd, width int) error {
	for y := yStart; y < yEnd; y++ {
		off := (y - yStart) * stride
		if _, err := w.Write(pix[off : off+width]); err != nil {
			return err
		}
	}
	return nil
}

// emitYCbCrAsRGBRows applies the YCbCr → RGB conversion per pixel and
// writes 3-bytes-per-pixel RGB rows to w.  The image's bounds are
// expected to begin at (0, 0); local row index inside the buffer is
// y - yStart so the same function serves both full-image and stripe
// buffers.
func emitYCbCrAsRGBRows(w io.Writer, img *image.YCbCr, yStart, yEnd, width int) error {
	row := make([]byte, width*3)
	for y := yStart; y < yEnd; y++ {
		ly := y - yStart
		i := 0
		for x := range width {
			yOff := img.YOffset(x, ly)
			cOff := img.COffset(x, ly)
			r, g, b := color.YCbCrToRGB(img.Y[yOff], img.Cb[cOff], img.Cr[cOff])
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

// emitYCbCrPlanesAsRGBRows emits Y, Cb, Cr as R, G, B respectively,
// i.e. without colour conversion.  This is the ColorTransform=0 / RGB
// pass-through case described in [decoder.isRGB].  Plane bytes use the
// chroma stride and subsample for Cb/Cr addressing.
func emitYCbCrPlanesAsRGBRows(w io.Writer, img *image.YCbCr, yStart, yEnd, width int) error {
	row := make([]byte, width*3)
	for y := yStart; y < yEnd; y++ {
		ly := y - yStart
		i := 0
		for x := range width {
			yOff := img.YOffset(x, ly)
			cOff := img.COffset(x, ly)
			row[i] = img.Y[yOff]
			row[i+1] = img.Cb[cOff]
			row[i+2] = img.Cr[cOff]
			i += 3
		}
		if _, err := w.Write(row); err != nil {
			return err
		}
	}
	return nil
}

// emitYCCKRows applies the YCCK → CMYK conversion (YCbCr → RGB, then
// invert RGB to obtain CMY) and writes 4-bytes-per-pixel CMYK rows
// where K is taken from blackPix without inversion.  This matches the
// net result of the existing applyBlack YCCK path composed with the
// outer dct CMYK inversion.
func emitYCCKRows(w io.Writer, img *image.YCbCr, blackPix []byte, blackStride, yStart, yEnd, width int) error {
	row := make([]byte, width*4)
	for y := yStart; y < yEnd; y++ {
		ly := y - yStart
		i := 0
		for x := range width {
			yOff := img.YOffset(x, ly)
			cOff := img.COffset(x, ly)
			r, g, b := color.YCbCrToRGB(img.Y[yOff], img.Cb[cOff], img.Cr[cOff])
			row[i] = 255 - r
			row[i+1] = 255 - g
			row[i+2] = 255 - b
			row[i+3] = blackPix[ly*blackStride+x]
			i += 4
		}
		if _, err := w.Write(row); err != nil {
			return err
		}
	}
	return nil
}

// emitRawCMYKRows writes the four component planes (Y, Cb, Cr, blackPix)
// as raw CMYK bytes without any inversion.  This matches the net result
// of the existing applyBlack non-YCCK path composed with the outer dct
// CMYK inversion (the two inversions cancel out).
func emitRawCMYKRows(w io.Writer, img *image.YCbCr, blackPix []byte, blackStride, yStart, yEnd, width int) error {
	row := make([]byte, width*4)
	for y := yStart; y < yEnd; y++ {
		ly := y - yStart
		i := 0
		for x := range width {
			yOff := img.YOffset(x, ly)
			cOff := img.COffset(x, ly)
			row[i] = img.Y[yOff]
			row[i+1] = img.Cb[cOff]
			row[i+2] = img.Cr[cOff]
			row[i+3] = blackPix[ly*blackStride+x]
			i += 4
		}
		if _, err := w.Write(row); err != nil {
			return err
		}
	}
	return nil
}
