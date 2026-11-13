// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package builder

import (
	"errors"
	"fmt"
	"maps"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/internal/imagedict"
)

// DrawXObject draws a PDF XObject on the page.
//
// This implements the PDF graphics operator "Do".
func (b *Builder) DrawXObject(obj graphics.XObject) {
	if b.Err != nil {
		return
	}
	// In uncolored patterns and Type 3 glyphs with d1, images are forbidden
	// but image masks are allowed.
	if b.State.ColorOpsForbidden && obj.Subtype() == "Image" && !graphics.IsImageMask(obj) {
		b.Err = ErrColorForbidden
		return
	}
	name := b.getXObjectName(obj)
	b.emit(content.OpXObject, name)
}

// DrawInlineImageRaw embeds a small image directly in the content stream.
//
// The dict should contain the image parameters using the abbreviated
// inline image keys (W, H, BPC, CS, etc.) as defined in PDF 2.0, Table 91.
// The data contains the (possibly compressed) image samples.
//
// The dict must give Width and Height as positive integers.  An image mask
// must not give a ColorSpace, and its BitsPerComponent, if given, must be 1.
// Other images need a ColorSpace other than Pattern: a device colour space,
// an Indexed array over one, or the name of a colour space in the builder's
// resources.  They also need a BitsPerComponent of 1, 2, 4, 8 or 16, unless
// the filter fixes the sample size (CCITTFaxDecode, DCTDecode); a given entry
// must agree with that size.
//
// For PDF 2.0, the Length entry is added if dict does not give it.  Before
// PDF 2.0, the data must not contain a white-space byte followed by "EI" and
// a delimiter, since readers would take this as the end of the data; the
// ASCIIHexDecode filter avoids this.
//
// Inline images should be used only for small images of up to 4096 bytes;
// for larger images use [Builder.DrawXObject] instead.
func (b *Builder) DrawInlineImageRaw(dict pdf.Dict, data []byte) {
	if b.Err != nil {
		return
	}
	if b.State.ColorOpsForbidden && !content.IsInlineImageMask(dict) {
		b.Err = ErrColorForbidden
		return
	}
	if err := content.ValidateInlineImageFilter(dict); err != nil {
		b.Err = err
		return
	}
	if err := checkInlineDict(dict, b.Resources); err != nil {
		b.Err = err
		return
	}
	dict, err := inlineLength(dict, data, b.version)
	if err != nil {
		b.Err = err
		return
	}
	b.emit(content.OpInlineImage, dict, pdf.String(data))
}

// inlineLength checks the Length entry of an inline image dictionary against
// the image data.  PDF 2.0 requires the entry; where it is missing, it is
// added to a copy of dict.  Before PDF 2.0, readers find the end of the data
// by searching for EI, so data which contains that pattern is rejected.
func inlineLength(dict pdf.Dict, data []byte, v pdf.Version) (pdf.Dict, error) {
	key, hasLength := imagedict.InlineKey(dict, "Length")
	if hasLength {
		obj := dict[key]
		if l, ok := obj.(pdf.Integer); !ok || int64(l) != int64(len(data)) {
			return nil, fmt.Errorf("inline image Length %v, but %d data bytes", obj, len(data))
		}
	}
	if v >= pdf.V2_0 {
		if !hasLength {
			dict = maps.Clone(dict)
			dict["L"] = pdf.Integer(len(data))
		}
	} else if content.ContainsInlineImageEnd(data) {
		return nil, errors.New("inline image data contains an EI marker")
	}
	return dict, nil
}

// checkInlineDict checks the required entries of an inline image dictionary.
// Width and Height must be positive integers.  An image mask must not have a
// ColorSpace entry, and its BitsPerComponent entry, if present, must be 1.
// Other images need a colour space other than Pattern, given inline or as the
// name of a colour space in res, and a BitsPerComponent entry of 1, 2, 4, 8 or
// 16.  The entry may be omitted where the filter fixes the sample size, and if
// present it must agree with that size.
func checkInlineDict(dict pdf.Dict, res *content.Resources) error {
	full := imagedict.ExpandInline(dict)
	for _, key := range []pdf.Name{"Width", "Height"} {
		obj, ok := full[key]
		if !ok {
			return fmt.Errorf("missing inline image %s", key)
		}
		if v, ok := obj.(pdf.Integer); !ok || v <= 0 {
			return fmt.Errorf("invalid inline image %s %v", key, obj)
		}
	}

	bpcObj, hasBPC := full["BitsPerComponent"]
	bpc, isInt := bpcObj.(pdf.Integer)
	if hasBPC && !isInt {
		return fmt.Errorf("invalid inline image BitsPerComponent %v", bpcObj)
	}

	if content.IsInlineImageMask(full) {
		if _, ok := full["ColorSpace"]; ok {
			return errors.New("ColorSpace not permitted for an inline image mask")
		}
		if hasBPC && bpc != 1 {
			return fmt.Errorf("inline image mask BitsPerComponent %d, must be 1", bpc)
		}
		return nil
	}

	cs := content.InlineImageColorSpace(full, res)
	if cs == nil {
		return errors.New("missing or invalid inline image ColorSpace")
	}
	if cs.Family() == color.FamilyPattern {
		return errors.New("inline image colour space must not be Pattern")
	}

	filters, err := pdf.NewCursor(nil).Filters(full)
	if err != nil {
		return err
	}
	fixed, isFixed := graphics.FilterSampleBits(filters)
	if !hasBPC {
		if isFixed {
			return nil
		}
		return errors.New("missing inline image BitsPerComponent")
	}
	if _, valid := imagedict.SampleBits(nil, bpc); !valid {
		return fmt.Errorf("invalid inline image BitsPerComponent %d", bpc)
	}
	if isFixed && int(bpc) != fixed {
		return fmt.Errorf("inline image BitsPerComponent %d, filter delivers %d", bpc, fixed)
	}
	return nil
}
