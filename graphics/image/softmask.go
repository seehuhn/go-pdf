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

	"seehuhn.de/go/geom/rect"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
)

// PDF 2.0 sections: 11.6.5 8.9.5

// SoftMask represents a soft-mask image used for graduated transparency.
// Soft-mask images are subsidiary image XObjects that provide alpha values for
// smooth transparency effects.
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

	// WriteData is a function that writes the grayscale mask data to the
	// provided writer. The data should be written row by row, with each row
	// containing Width samples, each sample using BitsPerComponent bits.
	WriteData func(io.Writer) error

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
		"Type":             pdf.Name("XObject"),
		"Subtype":          pdf.Name("Image"),
		"Width":            pdf.Integer(sm.Width),
		"Height":           pdf.Integer(sm.Height),
		"ColorSpace":       csEmbedded,
		"BitsPerComponent": pdf.Integer(sm.BitsPerComponent),
	}

	// Add Decode array if specified
	if sm.Decode != nil {
		var decode pdf.Array
		for _, v := range sm.Decode {
			decode = append(decode, pdf.Number(v))
		}
		dict["Decode"] = decode
	}

	// Add Interpolate flag if enabled
	if sm.Interpolate {
		dict["Interpolate"] = pdf.Boolean(true)
	}

	// Add Matte array if specified (for pre-blended data)
	if sm.Matte != nil {
		var matte pdf.Array
		for _, v := range sm.Matte {
			matte = append(matte, pdf.Number(v))
		}
		dict["Matte"] = matte
	}

	ref := rm.Alloc()

	// Use compression appropriate for grayscale data
	compress := pdf.FilterCompress{
		"Predictor":        pdf.Integer(12), // PNG UP predictor works well for grayscale
		"Colors":           pdf.Integer(1),  // Always 1 for grayscale
		"BitsPerComponent": pdf.Integer(sm.BitsPerComponent),
		"Columns":          pdf.Integer(sm.Width),
	}

	w, err := rm.Out().OpenStream(ref, dict, compress)
	if err != nil {
		return nil, err
	}

	err = sm.WriteData(w)
	if err != nil {
		return nil, err
	}

	err = w.Close()
	if err != nil {
		return nil, err
	}

	return ref, nil
}

// check validates the soft mask according to PDF specification requirements.
func (sm *SoftMask) check(out *pdf.Writer) error {
	if sm.Width <= 0 {
		return fmt.Errorf("invalid soft mask width %d", sm.Width)
	}
	if sm.Height <= 0 {
		return fmt.Errorf("invalid soft mask height %d", sm.Height)
	}
	if sm.WriteData == nil {
		return errors.New("WriteData function cannot be nil")
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
func ExtractSoftMask(x *pdf.Extractor, obj pdf.Object) (*SoftMask, error) {
	stm, err := x.GetStream(obj)
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
	if subtypeName, err := pdf.Optional(x.GetName(dict["Subtype"])); err != nil {
		return nil, err
	} else if subtypeName != "Image" && subtypeName != "" {
		return nil, pdf.Errorf("invalid Subtype %q for soft mask XObject", subtypeName)
	}

	// Validate ColorSpace is DeviceGray (required for soft masks)
	if csObj, ok := dict["ColorSpace"]; ok {
		cs, err := color.ExtractSpace(x, csObj)
		if err != nil {
			return nil, err
		}
		if cs.Family() != color.FamilyDeviceGray {
			return nil, pdf.Errorf("soft mask ColorSpace must be DeviceGray, got %s", cs.Family())
		}
	} else {
		return nil, pdf.Error("missing ColorSpace for soft mask")
	}

	// Validate forbidden fields (Table 143 restrictions)
	if isImageMask, err := x.GetBoolean(dict["ImageMask"]); err == nil && bool(isImageMask) {
		return nil, pdf.Error("ImageMask must be false or absent for soft masks")
	}

	if _, hasMask := dict["Mask"]; hasMask {
		return nil, pdf.Error("Mask entry forbidden in soft masks")
	}

	if _, hasSMask := dict["SMask"]; hasSMask {
		return nil, pdf.Error("SMask entry forbidden in soft masks")
	}

	// Extract required fields
	width, err := x.GetInteger(dict["Width"])
	if err != nil {
		return nil, err
	}
	if width <= 0 {
		return nil, pdf.Errorf("invalid soft mask width %d", width)
	}

	height, err := x.GetInteger(dict["Height"])
	if err != nil {
		return nil, err
	}
	if height <= 0 {
		return nil, pdf.Errorf("invalid soft mask height %d", height)
	}

	bpc, err := x.GetInteger(dict["BitsPerComponent"])
	if err != nil {
		return nil, err
	}

	softMask := &SoftMask{
		Width:            int(width),
		Height:           int(height),
		BitsPerComponent: int(bpc),
	}

	// Extract Decode array
	if decodeArray, err := pdf.Optional(x.GetArray(dict["Decode"])); err != nil {
		return nil, err
	} else if decodeArray != nil {
		if len(decodeArray) != 2 {
			return nil, pdf.Errorf("invalid Decode array length %d for soft mask (must be 2)", len(decodeArray))
		}
		softMask.Decode = make([]float64, 2)
		for i, val := range decodeArray {
			if num, err := x.GetNumber(val); err != nil {
				return nil, fmt.Errorf("invalid Decode[%d]: %w", i, err)
			} else {
				softMask.Decode[i] = num
			}
		}
	}

	// Extract Interpolate
	if interp, err := x.GetBoolean(dict["Interpolate"]); err == nil {
		softMask.Interpolate = bool(interp)
	}

	// Extract Matte array (specific to soft masks)
	if matteArray, err := pdf.Optional(x.GetArray(dict["Matte"])); err != nil {
		return nil, err
	} else if matteArray != nil {
		softMask.Matte = make([]float64, len(matteArray))
		for i, val := range matteArray {
			if num, err := x.GetNumber(val); err != nil {
				return nil, fmt.Errorf("invalid Matte[%d]: %w", i, err)
			} else {
				softMask.Matte[i] = num
			}
		}
	}

	softMask.WriteData = func(w io.Writer) error {
		r, err := pdf.GetStreamReader(x.R, stm)
		if err != nil {
			return err
		}
		defer r.Close()

		_, err = io.Copy(w, r)
		return err
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
		WriteData: func(w io.Writer) error {
			return writeSoftMaskData(w, img, bitsPerComponent)
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
