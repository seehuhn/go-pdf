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
	gocol "image/color"
	"io"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/metadata"
)

type Dict struct {
	// Width is the width of the image in pixels.
	Width int

	// Height is the height of the image in pixels.
	Height int

	// ColorSpace is the color space in which image samples are specified.
	// It can be any type of color space except Pattern.
	ColorSpace color.Space

	// BitsPerComponent is the number of bits used to represent each color component.
	// The value must be 1, 2, 4, 8, or (from PDF 1.5) 16.
	BitsPerComponent int

	// Intent (optional) is the name of a color rendering intent to be used in rendering the image.
	Intent graphics.RenderingIntent

	// MaskImage (optional) determines which parts of the image are to be
	// painted.
	//
	// Only one of MaskImage or MaskColors may be specified.
	MaskImage *ImageMask

	// MaskColors (optional) is an array of colors used for color key masking.
	// When specified, image samples with colors falling within the defined ranges
	// will not be painted, allowing the background to show through (similar to
	// chroma-key/green screen effects).
	//
	// The array contains pairs of min/max values for each color component:
	// [min1, max1, min2, max2, ..., minN, maxN] where N is the number of color
	// components in the image's color space. Each value must be in the range
	// 0 to (2^BitsPerComponent - 1) and represents raw color values before
	// any Decode array processing.
	//
	// A pixel is masked if ALL of its color components fall within their
	// respective min/max ranges.
	//
	// Only one of MaskImage or MaskColors may be specified.
	MaskColors []uint16

	// Decode (optional) is an array of numbers describing how to map image
	// samples into the range of values appropriate for the image's color
	// space. The slice must have twice the number of color components
	// required by ColorSpace.
	Decode []float64

	// Interpolate indicates whether image interpolation should be performed by
	// a PDF processor.
	Interpolate bool

	// Alternates (optional) is an array of alternate image dictionaries for this image.
	Alternates []*Dict

	// TODO(voss): SMask
	// TODO(voss): SMaskInData

	// Name is deprecated and should be left empty.
	// Only used in PDF 1.0 where it was the name used to reference the image
	// mask from within content streams.
	Name pdf.Name

	// TODO(voss): StructParent
	// TODO(voss): ID
	// TODO(voss): OPI

	// Metadata (optional) is a metadata stream containing metadata for the image.
	Metadata *metadata.Stream

	// TODO(voss): OC
	// TODO(voss): AF
	// TODO(voss): Measure
	// TODO(voss): PtData

	// WriteData is a function that writes the image data to the provided writer.
	// The data should be written row by row, with each row containing
	// Width * ColorSpace.Channels() samples, each sample using BitsPerComponent bits.
	WriteData func(io.Writer) error
}

var _ Image = (*Dict)(nil)

// Bounds returns the dimensions of the image.
func (d *Dict) Bounds() Rectangle {
	return Rectangle{
		XMin: 0,
		YMin: 0,
		XMax: d.Width,
		YMax: d.Height,
	}
}

// Subtype returns the PDF XObject subtype for images.
func (d *Dict) Subtype() pdf.Name {
	return pdf.Name("Image")
}

// FromImage creates a Dict from an image.Image.
// The ColorSpace and BitsPerComponent must be set appropriately for the image.
func FromImage(img image.Image, colorSpace color.Space, bitsPerComponent int) *Dict {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	return &Dict{
		Width:            width,
		Height:           height,
		ColorSpace:       colorSpace,
		BitsPerComponent: bitsPerComponent,
		WriteData: func(w io.Writer) error {
			return writeImageData(w, img, colorSpace, bitsPerComponent)
		},
	}
}

// FromImageMask creates an ImageMask from an image.Image.
// Only the alpha channel is used, with alpha values rounded to full opacity or full transparency.
func FromImageMask(img image.Image) *ImageMask {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	return &ImageMask{
		Width:  width,
		Height: height,
		WriteData: func(w io.Writer) error {
			return writeImageMaskData(w, img)
		},
	}
}

// FromImageWithMask creates a Dict with an associated ImageMask from two image.Image objects.
func FromImageWithMask(img image.Image, mask image.Image, colorSpace color.Space, bitsPerComponent int) *Dict {
	dict := FromImage(img, colorSpace, bitsPerComponent)
	dict.MaskImage = FromImageMask(mask)
	return dict
}

