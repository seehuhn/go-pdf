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
	// ColorSpace is the colour space in which image samples are specified.
	// It can be any type of colour space except Pattern.
	// This is required for images, but not permitted for image masks.
	ColorSpace color.Space

	// BitsPerComponent is the number of bits used to represent each colour component.
	// The value must be 1, 2, 4, 8, or (from PDF 1.5) 16.
	// If ImageMask is true, this must be 1.
	BitsPerComponent int

	// Intent (optional) is the name of a colour rendering intent to be used in rendering the image.
	// If ImageMask is true this must be the empty string.
	Intent graphics.RenderingIntent

	// IsImageMask indicates whether the image is treated as an image mask. If
	// this flag is true, the image describes a shape to be painted using the
	// current nonstroking colour.
	IsImageMask bool

	// MaskImage (optional) is an image mask which determines which parts of
	// the image are to be painted.
	//
	// Only one of MaskImage or MaskColor may be specified.
	MaskImage *Dict

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
	MaskColors []uint32

	// Decode (optional) is an array of numbers describing how to map image
	// samples into the range of values appropriate for the image's colour
	// space. If ImageMask is true, the array must be either [0 1] or [1 0];
	// otherwise, its length must be twice the number of colour components
	// required by ColorSpace.
	Decode []float64

	// Interpolate (optional) indicates whether image interpolation should be
	// performed by a PDF processor.
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

	// ID is the digital identifier of the image's parent Web Capture content set.
	// An indirect reference is preferred.
	// This is optional (PDF 1.3).
	// ID []byte

	// TODO(voss): OPI

	// Metadata (optional) is a metadata stream containing metadata for the image.
	Metadata *metadata.Stream

	// TODO(voss): OC
	// TODO(voss): AF
	// TODO(voss): Measure
	// TODO(voss): PtData

	Data image.Image
}

var _ pdf.Embedder[pdf.Unused] = (*Dict)(nil)

func (d *Dict) Embed(rm *pdf.ResourceManager) (pdf.Native, pdf.Unused, error) {
	var zero pdf.Unused

	if d.IsImageMask {
		if d.ColorSpace != nil {
			return nil, zero, errors.New("color space not allowed for image mask")
		}
		if d.BitsPerComponent != 1 {
			return nil, zero, fmt.Errorf("invalid BitsPerComponent %d for image mask", d.BitsPerComponent)
		}
		if d.Intent != "" {
			return nil, zero, errors.New("rendering intent not allowed for image mask")
		}
		if d.MaskImage != nil {
			return nil, zero, errors.New("MaskImage not allowed for image mask")
		}
		if len(d.MaskColors) > 0 {
			return nil, zero, errors.New("MaskColor not allowed for image mask")
		}
	} else {
		if d.ColorSpace == nil {
			return nil, zero, errors.New("missing color space")
		}

		numChannels := d.ColorSpace.Channels()

		if fam := d.ColorSpace.Family(); fam == color.FamilyPattern {
			return nil, zero, fmt.Errorf("invalid image color space %q", fam)
		}
		if d.Intent != "" {
			if err := pdf.CheckVersion(rm.Out, "rendering intents", pdf.V1_1); err != nil {
				return nil, zero, err
			}
		}
		if d.MaskImage != nil && d.MaskColors != nil {
			return nil, zero, errors.New("only one of MaskImage or MaskColor may be specified")
		}
		if d.MaskImage != nil && !d.MaskImage.IsImageMask {
			return nil, zero, errors.New("MaskImage must be an image mask")
		}
		if d.MaskColors != nil && len(d.MaskColors) != 2*numChannels {
			return nil, zero, errors.New("odd MaskColor length")
		}
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

	dim := d.Data.Bounds()
	width := dim.Dx()
	height := dim.Dy()

	cs, _, err := d.ColorSpace.Embed(rm)
	if err != nil {
		return nil, zero, err
	}

	dict := pdf.Dict{
		"Type":       pdf.Name("XObject"),
		"Subtype":    pdf.Name("Image"),
		"Width":      pdf.Integer(width),
		"Height":     pdf.Integer(height),
		"ColorSpace": cs,
	}
	if !d.IsImageMask {
		dict["BitsPerComponent"] = pdf.Integer(d.BitsPerComponent)
	}
	if d.Intent != "" {
		dict["Intent"] = pdf.Name(d.Intent)
	}
	if d.IsImageMask {
		dict["ImageMask"] = pdf.Boolean(true)
	}

	_ = dict
	panic("not implemented")
}
