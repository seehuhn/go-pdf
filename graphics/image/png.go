// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2023  Jochen Voss <voss@seehuhn.de>
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
	"fmt"
	"image"
	gocolor "image/color"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics/color"
)

// PNG represents an image which is stored losslessly in the PDF file.
// The encoding is similar to the PNG format.
type PNG struct {
	Data image.Image

	// ColorSpace is the color space of the image.
	// If this is not set, the image will be embedded as a DeviceRGB image.
	ColorSpace color.Space
}

// Subtype returns /Image.
// This implements the [Image] interface.
func (im *PNG) Subtype() pdf.Name {
	return pdf.Name("Image")
}

// Bounds implements the [Image] interface.
func (im *PNG) Bounds() Rectangle {
	b := im.Data.Bounds()
	return Rectangle{XMin: b.Min.X, YMin: b.Min.Y, XMax: b.Max.X, YMax: b.Max.Y}
}

// Embed ensures that the image is embedded in the PDF file.
// This implements the [Image] interface.
func (im *PNG) Embed(rm *pdf.ResourceManager) (pdf.Native, pdf.Unused, error) {
	var zero pdf.Unused
	ref := rm.Out.Alloc()
	src := im.Data

	width := src.Bounds().Dx()
	height := src.Bounds().Dy()

	var maskRef pdf.Reference
	if needsAlphaChannel(src) {
		maskRef = rm.Out.Alloc()
	}

	cs := im.ColorSpace
	if cs == nil {
		cs = color.DeviceRGBSpace
	}
	if cs.Channels() != 3 {
		return nil, zero, fmt.Errorf("unsupported color space: %v", cs.Family())
	}
	csEmbedded, _, err := pdf.ResourceManagerEmbed(rm, cs)
	if err != nil {
		return nil, zero, err
	}

	// see Table 87 of ISO 32000-2:2020
	imDict := pdf.Dict{
		"Type":             pdf.Name("XObject"),
		"Subtype":          pdf.Name("Image"),
		"Width":            pdf.Integer(width),
		"Height":           pdf.Integer(height),
		"ColorSpace":       csEmbedded,
		"BitsPerComponent": pdf.Integer(8),
	}
	var alpha []byte
	if maskRef != 0 {
		imDict["SMask"] = maskRef
		alpha = make([]byte, 0, width*height)
	}

	// write the image data
	// (and gather the alpha values at the same time)
	filter := pdf.FilterCompress{
		"Columns":   pdf.Integer(width),
		"Colors":    pdf.Integer(3),
		"Predictor": pdf.Integer(15),
	}
	stream, err := rm.Out.OpenStream(ref, imDict, filter)
	if err != nil {
		return nil, zero, err
	}
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			r, g, b, a := src.At(x, y).RGBA()
			_, err = stream.Write([]byte{byte(r >> 8), byte(g >> 8), byte(b >> 8)})
			if err != nil {
				return nil, zero, err
			}
			if alpha != nil {
				alpha = append(alpha, byte(a>>8))
			}
		}
	}
	err = stream.Close()
	if err != nil {
		return nil, zero, err
	}

	if maskRef != 0 {
		maskCS := color.DeviceGraySpace
		maskCSEmbedded, _, err := pdf.ResourceManagerEmbed(rm, maskCS)
		if err != nil {
			return nil, zero, err
		}

		maskDict := pdf.Dict{
			"Type":             pdf.Name("XObject"),
			"Subtype":          pdf.Name("Image"),
			"Width":            pdf.Integer(width),
			"Height":           pdf.Integer(height),
			"ColorSpace":       maskCSEmbedded,
			"BitsPerComponent": pdf.Integer(8),
		}

		// TODO(voss): is there a more appropriate compression type for the mask?
		filter = pdf.FilterCompress{
			"Columns":   pdf.Integer(width),
			"Predictor": pdf.Integer(15),
		}
		stream, err = rm.Out.OpenStream(maskRef, maskDict, filter)
		if err != nil {
			return nil, zero, err
		}
		_, err = stream.Write(alpha)
		if err != nil {
			return nil, zero, err
		}
		err = stream.Close()
		if err != nil {
			return nil, zero, err
		}
	}

	return ref, zero, nil
}

func needsAlphaChannel(img image.Image) bool {
	switch img.ColorModel() {
	case gocolor.GrayModel, gocolor.Gray16Model, gocolor.CMYKModel, gocolor.YCbCrModel:
		return false

	case gocolor.AlphaModel, gocolor.Alpha16Model:
		return true

	default:
		// check all pixels to see whether the alpha channel is actually used
		bounds := img.Bounds()
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				_, _, _, a := img.At(x, y).RGBA()
				if a != 0xffff { // not fully opaque
					return true
				}
			}
		}
		return false
	}
}
