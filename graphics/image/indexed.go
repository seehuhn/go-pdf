// Copyright 2024 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package image

import (
	"fmt"
	"io"

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

// ResourceName returns the empty string: Indexed does not expose a Name
// field.  Callers who need a specific resource-dict key should wrap the image
// in a [Dict] and set its Name field.  See [graphics.XObject.ResourceName].
func (im *Indexed) ResourceName() pdf.Name {
	return ""
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

	// The image dictionary owns the choice of stream filter, so it is left to
	// it rather than repeated here.
	dict := &Dict{
		Width:            im.Width,
		Height:           im.Height,
		ColorSpace:       im.ColorSpace,
		BitsPerComponent: 8,
		Data: NewFlateSource(im.Width, im.ColorSpace, 8,
			func(w io.Writer) error {
				_, err := w.Write(im.Pix)
				return err
			}),
	}
	return dict.Embed(rm)
}
