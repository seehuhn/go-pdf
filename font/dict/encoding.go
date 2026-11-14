// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package dict

import (
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/encoding"
	"seehuhn.de/go/postscript/type1/names"
)

// makeCodec creates a character codec from CMap and ToUnicode information.
// It uses the list of code space ranges from both sources, including those a
// CMap inherits through usecmap.
//
// If the list is invalid, the longest initial part of the list which forms a
// codec is used instead.  If no such part exists, or if the lookup tree for
// the full list would be too large, one-byte codes are used.
func makeCodec(cm *cmap.File, toUnicode *cmap.ToUnicodeFile) *charcode.Codec {
	var csr charcode.CodeSpaceRange
	for _, f := range cm.Chain() {
		for _, r := range f.CodeSpaceRange {
			if r.IsValid() {
				csr = append(csr, r)
			}
		}
	}
	if toUnicode != nil {
		for _, r := range toUnicode.CodeSpaceRange {
			if r.IsValid() {
				csr = append(csr, r)
			}
		}
	}

	if len(csr) == 0 {
		return charcode.SimpleCodec
	}
	codec, err := charcode.NewCodec(csr)
	switch err {
	case nil:
		return codec
	case charcode.ErrTooComplex:
		return charcode.SimpleCodec
	}

	// A list of ranges is invalid only if one range is a prefix of a code in
	// another, and adding ranges cannot remove such a conflict.  The valid
	// prefixes of the list therefore form an initial segment, and a binary
	// search finds the longest one.  A prefix with too large a lookup tree
	// counts as invalid, which can make the search stop short of the longest
	// working prefix, but the result stays deterministic.
	var best *charcode.Codec
	// invariant: csr[:lo] forms a codec or lo is 0, and csr[:hi+1] does
	// not or hi is len(csr)
	lo, hi := 0, len(csr)-1
	for lo < hi {
		mid := hi - (hi-lo)/2 // in (lo, hi]
		codec, err := charcode.NewCodec(csr[:mid])
		if err == nil {
			best, lo = codec, mid
		} else {
			hi = mid - 1
		}
	}
	if best != nil {
		return best
	}

	// As a fallback, use one-byte codes.
	return charcode.SimpleCodec
}

// SimpleTextMap creates a mapping from character codes to Unicode text strings
// for simple (non-composite) fonts. It combines information from the font's
// encoding and optional ToUnicode CMap.
func SimpleTextMap(postScriptName string, encoding encoding.Simple, toUnicode *cmap.ToUnicodeFile) map[byte]string {
	textMap := make(map[byte]string)
	for code := range 256 {
		glyphName := encoding(byte(code))
		text := names.ToUnicode(glyphName, postScriptName)
		if text != "" {
			textMap[byte(code)] = text
		}
	}
	if toUnicode != nil {
		codec, _ := charcode.NewCodec(charcode.Simple)
		for code, text := range toUnicode.All(codec) {
			if text != "" {
				textMap[byte(code)] = text
			}
		}
	}
	return textMap
}