func (d *Dict) Embed(rm *pdf.ResourceManager) (pdf.Native, pdf.Unused, error) {
	var zero pdf.Unused

	if err := d.check(rm.Out); err != nil {
		return nil, zero, err
	}

	csEmbedded, _, err := pdf.ResourceManagerEmbed(rm, d.ColorSpace)
	if err != nil {
		return nil, zero, err
	}

	dict := pdf.Dict{
		"Type":             pdf.Name("XObject"),
		"Subtype":          pdf.Name("Image"),
		"Width":            pdf.Integer(d.Width),
		"Height":           pdf.Integer(d.Height),
		"ColorSpace":       csEmbedded,
		"BitsPerComponent": pdf.Integer(d.BitsPerComponent),
	}
	if d.Intent != "" {
		dict["Intent"] = pdf.Name(d.Intent)
	}
	if d.MaskImage != nil {
		ref, _, err := d.MaskImage.Embed(rm)
		if err != nil {
			return nil, zero, err
		}
		dict["Mask"] = ref
	} else if d.MaskColors != nil {
		var mask pdf.Array
		for _, v := range d.MaskColors {
			mask = append(mask, pdf.Integer(v))
		}
		dict["Mask"] = mask
	}
	if d.Decode != nil {
		var decode pdf.Array
		for _, v := range d.Decode {
			decode = append(decode, pdf.Number(v))
		}
		dict["Decode"] = decode
	}
	if d.Interpolate {
		dict["Interpolate"] = pdf.Boolean(true)
	}
	if len(d.Alternates) > 0 {
		var alts pdf.Array
		for _, alt := range d.Alternates {
			ref, _, err := alt.Embed(rm)
			if err != nil {
				return nil, zero, err
			}
			alts = append(alts, ref)
		}
		dict["Alternates"] = alts
	}
	if d.Name != "" {
		dict["Name"] = d.Name
	}
	if d.Metadata != nil {
		ref, _, err := d.Metadata.Embed(rm)
		if err != nil {
			return nil, zero, err
		}
		dict["Metadata"] = ref
	}

	ref := rm.Out.Alloc()
	compress := pdf.FilterCompress{
		"Predictor":        pdf.Integer(15), // TODO(voss): check that this is a good choice
		"Colors":           pdf.Integer(d.ColorSpace.Channels()),
		"BitsPerComponent": pdf.Integer(d.BitsPerComponent),
		"Columns":          pdf.Integer(d.Width),
	}
	w, err := rm.Out.OpenStream(ref, dict, compress)
	if err != nil {
		return nil, zero, fmt.Errorf("cannot open image stream: %w", err)
	}

	err = d.WriteData(w)
	if err != nil {
		return nil, zero, err
	}

	err = w.Close()
	if err != nil {
		return nil, zero, err
	}
	return ref, zero, nil
}

func (d *Dict) check(out *pdf.Writer) error {
	if d.Width <= 0 {
		return fmt.Errorf("invalid image width %d", d.Width)
	}
	if d.Height <= 0 {
		return fmt.Errorf("invalid image height %d", d.Height)
	}
	if d.WriteData == nil {
		return errors.New("WriteData function cannot be nil")
	}

	if fam := d.ColorSpace.Family(); fam == color.FamilyPattern {
		return fmt.Errorf("invalid image color space %q", fam)
	}

	switch d.BitsPerComponent {
	case 1, 2, 4, 8:
		// pass
	case 16:
		if err := pdf.CheckVersion(out, "16 bits per image component", pdf.V1_5); err != nil {
			return err
		}
	default:
		return fmt.Errorf("invalid BitsPerComponent %d", d.BitsPerComponent)
	}

	if d.Intent != "" {
		if err := pdf.CheckVersion(out, "rendering intents", pdf.V1_1); err != nil {
			return err
		}
	}

	numChannels := d.ColorSpace.Channels()
	if d.MaskImage != nil || d.MaskColors != nil {
		if err := pdf.CheckVersion(out, "image masks", pdf.V1_3); err != nil {
			return err
		}
		if d.MaskImage != nil && d.MaskColors != nil {
			return errors.New("only one of MaskImage or MaskColors may be specified")
		}
		if d.MaskColors != nil && len(d.MaskColors) != 2*numChannels {
			return fmt.Errorf("wrong MaskColors length: expected %d, got %d",
				2*numChannels, len(d.MaskColors))
		}
		if d.MaskColors != nil {
			maxVal := uint16(1<<d.BitsPerComponent - 1)
			for i, v := range d.MaskColors {
				if v > maxVal {
					return fmt.Errorf("MaskColors[%d] value %d exceeds maximum %d", i, v, maxVal)
				}
			}
		}
	}
	if d.Decode != nil && len(d.Decode) != 2*numChannels {
		return fmt.Errorf("wrong Decode length: expected %d, got %d",
			2*numChannels, len(d.Decode))
	}

	if d.Alternates != nil {
		if err := pdf.CheckVersion(out, "image alternates", pdf.V1_3); err != nil {
			return err
		}
		for _, alt := range d.Alternates {
			if len(alt.Alternates) > 0 {
				return errors.New("alternates of alternates not allowed")
			}
		}
	}

	if d.Name != "" {
		v := pdf.GetVersion(out)
		if v >= pdf.V2_0 {
			return errors.New("unexpected /Name field")
		}
	}

	if d.Metadata != nil {
		if err := pdf.CheckVersion(out, "image metadata", pdf.V1_4); err != nil {
			return err
		}
	}

	return nil
}

