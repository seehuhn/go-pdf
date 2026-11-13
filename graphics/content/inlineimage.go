// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package content

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"slices"

	"seehuhn.de/go/membudget"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/internal/imagedict"
	"seehuhn.de/go/pdf/internal/limits"
)

// inlineFilterForbidden lists filters that PDF 2.0 §8.9.7 prohibits in inline
// images.  None of these filters has an abbreviated name.
var inlineFilterForbidden = map[pdf.Name]bool{
	"JBIG2Decode": true,
	"JPXDecode":   true,
	"Crypt":       true,
}

// ValidateInlineImageFilter reports an error if the Filter (or F) entry of an
// inline image dict names a filter that PDF 2.0 §8.9.7 prohibits in inline
// images: JBIG2Decode, JPXDecode, or Crypt.  Other entries of the dict are
// not inspected.
func ValidateInlineImageFilter(dict pdf.Dict) error {
	switch f := imagedict.InlineValue(dict, "Filter").(type) {
	case pdf.Name:
		if inlineFilterForbidden[f] {
			return fmt.Errorf("filter %s not permitted in inline image", f)
		}
	case pdf.Array:
		for _, elem := range f {
			name, ok := elem.(pdf.Name)
			if ok && inlineFilterForbidden[name] {
				return fmt.Errorf("filter %s not permitted in inline image", name)
			}
		}
	}
	return nil
}

// DecodeInlineImage decompresses the image data from an inline image operator.
// The operator must have name [OpInlineImage] with two arguments:
// a [pdf.Dict] containing image parameters and a [pdf.String] holding the
// raw image data.  res is the parsed resource dictionary of the surrounding
// content stream; it is used to resolve named colour spaces.
//
// Before any data is decoded, the entries the image data depends on are
// checked with the same rules as for image XObjects: Width and Height, and
// for an image which is not an image mask, ColorSpace and BitsPerComponent.
// An image whose filter fixes the sample size needs no BitsPerComponent
// entry.
func DecodeInlineImage(op Operator, res *Resources) ([]byte, error) {
	if op.Name != OpInlineImage {
		return nil, fmt.Errorf("expected %s operator, got %s", OpInlineImage, op.Name)
	}
	if len(op.Args) < 2 {
		return nil, fmt.Errorf("inline image: expected 2 arguments, got %d", len(op.Args))
	}

	dict, ok := op.Args[0].(pdf.Dict)
	if !ok {
		return nil, fmt.Errorf("inline image: expected Dict, got %T", op.Args[0])
	}
	rawData, ok := op.Args[1].(pdf.String)
	if !ok {
		return nil, fmt.Errorf("inline image: expected String, got %T", op.Args[1])
	}

	if err := ValidateInlineImageFilter(dict); err != nil {
		return nil, &pdf.MalformedFileError{Err: err}
	}
	// the values of an inline image are direct objects
	c := pdf.NewCursor(nil)
	full := imagedict.ExpandInline(dict)
	filters, err := c.Filters(full)
	if err != nil {
		return nil, err
	}
	sizeLimit, err := inlineImageSizeLimit(c, full, filters, res)
	if err != nil {
		return nil, err
	}

	data := []byte(rawData)
	if len(filters) == 0 {
		return data, nil
	}

	// per-decode working-memory budget, sized to the raw inline data
	budget := membudget.New(limits.StreamBudget(int64(len(data))))

	var r io.Reader = bytes.NewReader(data)
	var closers []io.Closer
	for _, f := range filters {
		rc, err := f.Decode(pdf.V2_0, r, budget)
		if err != nil {
			return nil, err
		}
		closers = append(closers, rc)
		r = rc
	}

	result, err := io.ReadAll(io.LimitReader(r, sizeLimit+1))
	for _, closer := range slices.Backward(closers) {
		if cerr := closer.Close(); err == nil {
			err = cerr
		}
	}
	if err != nil {
		return nil, fmt.Errorf("inline image: reading decompressed data: %w", err)
	}
	if int64(len(result)) > sizeLimit {
		return nil, &pdf.MalformedFileError{Err: errors.New("inline image data exceeds size limit")}
	}
	return result, nil
}

// inlineImageSizeLimit checks the entries of an inline image which its image
// data depends on, and returns the number of bytes the decoded data may have.
// full holds the entries of the image under their full names.
func inlineImageSizeLimit(c pdf.Cursor, full pdf.Dict, filters []pdf.Filter, res *Resources) (int64, error) {
	width, height, err := imagedict.ReadDimensions(c, full)
	if err != nil {
		return 0, err
	}
	if IsInlineImageMask(full) {
		return limits.ImageDataLimit(width, height, 1, 1), nil
	}

	cs := InlineImageColorSpace(full, res)
	if cs == nil {
		return 0, pdf.Error("missing or invalid inline image ColorSpace")
	}
	if cs.Family() == color.FamilyPattern {
		return 0, pdf.Error("Pattern colour space not permitted for images")
	}
	bpc, err := imagedict.ReadBitsPerComponent(c, full, filters)
	if err != nil {
		return 0, err
	}
	channels := cs.Channels()
	if err := imagedict.CheckDataSize(width, height, channels, bpc); err != nil {
		return 0, err
	}
	return limits.ImageDataLimit(width, height, channels, bpc), nil
}

// InlineImageColorSpace resolves the ColorSpace of an inline image dict.
// It handles the three forms allowed by PDF 2.0 §8.9.7: device colour space
// names (with abbreviations), the limited Indexed array form whose base is
// a device space, and resource references resolved through res.ColorSpace.
// It returns nil when CS is missing, malformed, or refers to a name that
// is not present in res.
func InlineImageColorSpace(dict pdf.Dict, res *Resources) color.Space {
	switch v := imagedict.InlineValue(dict, "ColorSpace").(type) {
	case nil:
		return nil
	case pdf.Name:
		if cs := color.ParseInlineDeviceName(v); cs != nil {
			return cs
		}
		if res != nil {
			return res.ColorSpace[v]
		}
		return nil
	case pdf.Array:
		return color.ParseInlineIndexed(v)
	}
	return nil
}

// IsInlineImageMask reports whether the parameter dictionary of an inline
// image describes an image mask.
func IsInlineImageMask(dict pdf.Dict) bool {
	b, ok := imagedict.InlineValue(dict, "ImageMask").(pdf.Boolean)
	return ok && bool(b)
}
