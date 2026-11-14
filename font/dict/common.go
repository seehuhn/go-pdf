// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package dict

import (
	"math"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/encoding"
	"seehuhn.de/go/pdf/internal/stdmtx"
)

// moreThanTen returns true if the flattened array has more than 10 elements.
// This is used to determine whether to inline arrays or use indirect objects.
func moreThanTen(a pdf.Array) bool {
	count := 0
	for _, obj := range a {
		if a, ok := obj.(pdf.Array); ok {
			count += len(a)
		} else {
			count++
		}
		if count > 10 {
			return true
		}
	}
	return false
}

// widthsAreCompatible returns true, if the glyph widths ww are compatible with
// the standard font metrics. The enc parameter provides the encoding used
// to map character codes to glyph names.  Codes which enc leaves to the
// built-in encoding are resolved through the standard font's built-in
// encoding.
func widthsAreCompatible(ww []float64, enc encoding.Simple, info *stdmtx.FontData) bool {
	for code := range 256 {
		glyphName := enc(byte(code))
		if glyphName == "" {
			continue
		}
		if glyphName == encoding.UseBuiltin {
			glyphName = info.Encoding[code]
		}
		if math.Abs(ww[code]-info.Width[glyphName]) > 0.5 {
			return false
		}
	}
	return true
}

// fontDescriptorIsCompatible checks if a font descriptor is compatible with
// standard font metrics. This is used to determine if a standard font can
// be used instead of embedding a custom font.
func fontDescriptorIsCompatible(fd *font.Descriptor, stdInfo *stdmtx.FontData) bool {
	if fd.FontFamily != "" && fd.FontFamily != stdInfo.FontFamily {
		return false
	}
	if fd.FontWeight != 0 && fd.FontWeight != stdInfo.FontWeight {
		return false
	}

	if fd.IsFixedPitch != stdInfo.IsFixedPitch {
		return false
	}
	if fd.IsSerif != stdInfo.IsSerif {
		return false
	}
	if fd.IsSymbolic != stdInfo.IsSymbolic {
		return false
	}
	if fd.IsItalic != (stdInfo.ItalicAngle != 0) {
		return false
	}
	if fd.IsScript || fd.IsAllCap || fd.IsSmallCap || fd.ForceBold {
		return false
	}

	if math.Abs(fd.ItalicAngle-stdInfo.ItalicAngle) > 0.1 {
		return false
	}
	if fd.Ascent != 0 && math.Abs(fd.Ascent-stdInfo.Ascent) > 0.5 {
		return false
	}
	if fd.Descent != 0 && math.Abs(fd.Descent-stdInfo.Descent) > 0.5 {
		return false
	}
	if fd.CapHeight != 0 && math.Abs(fd.CapHeight-stdInfo.CapHeight) > 0.5 {
		return false
	}
	if fd.XHeight != 0 && math.Abs(fd.XHeight-stdInfo.XHeight) > 0.5 {
		return false
	}
	if fd.StemV != 0 && math.Abs(fd.StemV-stdInfo.StemV) > 0.5 {
		return false
	}
	if fd.StemH != 0 && math.Abs(fd.StemH-stdInfo.StemH) > 0.5 {
		return false
	}

	return true
}

// codeKey identifies a character code together with its length in bytes.
// The value of a [charcode.Code] alone does not determine the bytes: an
// incomplete code at the end of a string can have the same value as a
// longer code whose last bytes are zero.
type codeKey struct {
	code charcode.Code
	n    int
}
