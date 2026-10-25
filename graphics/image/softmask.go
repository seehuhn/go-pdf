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

package image

import (
	"errors"
	"fmt"
	"image"
	"io"
	"slices"

	"seehuhn.de/go/geom/rect"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/internal/limits"
)

// PDF 2.0 sections: 11.6.5 8.9.5

// SoftMask represents a soft-mask image used for graduated transparency.
// Soft-mask images are subsidiary image XObjects that provide alpha values for
// smooth transparency effects.
//
// To extract a soft-mask image from a PDF file, use
// [seehuhn.de/go/pdf/graphics/extract.SoftMaskImage].
type SoftMask struct {
	// Width is the width of the soft mask in pixels.
	// If Matte is present, this must match the parent image width.
	Width int

	// Height is the height of the soft mask in pixels.
	// If Matte is present, this must match the parent image height.
	Height int

	// BitsPerComponent is the number of bits used to represent each grayscale
	// component. The value must be 1, 2, 4, 8, or 16.  For JPXDecode soft
	// masks, BitsPerComponent is 0, since the bit depth is given by the
	// JPEG 2000 data.
	BitsPerComponent int

	// Decode (optional) is an array of numbers describing how to map mask
	// samples into the range 0.0 to 1.0. If present, the array must contain
	// exactly 2 values [Dmin, Dmax]. Default: [0.0, 1.0] which maps 0 to
	// transparent, max to opaque.
	Decode []float64

	// Source writes the body of the soft-mask stream when the SoftMask is
	// embedded.  For raw grayscale pixel data use [FlateSource]; for JPEG
	// 2000 data with a single colour channel use [JPXSource].  Other
	// pre-encoded formats provide their own Source implementations.
	Source graphics.ImageData

	// Interpolate indicates whether mask interpolation should be performed by
	// a PDF processor to reduce pixelation in low-resolution masks.
	Interpolate bool

	// Matte (optional) specifies the matte color used for pre-blended image
	// data. The array must contain n values where n is the number of
	// components in the parent image's color space. When present, the parent
	// image data has been pre-multiplied with this matte color using the soft
	// mask as the alpha channel.
	Matte []float64
}

var _ graphics.Image = (*SoftMask)(nil)

// Bounds returns the dimensions of the soft mask.
func (sm *SoftMask) Bounds() rect.IntRect {
	return rect.IntRect{
		XMin: 0,
		YMin: 0,
		XMax: sm.Width,
		YMax: sm.Height,
	}
}

// Subtype returns "Image".
// This implements the [graphics.Image] interface.
func (sm *SoftMask) Subtype() pdf.Name {
	return pdf.Name("Image")
}

// ResourceName returns the empty string: soft masks are referenced from
// ExtGState / image dicts, not from a content stream's /XObject key.
// See [graphics.XObject.ResourceName].
func (sm *SoftMask) ResourceName() pdf.Name {
	return ""
}

// Embed embeds the soft mask as a PDF image XObject stream.
func (sm *SoftMask) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) {
	if err := sm.check(rm.Out()); err != nil {
		return nil, err
	}

	// Soft masks require PDF 1.4+
	if err := pdf.CheckVersion(rm.Out(), "soft mask images", pdf.V1_4); err != nil {
		return nil, err
	}

	// Soft-mask images must always use DeviceGray color space
	csEmbedded, err := rm.Embed(color.SpaceDeviceGray)
	if err != nil {
		return nil, err
	}

	// For JPXDecode soft masks, ColorSpace makes the colour specification
	// of the JPEG 2000 data irrelevant, and BitsPerComponent is omitted.
	dict := pdf.Dict{
		"Subtype":    pdf.Name("Image"),
		"Width":      pdf.Integer(sm.Width),
		"Height":     pdf.Integer(sm.Height),
		"ColorSpace": csEmbedded,
	}
	if sm.BitsPerComponent != 0 {
		dict["BitsPerComponent"] = pdf.Integer(sm.BitsPerComponent)
	}
	if rm.Out().GetOptions().HasAny(pdf.OptDictTypes) {
		dict["Type"] = pdf.Name("XObject")
	}

	// suppress default [0 1]
	if sm.Decode != nil && !slices.Equal(sm.Decode, DefaultDecode(color.SpaceDeviceGray, sm.BitsPerComponent)) {
		var decode pdf.Array
		for _, v := range sm.Decode {
			decode = append(decode, pdf.Number(v))
		}
		dict["Decode"] = decode
	}

	// add Interpolate flag if enabled
	if sm.Interpolate {
		dict["Interpolate"] = pdf.Boolean(true)
	}

	// add Matte array if specified (for pre-blended data)
	if sm.Matte != nil {
		var matte pdf.Array
		for _, v := range sm.Matte {
			matte = append(matte, pdf.Number(v))
		}
		dict["Matte"] = matte
	}

	ref := rm.Alloc()
	if err := sm.Source.WriteStream(rm, ref, dict); err != nil {
		return nil, err
	}
	return ref, nil
}

