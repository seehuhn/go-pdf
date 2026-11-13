// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package image

import (
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/internal/imagedict"
)

// The functions in this file read the entries which image XObjects and inline
// images have in common.  For an inline image, dict holds the entries under
// their full names, see [imagedict.ExpandInline].

// readSampleEntries reads the BitsPerComponent, Intent, Decode and Interpolate
// entries of a sampled image.  img.Width, img.Height and img.ColorSpace must
// already be set; for a JPXDecode image the colour space may be nil.
func (img *Dict) readSampleEntries(c pdf.Cursor, dict pdf.Dict, filters []pdf.Filter, hasJPX bool) error {
	// For JPXDecode images BitsPerComponent is optional and shall be ignored
	// if present; drop any value to keep round-trip parity with check(),
	// which requires img.BitsPerComponent == 0 for JPX.
	if hasJPX {
		img.BitsPerComponent = 0
	} else {
		bpc, err := imagedict.ReadBitsPerComponent(c, dict, filters)
		if err != nil {
			return err
		}
		img.BitsPerComponent = bpc
	}

	// JPXDecode images may lack a colour space and are covered by the
	// pixel-count cap of imagedict.ReadDimensions alone; their decoder is
	// responsible for honouring channel and bit-depth bounds from the JP2
	// codestream.
	if img.ColorSpace != nil && img.BitsPerComponent > 0 {
		err := imagedict.CheckDataSize(img.Width, img.Height,
			img.ColorSpace.Channels(), img.BitsPerComponent)
		if err != nil {
			return err
		}
	}

	if intent, err := pdf.Optional(c.Name(dict["Intent"])); err != nil {
		return err
	} else if intent != "" {
		img.Intent = graphics.RenderingIntent(intent)
	}

	// A JPXDecode image without a colour space ignores the Decode array, so
	// img.Decode stays nil.  For other JPXDecode images the default depends
	// on the bit depth of the JPEG 2000 data, so an absent array stays nil.
	if img.ColorSpace != nil {
		// honour a Decode array with at least 2*ncomp entries, ignoring any
		// surplus; a shorter array falls back to the default
		n := 2 * img.ColorSpace.Channels()
		if decodeArray, err := pdf.Optional(c.Array(dict["Decode"])); err != nil {
			return err
		} else if len(decodeArray) >= n {
			decode := make([]float64, n)
			valid := true
			for i := range decode {
				num, err := c.Number(decodeArray[i])
				if pdf.IsMalformed(err) {
					valid = false
					break
				} else if err != nil {
					return err
				}
				decode[i] = num
			}
			if valid {
				img.Decode = decode
			}
		}
		if img.Decode == nil && !hasJPX {
			img.Decode = DefaultDecode(img.ColorSpace, img.BitsPerComponent)
		}
	}

	interp, _, err := c.OptionalBoolean(dict["Interpolate"])
	if err != nil {
		return err
	}
	img.Interpolate = interp
	return nil
}

// readMaskEntries reads the Decode and Interpolate entries of an image mask.
// The BitsPerComponent entry is optional for image masks and, if present,
// must be 1.  An image mask has one bit per sample regardless, so the entry
// is not read.
func (m *Mask) readMaskEntries(c pdf.Cursor, dict pdf.Dict) error {
	// Decode must be [0 1] or [1 0] for image masks
	if decodeArray, err := pdf.Optional(c.Array(dict["Decode"])); err != nil {
		return err
	} else if len(decodeArray) == 2 {
		// a Decode array with a malformed element is dropped
		d0, err := c.Number(decodeArray[0])
		if pdf.IsReadError(err) {
			return err
		}
		d1, err2 := c.Number(decodeArray[1])
		if pdf.IsReadError(err2) {
			return err2
		}
		if err == nil && err2 == nil && d0 == 1 && d1 == 0 {
			m.Inverted = true
		}
	}

	interp, _, err := c.OptionalBoolean(dict["Interpolate"])
	if err != nil {
		return err
	}
	m.Interpolate = interp
	return nil
}
