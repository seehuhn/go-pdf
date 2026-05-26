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
	"seehuhn.de/go/pdf/internal/streamlimits"
	"seehuhn.de/go/pdf/opaque"
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
	// component. The value must be 1, 2, 4, 8, or 16.
	BitsPerComponent int

	// Decode (optional) is an array of numbers describing how to map mask
	// samples into the range 0.0 to 1.0. If present, the array must contain
	// exactly 2 values [Dmin, Dmax]. Default: [0.0, 1.0] which maps 0 to
	// transparent, max to opaque.
	Decode []float64

	// Source writes the body of the soft-mask stream when the SoftMask is
	// embedded.  For raw grayscale pixel data use [FlateSource];
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

	dict := pdf.Dict{
		"Subtype":          pdf.Name("Image"),
		"Width":            pdf.Integer(sm.Width),
		"Height":           pdf.Integer(sm.Height),
		"ColorSpace":       csEmbedded,
		"BitsPerComponent": pdf.Integer(sm.BitsPerComponent),
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
// so that 0 = transparent and 1 = fully opaque.
func (sm *SoftMask) LoadAlpha() (*image.Alpha, error) {
	raw, err := sm.Source.Pixels()
	if err != nil {
		return nil, err
	}

	width, height := sm.Width, sm.Height
	bpc := sm.BitsPerComponent
	raw = normalizeData(raw, expectedDataSize(width, 1, bpc, height))

	dMin, dMax := 0.0, 1.0
	if len(sm.Decode) >= 2 {
		dMin, dMax = sm.Decode[0], sm.Decode[1]
	}

	alpha := image.NewAlpha(image.Rect(0, 0, width, height))

	switch bpc {
	case 8:
		for y := range height {
			rowStart := y * width
			for x := range width {
				s := float64(raw[rowStart+x]) / 255
				alpha.Pix[y*alpha.Stride+x] = uint8((dMin + s*(dMax-dMin)) * 255)
			}
		}
	case 16:
		for y := range height {
			rowStart := y * width * 2
			for x := range width {
				idx := rowStart + x*2
				s := float64(uint16(raw[idx])<<8|uint16(raw[idx+1])) / 65535
				alpha.Pix[y*alpha.Stride+x] = uint8((dMin + s*(dMax-dMin)) * 255)
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
				alpha.Pix[y*alpha.Stride+x] = uint8((dMin + s*(dMax-dMin)) * 255)
			}
		}
	}

	return alpha, nil
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
	switch sm.BitsPerComponent {
	case 1, 2, 4, 8:
		// pass
	case 16:
		if err := pdf.CheckVersion(out, "16 bits per soft mask component", pdf.V1_5); err != nil {
			return err
		}
	default:
		return fmt.Errorf("invalid BitsPerComponent %d", sm.BitsPerComponent)
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
func ExtractSoftMask(x *pdf.Extractor, path *pdf.CycleCheck, obj pdf.Object, _ bool) (*SoftMask, error) {
	stm, err := x.GetStream(path, obj)
	if err != nil {
		return nil, err
	} else if stm == nil {
		return nil, pdf.Error("missing soft mask stream")
	}
	dict := stm.Dict

	// Check Type and Subtype
	if err := pdf.CheckDictType(x.R, dict, "XObject"); err != nil {
		return nil, err
	}
	if subtypeName, err := pdf.Optional(x.GetName(path, dict["Subtype"])); err != nil {
		return nil, err
	} else if subtypeName != "Image" && subtypeName != "" {
		return nil, pdf.Errorf("invalid Subtype %q for soft mask XObject", subtypeName)
	}

	// validate ColorSpace is DeviceGray (required for soft masks)
	if csObj, ok := dict["ColorSpace"]; ok {
		cs, err := color.ExtractSpace(x, path, csObj, false)
		if err != nil {
			return nil, err
		}
		if cs.Family() != color.FamilyDeviceGray {
			return nil, pdf.Errorf("soft mask ColorSpace must be DeviceGray, got %s", cs.Family())
		}
	}
	// missing ColorSpace defaults to DeviceGray (the only valid value)

	// ignore forbidden fields per Table 143 (permissive reading)
	delete(dict, "ImageMask")
	delete(dict, "Mask")
	delete(dict, "SMask")
	delete(dict, "SMaskInData")

	// extract required fields
	width, err := x.GetInteger(path, dict["Width"])
	if err != nil {
		return nil, err
	}
	if width <= 0 {
		return nil, pdf.Errorf("invalid soft mask width %d", width)
	}
	if width > streamlimits.MaxImageWidth {
		return nil, pdf.Errorf("soft mask width %d exceeds limit", width)
	}

	height, err := x.GetInteger(path, dict["Height"])
	if err != nil {
		return nil, err
	}
	if height <= 0 {
		return nil, pdf.Errorf("invalid soft mask height %d", height)
	}
	if height > streamlimits.MaxImageHeight {
		return nil, pdf.Errorf("soft mask height %d exceeds limit", height)
	}
	if streamlimits.ImagePixelsExceedLimit(int(width), int(height)) {
		return nil, pdf.Error("soft mask pixel count exceeds limit")
	}

	bpc, err := x.GetInteger(path, dict["BitsPerComponent"])
	if err != nil {
		return nil, err
	}
	// spec restricts BitsPerComponent to {1, 2, 4, 8, 16}; silently fall back
	// to 8 for malformed values so the soft mask round-trips.
	switch bpc {
	case 1, 2, 4, 8, 16:
		// pass
	default:
		bpc = 8
	}

	softMask := &SoftMask{
		Width:            int(width),
		Height:           int(height),
		BitsPerComponent: int(bpc),
	}

	// Extract Decode array
	if decodeArray, err := pdf.Optional(x.GetArray(path, dict["Decode"])); err != nil {
		return nil, err
	} else if decodeArray != nil {
		if len(decodeArray) != 2 {
			return nil, pdf.Errorf("invalid Decode array length %d for soft mask (must be 2)", len(decodeArray))
		}
		softMask.Decode = make([]float64, 2)
		for i, val := range decodeArray {
			if num, err := x.GetNumber(path, val); err != nil {
				return nil, fmt.Errorf("invalid Decode[%d]: %w", i, err)
			} else {
				softMask.Decode[i] = num
			}
		}
	}

	// Extract Interpolate
	if interp, err := x.GetBoolean(path, dict["Interpolate"]); err == nil {
		softMask.Interpolate = bool(interp)
	}

	// Extract Matte array (specific to soft masks); drop wholesale if it
	// exceeds the largest plausible parent-image channel count rather
	// than allocate up to maxArrayLen floats from an attacker-controlled
	// array.  Parity with the actual parent ncomp is validated by the
	// writer in (*Dict).check and defensively by renderers.
	if matteArray, err := pdf.Optional(x.GetArray(path, dict["Matte"])); err != nil {
		return nil, err
	} else if matteArray != nil && len(matteArray) <= streamlimits.MaxImageChannels {
		softMask.Matte = make([]float64, len(matteArray))
		for i, val := range matteArray {
			if num, err := x.GetNumber(path, val); err != nil {
				return nil, fmt.Errorf("invalid Matte[%d]: %w", i, err)
			} else {
				softMask.Matte[i] = num
			}
		}
	}

	softMask.Source = &streamData{
		inner:    opaque.ExtractStream(x, stm),
		maxBytes: ImageDataLimit(softMask.Width, softMask.Height, softMask.BitsPerComponent, color.SpaceDeviceGray),
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
		Source: &FlateSource{
			Predictor:        12,
			Width:            width,
			Colors:           1,
			BitsPerComponent: bitsPerComponent,
			WriteData: func(w io.Writer) error {
				return writeSoftMaskData(w, img, bitsPerComponent)
			},
		},
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
