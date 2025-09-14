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
	src := im.Data

	// Determine color space
	cs := im.ColorSpace
	if cs == nil {
		cs = color.SpaceDeviceRGB
	}

	// Create main image dict using existing Dict functionality
	dict := FromImage(src, cs, 8)

	// Add soft mask if alpha channel is needed
	if needsAlphaChannel(src) {
		dict.SMask = FromImageAlpha(src, 8)
	}

	// Let Dict handle all the PDF generation, validation, and embedding
	return pdf.ResourceManagerEmbed(rm, dict)
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