// LoadAlpha decodes the soft mask pixel data and returns an alpha image.
// Sample values are mapped through the Decode array (default [0, 1])
// so that 0 = transparent and 1 = fully opaque.  Values outside this
// range are clamped.
//
// For JPXDecode soft masks, the size of the result and the bit depth are
// taken from the JPEG 2000 data, which must have a single colour channel.
func (sm *SoftMask) LoadAlpha() (*image.Alpha, error) {
	raw, err := sm.Source.Pixels()
	if err != nil {
		return nil, err
	}

	dMin, dMax := 0.0, 1.0
	if len(sm.Decode) >= 2 {
		dMin, dMax = sm.Decode[0], sm.Decode[1]
	}

	if sm.Source.IsJPX() {
		im, err := decodeJPXMask(raw)
		if err != nil {
			return nil, err
		}
		maxVal := float64(int(1)<<im.BitsPerComponent - 1)
		alpha := image.NewAlpha(image.Rect(0, 0, im.Width, im.Height))
		for i, v := range im.Color[0] {
			alpha.Pix[i] = softMaskAlpha(float64(v)/maxVal, dMin, dMax)
		}
		return alpha, nil
	}

	width, height := sm.Width, sm.Height
	bpc := sm.BitsPerComponent
	raw = normalizeData(raw, expectedDataSize(width, 1, bpc, height))

	alpha := image.NewAlpha(image.Rect(0, 0, width, height))

	switch bpc {
	case 8:
		for y := range height {
			rowStart := y * width
			for x := range width {
				s := float64(raw[rowStart+x]) / 255
				alpha.Pix[y*alpha.Stride+x] = softMaskAlpha(s, dMin, dMax)
			}
		}
	case 16:
		for y := range height {
			rowStart := y * width * 2
			for x := range width {
				idx := rowStart + x*2
				s := float64(uint16(raw[idx])<<8|uint16(raw[idx+1])) / 65535
				alpha.Pix[y*alpha.Stride+x] = softMaskAlpha(s, dMin, dMax)
			}
		}
	case 1, 2, 4:
		samplesPerByte := 8 / bpc
		mask := uint8(1<<bpc - 1)
		maxVal := float64(mask)
		for y := range height {
			rowBytes := (width*bpc + 7) / 8
			rowStart := y * rowBytes
			for x := range width {
				byteIdx := rowStart + x/samplesPerByte
				bitOffset := (samplesPerByte - 1 - x%samplesPerByte) * bpc
				s := float64((raw[byteIdx]>>bitOffset)&mask) / maxVal
				alpha.Pix[y*alpha.Stride+x] = softMaskAlpha(s, dMin, dMax)
			}
		}
	}

	return alpha, nil
}

// softMaskAlpha maps a sample s, scaled to [0, 1], through the Decode
// range [dMin, dMax] to an 8-bit alpha value.
func softMaskAlpha(s, dMin, dMax float64) uint8 {
	return uint8(min(max(dMin+s*(dMax-dMin), 0), 1) * 255)
}

