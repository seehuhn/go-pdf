// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2024  Jochen Voss <voss@seehuhn.de>
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

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
)

// Indexed represents an image with an indexed color space.
type Indexed struct {
	Pix        []uint8
	Width      int
	Height     int
	ColorSpace color.Space
}

// NewIndexed returns a new Indexed image of the given size.
func NewIndexed(width, height int, cs color.Space) *Indexed {
	return &Indexed{
		Pix:        make([]uint8, width*height),
		Width:      width,
		Height:     height,
		ColorSpace: cs,
	}
}

// Bounds returns the image bounds.
// This implements the [graphics.Image] interface.
func (im *Indexed) Bounds() graphics.Rectangle {
	return graphics.Rectangle{XMax: im.Width, YMax: im.Height}
}

// Subtype returns /Image.
// This implements the [graphics.Image] interface.
func (im *Indexed) Subtype() pdf.Name {
	return "Image"
}

// Embed adds the image to the PDF file.
// This implements the [graphics.Image] interface.
func (im *Indexed) Embed(rm *pdf.EmbedHelper) (pdf.Native, pdf.Unused, error) {
	var zero pdf.Unused
	cs, ok := im.ColorSpace.(*color.SpaceIndexed)
	if !ok {
		return nil, zero, fmt.Errorf("Indexed: invalid color space %q", im.ColorSpace.Family())
	}

	maxCol := uint8(cs.NumCol - 1)
	for _, pix := range im.Pix {
		if pix > maxCol {
			return nil, zero, fmt.Errorf("Indexed: invalid color index %d", pix)
		}
	}

	csRef, _, err := pdf.EmbedHelperEmbed(rm, im.ColorSpace)
	if err != nil {
		return nil, zero, err
	}

	imDict := pdf.Dict{
		// "Type": pdf.Name("XObject"),
		"Subtype":          pdf.Name("Image"),
		"Width":            pdf.Integer(im.Width),
		"Height":           pdf.Integer(im.Height),
		"ColorSpace":       csRef,
		"BitsPerComponent": pdf.Integer(8),
	}
	filter := pdf.FilterCompress{
		"Columns":   pdf.Integer(im.Width),
		"Predictor": pdf.Integer(15),
	}
	ref := rm.Alloc()
	stream, err := rm.Out().OpenStream(ref, imDict, filter)
	if err != nil {
		return nil, zero, err
	}
	_, err = stream.Write(im.Pix)
	if err != nil {
		return nil, zero, err
	}
	err = stream.Close()
	if err != nil {
		return nil, zero, err
	}

	return ref, zero, nil
}
