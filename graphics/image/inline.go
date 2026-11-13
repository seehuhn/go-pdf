// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package image

import (
	"io"
	"slices"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/internal/imagedict"
)

// PDF 2.0 sections: 8.9.7

// ExtractInline reads the inline image of a BI … ID … EI operator.  res is
// the resource dictionary of the surrounding content stream, used to resolve
// a named colour space.  The result is a [*Mask] if the image is an image
// mask, and a [*Dict] otherwise.
//
// The entries of an inline image are read with the same rules as those of an
// image XObject.  The image data is decoded immediately, and the result does
// not refer back to the content stream, so it can also be embedded as an
// image XObject.
func ExtractInline(op content.Operator, res *content.Resources) (graphics.Image, error) {
	data, err := content.DecodeInlineImage(op, res)
	if err != nil {
		return nil, err
	}
	// DecodeInlineImage has checked the arguments, and has checked the
	// entries the image data depends on with the rules applied below
	dict := imagedict.ExpandInline(op.Args[0].(pdf.Dict))
	c := pdf.NewCursor(nil)

	width, height, err := imagedict.ReadDimensions(c, dict)
	if err != nil {
		return nil, err
	}

	if content.IsInlineImageMask(dict) {
		m := &Mask{Width: width, Height: height}
		if err := m.readMaskEntries(c, dict); err != nil {
			return nil, err
		}
		m.Source = NewCCITTFaxSource(width, sampleWriter(data, width, 1, 1, height))
		return m, nil
	}

	// DecodeInlineImage has rejected a missing or Pattern colour space
	cs := content.InlineImageColorSpace(dict, res)
	filters, err := c.Filters(dict)
	if err != nil {
		return nil, err
	}
	img := &Dict{Width: width, Height: height, ColorSpace: cs}
	if err := img.readSampleEntries(c, dict, filters, false); err != nil {
		return nil, err
	}
	img.Data = NewFlateSource(width, cs, img.BitsPerComponent,
		sampleWriter(data, width, cs.Channels(), img.BitsPerComponent, height))
	return img, nil
}

// sampleWriter returns a function which writes the decoded samples of an
// inline image.  The data is first brought to the size the image dimensions
// imply, as [Dict.Load] does, and the padding bits at the end of each row are
// cleared.  Neither change alters a decoded sample, and the CCITTFax encoder
// used for image masks rejects rows with padding bits set.
func sampleWriter(data []byte, width, channels, bpc, height int) func(io.Writer) error {
	bitsPerRow := width * channels * bpc
	bytesPerRow := (bitsPerRow + 7) / 8
	norm := normalizeData(slices.Clone(data), bytesPerRow*height)
	if pad := bitsPerRow % 8; pad != 0 {
		mask := byte(0xFF << (8 - pad))
		for i := bytesPerRow - 1; i < len(norm); i += bytesPerRow {
			norm[i] &= mask
		}
	}
	return func(w io.Writer) error {
		_, err := w.Write(norm)
		return err
	}
}