// check validates the soft mask according to PDF specification requirements.
func (sm *SoftMask) check(out *pdf.Writer) error {
	if sm.Width <= 0 {
		return fmt.Errorf("invalid soft mask width %d", sm.Width)
	}
	if sm.Height <= 0 {
		return fmt.Errorf("invalid soft mask height %d", sm.Height)
	}
	if sm.Source == nil {
		return errors.New("source cannot be nil")
	}

	// Validate BitsPerComponent
	switch {
	case sm.Source.IsJPX():
		if sm.BitsPerComponent != 0 {
			return errors.New("BitsPerComponent must be 0 for JPXDecode soft masks")
		}
	case sm.BitsPerComponent == 16:
		if err := pdf.CheckVersion(out, "16 bits per soft mask component", pdf.V1_5); err != nil {
			return err
		}
	case sm.BitsPerComponent != 1 && sm.BitsPerComponent != 2 &&
		sm.BitsPerComponent != 4 && sm.BitsPerComponent != 8:
		return fmt.Errorf("invalid BitsPerComponent %d", sm.BitsPerComponent)
	}

	if s, ok := sm.Source.(graphics.GeometryChecker); ok {
		if err := s.CheckGeometry(sm.Width, sm.Height, 1, sm.BitsPerComponent); err != nil {
			return err
		}
	}
	if s, ok := sm.Source.(*JPXSource); ok {
		if err := s.checkMask(0); err != nil {
			return err
		}
	}

	// Validate Decode array (must have exactly 2 elements for grayscale)
	if sm.Decode != nil && len(sm.Decode) != 2 {
		return fmt.Errorf("wrong Decode length: expected 2, got %d", len(sm.Decode))
	}

	// No additional validation needed for Matte - it's validated by the parent image
	// when the soft mask is associated with it

	return nil
}

