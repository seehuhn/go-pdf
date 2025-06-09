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

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/metadata"
)

type Dict struct {
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

	// Data describes the image data for the image.
	// The alpha channel is ignored.
	Data image.Image
}

var _ pdf.Embedder[pdf.Unused] = (*Dict)(nil)

func (d *Dict) Embed(rm *pdf.ResourceManager) (pdf.Native, pdf.Unused, error) {
	var zero pdf.Unused

	if d.ColorSpace == nil {
		return nil, zero, errors.New("missing color space")
	}

	numChannels := d.ColorSpace.Channels()

	if fam := d.ColorSpace.Family(); fam == color.FamilyPattern {
		return nil, zero, fmt.Errorf("invalid image color space %q", fam)
	}
	switch d.BitsPerComponent {
	case 1, 2, 4, 8:
		// pass
	case 16:
		if err := pdf.CheckVersion(rm.Out, "16 bits per component", pdf.V1_5); err != nil {
			return nil, zero, err
		}
	default:
		return nil, zero, fmt.Errorf("invalid BitsPerComponent %d", d.BitsPerComponent)
	}
	if d.Intent != "" {
		if err := pdf.CheckVersion(rm.Out, "rendering intents", pdf.V1_1); err != nil {
			return nil, zero, err
		}
	}
	if d.MaskImage != nil || d.MaskColors != nil {
		if err := pdf.CheckVersion(rm.Out, "image masks", pdf.V1_3); err != nil {
			return nil, zero, err
		}
		if d.MaskImage != nil && d.MaskColors != nil {
			return nil, zero, errors.New("only one of MaskImage or MaskColors may be specified")
		}
		if d.MaskColors != nil && len(d.MaskColors) != 2*numChannels {
			return nil, zero, fmt.Errorf("wrong MaskColors length: expected %d, got %d",
				2*numChannels, len(d.MaskColors))
		}
		if d.MaskColors != nil {
			maxVal := uint16(1<<d.BitsPerComponent - 1)
			for i, v := range d.MaskColors {
				if v > maxVal {
					return nil, zero, fmt.Errorf("MaskColors[%d] value %d exceeds maximum %d", i, v, maxVal)
				}
			}
		}
	}
	if d.Decode != nil && len(d.Decode) != 2*numChannels {
		return nil, zero, fmt.Errorf("wrong Decode length: expected %d, got %d",
			2*numChannels, len(d.Decode))
	}
	if d.Alternates != nil {
		if err := pdf.CheckVersion(rm.Out, "image alternates", pdf.V1_3); err != nil {
			return nil, zero, err
		}
		for _, alt := range d.Alternates {
			if len(alt.Alternates) > 0 {
				return nil, zero, errors.New("alternates of alternates not allowed")
			}
		}
	}
	if d.Name != "" {
		v := pdf.GetVersion(rm.Out)
		if v >= pdf.V2_0 {
			return nil, zero, errors.New("unexpected /Name field")
		}
	}
	if d.Metadata != nil {
		if err := pdf.CheckVersion(rm.Out, "image metadata", pdf.V1_4); err != nil {
			return nil, zero, err
		}
	}

	dim := d.Data.Bounds()
	width := dim.Dx()
	height := dim.Dy()

	cs, _, err := d.ColorSpace.Embed(rm)
	if err != nil {
		return nil, zero, err
	}

	dict := pdf.Dict{
		"Type":             pdf.Name("XObject"),
		"Subtype":          pdf.Name("Image"),
		"Width":            pdf.Integer(width),
		"Height":           pdf.Integer(height),
		"ColorSpace":       cs,
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

	_ = dict
	panic("not implemented")
}

type ImageMask struct {
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

	// Data describes the shape of the image mask. Only the alpha channel is
	// used, and alpha values are rounded to full opacity or full transparency.
	Data image.Image
}

var _ Image = (*ImageMask)(nil)

func (m *ImageMask) Bounds() Rectangle {
	dim := m.Data.Bounds()
	return Rectangle{
		XMin: dim.Min.X,
		YMin: dim.Min.Y,
		XMax: dim.Max.X,
		YMax: dim.Max.Y,
	}
}

func (m *ImageMask) Subtype() pdf.Name {
	return pdf.Name("Image")
}

func (m *ImageMask) Embed(rm *pdf.ResourceManager) (pdf.Native, pdf.Unused, error) {
	var zero pdf.Unused
	if m.Alternates != nil {
		if err := pdf.CheckVersion(rm.Out, "image alternates", pdf.V1_3); err != nil {
			return nil, zero, err
		}

		for _, alt := range m.Alternates {
			if len(alt.Alternates) > 0 {
				return nil, zero, errors.New("alternates of alternates not allowed")
			}
		}
	}
	if m.Name != "" {
		v := pdf.GetVersion(rm.Out)
		if v >= pdf.V2_0 {
			return nil, zero, errors.New("unexpected /Name field")
		}
	}
	if m.Metadata != nil {
		if err := pdf.CheckVersion(rm.Out, "image metadata", pdf.V1_4); err != nil {
			return nil, zero, err
		}
	}

	dim := m.Data.Bounds()
	width := dim.Dx()
	height := dim.Dy()

	dict := pdf.Dict{
		"Type":      pdf.Name("XObject"),
		"Subtype":   pdf.Name("Image"),
		"Width":     pdf.Integer(width),
		"Height":    pdf.Integer(height),
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
		// pdf.FilterCCITTFax{
		// 	"Columns":    pdf.Integer(width),
		// 	"EndOfLine":  pdf.Boolean(true),
		// 	"EndOfBlock": pdf.Boolean(true),
		// },
		pdf.FilterCompress{
			"BitsPerComponent": pdf.Integer(1),
			"Columns":          pdf.Integer(width),
			"Predictor":        pdf.Integer(15),
		},
	}
	w, err := rm.Out.OpenStream(ref, dict, filters...)
	if err != nil {
		return nil, zero, fmt.Errorf("cannot open image mask stream: %w", err)
	}

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
			_, _, _, a := m.Data.At(x, y).RGBA()
			if a < 0x8000 {
				buf[x/8] |= 1 << (7 - x%8)
			}
		}

		_, err = w.Write(buf)
		if err != nil {
			return nil, zero, err
		}
	}

	err = w.Close()
	if err != nil {
		return nil, zero, err
	}
	return ref, zero, nil
}
