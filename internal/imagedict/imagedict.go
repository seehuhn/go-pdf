// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

// Package imagedict reads the entries which image XObjects and inline images
// have in common, so that both follow the same rules.  For an inline image,
// the dictionary is first brought to full key names with [ExpandInline].
package imagedict

import (
	"fmt"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/internal/limits"
)

// PDF 2.0 sections: 8.9.5 8.9.7

// ReadDimensions reads the Width and Height entries of an image or image mask
// and checks them against the size limits.
func ReadDimensions(c pdf.Cursor, dict pdf.Dict) (width, height int, err error) {
	w, err := c.Integer(dict["Width"])
	if err != nil {
		return 0, 0, fmt.Errorf("missing or invalid Width: %w", err)
	}
	if w <= 0 {
		return 0, 0, pdf.Errorf("invalid image width %d", w)
	}
	if w > limits.MaxImageWidth {
		return 0, 0, pdf.Errorf("image width %d exceeds limit", w)
	}

	h, err := c.Integer(dict["Height"])
	if err != nil {
		return 0, 0, fmt.Errorf("missing or invalid Height: %w", err)
	}
	if h <= 0 {
		return 0, 0, pdf.Errorf("invalid image height %d", h)
	}
	if h > limits.MaxImageHeight {
		return 0, 0, pdf.Errorf("image height %d exceeds limit", h)
	}
	if limits.ImagePixelsExceedLimit(int(w), int(h)) {
		return 0, 0, pdf.Error("image pixel count exceeds limit")
	}
	return int(w), int(h), nil
}

// ReadBitsPerComponent returns the sample bit depth of a sampled image which
// does not use the JPXDecode filter.  Where the filter fixes the sample size,
// that size wins over the BitsPerComponent entry.  Otherwise the entry is
// required.
func ReadBitsPerComponent(c pdf.Cursor, dict pdf.Dict, filters []pdf.Filter) (int, error) {
	bpc, err := pdf.Optional(c.Integer(dict["BitsPerComponent"]))
	if err != nil {
		return 0, err
	}
	bits, ok := SampleBits(filters, bpc)
	if !ok {
		return 0, pdf.Error("missing or invalid BitsPerComponent")
	}
	return bits, nil
}

// SampleBits returns the sample bit depth of a sampled image which does not
// use the JPXDecode filter, given its filter chain and the value of its
// BitsPerComponent entry (0 if absent).  Where the filter fixes the sample
// size, that size wins over the entry.  Otherwise the entry must be one of the
// allowed bit depths; if it is not, ok is false.
func SampleBits(filters []pdf.Filter, bpc pdf.Integer) (bits int, ok bool) {
	if bits, ok := graphics.FilterSampleBits(filters); ok {
		return bits, true
	}
	switch bpc {
	case 1, 2, 4, 8, 16:
		return int(bpc), true
	}
	return 0, false
}

// CheckDataSize refines the pixel-count check of [ReadDimensions] for a
// sampled image whose number of colour components and bit depth are known.
// Filters which allocate their output up front bypass the limit on the
// decoded stream length, so the byte count implied by the image geometry is
// checked here.
func CheckDataSize(width, height, channels, bpc int) error {
	if limits.ImageBytesExceedLimit(width, height, channels, bpc) {
		return pdf.Error("image data exceeds size limit")
	}
	// the encoded-bytes cap admits up to 32× expansion into the per-channel
	// float32 buffer at bpc=1; cap the decoded form separately to bound
	// decoder memory use
	if limits.ImageDecodedExceedsLimit(width, height, channels) {
		return pdf.Error("image decoded data exceeds size limit")
	}
	return nil
}

// inlineAbbrev maps the full names of the inline image entries to their
// abbreviated keys (PDF 2.0, Table 91).
var inlineAbbrev = map[pdf.Name]pdf.Name{
	"BitsPerComponent": "BPC",
	"ColorSpace":       "CS",
	"Decode":           "D",
	"DecodeParms":      "DP",
	"Filter":           "F",
	"Height":           "H",
	"Interpolate":      "I",
	"ImageMask":        "IM",
	"Length":           "L",
	"Width":            "W",
}

// InlineKey returns the key under which an inline image dictionary holds the
// entry with the given full name.  Where both the abbreviated and the full
// key are present, the abbreviated key takes precedence.  If the entry is
// absent, ok is false.
func InlineKey(dict pdf.Dict, full pdf.Name) (key pdf.Name, ok bool) {
	if abbrev, hasAbbrev := inlineAbbrev[full]; hasAbbrev {
		if _, ok := dict[abbrev]; ok {
			return abbrev, true
		}
	}
	if _, ok := dict[full]; ok {
		return full, true
	}
	return "", false
}

// InlineValue returns the entry of an inline image dictionary with the given
// full name, or nil if the entry is absent.  The key is chosen as for
// [InlineKey], so the function works for a dictionary as found in a content
// stream and for the result of [ExpandInline] alike.
func InlineValue(dict pdf.Dict, full pdf.Name) pdf.Object {
	key, ok := InlineKey(dict, full)
	if !ok {
		return nil
	}
	return dict[key]
}

// ExpandInline returns the entries of an inline image dictionary under their
// full names, with the keys chosen as for [InlineKey].  Entries other than
// those of Table 91 are dropped, since an inline image ignores them.
// Abbreviated values, such as filter and colour space names, are kept as they
// are.
func ExpandInline(dict pdf.Dict) pdf.Dict {
	res := pdf.Dict{}
	for full := range inlineAbbrev {
		if key, ok := InlineKey(dict, full); ok {
			res[full] = dict[key]
		}
	}
	if v, ok := dict["Intent"]; ok {
		res["Intent"] = v
	}
	return res
}