// ExtractSoftMask extracts a soft-mask image from a PDF stream.
func ExtractSoftMask(c pdf.Cursor, obj pdf.Object, _ bool) (*SoftMask, error) {
	stm, err := c.Stream(obj)
	if err != nil {
		return nil, err
	} else if stm == nil {
		return nil, pdf.Error("missing soft mask stream")
	}
	dict := stm.Dict

	// Check Type and Subtype
	if err := c.CheckDictType(dict, "XObject"); err != nil {
		return nil, err
	}
	if subtypeName, err := pdf.Optional(c.Name(dict["Subtype"])); err != nil {
		return nil, err
	} else if subtypeName != "Image" && subtypeName != "" {
		return nil, pdf.Errorf("invalid Subtype %q for soft mask XObject", subtypeName)
	}

	filters, err := pdf.Optional(c.Filters(dict))
	if err != nil {
		return nil, err
	}
	jpxIndex := jpxFilterIndex(filters)

	// validate ColorSpace is DeviceGray (required for soft masks);
	// an unusable entry is dropped
	cs, err := c.DecodeOptional(dict["ColorSpace"], color.ExtractSpace)
	if err != nil {
		return nil, err
	}
	hasColorSpace := cs != nil
	if hasColorSpace && cs.Family() != color.FamilyDeviceGray {
		return nil, pdf.Errorf("soft mask ColorSpace must be DeviceGray, got %s", cs.Family())
	}
	// missing ColorSpace defaults to DeviceGray (the only valid value)

	// ImageMask, Mask, SMask and SMaskInData are forbidden by Table 143
	// and ignored here (permissive reading)

	// extract required fields
	width, err := c.Integer(dict["Width"])
	if err != nil {
		return nil, err
	}
	if width <= 0 {
		return nil, pdf.Errorf("invalid soft mask width %d", width)
	}
	if width > limits.MaxImageWidth {
		return nil, pdf.Errorf("soft mask width %d exceeds limit", width)
	}

	height, err := c.Integer(dict["Height"])
	if err != nil {
		return nil, err
	}
	if height <= 0 {
		return nil, pdf.Errorf("invalid soft mask height %d", height)
	}
	if height > limits.MaxImageHeight {
		return nil, pdf.Errorf("soft mask height %d exceeds limit", height)
	}
	if limits.ImagePixelsExceedLimit(int(width), int(height)) {
		return nil, pdf.Error("soft mask pixel count exceeds limit")
	}

	// spec restricts BitsPerComponent to {1, 2, 4, 8, 16}; silently fall back
	// to 8 for malformed values so the soft mask round-trips.  Where the
	// filter fixes the sample size, that size wins over the entry.  For
	// JPXDecode soft masks the entry is ignored.
	var bpc int
	bits, fixed := graphics.FilterSampleBits(filters)
	switch {
	case jpxIndex >= 0:
		// the bit depth is given by the JPEG 2000 data
	case fixed:
		bpc = bits
	default:
		val, err := c.Integer(dict["BitsPerComponent"])
		if err != nil {
			return nil, err
		}
		switch val {
		case 1, 2, 4, 8, 16:
			bpc = int(val)
		default:
			bpc = 8
		}
	}

	softMask := &SoftMask{
		Width:            int(width),
		Height:           int(height),
		BitsPerComponent: bpc,
	}

	// Extract Decode array.  For JPXDecode soft masks without ColorSpace,
	// the array is ignored.
	var decodeObj pdf.Object
	if jpxIndex < 0 || hasColorSpace {
		decodeObj = dict["Decode"]
	}
	if decodeArray, err := pdf.Optional(c.Array(decodeObj)); err != nil {
		return nil, err
	} else if len(decodeArray) == 2 {
		// a Decode array of the wrong length, or with a malformed element,
		// is dropped
		decode := make([]float64, 2)
		for i, val := range decodeArray {
			num, err := c.Number(val)
			if pdf.IsMalformed(err) {
				decode = nil
				break
			} else if err != nil {
				return nil, err
			}
			decode[i] = num
		}
		softMask.Decode = decode
	}

	// Extract Interpolate
	interp, _, err := c.OptionalBoolean(dict["Interpolate"])
	if err != nil {
		return nil, err
	}
	softMask.Interpolate = interp

	// Extract Matte array (specific to soft masks); drop wholesale if it
	// exceeds the largest plausible parent-image channel count rather
	// than allocate up to maxArrayLen floats from an attacker-controlled
	// array.  Whether it fits the parent image is checked by
	// [ExtractDict], which drops a Matte that does not, and by the writer.
	if matteArray, err := pdf.Optional(c.Array(dict["Matte"])); err != nil {
		return nil, err
	} else if matteArray != nil && len(matteArray) <= limits.MaxImageChannels {
		// the array is positional, so one malformed element drops all of it
		matte := make([]float64, len(matteArray))
		for i, val := range matteArray {
			num, err := c.Number(val)
			if pdf.IsMalformed(err) {
				matte = nil
				break
			} else if err != nil {
				return nil, err
			}
			matte[i] = num
		}
		softMask.Matte = matte
	}

	softMask.Source, err = newStreamData(c, stm, jpxIndex,
		ImageDataLimit(softMask.Width, softMask.Height, softMask.BitsPerComponent, color.SpaceDeviceGray))
	if err != nil {
		return nil, err
	}

	return softMask, nil
}

// FromImageAlpha creates a SoftMask from the alpha channel of an image.Image.
// The alpha channel is converted to grayscale values where 0=transparent, max=opaque.
func FromImageAlpha(img image.Image, bitsPerComponent int) *SoftMask {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	return &SoftMask{
		Width:            width,
		Height:           height,
		BitsPerComponent: bitsPerComponent,
		Source: NewFlateSource(width, color.SpaceDeviceGray, bitsPerComponent,
			func(w io.Writer) error {
				return writeSoftMaskData(w, img, bitsPerComponent)
			}),
	}
}

// writeSoftMaskData writes soft mask data from an image.Image's alpha channel to the provided writer.
func writeSoftMaskData(w io.Writer, img image.Image, bitsPerComponent int) error {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	buf := NewPixelRow(width, bitsPerComponent)
	shift := 16 - bitsPerComponent

	for y := range height {
		buf.Reset()

		for x := range width {
			_, _, _, a := img.At(bounds.Min.X+x, bounds.Min.Y+y).RGBA()
			// Convert 16-bit alpha to the specified bit depth (0=transparent, max=opaque)
			buf.AppendBits(uint16(a >> shift))
		}

		_, err := w.Write(buf.Bytes())
		if err != nil {
			return err
		}
	}
	return nil
}
