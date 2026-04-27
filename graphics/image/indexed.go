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
	"errors"
	"fmt"

	"seehuhn.de/go/geom/rect"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics/color"
)

// PDF 2.0 sections: 8.9.5

// Indexed represents an image with an indexed color space.
type Indexed struct {
	Pix        []uint8
	Width      int
	Height     int
	ColorSpace color.Space

	// Name is the PDF resource-dictionary key under which this image is
	// referenced in content streams.  See [Dict.Name] for full semantics.
	Name pdf.Name
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
func (im *Indexed) Bounds() rect.IntRect {
	return rect.IntRect{XMax: im.Width, YMax: im.Height}
}

// Subtype returns /Image.
// This implements the [graphics.Image] interface.
func (im *Indexed) Subtype() pdf.Name {
	return "Image"
}

// ResourceName returns the preferred resource-dictionary key for this image.
// See [graphics.XObject.ResourceName].
func (im *Indexed) ResourceName() pdf.Name {
	return im.Name
}

// Embed adds the image to the PDF file.
// This implements the [graphics.Image] interface.
func (im *Indexed) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) {
	cs, ok := im.ColorSpace.(*color.SpaceIndexed)
	if !ok {
		return nil, fmt.Errorf("Indexed: invalid color space %q", im.ColorSpace.Family())
	}

	maxCol := uint8(cs.NumCol - 1)
	for _, pix := range im.Pix {
		if pix > maxCol {
			return nil, fmt.Errorf("Indexed: invalid color index %d", pix)
		}
	}

	csRef, err := rm.Embed(im.ColorSpace)
	if err != nil {
		return nil, err
	}

	imDict := pdf.Dict{
		// "Type": pdf.Name("XObject"),
		"Subtype":          pdf.Name("Image"),
		"Width":            pdf.Integer(im.Width),
		"Height":           pdf.Integer(im.Height),
		"ColorSpace":       csRef,
		"BitsPerComponent": pdf.Integer(8),
	}
	switch v := pdf.GetVersion(rm.Out()); {
	case v == pdf.V1_0 && im.Name == "":
		return nil, errors.New("missing image /Name field")
	case v >= pdf.V2_0 && im.Name != "":
		return nil, errors.New("unexpected /Name field")
	}
	if im.Name != "" {
		imDict["Name"] = im.Name
	}
	filter := pdf.FilterCompress{
		Columns:   im.Width,
		Predictor: pdf.FlatePredictorPNGOptimum,
	}
	ref := rm.Alloc()
	stream, err := rm.Out().OpenStream(ref, imDict, filter)
	if err != nil {
		return nil, err
	}
	_, err = stream.Write(im.Pix)
	if err != nil {
		return nil, err
	}
	err = stream.Close()
	if err != nil {
		return nil, err
	}

	return ref, nil
}