func rgbToCMYK(r, g, b uint16) (c, m, y, k uint16) {
	maxVal := max(r, g, b)

	if maxVal == 0 {
		return 0, 0, 0, 0xffff
	}

	k = 0xffff - maxVal

	c = uint16((uint32(maxVal-r) * 0xffff) / uint32(maxVal))
	m = uint16((uint32(maxVal-g) * 0xffff) / uint32(maxVal))
	y = uint16((uint32(maxVal-b) * 0xffff) / uint32(maxVal))

	return c, m, y, k
}

// writeImageData writes image data from an image.Image to the provided writer.
func writeImageData(w io.Writer, img image.Image, colorSpace color.Space, bitsPerComponent int) error {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	numChannels := colorSpace.Channels()

	buf := NewPixelRow(width*numChannels, bitsPerComponent)
	for y := range height {
		buf.Reset()

		for x := range width {
			pixCol := img.At(bounds.Min.X+x, bounds.Min.Y+y)
			shift := 16 - bitsPerComponent
			switch colorSpace.Family() {
			case color.FamilyDeviceGray, color.FamilyCalGray:
				g16 := gocol.Gray16Model.Convert(pixCol).(gocol.Gray16).Y
				buf.AppendBits(g16 >> shift)

			case color.FamilyDeviceRGB, color.FamilyCalRGB:
				c16 := gocol.NRGBA64Model.Convert(pixCol).(gocol.NRGBA64)
				buf.AppendBits(c16.R >> shift)
				buf.AppendBits(c16.G >> shift)
				buf.AppendBits(c16.B >> shift)

			case color.FamilyDeviceCMYK:
				c16 := gocol.NRGBA64Model.Convert(pixCol).(gocol.NRGBA64)
				c, m, y, k := rgbToCMYK(c16.R, c16.G, c16.B)
				buf.AppendBits(c >> shift)
				buf.AppendBits(m >> shift)
				buf.AppendBits(y >> shift)
				buf.AppendBits(k >> shift)

			// TODO(voss): implement the remaining color spaces
			case color.FamilyLab:
				return errors.New("Lab color space not implemented")
			case color.FamilyICCBased:
				return errors.New("ICCBased color space not implemented")
			case color.FamilyIndexed:
				return errors.New("Indexed color space not implemented")
			case color.FamilySeparation:
				return errors.New("Separation color space not implemented")
			case color.FamilyDeviceN:
				return errors.New("DeviceN color space not implemented")
			}
		}

		_, err := w.Write(buf.Bytes())
		if err != nil {
			return err
		}
	}
	return nil
}

// writeImageMaskData writes mask data from an image.Image to the provided writer.
func writeImageMaskData(w io.Writer, img image.Image) error {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	// Mask data is encoded as a continuous bit stream, with the high-order bit
	// of each byte first. Each row starts at a new byte boundary.
	// 0 = opaque, 1 = transparent
	rowBytes := (width + 7) / 8
	buf := make([]byte, rowBytes)
	for y := range height {
		for i := range buf {
			buf[i] = 0
		}

		for x := range width {
			_, _, _, a := img.At(bounds.Min.X+x, bounds.Min.Y+y).RGBA()
			if a < 0x8000 {
				buf[x/8] |= 1 << (7 - x%8)
			}
		}

		_, err := w.Write(buf)
		if err != nil {
			return err
		}
	}
	return nil
}

