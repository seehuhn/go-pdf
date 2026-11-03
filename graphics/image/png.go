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
	"errors"
	"image"
	gocolor "image/color"

	"seehuhn.de/go/pdf/graphics/color"
)

// PNG creates an image dictionary for lossless storage similar to PNG format.
// The image is stored with 8 bits per component and uses PNG-style predictors
// for compression. If the image has an alpha channel, a soft mask is automatically
// created.
//
// If colorSpace is nil, DeviceRGB will be used as the default color space.
func PNG(img image.Image, colorSpace color.Space) (*Dict, error) {
	if img == nil {
		return nil, errors.New("image cannot be nil")
	}

	cs := colorSpace
	if cs == nil {
		cs = color.SpaceDeviceRGB
	}

	dict := FromImage(img, cs, 8)

	// soft mask for transparent pixels
	if needsAlphaChannel(img) {
		dict.SMask = FromImageAlpha(img, 8)
	}

	return dict, nil
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