type ImageMask struct {
	// Width is the width of the image mask in pixels.
	Width int

	// Height is the height of the image mask in pixels.
	Height int

	// WriteData is a function that writes the mask data to the provided writer.
	// The data should be written as a continuous bit stream, with each row
	// starting at a new byte boundary. 0 = opaque, 1 = transparent.
	WriteData func(io.Writer) error

	// Interpolate enables edge smoothing for the mask to reduce jagged
	// appearance in low-resolution stencil masks.
	Interpolate bool

	// Alternates (optional) is an array of alternate image dictionaries for this image.
	Alternates []*ImageMask

	// TODO(voss): SMask
	// TODO(voss): SMaskInData

	// Name is deprecated and should be left empty.
	// Only used in PDF 1.0 where it was the name used to reference the image
	// from within content streams.
	Name pdf.Name

	// TODO(voss): StructParent
	// TODO(voss): ID
	// TODO(voss): OPI

	// Metadata (optional) is a metadata stream containing metadata for the image.
	Metadata *metadata.Stream

	// TODO(voss): OC
	// TODO(voss): AF
	// TODO(voss): Measure
	// TODO(voss): PtData
}

var _ Image = (*ImageMask)(nil)

func (m *ImageMask) Bounds() Rectangle {
	return Rectangle{
		XMin: 0,
		YMin: 0,
		XMax: m.Width,
		YMax: m.Height,
	}
}

func (m *ImageMask) Subtype() pdf.Name {
	return pdf.Name("Image")
}

func (m *ImageMask) Embed(rm *pdf.ResourceManager) (pdf.Native, pdf.Unused, error) {
	var zero pdf.Unused

	if err := m.check(rm.Out); err != nil {
		return nil, zero, err
	}

	dict := pdf.Dict{
		"Type":      pdf.Name("XObject"),
		"Subtype":   pdf.Name("Image"),
		"Width":     pdf.Integer(m.Width),
		"Height":    pdf.Integer(m.Height),
		"ImageMask": pdf.Boolean(true),
	}
	if m.Interpolate {
		dict["Interpolate"] = pdf.Boolean(true)
	}
	if len(m.Alternates) > 0 {
		var alts pdf.Array
		for _, alt := range m.Alternates {
			ref, _, err := alt.Embed(rm)
			if err != nil {
				return nil, zero, err
			}
			alts = append(alts, ref)
		}
		dict["Alternates"] = alts
	}
	if m.Name != "" {
		dict["Name"] = m.Name
	}
	if m.Metadata != nil {
		ref, _, err := m.Metadata.Embed(rm)
		if err != nil {
			return nil, zero, err
		}
		dict["Metadata"] = ref
	}

	ref := rm.Out.Alloc()
	filters := []pdf.Filter{
		pdf.FilterCCITTFax{
			"Columns": pdf.Integer(m.Width),
			"K":       pdf.Integer(-1),
		},
	}
	w, err := rm.Out.OpenStream(ref, dict, filters...)
	if err != nil {
		return nil, zero, fmt.Errorf("cannot open image mask stream: %w", err)
	}

	err = m.WriteData(w)
	if err != nil {
		return nil, zero, err
	}

	err = w.Close()
	if err != nil {
		return nil, zero, err
	}
	return ref, zero, nil
}

func (m *ImageMask) check(out *pdf.Writer) error {
	if m.Width <= 0 {
		return fmt.Errorf("invalid image mask width %d", m.Width)
	}
	if m.Height <= 0 {
		return fmt.Errorf("invalid image mask height %d", m.Height)
	}
	if m.WriteData == nil {
		return errors.New("WriteData function cannot be nil")
	}

	if m.Alternates != nil {
		if err := pdf.CheckVersion(out, "image alternates", pdf.V1_3); err != nil {
			return err
		}

		for _, alt := range m.Alternates {
			if len(alt.Alternates) > 0 {
				return errors.New("alternates of alternates not allowed")
			}
		}
	}
	if m.Name != "" {
		v := pdf.GetVersion(out)
		if v >= pdf.V2_0 {
			return errors.New("unexpected /Name field")
		}
	}
	if m.Metadata != nil {
		if err := pdf.CheckVersion(out, "image metadata", pdf.V1_4); err != nil {
			return err
		}
	}

	return nil
}
